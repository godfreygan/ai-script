package repo

import (
	"context"
	"errors"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type FeatureFlagRepo struct {
	db  *gorm.DB
	rdb *redis.Client
}

func (r *FeatureFlagRepo) WithDB(db *gorm.DB) *FeatureFlagRepo {
	return &FeatureFlagRepo{db: db, rdb: r.rdb}
}

func (r *FeatureFlagRepo) WithRedis(rdb *redis.Client) *FeatureFlagRepo {
	return &FeatureFlagRepo{db: r.db, rdb: rdb}
}

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
	loader := func(ctx context.Context) (*model.FeatureFlag, error) {
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
	return Get(ctx, r.rdb, cacheKey("feature_flag", key), loader, 5*time.Minute)
}

func (r *FeatureFlagRepo) Create(ctx context.Context, f *model.FeatureFlag) error {
	if err := r.db.WithContext(ctx).Create(f).Error; err != nil {
		return err
	}
	Delete(ctx, r.rdb, cacheKey("feature_flag", f.Key))
	return nil
}

func (r *FeatureFlagRepo) Update(ctx context.Context, f *model.FeatureFlag) error {
	if err := r.db.WithContext(ctx).Model(&model.FeatureFlag{}).Select("*").Omit("created_at").Where("id = ?", f.ID).Updates(f).Error; err != nil {
		return err
	}
	Delete(ctx, r.rdb, cacheKey("feature_flag", f.Key))
	return nil
}

func (r *FeatureFlagRepo) Delete(ctx context.Context, id int64) error {
	var f model.FeatureFlag
	if err := r.db.WithContext(ctx).First(&f, id).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Delete(&model.FeatureFlag{}, id).Error; err != nil {
		return err
	}
	Delete(ctx, r.rdb, cacheKey("feature_flag", f.Key))
	return nil
}
