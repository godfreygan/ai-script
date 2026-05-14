package service

import (
	"context"
	"testing"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
)

func newDeptService(t *testing.T) (DeptService, *repo.Repositories) {
	db := newTestDB(t, &model.Department{}, &model.User{})
	r := newTestRepos(db)
	return &deptService{dept: r.Dept, log: newNopLog()}, r
}

// ==================== List ====================

func TestDeptService_List(t *testing.T) {
	ctx := context.Background()
	s, r := newDeptService(t)

	// 预置部门
	for _, name := range []string{"Engineering", "Sales", "Marketing"} {
		d := &model.Department{Name: name, Status: 1}
		if err := r.Dept.Create(ctx, d); err != nil {
			t.Fatalf("create dept: %v", err)
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

func TestDeptService_Get(t *testing.T) {
	ctx := context.Background()
	s, r := newDeptService(t)

	// 预置部门
	d := &model.Department{Name: "Engineering", Status: 1}
	if err := r.Dept.Create(ctx, d); err != nil {
		t.Fatalf("create dept: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		got, err := s.Get(ctx, d.ID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.Name != "Engineering" {
			t.Fatalf("name=%s want Engineering", got.Name)
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

func TestDeptService_Create(t *testing.T) {
	ctx := context.Background()
	s, r := newDeptService(t)

	t.Run("normal without parent", func(t *testing.T) {
		d, err := s.Create(ctx, &CreateDeptInput{Name: "Engineering", ParentID: 0, Sort: 1})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if d.Name != "Engineering" {
			t.Fatalf("name=%s want Engineering", d.Name)
		}
		wantPath := "/" + itoa64(d.ID)
		if d.Path != wantPath {
			t.Fatalf("path=%s want %s", d.Path, wantPath)
		}
	})

	t.Run("normal with parent", func(t *testing.T) {
		// 先创建父部门
		parent := &model.Department{Name: "Root", Path: "/1", Status: 1}
		if err := r.Dept.Create(ctx, parent); err != nil {
			t.Fatalf("create parent: %v", err)
		}

		d, err := s.Create(ctx, &CreateDeptInput{Name: "Sub", ParentID: parent.ID, Sort: 2})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		wantPath := parent.Path + "/" + itoa64(d.ID)
		if d.Path != wantPath {
			t.Fatalf("path=%s want %s", d.Path, wantPath)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		_, err := s.Create(ctx, nil)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateDeptInput{Name: "   "})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("parent not found", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateDeptInput{Name: "Sub", ParentID: 99999})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})
}

// ==================== Update ====================

func TestDeptService_Update(t *testing.T) {
	ctx := context.Background()
	s, r := newDeptService(t)

	// 预置部门
	d := &model.Department{Name: "Old", Sort: 1, Status: 1}
	if err := r.Dept.Create(ctx, d); err != nil {
		t.Fatalf("create dept: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		updated, err := s.Update(ctx, d.ID, "New", 2, 0)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Name != "New" {
			t.Fatalf("name=%s want New", updated.Name)
		}
		if updated.Sort != 2 {
			t.Fatalf("sort=%d want 2", updated.Sort)
		}
		// status=0 在 Update 语义中表示"不修改"，因此保持原值 1
		if updated.Status != 1 {
			t.Fatalf("status=%d want 1 (0 means no-change)", updated.Status)
		}
	})

	t.Run("update only name", func(t *testing.T) {
		updated, err := s.Update(ctx, d.ID, "OnlyName", 0, 0)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Name != "OnlyName" {
			t.Fatalf("name=%s want OnlyName", updated.Name)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		_, err := s.Update(ctx, 0, "New", 0, 0)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := s.Update(ctx, 99999, "New", 0, 0)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// ==================== Delete ====================

func TestDeptService_Delete(t *testing.T) {
	ctx := context.Background()
	s, r := newDeptService(t)

	// 预置部门
	d := &model.Department{Name: "ToDelete", Status: 1}
	if err := r.Dept.Create(ctx, d); err != nil {
		t.Fatalf("create dept: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.Delete(ctx, d.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// 验证已删除
		_, err := r.Dept.Get(ctx, d.ID)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("dept should be deleted")
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
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("dept has users", func(t *testing.T) {
		// 创建部门
		d2 := &model.Department{Name: "HasUsers", Status: 1}
		if err := r.Dept.Create(ctx, d2); err != nil {
			t.Fatalf("create dept: %v", err)
		}
		// 创建关联用户
		u := &model.User{Username: "user1", DeptID: d2.ID, Status: 1}
		if err := r.User.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}

		err := s.Delete(ctx, d2.ID)
		if !isErr(err, errcode.ErrConflict) {
			t.Fatalf("want ErrConflict, got %v", err)
		}
	})
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
