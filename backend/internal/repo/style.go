package repo

import (
	"context"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type StyleRepo struct{ db *gorm.DB }

func (r *StyleRepo) List(ctx context.Context, projectID int64) ([]model.Style, error) {
	var list []model.Style
	tx := r.db.WithContext(ctx).Where("project_id = ? OR project_id = 0", projectID)
	if err := tx.Order("id desc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *StyleRepo) Get(ctx context.Context, id int64) (*model.Style, error) {
	var s model.Style
	if err := r.db.WithContext(ctx).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *StyleRepo) Create(ctx context.Context, s *model.Style) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *StyleRepo) Update(ctx context.Context, s *model.Style) error {
	return r.db.WithContext(ctx).Model(&model.Style{}).Select("*").Omit("created_at").Where("id = ?", s.ID).Updates(s).Error
}

func (r *StyleRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Style{}, id).Error
}
