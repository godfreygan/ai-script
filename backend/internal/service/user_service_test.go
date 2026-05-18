package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"gorm.io/gorm"
)

func newUserService(t *testing.T) (UserService, *repo.Repositories) {
	db := newTestDB(t, &model.User{}, &model.Role{}, &model.UserRole{})
	r := newTestRepos(db)
	return &userService{user: r.User, role: r.Role, log: newNopLog()}, r
}

// ==================== Me ====================

func TestUserService_Me(t *testing.T) {
	ctx := context.Background()
	s, r := newUserService(t)

	// 预置用户
	u := &model.User{Username: "alice", PasswordHash: "secret", Nickname: "Alice", Status: 1}
	if err := r.User.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		got, err := s.Me(ctx, u.ID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.Username != "alice" {
			t.Fatalf("username=%s want alice", got.Username)
		}
		if got.PasswordHash != "" {
			t.Fatalf("password should be sanitized")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := s.Me(ctx, 99999)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// ==================== List ====================

func TestUserService_List(t *testing.T) {
	ctx := context.Background()
	s, r := newUserService(t)

	// 预置用户
	for _, name := range []string{"alice", "bob", "charlie"} {
		u := &model.User{Username: name, PasswordHash: "h", Status: 1}
		if err := r.User.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}

	t.Run("normal", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListUsersQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 3 {
			t.Fatalf("total=%d len=%d want 3,3", total, len(list))
		}
		for _, u := range list {
			if u.PasswordHash != "" {
				t.Fatalf("password should be sanitized")
			}
		}
	})

	t.Run("query filter", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListUsersQuery{Q: "ali", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].Username != "alice" {
			t.Fatalf("username=%s want alice", list[0].Username)
		}
	})
}

// ==================== Get ====================

func TestUserService_Get(t *testing.T) {
	ctx := context.Background()
	s, r := newUserService(t)

	// 预置用户 + 角色
	u := &model.User{Username: "alice", PasswordHash: "h", Status: 1}
	if err := r.User.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	role := &model.Role{Code: "admin", Name: "Admin", Status: 1}
	if err := r.Role.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := r.User.SetRoles(ctx, u.ID, []int64{role.ID}); err != nil {
		t.Fatalf("set roles: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		ur, err := s.Get(ctx, u.ID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if ur.Username != "alice" {
			t.Fatalf("username=%s want alice", ur.Username)
		}
		if len(ur.Roles) != 1 || ur.Roles[0] != role.ID {
			t.Fatalf("roles=%v want [%d]", ur.Roles, role.ID)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		_, err := s.Get(ctx, 0)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := s.Get(ctx, 99999)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// ==================== Create ====================

func TestUserService_Create(t *testing.T) {
	ctx := context.Background()
	s, r := newUserService(t)

	// 预置角色
	role := &model.Role{Code: "editor", Name: "Editor", Status: 1}
	if err := r.Role.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		u, err := s.Create(ctx, &CreateUserInput{
			Username: "alice",
			Password: "Password123",
			Nickname: "Alice",
			Email:    "alice@example.com",
			Phone:    "13800138000",
			DeptID:   1,
			RoleIDs:  []int64{role.ID},
			Status:   1,
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if u.Username != "alice" {
			t.Fatalf("username=%s want alice", u.Username)
		}
		if u.PasswordHash != "" {
			t.Fatalf("password should be sanitized")
		}
		// 验证角色绑定
		roles, _ := r.User.GetUserRoles(ctx, u.ID)
		if len(roles) != 1 || roles[0] != role.ID {
			t.Fatalf("roles=%v want [%d]", roles, role.ID)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		_, err := s.Create(ctx, nil)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("empty username", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateUserInput{Username: "   ", Password: "Password123"})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("password too short", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateUserInput{Username: "alice", Password: "123"})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("password weak variety", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateUserInput{Username: "alice", Password: "password"})
		if !isErr(err, errcode.ErrWeakPassword) {
			t.Fatalf("want ErrWeakPassword, got %v", err)
		}
	})

	t.Run("password contains username", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateUserInput{Username: "alice", Password: "Alice1234"})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("duplicate username", func(t *testing.T) {
		// 第一个用户
		_, err := s.Create(ctx, &CreateUserInput{Username: "dup", Password: "Password123"})
		if err != nil {
			t.Fatalf("first create: %v", err)
		}
		// 重复用户名
		_, err = s.Create(ctx, &CreateUserInput{Username: "dup", Password: "Password123"})
		if !isErr(err, errcode.ErrConflict) {
			t.Fatalf("want ErrConflict, got %v", err)
		}
	})

	t.Run("super long username", func(t *testing.T) {
		longName := strings.Repeat("a", 200)
		u, err := s.Create(ctx, &CreateUserInput{Username: longName, Password: "Password123"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if u.Username != longName {
			t.Fatalf("username mismatch")
		}
	})

	t.Run("default status", func(t *testing.T) {
		u, err := s.Create(ctx, &CreateUserInput{Username: "status_test", Password: "Password123"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if u.Status != 1 {
			t.Fatalf("status=%d want 1", u.Status)
		}
	})
}

// ==================== Update ====================

func TestUserService_Update(t *testing.T) {
	ctx := context.Background()
	s, r := newUserService(t)

	// 预置用户
	u := &model.User{Username: "alice", Nickname: "Old", Email: "old@example.com", Status: 1}
	if err := r.User.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	role := &model.Role{Code: "editor", Name: "Editor", Status: 1}
	if err := r.Role.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		newName := "New"
		newEmail := "new@example.com"
		newStatus := int8(0)
		updated, err := s.Update(ctx, u.ID, &UpdateUserInput{
			Nickname: &newName,
			Email:    &newEmail,
			Status:   &newStatus,
			RoleIDs:  &[]int64{role.ID},
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Nickname != "New" {
			t.Fatalf("nickname=%s want New", updated.Nickname)
		}
		if updated.Email != "new@example.com" {
			t.Fatalf("email=%s want new@example.com", updated.Email)
		}
		if updated.Status != 0 {
			t.Fatalf("status=%d want 0", updated.Status)
		}
		// 验证角色更新
		roles, _ := r.User.GetUserRoles(ctx, u.ID)
		if len(roles) != 1 || roles[0] != role.ID {
			t.Fatalf("roles=%v want [%d]", roles, role.ID)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		_, err := s.Update(ctx, 0, &UpdateUserInput{})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		_, err := s.Update(ctx, u.ID, nil)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := s.Update(ctx, 99999, &UpdateUserInput{})
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("partial update", func(t *testing.T) {
		// 只更新 nickname，其他字段不变
		partialName := "Partial"
		updated, err := s.Update(ctx, u.ID, &UpdateUserInput{Nickname: &partialName})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Nickname != "Partial" {
			t.Fatalf("nickname=%s want Partial", updated.Nickname)
		}
	})
}

// ==================== Delete ====================

func TestUserService_Delete(t *testing.T) {
	ctx := context.Background()
	s, r := newUserService(t)

	// 预置一个占位用户让 ID 从 2 开始，避免触发 super admin 保护
	placeholder := &model.User{Username: "placeholder", Status: 1}
	if err := r.User.Create(ctx, placeholder); err != nil {
		t.Fatalf("create placeholder user: %v", err)
	}
	u := &model.User{Username: "alice", Status: 1}
	if err := r.User.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.Delete(ctx, u.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// 验证已删除（repo 层返回原始 gorm 错误）
		_, err := r.User.GetByID(ctx, u.ID)
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("user should be deleted, got err=%v", err)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		err := s.Delete(ctx, 0)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("super admin cannot be deleted", func(t *testing.T) {
		err := s.Delete(ctx, 1)
		if !isErr(err, errcode.ErrConflict) {
			t.Fatalf("want ErrConflict, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := s.Delete(ctx, 99999)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// ==================== ResetPassword ====================

func TestUserService_ResetPassword(t *testing.T) {
	ctx := context.Background()
	s, r := newUserService(t)

	// 预置用户
	u := &model.User{Username: "alice", PasswordHash: "old_hash", Status: 1}
	if err := r.User.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.ResetPassword(ctx, u.ID, "NewPass456"); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// 验证密码已更新
		fresh, _ := r.User.GetByID(ctx, u.ID)
		if fresh.PasswordHash == "old_hash" {
			t.Fatalf("password should be updated")
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		err := s.ResetPassword(ctx, 0, "NewPass456")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := s.ResetPassword(ctx, 99999, "NewPass456")
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("password too short", func(t *testing.T) {
		err := s.ResetPassword(ctx, u.ID, "123")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("password weak variety", func(t *testing.T) {
		err := s.ResetPassword(ctx, u.ID, "password")
		if !isErr(err, errcode.ErrWeakPassword) {
			t.Fatalf("want ErrWeakPassword, got %v", err)
		}
	})

	t.Run("password contains username", func(t *testing.T) {
		err := s.ResetPassword(ctx, u.ID, "Alice1234")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})
}
