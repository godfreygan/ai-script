package service

import (
	"context"
	"errors"
	"strings"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RoleService struct {
	role     *repo.RoleRepo
	enforcer *casbin.Enforcer
	log      *zap.Logger
}

type CreateRoleInput struct {
	Code        string   `json:"code" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	DataScope   string   `json:"data_scope"` // self/dept/all
	Permissions []string `json:"permissions"`
}

type UpdateRoleInput struct {
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	DataScope   *string   `json:"data_scope"`
	Status      *int8     `json:"status"`
	Permissions *[]string `json:"permissions"`
}

func validDataScope(s string) bool {
	switch s {
	case "self", "dept", "all":
		return true
	}
	return false
}

func (s *RoleService) List(ctx context.Context) ([]model.Role, error) {
	list, err := s.role.List(ctx)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return list, nil
}

type RoleWithPermissions struct {
	*model.Role
	Permissions []string `json:"permissions"`
}

func (s *RoleService) Get(ctx context.Context, id int64) (*RoleWithPermissions, error) {
	if id <= 0 {
		return nil, errcode.ErrParam.WithMsg("invalid role id")
	}
	r, err := s.role.Get(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	perms, err := s.role.GetRolePermissionCodes(ctx, id)
	if err != nil {
		s.log.Warn("load role permissions failed", zap.Int64("role_id", id), zap.Error(err))
	}
	return &RoleWithPermissions{Role: r, Permissions: perms}, nil
}

func (s *RoleService) ListPermissions(ctx context.Context) ([]model.Permission, error) {
	list, err := s.role.ListPermissions(ctx)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return list, nil
}

func (s *RoleService) Create(ctx context.Context, in *CreateRoleInput) (*model.Role, error) {
	if in == nil {
		return nil, errcode.ErrParam
	}
	code := strings.TrimSpace(in.Code)
	name := strings.TrimSpace(in.Name)
	if code == "" || name == "" {
		return nil, errcode.ErrParam.WithMsg("code and name required")
	}
	scope := in.DataScope
	if scope == "" {
		scope = "self"
	}
	if !validDataScope(scope) {
		return nil, errcode.ErrParam.WithMsg("invalid data_scope")
	}
	// 重复 code 校验
	exist, err := s.role.GetByCode(ctx, code)
	if err == nil && exist != nil {
		return nil, errcode.ErrConflict.WithMsg("role code already exists")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	r := &model.Role{
		Code:        code,
		Name:        name,
		Description: in.Description,
		DataScope:   scope,
		IsSystem:    0,
		Status:      1,
	}
	if err := s.role.Create(ctx, r); err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if len(in.Permissions) > 0 {
		if err := s.role.SetRolePermissions(ctx, r.ID, in.Permissions); err != nil {
			return nil, errcode.ErrInternal.Wrap(err)
		}
		if err := s.SyncCasbin(ctx); err != nil {
			s.log.Warn("sync casbin failed after create role", zap.Int64("role_id", r.ID), zap.Error(err))
		}
	}
	return r, nil
}

func (s *RoleService) Update(ctx context.Context, id int64, in *UpdateRoleInput) (*model.Role, error) {
	if id <= 0 || in == nil {
		return nil, errcode.ErrParam
	}
	r, err := s.role.Get(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, errcode.ErrParam.WithMsg("name cannot be empty")
		}
		r.Name = name
	}
	if in.Description != nil {
		r.Description = *in.Description
	}
	if in.DataScope != nil {
		if !validDataScope(*in.DataScope) {
			return nil, errcode.ErrParam.WithMsg("invalid data_scope")
		}
		r.DataScope = *in.DataScope
	}
	if in.Status != nil {
		r.Status = *in.Status
	}
	if err := s.role.Update(ctx, r); err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if in.Permissions != nil {
		if err := s.role.SetRolePermissions(ctx, id, *in.Permissions); err != nil {
			return nil, errcode.ErrInternal.Wrap(err)
		}
		if err := s.SyncCasbin(ctx); err != nil {
			s.log.Warn("sync casbin failed after update role", zap.Int64("role_id", id), zap.Error(err))
		}
	}
	return r, nil
}

func (s *RoleService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return errcode.ErrParam.WithMsg("invalid role id")
	}
	if err := s.role.Delete(ctx, id); err != nil {
		switch {
		case errors.Is(err, repo.ErrRoleHasUsers):
			return errcode.ErrConflict.WithMsg("role still has users")
		case errors.Is(err, repo.ErrConflict):
			return errcode.ErrConflict.WithMsg("system role cannot be deleted")
		case errors.Is(err, gorm.ErrRecordNotFound):
			return errcode.ErrNotFound
		default:
			return errcode.ErrInternal.Wrap(err)
		}
	}
	if err := s.SyncCasbin(ctx); err != nil {
		s.log.Warn("sync casbin failed after delete role", zap.Int64("role_id", id), zap.Error(err))
	}
	return nil
}

// SyncCasbin 把数据库里的角色-权限映射重写到 Casbin
// Casbin policy 用 (role_code, resource, action)
func (s *RoleService) SyncCasbin(ctx context.Context) error {
	if s.enforcer == nil {
		return nil
	}
	pairs, err := s.role.AllRolePermissions(ctx)
	if err != nil {
		return err
	}
	// 清空所有 p 规则
	if _, err := s.enforcer.RemoveFilteredPolicy(0); err != nil {
		return err
	}
	for _, p := range pairs {
		role := p[0]
		resource, action := splitResAct(p[1])
		if _, err := s.enforcer.AddPolicy(role, resource, action); err != nil {
			return err
		}
	}
	return s.enforcer.SavePolicy()
}

func splitResAct(s string) (string, string) {
	if i := strings.IndexByte(s, '|'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}
