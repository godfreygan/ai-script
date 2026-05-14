package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type deptService struct {
	dept *repo.DeptRepo
	log  *zap.Logger
}

type CreateDeptInput struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	ParentID int64  `json:"parent_id" binding:"gte=0"`
	Sort     int    `json:"sort" binding:"gte=0,lte=9999"`
}

func (s *deptService) List(ctx context.Context) ([]model.Department, error) {
	list, err := s.dept.List(ctx)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return list, nil
}

func (s *deptService) Get(ctx context.Context, id int64) (*model.Department, error) {
	if id <= 0 {
		return nil, errcode.ErrParam.WithMsg("invalid dept id")
	}
	d, err := s.dept.Get(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return d, nil
}

func (s *deptService) Create(ctx context.Context, in *CreateDeptInput) (*model.Department, error) {
	if in == nil {
		return nil, errcode.ErrParam
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errcode.ErrParam.WithMsg("dept name required")
	}
	d := &model.Department{
		Name:     name,
		ParentID: in.ParentID,
		Sort:     in.Sort,
		Status:   1,
	}
	parentPath := "/"
	if in.ParentID > 0 {
		parent, err := s.dept.Get(ctx, in.ParentID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errcode.ErrParam.WithMsg("parent dept not found")
			}
			return nil, errcode.ErrInternal.Wrap(err)
		}
		if parent.Path != "" {
			parentPath = parent.Path + "/"
		}
	}
	// 先占位写入拿 ID,再回写完整 path
	if err := s.dept.Create(ctx, d); err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	d.Path = parentPath + strconv.FormatInt(d.ID, 10)
	if err := s.dept.Update(ctx, d); err != nil {
		s.log.Warn("write back dept path failed", zap.Int64("dept_id", d.ID), zap.Error(err))
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return d, nil
}

func (s *deptService) Update(ctx context.Context, id int64, name string, sort int, status int8) (*model.Department, error) {
	if id <= 0 {
		return nil, errcode.ErrParam.WithMsg("invalid dept id")
	}
	d, err := s.dept.Get(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if v := strings.TrimSpace(name); v != "" {
		d.Name = v
	}
	if sort != 0 {
		d.Sort = sort
	}
	if status != 0 {
		d.Status = status
	}
	if err := s.dept.Update(ctx, d); err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return d, nil
}

func (s *deptService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return errcode.ErrParam.WithMsg("invalid dept id")
	}
	if err := s.dept.Delete(ctx, id); err != nil {
		if errors.Is(err, repo.ErrDeptHasUsers) {
			return errcode.ErrConflict.WithMsg("dept still has users")
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrNotFound
		}
		return errcode.ErrInternal.Wrap(err)
	}
	return nil
}
