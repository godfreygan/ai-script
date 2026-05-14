package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newRoleService(t *testing.T) (RoleService, *repo.Repositories, *gorm.DB) {
	db := newTestDB(t, &model.Role{}, &model.Permission{}, &model.RolePermission{}, &model.User{}, &model.UserRole{})
	r := newTestRepos(db)
	// Casbin enforcer 需要真实实例，但我们可以用 nil 测试 SyncCasbin 的短路逻辑
	return &roleService{role: r.Role, db: db, enforcer: nil, log: zap.NewNop()}, r, db
}

func newRoleServiceWithEnforcer(t *testing.T) (RoleService, *repo.Repositories, *gorm.DB, *casbin.Enforcer) {
	db := newTestDB(t, &model.Role{}, &model.Permission{}, &model.RolePermission{}, &model.User{}, &model.UserRole{})
	r := newTestRepos(db)
	// 创建内存 Casbin enforcer（需要显式 model + file adapter，否则 SavePolicy 会 panic）
	m, err := casbinmodel.NewModelFromString(`
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
`)
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	f, err := os.CreateTemp("", "casbin_*.csv")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	a := fileadapter.NewAdapter(f.Name())
	enforcer, err := casbin.NewEnforcer(m, a)
	if err != nil {
		t.Fatalf("create enforcer: %v", err)
	}
	return &roleService{role: r.Role, db: db, enforcer: enforcer, log: zap.NewNop()}, r, db, enforcer
}

// ==================== List ====================

func TestRoleService_List(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newRoleService(t)

	// 预置角色
	for _, code := range []string{"admin", "user", "guest"} {
		role := &model.Role{Code: code, Name: code, Status: 1}
		if err := r.Role.Create(ctx, role); err != nil {
			t.Fatalf("create role: %v", err)
		}
	}

	t.Run("normal", func(t *testing.T) {
		list, err := s.List(ctx)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(list) != 3 {
			t.Fatalf("len=%d want 3", len(list))
		}
	})
}

// ==================== Get ====================

func TestRoleService_Get(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newRoleService(t)

	// 预置角色 + 权限
	role := &model.Role{Code: "admin", Name: "Admin", Status: 1}
	if err := r.Role.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}
	perm := &model.Permission{Code: "user:read", Resource: "user", Action: "read"}
	if err := r.DB.Create(perm).Error; err != nil {
		t.Fatalf("create permission: %v", err)
	}
	// 注意：RolePermission 需要手动插入，因为 SetRolePermissions 是按 code 查找的
	if err := r.Role.SetRolePermissions(ctx, role.ID, []string{"user:read"}); err != nil {
		t.Fatalf("set permissions: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		got, err := s.Get(ctx, role.ID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.Code != "admin" {
			t.Fatalf("code=%s want admin", got.Code)
		}
		if len(got.Permissions) != 1 || got.Permissions[0] != "user:read" {
			t.Fatalf("permissions=%v want [user:read]", got.Permissions)
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

// ==================== ListPermissions ====================

func TestRoleService_ListPermissions(t *testing.T) {
	ctx := context.Background()
	s, _, db := newRoleService(t)

	// 预置权限
	for _, code := range []string{"user:read", "user:write"} {
		perm := &model.Permission{Code: code, Resource: "user", Action: strings.Split(code, ":")[1]}
		if err := db.Create(perm).Error; err != nil {
			t.Fatalf("create permission: %v", err)
		}
	}

	t.Run("normal", func(t *testing.T) {
		list, err := s.ListPermissions(ctx)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("len=%d want 2", len(list))
		}
	})
}

// ==================== Create ====================

func TestRoleService_Create(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newRoleService(t)

	// 预置权限
	perm := &model.Permission{Code: "user:read", Resource: "user", Action: "read"}
	if err := r.DB.Create(perm).Error; err != nil {
		t.Fatalf("create permission: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		role, err := s.Create(ctx, &CreateRoleInput{
			Code:        "editor",
			Name:        "Editor",
			Description: "Can edit",
			DataScope:   "dept",
			Permissions: []string{"user:read"},
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if role.Code != "editor" {
			t.Fatalf("code=%s want editor", role.Code)
		}
		if role.DataScope != "dept" {
			t.Fatalf("data_scope=%s want dept", role.DataScope)
		}
		// 验证权限已绑定
		perms, _ := r.Role.GetRolePermissionCodes(ctx, role.ID)
		if len(perms) != 1 || perms[0] != "user:read" {
			t.Fatalf("permissions=%v want [user:read]", perms)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		_, err := s.Create(ctx, nil)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("empty code", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateRoleInput{Code: "  ", Name: "Name"})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateRoleInput{Code: "code", Name: "  "})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("invalid data scope", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateRoleInput{Code: "code", Name: "Name", DataScope: "invalid"})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("default data scope", func(t *testing.T) {
		role, err := s.Create(ctx, &CreateRoleInput{Code: "viewer", Name: "Viewer"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if role.DataScope != "self" {
			t.Fatalf("data_scope=%s want self", role.DataScope)
		}
	})

	t.Run("duplicate code", func(t *testing.T) {
		_, _ = s.Create(ctx, &CreateRoleInput{Code: "dup", Name: "Dup"})
		_, err := s.Create(ctx, &CreateRoleInput{Code: "dup", Name: "Dup2"})
		if !isErr(err, errcode.ErrConflict) {
			t.Fatalf("want ErrConflict, got %v", err)
		}
	})

	t.Run("super long description", func(t *testing.T) {
		longDesc := strings.Repeat("a", 1000)
		role, err := s.Create(ctx, &CreateRoleInput{
			Code:        "longdesc",
			Name:        "Long",
			Description: longDesc,
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if role.Description != longDesc {
			t.Fatalf("description mismatch")
		}
	})
}

// ==================== Update ====================

func TestRoleService_Update(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newRoleService(t)

	// 预置角色
	role := &model.Role{Code: "editor", Name: "Editor", DataScope: "self", Status: 1}
	if err := r.Role.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		newName := "Senior Editor"
		newScope := "all"
		newStatus := int8(0)
		updated, err := s.Update(ctx, role.ID, &UpdateRoleInput{
			Name:      &newName,
			DataScope: &newScope,
			Status:    &newStatus,
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Name != "Senior Editor" {
			t.Fatalf("name=%s want Senior Editor", updated.Name)
		}
		if updated.DataScope != "all" {
			t.Fatalf("data_scope=%s want all", updated.DataScope)
		}
		if updated.Status != 0 {
			t.Fatalf("status=%d want 0", updated.Status)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		_, err := s.Update(ctx, 0, &UpdateRoleInput{})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		_, err := s.Update(ctx, role.ID, nil)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := s.Update(ctx, 99999, &UpdateRoleInput{})
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		emptyName := "  "
		_, err := s.Update(ctx, role.ID, &UpdateRoleInput{Name: &emptyName})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("invalid data scope", func(t *testing.T) {
		badScope := "invalid"
		_, err := s.Update(ctx, role.ID, &UpdateRoleInput{DataScope: &badScope})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("update permissions", func(t *testing.T) {
		// 预置权限
		perm := &model.Permission{Code: "user:write", Resource: "user", Action: "write"}
		if err := r.DB.Create(perm).Error; err != nil {
			t.Fatalf("create permission: %v", err)
		}
		perms := []string{"user:write"}
		_, err := s.Update(ctx, role.ID, &UpdateRoleInput{Permissions: &perms})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// 验证权限更新
		gotPerms, _ := r.Role.GetRolePermissionCodes(ctx, role.ID)
		if len(gotPerms) != 1 || gotPerms[0] != "user:write" {
			t.Fatalf("permissions=%v want [user:write]", gotPerms)
		}
	})
}

// ==================== Delete ====================

func TestRoleService_Delete(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newRoleService(t)

	// 预置角色
	role := &model.Role{Code: "todelete", Name: "ToDelete", Status: 1}
	if err := r.Role.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.Delete(ctx, role.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// 验证已删除（repo 层返回原始 gorm 错误）
		_, err := r.Role.Get(ctx, role.ID)
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("role should be deleted, got err=%v", err)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		err := s.Delete(ctx, 0)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := s.Delete(ctx, 99999)
		if err == nil {
			t.Fatalf("want error, got nil")
		}
	})

	t.Run("role has users", func(t *testing.T) {
		// 创建角色
		role2 := &model.Role{Code: "hasusers", Name: "HasUsers", Status: 1}
		if err := r.Role.Create(ctx, role2); err != nil {
			t.Fatalf("create role: %v", err)
		}
		// 创建用户并绑定角色
		u := &model.User{Username: "user1", Status: 1}
		if err := r.User.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
		if err := r.User.SetRoles(ctx, u.ID, []int64{role2.ID}); err != nil {
			t.Fatalf("set roles: %v", err)
		}

		err := s.Delete(ctx, role2.ID)
		if !isErr(err, errcode.ErrConflict) {
			t.Fatalf("want ErrConflict, got %v", err)
		}
	})

	t.Run("system role cannot be deleted", func(t *testing.T) {
		// 创建系统角色
		role3 := &model.Role{Code: "system", Name: "System", IsSystem: 1, Status: 1}
		if err := r.Role.Create(ctx, role3); err != nil {
			t.Fatalf("create role: %v", err)
		}

		err := s.Delete(ctx, role3.ID)
		if !isErr(err, errcode.ErrConflict) {
			t.Fatalf("want ErrConflict, got %v", err)
		}
	})
}

// ==================== SyncCasbin ====================

func TestRoleService_SyncCasbin(t *testing.T) {
	ctx := context.Background()

	t.Run("nil enforcer", func(t *testing.T) {
		s, _, _ := newRoleService(t)
		if err := s.SyncCasbin(ctx); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("normal with enforcer", func(t *testing.T) {
		s, r, _, enforcer := newRoleServiceWithEnforcer(t)

		// 预置角色 + 权限
		role := &model.Role{Code: "admin", Name: "Admin", Status: 1}
		if err := r.Role.Create(ctx, role); err != nil {
			t.Fatalf("create role: %v", err)
		}
		perm1 := &model.Permission{Code: "user:read", Resource: "user", Action: "read"}
		if err := r.DB.Create(perm1).Error; err != nil {
			t.Fatalf("create permission: %v", err)
		}
		perm2 := &model.Permission{Code: "user:write", Resource: "user", Action: "write"}
		if err := r.DB.Create(perm2).Error; err != nil {
			t.Fatalf("create permission: %v", err)
		}
		if err := r.Role.SetRolePermissions(ctx, role.ID, []string{"user:read", "user:write"}); err != nil {
			t.Fatalf("set permissions: %v", err)
		}

		if err := s.SyncCasbin(ctx); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		// 验证 Casbin 策略
		policies, err := enforcer.GetPolicy()
		if err != nil {
			t.Fatalf("get policy: %v", err)
		}
		if len(policies) != 2 {
			t.Fatalf("policies=%d want 2", len(policies))
		}
	})
}

// ==================== splitResAct ====================

func TestSplitResAct(t *testing.T) {
	cases := []struct {
		in         string
		wantRes    string
		wantAction string
	}{
		{"user|read", "user", "read"},
		{"user|write", "user", "write"},
		{"user", "user", ""},
		{"", "", ""},
		{"a|b|c", "a", "b|c"},
	}
	for _, c := range cases {
		res, act := splitResAct(c.in)
		if res != c.wantRes || act != c.wantAction {
			t.Errorf("splitResAct(%q)=(%q,%q) want (%q,%q)", c.in, res, act, c.wantRes, c.wantAction)
		}
	}
}

// ==================== validDataScope ====================

func TestValidDataScope(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"self", true},
		{"dept", true},
		{"all", true},
		{"", false},
		{"invalid", false},
		{"SELF", false},
	}
	for _, c := range cases {
		if got := validDataScope(c.in); got != c.want {
			t.Errorf("validDataScope(%q)=%v want %v", c.in, got, c.want)
		}
	}
}
