package service

import (
	"context"

	"encoding/json"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
)

type projectService struct {
	project *repo.ProjectRepo
	log     *zap.Logger
}

type CreateProjectInput struct {
	Code              string   `json:"code" binding:"required,min=1,max=100"`
	Name              string   `json:"name" binding:"required,min=1,max=100"`
	Description       string   `json:"description" binding:"omitempty,max=500"`
	DeptID            int64    `json:"dept_id" binding:"omitempty,gte=1"`
	DefaultPipelineID int64    `json:"default_pipeline_id" binding:"omitempty,gte=1"`
	CoverURL          string   `json:"cover_url" binding:"omitempty,max=500"`
	Tags              []string `json:"tags"`
}

type UpdateProjectInput struct {
	Name              *string  `json:"name" binding:"omitempty,min=1,max=100"`
	Description       *string  `json:"description" binding:"omitempty,max=500"`
	Status            *int8    `json:"status" binding:"omitempty,gte=0,lte=1"`
	DefaultPipelineID *int64   `json:"default_pipeline_id" binding:"omitempty,gte=1"`
	CoverURL          *string  `json:"cover_url" binding:"omitempty,max=500"`
	Tags              []string `json:"tags"`
}

func (s *projectService) List(ctx context.Context, q *repo.ListProjectsQuery) ([]model.Project, int64, error) {
	return s.project.List(ctx, q)
}

func (s *projectService) Create(ctx context.Context, in *CreateProjectInput, uid, deptID int64) (*model.Project, error) {
	if in.DeptID == 0 {
		in.DeptID = deptID
	}
	p := &model.Project{
		Code:              in.Code,
		Name:              in.Name,
		Description:       in.Description,
		OwnerID:           uid,
		DeptID:            in.DeptID,
		Status:            1,
		DefaultPipelineID: in.DefaultPipelineID,
		CoverURL:          in.CoverURL,
	}
	if len(in.Tags) > 0 {
		b, err := json.Marshal(in.Tags)
		if err != nil {
			return nil, errcode.ErrParam.Wrap(err)
		}
		p.Tags = model.JSON(b)
	}
	p.CreatedBy = uid
	p.UpdatedBy = uid
	if err := s.project.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *projectService) Get(ctx context.Context, id int64) (*model.Project, error) {
	p, err := s.project.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return p, nil
}

func (s *projectService) Update(ctx context.Context, id int64, in *UpdateProjectInput, uid int64) (*model.Project, error) {
	p, err := s.project.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	if in.Name != nil {
		p.Name = *in.Name
	}
	if in.Description != nil {
		p.Description = *in.Description
	}
	if in.Status != nil {
		p.Status = *in.Status
	}
	if in.DefaultPipelineID != nil {
		p.DefaultPipelineID = *in.DefaultPipelineID
	}
	if in.CoverURL != nil {
		p.CoverURL = *in.CoverURL
	}
	if in.Tags != nil {
		b, err := json.Marshal(in.Tags)
		if err != nil {
			return nil, errcode.ErrParam.Wrap(err)
		}
		p.Tags = model.JSON(b)
	}
	p.UpdatedBy = uid
	if err := s.project.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *projectService) Delete(ctx context.Context, id int64) error {
	return s.project.Delete(ctx, id)
}

// =============== 成员 ===============

func (s *projectService) ListMembers(ctx context.Context, projectID int64) ([]model.ProjectMember, error) {
	return s.project.ListMembers(ctx, projectID)
}

func (s *projectService) AddMember(ctx context.Context, projectID, userID int64, roleInProject string) error {
	if projectID <= 0 || userID <= 0 {
		return errcode.ErrParam
	}
	if roleInProject == "" {
		roleInProject = "editor"
	}
	// 项目必须存在
	if _, err := s.project.GetByID(ctx, projectID); err != nil {
		return errcode.ErrNotFound
	}
	// 幂等:已是成员则直接返回成功,避免脏数据(目前表无唯一约束)
	exists, err := s.project.IsMember(ctx, projectID, userID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.project.AddMember(ctx, &model.ProjectMember{
		ProjectID:     projectID,
		UserID:        userID,
		RoleInProject: roleInProject,
	})
}

func (s *projectService) RemoveMember(ctx context.Context, projectID, userID int64) error {
	if projectID <= 0 || userID <= 0 {
		return errcode.ErrParam
	}
	return s.project.RemoveMember(ctx, projectID, userID)
}
