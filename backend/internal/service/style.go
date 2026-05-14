// Style 服务:风格库 CRUD。
package service

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
)

type styleService struct {
	r   *repo.Repositories
	log *zap.Logger
}

type CreateStyleInput struct {
	ProjectID       int64    `json:"project_id" binding:"required,gte=1"`
	Name            string   `json:"name" binding:"required,min=1,max=100"`
	ArtStyle        string   `json:"art_style" binding:"omitempty,min=1,max=100"`
	ColorTone       string   `json:"color_tone" binding:"omitempty,min=1,max=100"`
	Lighting        string   `json:"lighting" binding:"omitempty,min=1,max=100"`
	ReferenceImages []string `json:"reference_images"`
	LoraID          string   `json:"lora_id" binding:"omitempty,max=100"`
	Description     string   `json:"description" binding:"omitempty,max=500"`
}

type UpdateStyleInput struct {
	Name            *string  `json:"name" binding:"omitempty,min=1,max=100"`
	ArtStyle        *string  `json:"art_style" binding:"omitempty,min=1,max=100"`
	ColorTone       *string  `json:"color_tone" binding:"omitempty,min=1,max=100"`
	Lighting        *string  `json:"lighting" binding:"omitempty,min=1,max=100"`
	ReferenceImages []string `json:"reference_images"`
	LoraID          *string  `json:"lora_id" binding:"omitempty,max=100"`
	Description     *string  `json:"description" binding:"omitempty,max=500"`
	Status          *int8    `json:"status" binding:"omitempty,gte=0,lte=1"`
}

func (s *styleService) List(ctx context.Context, projectID int64) ([]model.Style, error) {
	return s.r.Style.List(ctx, projectID)
}

func (s *styleService) Get(ctx context.Context, id int64) (*model.Style, error) {
	st, err := s.r.Style.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return st, nil
}

func (s *styleService) Create(ctx context.Context, in *CreateStyleInput, uid int64) (*model.Style, error) {
	if in.Name == "" {
		return nil, errcode.ErrParam.WithMsg("name is required")
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

func (s *styleService) Update(ctx context.Context, id int64, in *UpdateStyleInput) (*model.Style, error) {
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

func (s *styleService) Delete(ctx context.Context, id int64) error {
	return s.r.Style.Delete(ctx, id)
}
