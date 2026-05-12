package repo

import (
	"context"
	"errors"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type FeatureFlagRepo struct{ db *gorm.DB }

func (r *FeatureFlagRepo) List(ctx context.Context) ([]model.FeatureFlag, error) {
	var list []model.FeatureFlag
	err := r.db.WithContext(ctx).Order("id desc").Find(&list).Error
	return list, err
}

func (r *FeatureFlagRepo) Get(ctx context.Context, id int64) (*model.FeatureFlag, error) {
	var f model.FeatureFlag
	if err := r.db.WithContext(ctx).First(&f, id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FeatureFlagRepo) GetByKey(ctx context.Context, key string) (*model.FeatureFlag, error) {
	var f model.FeatureFlag
	err := r.db.WithContext(ctx).Where("`key` = ?", key).First(&f).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

func (r *FeatureFlagRepo) Create(ctx context.Context, f *model.FeatureFlag) error {
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *FeatureFlagRepo) Update(ctx context.Context, f *model.FeatureFlag) error {
	return r.db.WithContext(ctx).Save(f).Error
}

func (r *FeatureFlagRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.FeatureFlag{}, id).Error
}
