package service

import (
	"context"
	"errors"
	"strings"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService struct {
	user *repo.UserRepo
	role *repo.RoleRepo
	log  *zap.Logger
}

type CreateUserInput struct {
	Username string  `json:"username" binding:"required"`
	Password string  `json:"password" binding:"required,min=6"`
	Nickname string  `json:"nickname"`
	Email    string  `json:"email"`
	Phone    string  `json:"phone"`
	DeptID   int64   `json:"dept_id"`
	RoleIDs  []int64 `json:"role_ids"`
	Status   int8    `json:"status"`
}

type UpdateUserInput struct {
	Nickname *string  `json:"nickname"`
	Email    *string  `json:"email"`
	Phone    *string  `json:"phone"`
	DeptID   *int64   `json:"dept_id"`
	Status   *int8    `json:"status"`
	RoleIDs  *[]int64 `json:"role_ids"`
}

// sanitize 把 User 里的敏感字段抹掉,避免泄露到前端
func sanitizeUser(u *model.User) *model.User {
	if u == nil {
		return nil
	}
	u.PasswordHash = ""
	return u
}

func (s *UserService) Me(ctx context.Context, uid int64) (*model.User, error) {
	u, err := s.user.GetByID(ctx, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return sanitizeUser(u), nil
}

type UserWithRoles struct {
	*model.User
	Roles []int64 `json:"role_ids"`
}

func (s *UserService) List(ctx context.Context, q *repo.ListUsersQuery) ([]model.User, int64, error) {
	list, total, err := s.user.List(ctx, q)
	if err != nil {
		return nil, 0, errcode.ErrInternal.Wrap(err)
	}
	for i := range list {
		list[i].PasswordHash = ""
	}
	return list, total, nil
}

func (s *UserService) Get(ctx context.Context, id int64) (*UserWithRoles, error) {
	if id <= 0 {
		return nil, errcode.ErrParam.WithMsg("invalid user id")
	}
	u, err := s.user.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	roles, err := s.user.GetUserRoles(ctx, id)
	if err != nil {
		s.log.Warn("load user roles failed", zap.Int64("uid", id), zap.Error(err))
	}
	return &UserWithRoles{User: sanitizeUser(u), Roles: roles}, nil
}

func (s *UserService) Create(ctx context.Context, in *CreateUserInput) (*model.User, error) {
	if in == nil {
		return nil, errcode.ErrParam
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" {
		return nil, errcode.ErrParam.WithMsg("username required")
	}
	if len(in.Password) < 6 {
		return nil, errcode.ErrParam.WithMsg("password too short")
	}
	// 用户名重复校验:必须区分 not found / 其它错误
	exist, err := s.user.GetByUsername(ctx, in.Username)
	if err == nil && exist != nil {
		return nil, errcode.ErrConflict.WithMsg("username already exists")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	status := int8(1)
	if in.Status != 0 {
		status = in.Status
	}
	u := &model.User{
		Username:     in.Username,
		PasswordHash: string(hash),
		Nickname:     in.Nickname,
		Email:        in.Email,
		Phone:        in.Phone,
		DeptID:       in.DeptID,
		Status:       status,
	}
	if err := s.user.Create(ctx, u); err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if len(in.RoleIDs) > 0 {
		if err := s.user.SetRoles(ctx, u.ID, in.RoleIDs); err != nil {
			return nil, errcode.ErrInternal.Wrap(err)
		}
	}
	return sanitizeUser(u), nil
}

func (s *UserService) Update(ctx context.Context, id int64, in *UpdateUserInput) (*model.User, error) {
	if id <= 0 || in == nil {
		return nil, errcode.ErrParam
	}
	u, err := s.user.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if in.Nickname != nil {
		u.Nickname = *in.Nickname
	}
	if in.Email != nil {
		u.Email = *in.Email
	}
	if in.Phone != nil {
		u.Phone = *in.Phone
	}
	if in.DeptID != nil {
		u.DeptID = *in.DeptID
	}
	if in.Status != nil {
		u.Status = *in.Status
	}
	if err := s.user.Update(ctx, u); err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if in.RoleIDs != nil {
		if err := s.user.SetRoles(ctx, id, *in.RoleIDs); err != nil {
			return nil, errcode.ErrInternal.Wrap(err)
		}
	}
	return sanitizeUser(u), nil
}

func (s *UserService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return errcode.ErrParam
	}
	if id == 1 {
		return errcode.ErrConflict.WithMsg("super admin cannot be deleted")
	}
	if err := s.user.Delete(ctx, id); err != nil {
		return errcode.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *UserService) ResetPassword(ctx context.Context, id int64, newPw string) error {
	if id <= 0 {
		return errcode.ErrParam
	}
	if len(newPw) < 6 {
		return errcode.ErrParam.WithMsg("password too short")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if err != nil {
		return errcode.ErrInternal.Wrap(err)
	}
	if err := s.user.UpdatePassword(ctx, id, string(hash)); err != nil {
		return errcode.ErrInternal.Wrap(err)
	}
	return nil
}
