package service

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
)

type ProjectService struct {
	project *repo.ProjectRepo
	log     *zap.Logger
}

type CreateProjectInput struct {
	Code              string   `json:"code" binding:"required"`
	Name              string   `json:"name" binding:"required"`
	Description       string   `json:"description"`
	DeptID            int64    `json:"dept_id"`
	DefaultPipelineID int64    `json:"default_pipeline_id"`
	CoverURL          string   `json:"cover_url"`
	Tags              []string `json:"tags"`
}

type UpdateProjectInput struct {
	Name              *string  `json:"name"`
	Description       *string  `json:"description"`
	Status            *int8    `json:"status"`
	DefaultPipelineID *int64   `json:"default_pipeline_id"`
	CoverURL          *string  `json:"cover_url"`
	Tags              []string `json:"tags"`
}

func (s *ProjectService) List(ctx context.Context, q *repo.ListProjectsQuery) ([]model.Project, int64, error) {
	return s.project.List(ctx, q)
}

func (s *ProjectService) Create(ctx context.Context, in *CreateProjectInput, uid, deptID int64) (*model.Project, error) {
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
	p.CreatedBy = uid
	p.UpdatedBy = uid
	if err := s.project.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProjectService) Get(ctx context.Context, id int64) (*model.Project, error) {
	p, err := s.project.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return p, nil
}

func (s *ProjectService) Update(ctx context.Context, id int64, in *UpdateProjectInput, uid int64) (*model.Project, error) {
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
	p.UpdatedBy = uid
	if err := s.project.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProjectService) Delete(ctx context.Context, id int64) error {
	return s.project.Delete(ctx, id)
}

// =============== 成员 ===============

func (s *ProjectService) ListMembers(ctx context.Context, projectID int64) ([]model.ProjectMember, error) {
	return s.project.ListMembers(ctx, projectID)
}

func (s *ProjectService) AddMember(ctx context.Context, projectID, userID int64, roleInProject string) error {
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

func (s *ProjectService) RemoveMember(ctx context.Context, projectID, userID int64) error {
	if projectID <= 0 || userID <= 0 {
		return errcode.ErrParam
	}
	return s.project.RemoveMember(ctx, projectID, userID)
}
