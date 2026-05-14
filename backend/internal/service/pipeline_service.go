package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// 本文件聚焦 PipelineService 的 CRUD / 运行历史查询。
// 异步任务相关(Run / SetDeps / pipelineRegistry)定义保留在 generation.go,这里只补充元数据 API。

// CreatePipelineInput 新建流水线
type CreatePipelineInput struct {
	ProjectID   int64           `json:"project_id" binding:"required,gte=1"`
	Name        string          `json:"name" binding:"required,min=1,max=100"`
	Description string          `json:"description" binding:"omitempty,max=500"`
	DAG         json.RawMessage `json:"dag" binding:"required"`
	IsTemplate  int8            `json:"is_template" binding:"gte=0,lte=1"`
	Enabled     int8            `json:"enabled" binding:"gte=0,lte=1"`
}

// UpdatePipelineInput 更新流水线
type UpdatePipelineInput struct {
	Name        *string         `json:"name" binding:"omitempty,min=1,max=100"`
	Description *string         `json:"description" binding:"omitempty,max=500"`
	DAG         json.RawMessage `json:"dag"`
	IsTemplate  *int8           `json:"is_template" binding:"omitempty,gte=0,lte=1"`
	Enabled     *int8           `json:"enabled" binding:"omitempty,gte=0,lte=1"`
}

func (s *pipelineService) List(ctx context.Context, q *repo.ListPipelinesQuery) ([]model.Pipeline, int64, error) {
	return s.r.Pipeline.List(ctx, q)
}

func (s *pipelineService) Get(ctx context.Context, id int64) (*model.Pipeline, error) {
	p, err := s.r.Pipeline.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return p, nil
}

func (s *pipelineService) Create(ctx context.Context, in *CreatePipelineInput, uid int64) (*model.Pipeline, error) {
	if err := validateDAG(in.DAG); err != nil {
		return nil, errcode.ErrParam.Wrap(err)
	}
	enabled := in.Enabled
	if enabled == 0 {
		enabled = 1
	}
	p := &model.Pipeline{
		ProjectID:   in.ProjectID,
		Name:        in.Name,
		Description: in.Description,
		DAG:         model.JSON(in.DAG),
		IsTemplate:  in.IsTemplate,
		Enabled:     enabled,
		CreatedBy:   uid,
	}
	if err := s.r.Pipeline.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *pipelineService) Update(ctx context.Context, id int64, in *UpdatePipelineInput) (*model.Pipeline, error) {
	p, err := s.r.Pipeline.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	if in.Name != nil {
		p.Name = *in.Name
	}
	if in.Description != nil {
		p.Description = *in.Description
	}
	if len(in.DAG) > 0 {
		if err := validateDAG(in.DAG); err != nil {
			return nil, errcode.ErrParam.Wrap(err)
		}
		p.DAG = model.JSON(in.DAG)
	}
	if in.IsTemplate != nil {
		p.IsTemplate = *in.IsTemplate
	}
	if in.Enabled != nil {
		p.Enabled = *in.Enabled
	}
	if err := s.r.Pipeline.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *pipelineService) Delete(ctx context.Context, id int64) error {
	return s.r.Pipeline.Delete(ctx, id)
}

// ListRuns 某条流水线的历史运行记录
func (s *pipelineService) ListRuns(ctx context.Context, pipelineID int64, page, size int) ([]model.PipelineRun, int64, error) {
	return s.r.Pipeline.ListRuns(ctx, pipelineID, page, size)
}

// GetRun 单次运行详情
func (s *pipelineService) GetRun(ctx context.Context, runID int64) (*model.PipelineRun, error) {
	run, err := s.r.Pipeline.GetRun(ctx, runID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return run, nil
}

// ListSteps 单次运行的所有 step_runs
func (s *pipelineService) ListSteps(ctx context.Context, runID int64) ([]model.StepRun, error) {
	return s.r.Pipeline.ListSteps(ctx, runID)
}

// runPayload 与 pipeline.RunPayload 保持一致,避免循环 import
type runPayload struct {
	RunID      int64          `json:"run_id"`
	PipelineID int64          `json:"pipeline_id"`
	Input      map[string]any `json:"input"`
	Overrides  map[string]any `json:"overrides"`
}

// HandleRunTask 是 worker 端 pipeline.run 任务处理器。
//
// 修复:为下游 AI 调用/长耗时节点添加 context timeout(默认 600s),防止 goroutine 泄漏。
// 数据库写操作仍使用原始 ctx,避免超时上下文被取消导致状态写不回。
func (s *pipelineService) HandleRunTask() asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p runPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode pipeline.run payload: %w", err)
		}
		if p.RunID == 0 || p.PipelineID == 0 {
			return fmt.Errorf("pipeline.run: missing run_id or pipeline_id")
		}

		// 幂等检查
		run, err := s.r.Pipeline.GetRun(ctx, p.RunID)
		if err != nil {
			return fmt.Errorf("pipeline.run: load run %d: %w", p.RunID, err)
		}
		if run.Status == "succeeded" || run.Status == "failed" || run.Status == "cancelled" {
			s.log.Info("pipeline.run skipped (already terminal)",
				zap.Int64("run_id", p.RunID),
				zap.String("status", run.Status))
			return nil
		}

		pl, err := s.r.Pipeline.Get(ctx, p.PipelineID)
		if err != nil {
			_ = s.r.Pipeline.UpdateRunStatus(ctx, p.RunID, "failed", truncate(err.Error(), 1000))
			return fmt.Errorf("pipeline.run: load pipeline %d: %w", p.PipelineID, err)
		}
		if len(pl.DAG) == 0 {
			_ = s.r.Pipeline.UpdateRunStatus(ctx, p.RunID, "failed", "empty dag")
			return fmt.Errorf("pipeline.run: pipeline %d has empty dag", p.PipelineID)
		}

		// 原子化抢占: 只有 queued 才能抢到 running,防止并发 worker 重复执行
		if run.Status == "queued" {
			ok, err := s.r.Pipeline.UpdateRunStatusIf(ctx, p.RunID, "queued", "running", "")
			if err != nil {
				return fmt.Errorf("pipeline.run: acquire run %d: %w", p.RunID, err)
			}
			if !ok {
				s.log.Info("pipeline.run skipped (concurrent execution)",
					zap.Int64("run_id", p.RunID))
				return nil
			}
		}

		// 合并 overrides 到 input
		input := map[string]any{}
		for k, v := range p.Input {
			input[k] = v
		}
		for k, v := range p.Overrides {
			input[k] = v
		}

		s.log.Info("pipeline.run start",
			zap.Int64("pipeline_id", pl.ID),
			zap.Int64("run_id", run.ID),
			zap.Int("input_keys", len(input)),
		)

		// 为长耗时 pipeline 执行设置超时上下文
		runCtx, cancel := context.WithTimeout(ctx, getTimeout("TIMEOUT_PIPELINE_RUN", 600))
		defer cancel()

		out, err := s.registry.Execute(runCtx, []byte(pl.DAG), input, run.ID)
		if err != nil {
			_ = s.r.Pipeline.UpdateRunStatus(ctx, run.ID, "failed", truncate(err.Error(), 1000))
			s.log.Warn("pipeline.run failed",
				zap.Int64("run_id", run.ID),
				zap.Error(err),
			)
			return err
		}
		if out != nil {
			outBytes, err := json.Marshal(out)
			if err != nil {
				s.log.Warn("pipeline.run: marshal output failed", zap.Int64("run_id", run.ID), zap.Error(err))
			} else {
				_ = s.r.Pipeline.UpdateRunOutput(ctx, run.ID, model.JSON(outBytes))
			}
		}
		_ = s.r.Pipeline.UpdateRunStatus(ctx, run.ID, "succeeded", "")
		s.log.Info("pipeline.run done",
			zap.Int64("run_id", run.ID),
		)
		return nil
	}
}

// validateDAG 对流水线 DAG 做基本结构校验:合法 JSON、节点非空、边引用存在、无环。
func validateDAG(dagJSON json.RawMessage) error {
	if len(dagJSON) == 0 {
		return errors.New("dag is empty")
	}
	var dag struct {
		Nodes []struct {
			ID      string         `json:"id"`
			Type    string         `json:"type"`
			ModelID int64          `json:"model_id"`
			Params  map[string]any `json:"params"`
		} `json:"nodes"`
		Edges []struct {
			From    string            `json:"from"`
			To      string            `json:"to"`
			Mapping map[string]string `json:"mapping"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(dagJSON, &dag); err != nil {
		return fmt.Errorf("invalid dag json: %w", err)
	}
	if len(dag.Nodes) == 0 {
		return errors.New("dag has no nodes")
	}
	idx := make(map[string]bool, len(dag.Nodes))
	for _, n := range dag.Nodes {
		if n.ID == "" {
			return errors.New("dag node id is empty")
		}
		if n.Type == "" {
			return errors.New("dag node type is empty")
		}
		idx[n.ID] = true
	}
	inDeg := make(map[string]int, len(dag.Nodes))
	for id := range idx {
		inDeg[id] = 0
	}
	outAdj := make(map[string][]string)
	for _, e := range dag.Edges {
		if !idx[e.From] {
			return fmt.Errorf("dag edge.from %q not in nodes", e.From)
		}
		if !idx[e.To] {
			return fmt.Errorf("dag edge.to %q not in nodes", e.To)
		}
		outAdj[e.From] = append(outAdj[e.From], e.To)
		inDeg[e.To]++
	}
	// Kahn 拓扑排序检测环
	current := make([]string, 0, len(dag.Nodes))
	for id, d := range inDeg {
		if d == 0 {
			current = append(current, id)
		}
	}
	if len(current) == 0 {
		return errors.New("dag has cycle (no zero-indegree node)")
	}
	visited := 0
	for len(current) > 0 {
		next := make([]string, 0, len(dag.Nodes))
		for _, id := range current {
			visited++
			for _, to := range outAdj[id] {
				inDeg[to]--
				if inDeg[to] == 0 {
					next = append(next, to)
				}
			}
		}
		current = next
	}
	if visited != len(dag.Nodes) {
		return errors.New("dag has cycle")
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
