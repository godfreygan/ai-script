package service

import (
	"context"
	"testing"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

func newAuthService(t *testing.T) (*authService, *repo.Repositories) {
	db := newTestDB(t, &model.User{}, &model.Role{}, &model.UserRole{})
	r := newTestRepos(db)
	jwtMgr := jwt.NewManager("test-secret", 7200, 86400)
	return &authService{user: r.User, jwt: jwtMgr, log: newNopLog()}, r
}

// ==================== Login ====================

func TestAuthService_Login(t *testing.T) {
	ctx := context.Background()
	s, r := newAuthService(t)

	// 预置用户（密码: Password123）
	u := &model.User{Username: "alice", PasswordHash: testHash("Password123"), Status: 1, DeptID: 10}
	if err := r.User.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	// 预置角色
	role := &model.Role{Code: "admin", Name: "Admin", Status: 1}
	if err := r.Role.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := r.User.SetRoles(ctx, u.ID, []int64{role.ID}); err != nil {
		t.Fatalf("set roles: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		res, err := s.Login(ctx, "alice", "Password123", "127.0.0.1")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if res.AccessToken == "" {
			t.Fatalf("access_token empty")
		}
		if res.RefreshToken == "" {
			t.Fatalf("refresh_token empty")
		}
		if res.User.Username != "alice" {
			t.Fatalf("username=%s want alice", res.User.Username)
		}
		if res.User.PasswordHash != "" {
			t.Fatalf("password should be sanitized")
		}
		if len(res.Roles) != 1 || res.Roles[0] != "admin" {
			t.Fatalf("roles=%v want [admin]", res.Roles)
		}
		if res.ExpiresIn != accessTokenExpiresIn {
			t.Fatalf("expires_in=%d want %d", res.ExpiresIn, accessTokenExpiresIn)
		}
		// 验证 last_login_at 已更新
		if res.User.LastLoginAt == nil {
			t.Fatalf("LastLoginAt should be set")
		}
	})

	t.Run("empty username", func(t *testing.T) {
		_, err := s.Login(ctx, "", "Password123", "")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("empty password", func(t *testing.T) {
		_, err := s.Login(ctx, "alice", "", "")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		_, err := s.Login(ctx, "nobody", "Password123", "")
		if !isErr(err, errcode.ErrUnauthorized) {
			t.Fatalf("want ErrUnauthorized, got %v", err)
		}
	})

	t.Run("user disabled", func(t *testing.T) {
		// 创建禁用用户（Status=0 会被 gorm default:1 覆盖，需显式更新）
		u2 := &model.User{Username: "disabled", PasswordHash: testHash("Password123"), Status: 0}
		if err := r.User.Create(ctx, u2); err != nil {
			t.Fatalf("create user: %v", err)
		}
		if err := r.DB.Model(u2).Update("status", 0).Error; err != nil {
			t.Fatalf("update status: %v", err)
		}
		_, err := s.Login(ctx, "disabled", "Password123", "")
		if !isErr(err, errcode.ErrUnauthorized) {
			t.Fatalf("want ErrUnauthorized, got %v", err)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		_, err := s.Login(ctx, "alice", "WrongPass123", "")
		if !isErr(err, errcode.ErrUnauthorized) {
			t.Fatalf("want ErrUnauthorized, got %v", err)
		}
	})

	t.Run("roles fallback to viewer", func(t *testing.T) {
		// 创建无角色用户
		u3 := &model.User{Username: "novice", PasswordHash: testHash("Password123"), Status: 1}
		if err := r.User.Create(ctx, u3); err != nil {
			t.Fatalf("create user: %v", err)
		}
		res, err := s.Login(ctx, "novice", "Password123", "")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(res.Roles) != 1 || res.Roles[0] != "viewer" {
			t.Fatalf("roles=%v want [viewer]", res.Roles)
		}
	})

	t.Run("weak password warning", func(t *testing.T) {
		// 使用弱密码创建用户（纯数字）
		u4 := &model.User{Username: "weak", PasswordHash: testHash("12345678"), Status: 1}
		if err := r.User.Create(ctx, u4); err != nil {
			t.Fatalf("create user: %v", err)
		}
		// 弱密码只打日志，不阻止登录
		_, err := s.Login(ctx, "weak", "12345678", "127.0.0.1")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("update last login error ignored", func(t *testing.T) {
		// 正常登录，即使 updateLastLogin 有错误也会被忽略（因为直接操作 DB 不会出错）
		res, err := s.Login(ctx, "alice", "Password123", "127.0.0.1")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if res.User.LastLoginAt == nil {
			t.Fatalf("LastLoginAt should be set")
		}
	})
}

// ==================== Refresh ====================

func TestAuthService_Refresh(t *testing.T) {
	ctx := context.Background()
	s, r := newAuthService(t)

	// 预置用户
	u := &model.User{Username: "alice", PasswordHash: testHash("Password123"), Status: 1, DeptID: 10}
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

	// 签发有效 refresh token
	_, validRefresh, _ := s.jwt.Issue(&jwt.Claims{UserID: u.ID, Username: u.Username, DeptID: u.DeptID, Roles: []string{"admin"}})

	t.Run("normal", func(t *testing.T) {
		res, err := s.Refresh(ctx, validRefresh)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if res.AccessToken == "" {
			t.Fatalf("access_token empty")
		}
		if res.User.Username != "alice" {
			t.Fatalf("username=%s want alice", res.User.Username)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := s.Refresh(ctx, "")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := s.Refresh(ctx, "invalid-token")
		if !isErr(err, errcode.ErrTokenInvalid) {
			t.Fatalf("want ErrTokenInvalid, got %v", err)
		}
	})

	t.Run("access token cannot refresh", func(t *testing.T) {
		access, _, _ := s.jwt.Issue(&jwt.Claims{UserID: u.ID, Username: u.Username})
		_, err := s.Refresh(ctx, access)
		if !isErr(err, errcode.ErrTokenInvalid) {
			t.Fatalf("want ErrTokenInvalid, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		// 用不存在的用户 ID 签发 token
		_, refresh, _ := s.jwt.Issue(&jwt.Claims{UserID: 99999, Username: "ghost"})
		_, err := s.Refresh(ctx, refresh)
		if !isErr(err, errcode.ErrUnauthorized) {
			t.Fatalf("want ErrUnauthorized, got %v", err)
		}
	})

	t.Run("user disabled", func(t *testing.T) {
		// 创建禁用用户并签发 token（Status=0 会被 gorm default:1 覆盖，需显式更新）
		u2 := &model.User{Username: "disabled2", PasswordHash: testHash("x"), Status: 0}
		if err := r.User.Create(ctx, u2); err != nil {
			t.Fatalf("create user: %v", err)
		}
		if err := r.DB.Model(u2).Update("status", 0).Error; err != nil {
			t.Fatalf("update status: %v", err)
		}
		_, refresh, _ := s.jwt.Issue(&jwt.Claims{UserID: u2.ID, Username: u2.Username})
		_, err := s.Refresh(ctx, refresh)
		if !isErr(err, errcode.ErrUnauthorized) {
			t.Fatalf("want ErrUnauthorized, got %v", err)
		}
	})

	t.Run("roles fallback to viewer", func(t *testing.T) {
		// 创建无角色用户并签发 token
		u3 := &model.User{Username: "novice2", PasswordHash: testHash("x"), Status: 1}
		if err := r.User.Create(ctx, u3); err != nil {
			t.Fatalf("create user: %v", err)
		}
		_, refresh, _ := s.jwt.Issue(&jwt.Claims{UserID: u3.ID, Username: u3.Username})
		res, err := s.Refresh(ctx, refresh)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(res.Roles) != 1 || res.Roles[0] != "viewer" {
			t.Fatalf("roles=%v want [viewer]", res.Roles)
		}
	})
}

// ==================== ChangePassword ====================

func TestAuthService_ChangePassword(t *testing.T) {
	ctx := context.Background()
	s, r := newAuthService(t)

	// 预置用户
	u := &model.User{Username: "alice", PasswordHash: testHash("OldPass123"), Status: 1}
	if err := r.User.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.ChangePassword(ctx, u.ID, "OldPass123", "NewPass456"); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// 验证密码已更新（通过登录验证）
		_, err := s.Login(ctx, "alice", "NewPass456", "")
		if err != nil {
			t.Fatalf("login with new password failed: %v", err)
		}
	})

	t.Run("invalid uid", func(t *testing.T) {
		err := s.ChangePassword(ctx, 0, "old", "new")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		err := s.ChangePassword(ctx, 99999, "old", "new")
		if !isErr(err, errcode.ErrUnauthorized) {
			t.Fatalf("want ErrUnauthorized, got %v", err)
		}
	})

	t.Run("new password too short", func(t *testing.T) {
		err := s.ChangePassword(ctx, u.ID, "OldPass123", "123")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("new password same as old", func(t *testing.T) {
		err := s.ChangePassword(ctx, u.ID, "OldPass123", "OldPass123")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("wrong old password", func(t *testing.T) {
		err := s.ChangePassword(ctx, u.ID, "WrongOld123", "NewPass456")
		if !isErr(err, errcode.ErrUnauthorized) {
			t.Fatalf("want ErrUnauthorized, got %v", err)
		}
	})

	t.Run("new password contains username", func(t *testing.T) {
		err := s.ChangePassword(ctx, u.ID, "OldPass123", "Alice1234")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("password weak variety", func(t *testing.T) {
		err := s.ChangePassword(ctx, u.ID, "OldPass123", "password")
		if !isErr(err, errcode.ErrWeakPassword) {
			t.Fatalf("want ErrWeakPassword, got %v", err)
		}
	})
}

func testHash(password string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash)
}
