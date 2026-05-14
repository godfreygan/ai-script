package repo

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type ShortVideoRepo struct{ db *gorm.DB }

type ListShortVideosQuery struct {
	Page, PageSize int
	ProjectID      int64
	StoryboardID   int64
	Status         string
}

func (r *ShortVideoRepo) List(ctx context.Context, q *ListShortVideosQuery) ([]model.ShortVideo, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.ShortVideo{})
	if q.ProjectID != 0 {
		tx = tx.Where("project_id = ?", q.ProjectID)
	}
	if q.StoryboardID != 0 {
		tx = tx.Where("storyboard_id = ?", q.StoryboardID)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := pagination(q.Page, q.PageSize)
	var list []model.ShortVideo
	if err := tx.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *ShortVideoRepo) Get(ctx context.Context, id int64) (*model.ShortVideo, error) {
	var s model.ShortVideo
	if err := r.db.WithContext(ctx).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ShortVideoRepo) Create(ctx context.Context, s *model.ShortVideo) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *ShortVideoRepo) Update(ctx context.Context, s *model.ShortVideo) error {
	return r.db.WithContext(ctx).Model(&model.ShortVideo{}).Select("*").Omit("created_at").Where("id = ?", s.ID).Updates(s).Error
}

func (r *ShortVideoRepo) UpdateStatus(ctx context.Context, id int64, status, errMsg string) error {
	return r.db.WithContext(ctx).Model(&model.ShortVideo{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "error_msg": errMsg}).Error
}

func (r *ShortVideoRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ShortVideo{}, id).Error
}
