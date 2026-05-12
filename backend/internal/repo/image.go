package repo

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type ImageRepo struct{ db *gorm.DB }

type ListImagesQuery struct {
	Page, PageSize int
	ProjectID      int64
	StoryboardID   int64
	Status         int8
}

func (r *ImageRepo) List(ctx context.Context, q *ListImagesQuery) ([]model.Image, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.Image{})
	if q.ProjectID != 0 {
		tx = tx.Where("project_id = ?", q.ProjectID)
	}
	if q.StoryboardID != 0 {
		tx = tx.Where("storyboard_id = ?", q.StoryboardID)
	}
	if q.Status != 0 {
		tx = tx.Where("status = ?", q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := pagination(q.Page, q.PageSize)
	var list []model.Image
	if err := tx.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *ImageRepo) Get(ctx context.Context, id int64) (*model.Image, error) {
	var img model.Image
	if err := r.db.WithContext(ctx).First(&img, id).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

func (r *ImageRepo) Create(ctx context.Context, img *model.Image) error {
	return r.db.WithContext(ctx).Create(img).Error
}

func (r *ImageRepo) Update(ctx context.Context, img *model.Image) error {
	return r.db.WithContext(ctx).Save(img).Error
}

func (r *ImageRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Image{}, id).Error
}
