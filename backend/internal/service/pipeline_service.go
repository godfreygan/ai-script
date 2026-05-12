package service

import (
	"context"
	"encoding/json"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
)

// 本文件聚焦 PipelineService 的 CRUD / 运行历史查询。
// 异步任务相关(Run / SetDeps / pipelineRegistry)定义保留在 generation.go,这里只补充元数据 API。

// CreatePipelineInput 新建流水线
type CreatePipelineInput struct {
	ProjectID   int64           `json:"project_id"`
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	DAG         json.RawMessage `json:"dag" binding:"required"`
	IsTemplate  int8            `json:"is_template"`
	Enabled     int8            `json:"enabled"`
}

// UpdatePipelineInput 更新流水线
type UpdatePipelineInput struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	DAG         json.RawMessage `json:"dag"`
	IsTemplate  *int8           `json:"is_template"`
	Enabled     *int8           `json:"enabled"`
}

func (s *PipelineService) List(ctx context.Context, q *repo.ListPipelinesQuery) ([]model.Pipeline, int64, error) {
	return s.r.Pipeline.List(ctx, q)
}

func (s *PipelineService) Get(ctx context.Context, id int64) (*model.Pipeline, error) {
	p, err := s.r.Pipeline.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return p, nil
}

func (s *PipelineService) Create(ctx context.Context, in *CreatePipelineInput, uid int64) (*model.Pipeline, error) {
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

func (s *PipelineService) Update(ctx context.Context, id int64, in *UpdatePipelineInput) (*model.Pipeline, error) {
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

func (s *PipelineService) Delete(ctx context.Context, id int64) error {
	return s.r.Pipeline.Delete(ctx, id)
}

// ListRuns 某条流水线的历史运行记录
func (s *PipelineService) ListRuns(ctx context.Context, pipelineID int64, page, size int) ([]model.PipelineRun, int64, error) {
	return s.r.Pipeline.ListRuns(ctx, pipelineID, page, size)
}

// GetRun 单次运行详情
func (s *PipelineService) GetRun(ctx context.Context, runID int64) (*model.PipelineRun, error) {
	run, err := s.r.Pipeline.GetRun(ctx, runID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return run, nil
}

// ListSteps 单次运行的所有 step_runs
func (s *PipelineService) ListSteps(ctx context.Context, runID int64) ([]model.StepRun, error) {
	return s.r.Pipeline.ListSteps(ctx, runID)
}
