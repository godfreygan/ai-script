// Style 服务:风格库 CRUD。
package service

import (
	"context"
	"errors"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"go.uber.org/zap"
)

type StyleService struct {
	r   *repo.Repositories
	log *zap.Logger
}

type CreateStyleInput struct {
	ProjectID       int64    `json:"project_id"`
	Name            string   `json:"name" binding:"required"`
	ArtStyle        string   `json:"art_style"`
	ColorTone       string   `json:"color_tone"`
	Lighting        string   `json:"lighting"`
	ReferenceImages []string `json:"reference_images"`
	LoraID          string   `json:"lora_id"`
	Description     string   `json:"description"`
}

type UpdateStyleInput struct {
	Name            *string  `json:"name"`
	ArtStyle        *string  `json:"art_style"`
	ColorTone       *string  `json:"color_tone"`
	Lighting        *string  `json:"lighting"`
	ReferenceImages []string `json:"reference_images"`
	LoraID          *string  `json:"lora_id"`
	Description     *string  `json:"description"`
	Status          *int8    `json:"status"`
}

func (s *StyleService) List(ctx context.Context, projectID int64) ([]model.Style, error) {
	return s.r.Style.List(ctx, projectID)
}

func (s *StyleService) Get(ctx context.Context, id int64) (*model.Style, error) {
	return s.r.Style.Get(ctx, id)
}

func (s *StyleService) Create(ctx context.Context, in *CreateStyleInput, uid int64) (*model.Style, error) {
	if in.Name == "" {
		return nil, errors.New("name is required")
	}
	st := &model.Style{
		ProjectID:       in.ProjectID,
		Name:            in.Name,
		ArtStyle:        in.ArtStyle,
		ColorTone:       in.ColorTone,
		Lighting:        in.Lighting,
		ReferenceImages: toJSON(in.ReferenceImages),
		LoraID:          in.LoraID,
		Description:     in.Description,
		Status:          1,
		CreatedBy:       uid,
	}
	if err := s.r.Style.Create(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *StyleService) Update(ctx context.Context, id int64, in *UpdateStyleInput) (*model.Style, error) {
	st, err := s.r.Style.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		st.Name = *in.Name
	}
	if in.ArtStyle != nil {
		st.ArtStyle = *in.ArtStyle
	}
	if in.ColorTone != nil {
		st.ColorTone = *in.ColorTone
	}
	if in.Lighting != nil {
		st.Lighting = *in.Lighting
	}
	if in.ReferenceImages != nil {
		st.ReferenceImages = toJSON(in.ReferenceImages)
	}
	if in.LoraID != nil {
		st.LoraID = *in.LoraID
	}
	if in.Description != nil {
		st.Description = *in.Description
	}
	if in.Status != nil {
		st.Status = *in.Status
	}
	if err := s.r.Style.Update(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *StyleService) Delete(ctx context.Context, id int64) error {
	return s.r.Style.Delete(ctx, id)
}
