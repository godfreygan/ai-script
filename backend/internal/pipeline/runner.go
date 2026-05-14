package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/ws"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

// DAG 定义
type DAG struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	ModelID int64          `json:"model_id"`
	Params  map[string]any `json:"params"`
}

type Edge struct {
	From    string            `json:"from"`
	To      string            `json:"to"`
	Mapping map[string]string `json:"mapping"`
}

// RunPayload pipeline.run 任务负载
type RunPayload struct {
	RunID      int64          `json:"run_id"` // P0 #4: 预先 service 层创建,worker 不再造新行
	PipelineID int64          `json:"pipeline_id"`
	Input      map[string]any `json:"input"`
	Overrides  map[string]any `json:"overrides"`
}

// NodeContext 给 NodeHandler 的执行上下文
type NodeContext struct {
	Ctx       context.Context
	RunID     int64
	NodeID    string
	NodeType  string
	ModelID   int64
	Params    map[string]any
	Input     map[string]any
	Logger    *zap.Logger
	Publisher func(percent float64, msg string) // 上抛节点级进度
}

// NodeHandler 在 DAG runner 中实际执行单个节点
type NodeHandler func(nc *NodeContext) (output map[string]any, err error)

// NodeHandlerRegistry 节点处理器注册中心(in-process)
type NodeHandlerRegistry struct {
	mu sync.RWMutex
	h  map[string]NodeHandler
}

func NewNodeHandlerRegistry() *NodeHandlerRegistry {
	return &NodeHandlerRegistry{h: make(map[string]NodeHandler)}
}

func (r *NodeHandlerRegistry) Register(nodeType string, h NodeHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.h[nodeType] = h
}

func (r *NodeHandlerRegistry) Get(nodeType string) (NodeHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.h[nodeType]
	return h, ok
}

// RunnerOption 是 Runner 的可选配置函数
type RunnerOption func(*Runner)

// WithMaxConcurrency 设置同层最大并发数（默认 10）
func WithMaxConcurrency(n int) RunnerOption {
	return func(r *Runner) {
		if n > 0 {
			r.maxConcurrency = n
		}
	}
}

// Runner 真正的 DAG 执行器
type Runner struct {
	nodeReg        *NodeHandlerRegistry
	repos          *repo.Repositories
	hub            *ws.Hub
	log            *zap.Logger
	maxConcurrency int
	maxAttempts    int // 默认 3,可配置
}

func NewRunner(reg *NodeHandlerRegistry, repos *repo.Repositories, hub *ws.Hub, log *zap.Logger, opts ...RunnerOption) *Runner {
	r := &Runner{
		nodeReg:        reg,
		repos:          repos,
		hub:            hub,
		log:            log,
		maxConcurrency: 10,
		maxAttempts:    3,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// SetMaxAttempts 设置节点最大重试次数(含首次执行),默认 3。
func (r *Runner) SetMaxAttempts(n int) {
	if n < 1 {
		n = 1
	}
	r.maxAttempts = n
}

// publishToRun 发布到 pipeline:<runID> 主题
func (r *Runner) publishToRun(runID int64, evType string, pct float64, msg string) {
	if r.hub == nil {
		return
	}
	r.hub.Publish(fmt.Sprintf("pipeline:%d", runID),
		ws.Event{Type: evType, Percent: pct, Message: msg})
}

// publishStep 发布单步进度
func (r *Runner) publishStep(runID int64, nodeID string, evType string, pct float64, msg string) {
	if r.hub == nil {
		return
	}
	r.hub.Publish(fmt.Sprintf("pipeline:%d", runID),
		ws.Event{Type: evType, Percent: pct, Message: fmt.Sprintf("[%s] %s", nodeID, msg)})
}

// Execute 实际编排:
//   - 拓扑排序(入度=0 的节点先执行)
//   - 同层节点并行
//   - 节点输出按 edge.mapping 映射到下游 input
//   - 每个节点写一行 step_runs(可重试)
//   - 任一节点失败 → 整个 run 标记为 failed
func (r *Runner) Execute(ctx context.Context, dagJSON []byte, input map[string]any, runID int64) (map[string]any, error) {
	var dag DAG
	if err := json.Unmarshal(dagJSON, &dag); err != nil {
		return nil, fmt.Errorf("decode dag: %w", err)
	}
	if len(dag.Nodes) == 0 {
		return nil, errors.New("empty dag")
	}

	// 1) 建立索引 / 入度
	idx := make(map[string]*Node, len(dag.Nodes))
	inDeg := make(map[string]int, len(dag.Nodes))
	for i := range dag.Nodes {
		n := &dag.Nodes[i]
		idx[n.ID] = n
		inDeg[n.ID] = 0
	}
	outAdj := make(map[string][]Edge)
	for _, e := range dag.Edges {
		if _, ok := idx[e.From]; !ok {
			return nil, fmt.Errorf("edge.from %q not in nodes", e.From)
		}
		if _, ok := idx[e.To]; !ok {
			return nil, fmt.Errorf("edge.to %q not in nodes", e.To)
		}
		outAdj[e.From] = append(outAdj[e.From], e)
		inDeg[e.To]++
	}

	// 2) Kahn 拓扑分层
	layers := [][]string{}
	current := []string{}
	for id, d := range inDeg {
		if d == 0 {
			current = append(current, id)
		}
	}
	if len(current) == 0 {
		return nil, errors.New("dag has cycle (no zero-indegree node)")
	}
	visited := 0
	for len(current) > 0 {
		layers = append(layers, current)
		next := []string{}
		for _, id := range current {
			visited++
			for _, e := range outAdj[id] {
				inDeg[e.To]--
				if inDeg[e.To] == 0 {
					next = append(next, e.To)
				}
			}
		}
		current = next
	}
	if visited != len(dag.Nodes) {
		return nil, errors.New("dag has cycle")
	}

	// 3) 节点输出表
	outputs := make(map[string]map[string]any, len(dag.Nodes))
	var outMu sync.Mutex

	totalNodes := len(dag.Nodes)
	finished := 0
	var finMu sync.Mutex

	r.publishToRun(runID, "progress", 0.01, fmt.Sprintf("start dag: %d 节点 / %d 层", totalNodes, len(layers)))

	// 4) 按层执行,同层并行（受 maxConcurrency 限制）
	for li, layer := range layers {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var wg sync.WaitGroup
		errCh := make(chan error, len(layer))
		sem := semaphore.NewWeighted(int64(r.maxConcurrency))
		for _, nodeID := range layer {
			nodeID := nodeID
			node := idx[nodeID]
			handler, ok := r.nodeReg.Get(node.Type)
			if !ok {
				return nil, fmt.Errorf("no handler registered for node type %q", node.Type)
			}

			// 计算本节点 input:从所有上游 edge.mapping[输出键]→输入键 映射
			nodeInput := make(map[string]any)
			for k, v := range input { // 全局 input 兜底
				nodeInput[k] = v
			}
			for upID, edges := range outAdj {
				for _, e := range edges {
					if e.To != nodeID {
						continue
					}
					outMu.Lock()
					up := outputs[upID]
					outMu.Unlock()
					if up == nil {
						continue
					}
					if len(e.Mapping) == 0 {
						// 默认整包 merge
						for k, v := range up {
							nodeInput[k] = v
						}
						continue
					}
					for srcKey, dstKey := range e.Mapping {
						if v, ok := up[srcKey]; ok {
							nodeInput[dstKey] = v
						}
					}
				}
			}

			// 创建 step_run 记录
			step := &model.StepRun{
				RunID:    runID,
				NodeID:   node.ID,
				NodeType: node.Type,
				ModelID:  node.ModelID,
				Status:   "running",
				Attempt:  1,
			}
			now := time.Now()
			step.StartedAt = &now
			inBytes, _ := json.Marshal(nodeInput)
			step.Input = model.JSON(inBytes)
			if err := r.repos.Pipeline.CreateStep(ctx, step); err != nil {
				r.log.Warn("create step run failed", zap.Error(err))
			}

			if err := sem.Acquire(ctx, 1); err != nil {
				return nil, err
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer sem.Release(1)
				defer func() {
					if rec := recover(); rec != nil {
						r.log.Error("node handler panic",
							zap.Int64("run_id", runID),
							zap.String("node_id", nodeID),
							zap.Any("panic", rec),
						)
						step.Status = "failed"
						step.ErrorMsg = fmt.Sprintf("panic: %v", rec)
						_ = r.repos.Pipeline.UpdateStep(ctx, step)
						r.publishStep(runID, nodeID, "error", 0, fmt.Sprintf("panic: %v", rec))
						errCh <- fmt.Errorf("node %s panic: %v", nodeID, rec)
					}
				}()
				var out map[string]any
				var herr error
				for attempt := 1; attempt <= r.maxAttempts; attempt++ {
					if attempt > 1 {
						time.Sleep(time.Duration(attempt-1) * 2 * time.Second) // 线性退避 2s,4s,...
						step.Attempt = attempt
						step.Status = "running"
						_ = r.repos.Pipeline.UpdateStep(ctx, step)
					}
					r.publishStep(runID, nodeID, "progress", 0, fmt.Sprintf("start (attempt %d/%d)", attempt, r.maxAttempts))
					nc := &NodeContext{
						Ctx:      ctx,
						RunID:    runID,
						NodeID:   node.ID,
						NodeType: node.Type,
						ModelID:  node.ModelID,
						Params:   node.Params,
						Input:    nodeInput,
						Logger:   r.log,
						Publisher: func(pct float64, msg string) {
							r.publishStep(runID, nodeID, "progress", pct, msg)
						},
					}
					out, herr = handler(nc)
					if herr == nil {
						break
					}
					r.log.Warn("node handler error, will retry", zap.String("node_id", nodeID), zap.Int("attempt", attempt), zap.Error(herr))
				}
				if out == nil {
					out = map[string]any{}
				}
				outMu.Lock()
				outputs[nodeID] = out
				outMu.Unlock()

				end := time.Now()
				step.EndedAt = &end
				if outBytes, e := json.Marshal(out); e == nil {
					step.Output = model.JSON(outBytes)
				}
				if herr != nil {
					step.Status = "failed"
					step.ErrorMsg = truncate(herr.Error(), 1000)
					_ = r.repos.Pipeline.UpdateStep(ctx, step)
					r.publishStep(runID, nodeID, "error", 0, herr.Error())
					errCh <- fmt.Errorf("node %s: %w", nodeID, herr)
					return
				}
				step.Status = "succeeded"
				_ = r.repos.Pipeline.UpdateStep(ctx, step)

				finMu.Lock()
				finished++
				pct := float64(finished) / float64(totalNodes)
				finMu.Unlock()
				r.publishStep(runID, nodeID, "progress", 1, "done")
				r.publishToRun(runID, "progress", pct, fmt.Sprintf("layer %d/%d done %s", li+1, len(layers), nodeID))
			}()
		}
		wg.Wait()
		close(errCh)
		var firstErr error
		for e := range errCh {
			if firstErr == nil {
				firstErr = e
			}
		}
		if firstErr != nil {
			r.publishToRun(runID, "error", 0, firstErr.Error())
			return nil, firstErr
		}
	}

	// 5) 把所有节点输出聚合返回(终端节点的 output 也在里面)
	final := map[string]any{
		"nodes": outputs,
	}
	r.publishToRun(runID, "done", 1.0, "pipeline done")
	return final, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// NoopHandler 占位 asynq 处理器 — 旧版本残留,保留以避免 worker/main.go 编译失败
//
// 注意:它仅作为 asynq.Task 占位,不再用于 DAG 节点。DAG 节点使用 NodeHandlerRegistry。
func NoopHandler(name string) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p map[string]any
		_ = json.Unmarshal(t.Payload(), &p)
		_ = errors.New("not implemented: " + name)
		return nil
	}
}
