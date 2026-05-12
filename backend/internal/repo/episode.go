package repo

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type EpisodeRepo struct{ db *gorm.DB }

func (r *EpisodeRepo) ListByScript(ctx context.Context, scriptID int64) ([]model.Episode, error) {
	var list []model.Episode
	err := r.db.WithContext(ctx).Where("script_id = ?", scriptID).
		Order("ep_no asc").Find(&list).Error
	return list, err
}

func (r *EpisodeRepo) Get(ctx context.Context, id int64) (*model.Episode, error) {
	var e model.Episode
	if err := r.db.WithContext(ctx).First(&e, id).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EpisodeRepo) Create(ctx context.Context, e *model.Episode) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *EpisodeRepo) BulkCreate(ctx context.Context, list []model.Episode) error {
	if len(list) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(list, 100).Error
}

func (r *EpisodeRepo) Update(ctx context.Context, e *model.Episode) error {
	return r.db.WithContext(ctx).Save(e).Error
}

func (r *EpisodeRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Episode{}, id).Error
}

func (r *EpisodeRepo) DeleteByScript(ctx context.Context, scriptID int64) error {
	return r.db.WithContext(ctx).Where("script_id = ?", scriptID).Delete(&model.Episode{}).Error
}
