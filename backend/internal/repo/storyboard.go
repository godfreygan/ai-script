package repo

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type StoryboardRepo struct{ db *gorm.DB }

func (r *StoryboardRepo) ListByEpisode(ctx context.Context, episodeID int64) ([]model.Storyboard, error) {
	var list []model.Storyboard
	err := r.db.WithContext(ctx).Where("episode_id = ?", episodeID).
		Order("shot_no asc").Find(&list).Error
	return list, err
}

func (r *StoryboardRepo) Get(ctx context.Context, id int64) (*model.Storyboard, error) {
	var s model.Storyboard
	if err := r.db.WithContext(ctx).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *StoryboardRepo) Create(ctx context.Context, s *model.Storyboard) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *StoryboardRepo) BulkCreate(ctx context.Context, list []model.Storyboard) error {
	if len(list) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(list, 100).Error
}

func (r *StoryboardRepo) Update(ctx context.Context, s *model.Storyboard) error {
	return r.db.WithContext(ctx).Model(&model.Storyboard{}).Select("*").Omit("created_at").Where("id = ?", s.ID).Updates(s).Error
}

func (r *StoryboardRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Storyboard{}, id).Error
}

func (r *StoryboardRepo) DeleteByEpisode(ctx context.Context, episodeID int64) error {
	return r.db.WithContext(ctx).Where("episode_id = ?", episodeID).
		Delete(&model.Storyboard{}).Error
}

// ApplyStyle 关联风格
func (r *StoryboardRepo) ApplyStyle(ctx context.Context, storyboardID, styleID, userID int64) error {
	return r.db.WithContext(ctx).Save(&model.StoryboardStyle{
		StoryboardID: storyboardID,
		StyleID:      styleID,
		AppliedBy:    userID,
	}).Error
}

func (r *StoryboardRepo) ListStyles(ctx context.Context, storyboardID int64) ([]model.StoryboardStyle, error) {
	var list []model.StoryboardStyle
	err := r.db.WithContext(ctx).Where("storyboard_id = ?", storyboardID).Find(&list).Error
	return list, err
}
