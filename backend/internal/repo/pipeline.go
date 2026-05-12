package repo

import (
	"context"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type PipelineRepo struct{ db *gorm.DB }

type ListPipelinesQuery struct {
	Page, PageSize int
	ProjectID      int64
	IsTemplate     int8
	Enabled        int8
}

func (r *PipelineRepo) List(ctx context.Context, q *ListPipelinesQuery) ([]model.Pipeline, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.Pipeline{})
	if q.ProjectID != 0 {
		tx = tx.Where("project_id = ? OR project_id = 0", q.ProjectID)
	}
	if q.IsTemplate != 0 {
		tx = tx.Where("is_template = ?", q.IsTemplate)
	}
	if q.Enabled != 0 {
		tx = tx.Where("enabled = ?", q.Enabled)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := pagination(q.Page, q.PageSize)
	var list []model.Pipeline
	if err := tx.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *PipelineRepo) Get(ctx context.Context, id int64) (*model.Pipeline, error) {
	var p model.Pipeline
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PipelineRepo) Create(ctx context.Context, p *model.Pipeline) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *PipelineRepo) Update(ctx context.Context, p *model.Pipeline) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *PipelineRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Pipeline{}, id).Error
}

// =============== PipelineRun / StepRun ===============

func (r *PipelineRepo) CreateRun(ctx context.Context, run *model.PipelineRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

func (r *PipelineRepo) GetRun(ctx context.Context, id int64) (*model.PipelineRun, error) {
	var run model.PipelineRun
	if err := r.db.WithContext(ctx).First(&run, id).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *PipelineRepo) ListRuns(ctx context.Context, pipelineID int64, page, size int) ([]model.PipelineRun, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.PipelineRun{})
	if pipelineID != 0 {
		tx = tx.Where("pipeline_id = ?", pipelineID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	p, s := pagination(page, size)
	var list []model.PipelineRun
	if err := tx.Order("id desc").Offset((p - 1) * s).Limit(s).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *PipelineRepo) UpdateRunStatus(ctx context.Context, id int64, status, errMsg string) error {
	upd := map[string]any{"status": status, "error_msg": errMsg}
	switch status {
	case "running":
		upd["started_at"] = time.Now()
	case "succeeded", "failed", "cancelled":
		upd["ended_at"] = time.Now()
	}
	return r.db.WithContext(ctx).Model(&model.PipelineRun{}).Where("id = ?", id).Updates(upd).Error
}

func (r *PipelineRepo) UpdateRunOutput(ctx context.Context, id int64, output model.JSON) error {
	return r.db.WithContext(ctx).Model(&model.PipelineRun{}).Where("id = ?", id).
		Update("output", output).Error
}

// =============== StepRun ===============

func (r *PipelineRepo) CreateStep(ctx context.Context, sr *model.StepRun) error {
	return r.db.WithContext(ctx).Create(sr).Error
}

func (r *PipelineRepo) UpdateStep(ctx context.Context, sr *model.StepRun) error {
	return r.db.WithContext(ctx).Save(sr).Error
}

func (r *PipelineRepo) ListSteps(ctx context.Context, runID int64) ([]model.StepRun, error) {
	var list []model.StepRun
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("id asc").Find(&list).Error
	return list, err
}

func (r *PipelineRepo) GetStep(ctx context.Context, id int64) (*model.StepRun, error) {
	var s model.StepRun
	if err := r.db.WithContext(ctx).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}
