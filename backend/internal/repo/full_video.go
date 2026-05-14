package repo

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type FullVideoRepo struct{ db *gorm.DB }

type ListFullVideosQuery struct {
	Page, PageSize int
	ProjectID      int64
	Status         string
}

func (r *FullVideoRepo) List(ctx context.Context, q *ListFullVideosQuery) ([]model.FullVideo, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.FullVideo{})
	if q.ProjectID != 0 {
		tx = tx.Where("project_id = ?", q.ProjectID)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := pagination(q.Page, q.PageSize)
	var list []model.FullVideo
	if err := tx.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *FullVideoRepo) Get(ctx context.Context, id int64) (*model.FullVideo, error) {
	var f model.FullVideo
	if err := r.db.WithContext(ctx).First(&f, id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FullVideoRepo) Create(ctx context.Context, f *model.FullVideo) error {
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *FullVideoRepo) Update(ctx context.Context, f *model.FullVideo) error {
	return r.db.WithContext(ctx).Model(&model.FullVideo{}).Select("*").Omit("created_at").Where("id = ?", f.ID).Updates(f).Error
}

func (r *FullVideoRepo) UpdateStatus(ctx context.Context, id int64, status string, progress int, errMsg string) error {
	return r.db.WithContext(ctx).Model(&model.FullVideo{}).Where("id = ?", id).
		Updates(map[string]any{
			"status":          status,
			"render_progress": progress,
			"error_msg":       errMsg,
		}).Error
}

func (r *FullVideoRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.FullVideo{}, id).Error
}
