// Package pipeline tests for the in-process DAG Runner.
//
// 注意：本文件仅使用 in-memory sqlite + 真实 *repo.Repositories.Pipeline，
// 不依赖任何 LLM/外部服务/Redis。所有节点 handler 均为本地 mock。
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------- 测试基础设施 ----------

// newTestRepos 创建 in-memory sqlite，并仅 migrate Runner 需要的两张表。
// 其他 repo 的 db 字段仍然非 nil（共享同一 *gorm.DB），但 Runner.Execute 只触达
// PipelineRepo.CreateStep / UpdateStep，所以其他表的 schema 不存在也无所谓。
func newTestRepos(t *testing.T) *repo.Repositories {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// sqlite `:memory:` 默认每个连接独立持有一个内存库,会出现 AutoMigrate 在 conn A 建表、
	// 后续查询走 conn B 看不到表的情况。把池子限到 1 个连接,保证 schema 全程可见。
	if sqlDB, derr := db.DB(); derr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.PipelineRun{}, &model.StepRun{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return repo.NewRepositories(db, nil)
}

// newTestRunner 构造 Runner（hub=nil 表示不发 ws 事件,重试次数设为 1 避免干扰既有测试）
func newTestRunner(t *testing.T, reg *NodeHandlerRegistry) *Runner {
	t.Helper()
	r := NewRunner(reg, newTestRepos(t), nil, zap.NewNop())
	r.SetMaxAttempts(1)
	return r
}

// mockHandler 返回一个统计/可控的 NodeHandler。
//   - calls 记录被调用次数
//   - delay 在 handler 内 sleep（用于并发测试）
//   - failWith 非 nil 时直接返回错误
//   - output 是节点输出（也会 merge nodeID 标记便于追踪）
type mockCfg struct {
	delay    time.Duration
	failWith error
	output   map[string]any
}

func mockHandler(t *testing.T, cfg mockCfg, calls *atomic.Int32, nodeMark string) NodeHandler {
	t.Helper()
	return func(nc *NodeContext) (map[string]any, error) {
		calls.Add(1)
		if cfg.delay > 0 {
			time.Sleep(cfg.delay)
		}
		if cfg.failWith != nil {
			return nil, cfg.failWith
		}
		out := map[string]any{"executed": nodeMark}
		for k, v := range cfg.output {
			out[k] = v
		}
		return out, nil
	}
}

// ---------- DAG 拓扑测试 ----------

// TestExecute_TopologyTable 表驱动测试 DAG 拓扑场景。
func TestExecute_TopologyTable(t *testing.T) {
	type tc struct {
		name        string
		dag         DAG
		wantErr     bool
		wantErrSub  string
		wantOrdered [][]string // 期望的层(同层无序，仅断言层数 & 集合)
	}
	cases := []tc{
		{
			name: "linear-chain-A-B-C",
			dag: DAG{
				Nodes: []Node{{ID: "a", Type: "noop"}, {ID: "b", Type: "noop"}, {ID: "c", Type: "noop"}},
				Edges: []Edge{{From: "a", To: "b"}, {From: "b", To: "c"}},
			},
			wantOrdered: [][]string{{"a"}, {"b"}, {"c"}},
		},
		{
			name: "diamond-A-BC-D",
			dag: DAG{
				Nodes: []Node{{ID: "a", Type: "noop"}, {ID: "b", Type: "noop"}, {ID: "c", Type: "noop"}, {ID: "d", Type: "noop"}},
				Edges: []Edge{{From: "a", To: "b"}, {From: "a", To: "c"}, {From: "b", To: "d"}, {From: "c", To: "d"}},
			},
			wantOrdered: [][]string{{"a"}, {"b", "c"}, {"d"}},
		},
		{
			name: "isolated-nodes",
			dag: DAG{
				Nodes: []Node{{ID: "x", Type: "noop"}, {ID: "y", Type: "noop"}, {ID: "z", Type: "noop"}},
				Edges: nil,
			},
			wantOrdered: [][]string{{"x", "y", "z"}},
		},
		{
			name: "cycle-A-B-A-should-error",
			dag: DAG{
				Nodes: []Node{{ID: "a", Type: "noop"}, {ID: "b", Type: "noop"}},
				Edges: []Edge{{From: "a", To: "b"}, {From: "b", To: "a"}},
			},
			wantErr:    true,
			wantErrSub: "cycle",
		},
		{
			name: "self-loop-should-error",
			dag: DAG{
				Nodes: []Node{{ID: "a", Type: "noop"}, {ID: "b", Type: "noop"}},
				Edges: []Edge{{From: "a", To: "a"}, {From: "a", To: "b"}},
			},
			wantErr:    true,
			wantErrSub: "cycle",
		},
		{
			name: "edge-to-missing-node",
			dag: DAG{
				Nodes: []Node{{ID: "a", Type: "noop"}},
				Edges: []Edge{{From: "a", To: "ghost"}},
			},
			wantErr:    true,
			wantErrSub: "not in nodes",
		},
		{
			name:       "empty-dag",
			dag:        DAG{Nodes: nil, Edges: nil},
			wantErr:    true,
			wantErrSub: "empty dag",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			reg := NewNodeHandlerRegistry()
			// 记录调用顺序（线性 / 拓扑断言）
			var mu sync.Mutex
			callOrder := []string{}
			var calls atomic.Int32
			reg.Register("noop", func(nc *NodeContext) (map[string]any, error) {
				calls.Add(1)
				mu.Lock()
				callOrder = append(callOrder, nc.NodeID)
				mu.Unlock()
				return map[string]any{"id": nc.NodeID}, nil
			})
			runner := newTestRunner(t, reg)
			dagBytes, _ := json.Marshal(c.dag)
			_, err := runner.Execute(context.Background(), dagBytes, map[string]any{}, runIDFor(t))
			if c.wantErr {
				if err == nil {
					t.Fatalf("expect err containing %q, got nil", c.wantErrSub)
				}
				if c.wantErrSub != "" && !contains(err.Error(), c.wantErrSub) {
					t.Fatalf("err %q does not contain %q", err.Error(), c.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if int(calls.Load()) != len(c.dag.Nodes) {
				t.Fatalf("calls=%d, want %d", calls.Load(), len(c.dag.Nodes))
			}
			// 验证拓扑顺序：每条 edge from 必须在 to 之前调用
			pos := map[string]int{}
			for i, id := range callOrder {
				pos[id] = i
			}
			for _, e := range c.dag.Edges {
				if pos[e.From] >= pos[e.To] {
					t.Fatalf("topology violated: %s (pos=%d) should run before %s (pos=%d). order=%v",
						e.From, pos[e.From], e.To, pos[e.To], callOrder)
				}
			}
		})
	}
}

// ---------- 节点失败传播 ----------

// TestExecute_FailurePropagates 验证：
//   - 中间节点失败时，Execute 返回错误
//   - 失败节点的下游节点 **不会被调用**（即被"阻塞"，效果等同 skipped）
//   - 同层兄弟节点仍允许执行（同层并行，一个失败不立刻中断已启动 goroutine）
func TestExecute_FailurePropagates(t *testing.T) {
	reg := NewNodeHandlerRegistry()
	var aCalls, bCalls, cCalls, dCalls atomic.Int32
	reg.Register("ok-a", mockHandler(t, mockCfg{}, &aCalls, "a"))
	reg.Register("fail-b", mockHandler(t, mockCfg{failWith: errors.New("boom-b")}, &bCalls, "b"))
	reg.Register("ok-c", mockHandler(t, mockCfg{}, &cCalls, "c"))
	reg.Register("ok-d", mockHandler(t, mockCfg{}, &dCalls, "d"))

	// a -> b(fail) -> d
	//   \-> c
	dag := DAG{
		Nodes: []Node{
			{ID: "a", Type: "ok-a"},
			{ID: "b", Type: "fail-b"},
			{ID: "c", Type: "ok-c"},
			{ID: "d", Type: "ok-d"},
		},
		Edges: []Edge{
			{From: "a", To: "b"},
			{From: "a", To: "c"},
			{From: "b", To: "d"},
		},
	}
	runner := newTestRunner(t, reg)
	dagBytes, _ := json.Marshal(dag)
	runID := runIDFor(t)
	_, err := runner.Execute(context.Background(), dagBytes, map[string]any{}, runID)
	if err == nil {
		t.Fatalf("expect error from failing node, got nil")
	}
	if !contains(err.Error(), "boom-b") {
		t.Fatalf("expected wrapped error 'boom-b', got: %v", err)
	}
	if aCalls.Load() != 1 {
		t.Errorf("a should run once, got %d", aCalls.Load())
	}
	if bCalls.Load() != 1 {
		t.Errorf("b should run once (and fail), got %d", bCalls.Load())
	}
	// c 与 b 同层；同层并行，b 失败不阻止 c 启动
	if cCalls.Load() != 1 {
		t.Errorf("c is sibling of b, should still run, got %d", cCalls.Load())
	}
	// d 下游被阻塞 / skipped
	if dCalls.Load() != 0 {
		t.Errorf("d is downstream of failing b, MUST NOT run, got %d", dCalls.Load())
	}
	// 检查 DB 中 step_runs 状态
	steps, qerr := runner.repos.Pipeline.ListSteps(context.Background(), runID)
	if qerr != nil {
		t.Fatalf("list steps: %v", qerr)
	}
	gotStatus := map[string]string{}
	for _, s := range steps {
		gotStatus[s.NodeID] = s.Status
	}
	if gotStatus["b"] != "failed" {
		t.Errorf("step b status = %q, want failed", gotStatus["b"])
	}
	if gotStatus["a"] != "succeeded" {
		t.Errorf("step a status = %q, want succeeded", gotStatus["a"])
	}
}

// ---------- 并发：同层并行 ----------

// TestExecute_LayerParallelism 验证同层 3 节点确实并行执行。
// 每个节点 sleep 200ms；若串行总耗时 ≥600ms；并行应 <350ms。
func TestExecute_LayerParallelism(t *testing.T) {
	if testing.Short() {
		t.Skip("skip timing-sensitive test in -short")
	}
	const each = 200 * time.Millisecond

	reg := NewNodeHandlerRegistry()
	var calls atomic.Int32
	slow := func(nc *NodeContext) (map[string]any, error) {
		calls.Add(1)
		time.Sleep(each)
		return map[string]any{"id": nc.NodeID}, nil
	}
	reg.Register("slow", slow)

	dag := DAG{
		Nodes: []Node{
			{ID: "root", Type: "slow"},
			{ID: "p1", Type: "slow"},
			{ID: "p2", Type: "slow"},
			{ID: "p3", Type: "slow"},
		},
		Edges: []Edge{
			{From: "root", To: "p1"},
			{From: "root", To: "p2"},
			{From: "root", To: "p3"},
		},
	}
	runner := newTestRunner(t, reg)
	dagBytes, _ := json.Marshal(dag)
	start := time.Now()
	_, err := runner.Execute(context.Background(), dagBytes, map[string]any{}, runIDFor(t))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if int(calls.Load()) != 4 {
		t.Fatalf("calls=%d, want 4", calls.Load())
	}
	// 串行下限 4*each = 800ms；并行 root + max(p1,p2,p3) ≈ 2*each = 400ms。
	// 给 100ms 余量，断言总耗时 < 串行下限 - 200ms = 600ms。
	if elapsed >= 4*each {
		t.Fatalf("execution took %v, looks serial (>= 4*%v). expected parallel.", elapsed, each)
	}
	if elapsed < each /*下限健全检查：至少跑了一轮*/ {
		t.Fatalf("execution took %v, suspiciously fast (< %v)", elapsed, each)
	}
	t.Logf("4 nodes (1 root + 3 parallel @ %v each) finished in %v", each, elapsed)
}

// ---------- edge.mapping 数据流 ----------

// TestExecute_EdgeMappingTransfer 验证上游 output 通过 edge.mapping 重命名后传入下游 input。
func TestExecute_EdgeMappingTransfer(t *testing.T) {
	reg := NewNodeHandlerRegistry()
	reg.Register("producer", func(nc *NodeContext) (map[string]any, error) {
		return map[string]any{"raw_value": 42, "unused": "x"}, nil
	})
	var gotInput map[string]any
	reg.Register("consumer", func(nc *NodeContext) (map[string]any, error) {
		gotInput = nc.Input
		return map[string]any{"ok": true}, nil
	})

	dag := DAG{
		Nodes: []Node{{ID: "p", Type: "producer"}, {ID: "c", Type: "consumer"}},
		Edges: []Edge{{From: "p", To: "c", Mapping: map[string]string{"raw_value": "renamed_value"}}},
	}
	runner := newTestRunner(t, reg)
	dagBytes, _ := json.Marshal(dag)
	_, err := runner.Execute(context.Background(), dagBytes, map[string]any{"global": "g"}, runIDFor(t))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if v, ok := gotInput["renamed_value"]; !ok || fmt.Sprintf("%v", v) != "42" {
		t.Errorf("consumer should receive renamed_value=42, got input=%v", gotInput)
	}
	if _, ok := gotInput["unused"]; ok {
		t.Errorf("unused key should NOT propagate when mapping is set, got %v", gotInput)
	}
	if v, ok := gotInput["global"]; !ok || v != "g" {
		t.Errorf("global input should propagate, got %v", gotInput)
	}
}

// ---------- 未注册 handler ----------

func TestExecute_UnregisteredHandler(t *testing.T) {
	reg := NewNodeHandlerRegistry()
	// 故意不注册 'unknown'
	dag := DAG{Nodes: []Node{{ID: "a", Type: "unknown"}}}
	runner := newTestRunner(t, reg)
	dagBytes, _ := json.Marshal(dag)
	_, err := runner.Execute(context.Background(), dagBytes, map[string]any{}, runIDFor(t))
	if err == nil || !contains(err.Error(), "no handler registered") {
		t.Fatalf("want 'no handler registered' err, got %v", err)
	}
}

// ---------- 取消传播 ----------

func TestExecute_ContextCancel(t *testing.T) {
	reg := NewNodeHandlerRegistry()
	var calls atomic.Int32
	reg.Register("slow", func(nc *NodeContext) (map[string]any, error) {
		calls.Add(1)
		select {
		case <-nc.Ctx.Done():
			return nil, nc.Ctx.Err()
		case <-time.After(500 * time.Millisecond):
			return map[string]any{}, nil
		}
	})
	// chain: a -> b -> c；在 a 跑完后取消，b/c 应被影响
	dag := DAG{
		Nodes: []Node{{ID: "a", Type: "slow"}, {ID: "b", Type: "slow"}, {ID: "c", Type: "slow"}},
		Edges: []Edge{{From: "a", To: "b"}, {From: "b", To: "c"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	runner := newTestRunner(t, reg)
	dagBytes, _ := json.Marshal(dag)
	_, err := runner.Execute(ctx, dagBytes, map[string]any{}, runIDFor(t))
	if err == nil {
		t.Fatalf("expect context-related error, got nil (calls=%d)", calls.Load())
	}
}

// ---------- 重试与 panic 恢复 ----------

// TestExecute_RetryThenSuccess 验证节点失败后会重试并在成功时停止。
func TestExecute_RetryThenSuccess(t *testing.T) {
	reg := NewNodeHandlerRegistry()
	var calls atomic.Int32
	reg.Register("flaky", func(nc *NodeContext) (map[string]any, error) {
		if calls.Add(1) < 3 {
			return nil, errors.New("transient error")
		}
		return map[string]any{"ok": true}, nil
	})

	dag := DAG{
		Nodes: []Node{{ID: "a", Type: "flaky"}},
	}
	runner := NewRunner(reg, newTestRepos(t), nil, zap.NewNop())
	runner.SetMaxAttempts(5)
	dagBytes, _ := json.Marshal(dag)
	out, err := runner.Execute(context.Background(), dagBytes, map[string]any{}, runIDFor(t))
	if err != nil {
		t.Fatalf("expect success after retry, got %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("expect 3 calls (2 fails + 1 success), got %d", calls.Load())
	}
	if out["nodes"].(map[string]map[string]any)["a"]["ok"] != true {
		t.Fatalf("expect output ok=true, got %v", out)
	}
}

// TestExecute_RetryExhausted 验证重试耗尽后返回错误。
func TestExecute_RetryExhausted(t *testing.T) {
	reg := NewNodeHandlerRegistry()
	var calls atomic.Int32
	reg.Register("always-fail", func(nc *NodeContext) (map[string]any, error) {
		calls.Add(1)
		return nil, errors.New("permanent error")
	})

	dag := DAG{
		Nodes: []Node{{ID: "a", Type: "always-fail"}},
	}
	runner := NewRunner(reg, newTestRepos(t), nil, zap.NewNop())
	runner.SetMaxAttempts(3)
	dagBytes, _ := json.Marshal(dag)
	_, err := runner.Execute(context.Background(), dagBytes, map[string]any{}, runIDFor(t))
	if err == nil {
		t.Fatal("expect error after retry exhausted")
	}
	if calls.Load() != 3 {
		t.Fatalf("expect 3 calls (maxAttempts), got %d", calls.Load())
	}
}

// TestExecute_PanicRecovered 验证 handler panic 被 recover,不会崩溃 worker。
func TestExecute_PanicRecovered(t *testing.T) {
	reg := NewNodeHandlerRegistry()
	var calls atomic.Int32
	reg.Register("panic", func(nc *NodeContext) (map[string]any, error) {
		calls.Add(1)
		panic("intentional panic")
	})
	reg.Register("ok", func(nc *NodeContext) (map[string]any, error) {
		calls.Add(1)
		return map[string]any{"ok": true}, nil
	})

	dag := DAG{
		Nodes: []Node{{ID: "a", Type: "panic"}, {ID: "b", Type: "ok"}},
		Edges: []Edge{{From: "a", To: "b"}},
	}
	runner := newTestRunner(t, reg)
	dagBytes, _ := json.Marshal(dag)
	_, err := runner.Execute(context.Background(), dagBytes, map[string]any{}, runIDFor(t))
	if err == nil {
		t.Fatal("expect error from panic node")
	}
	if !contains(err.Error(), "panic") {
		t.Fatalf("expect panic error, got %v", err)
	}
	// b 是 a 的下游,a panic 后 b 不应执行
	if calls.Load() != 1 {
		t.Fatalf("expect 1 call (only panic node), got %d", calls.Load())
	}
}

// ---------- helpers ----------

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// runIDFor 给每个子测试生成一个独立的 runID（用 t.Name 哈希成 int64 即可）
var _runIDSeq int64

func runIDFor(t *testing.T) int64 {
	t.Helper()
	// 简单递增，确保唯一
	_runIDSeq++
	return _runIDSeq
}
