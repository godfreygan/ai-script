package repo

import (
	"context"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type PromptRepo struct{ db *gorm.DB }

func (r *PromptRepo) ListByEpisode(ctx context.Context, episodeID int64) ([]model.EpisodePrompt, error) {
	var list []model.EpisodePrompt
	err := r.db.WithContext(ctx).Where("episode_id = ?", episodeID).
		Order("version desc").Find(&list).Error
	return list, err
}

func (r *PromptRepo) GetCurrent(ctx context.Context, episodeID int64) (*model.EpisodePrompt, error) {
	var p model.EpisodePrompt
	err := r.db.WithContext(ctx).Where("episode_id = ? AND is_current = 1", episodeID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PromptRepo) Get(ctx context.Context, id int64) (*model.EpisodePrompt, error) {
	var p model.EpisodePrompt
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateAsCurrent 新建并设为当前版本(自动 +1)
func (r *PromptRepo) CreateAsCurrent(ctx context.Context, p *model.EpisodePrompt) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxVer int
		_ = tx.Model(&model.EpisodePrompt{}).Where("episode_id = ?", p.EpisodeID).
			Select("COALESCE(MAX(version),0)").Scan(&maxVer).Error
		p.Version = maxVer + 1
		p.IsCurrent = 1
		if err := tx.Model(&model.EpisodePrompt{}).Where("episode_id = ?", p.EpisodeID).
			Update("is_current", 0).Error; err != nil {
			return err
		}
		return tx.Create(p).Error
	})
}

func (r *PromptRepo) Update(ctx context.Context, p *model.EpisodePrompt) error {
	return r.db.WithContext(ctx).Model(&model.EpisodePrompt{}).Select("*").Omit("created_at").Where("id = ?", p.ID).Updates(p).Error
}

func (r *PromptRepo) SetCurrent(ctx context.Context, episodeID, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.EpisodePrompt{}).Where("episode_id = ?", episodeID).
			Update("is_current", 0).Error; err != nil {
			return err
		}
		return tx.Model(&model.EpisodePrompt{}).Where("id = ?", id).
			Update("is_current", 1).Error
	})
}
