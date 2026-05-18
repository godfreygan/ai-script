package service

import (
	"context"
	"testing"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
)

func newProjectService(t *testing.T) (ProjectService, *repo.Repositories) {
	db := newTestDB(t, &model.Project{}, &model.ProjectMember{})
	r := newTestRepos(db)
	return &projectService{project: r.Project, log: newNopLog()}, r
}

// ==================== List ====================

func TestProjectService_List(t *testing.T) {
	ctx := context.Background()
	s, r := newProjectService(t)

	// 预置项目
	projects := []*model.Project{
		{Code: "p1", Name: "Alpha", OwnerID: 1, DeptID: 10, Status: 1},
		{Code: "p2", Name: "Beta", OwnerID: 2, DeptID: 20, Status: 1},
		{Code: "p3", Name: "Gamma", OwnerID: 1, DeptID: 10, Status: 0},
	}
	for _, p := range projects {
		if err := r.Project.Create(ctx, p); err != nil {
			t.Fatalf("create project: %v", err)
		}
	}

	t.Run("normal", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListProjectsQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 3 {
			t.Fatalf("total=%d len=%d want 3,3", total, len(list))
		}
	})

	t.Run("query filter by name", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListProjectsQuery{Q: "Alp", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].Name != "Alpha" {
			t.Fatalf("name=%s want Alpha", list[0].Name)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListProjectsQuery{Status: 0, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// repo 层 Status 过滤条件是 q.Status != 0 才生效，所以传入 0 时不过滤，返回全部 3 条
		if total != 3 || len(list) != 3 {
			t.Fatalf("total=%d len=%d want 3,3 (status=0 filter not applied in repo)", total, len(list))
		}
	})

	t.Run("filter by dept_id", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListProjectsQuery{DeptID: 20, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].Name != "Beta" {
			t.Fatalf("name=%s want Beta", list[0].Name)
		}
	})

	t.Run("filter by owner_id", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListProjectsQuery{OwnerID: 1, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 2 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 2,2", total, len(list))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListProjectsQuery{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 3,2", total, len(list))
		}
	})
}

// ==================== Create ====================

func TestProjectService_Create(t *testing.T) {
	ctx := context.Background()
	s, _ := newProjectService(t)

	t.Run("normal", func(t *testing.T) {
		p, err := s.Create(ctx, &CreateProjectInput{
			Code:              "proj-a",
			Name:              "Project A",
			Description:       "desc",
			DeptID:            10,
			DefaultPipelineID: 5,
			CoverURL:          "http://cover.jpg",
			Tags:              []string{"t1", "t2"},
		}, 100, 10)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.Code != "proj-a" {
			t.Fatalf("code=%s want proj-a", p.Code)
		}
		if p.Name != "Project A" {
			t.Fatalf("name=%s want Project A", p.Name)
		}
		if p.OwnerID != 100 {
			t.Fatalf("owner_id=%d want 100", p.OwnerID)
		}
		if p.DeptID != 10 {
			t.Fatalf("dept_id=%d want 10", p.DeptID)
		}
		if p.Status != 1 {
			t.Fatalf("status=%d want 1", p.Status)
		}
		if p.DefaultPipelineID != 5 {
			t.Fatalf("default_pipeline_id=%d want 5", p.DefaultPipelineID)
		}
		if p.CoverURL != "http://cover.jpg" {
			t.Fatalf("cover_url=%s want http://cover.jpg", p.CoverURL)
		}
		if p.CreatedBy != 100 || p.UpdatedBy != 100 {
			t.Fatalf("created_by/updated_by mismatch")
		}
	})

	t.Run("default dept_id from param", func(t *testing.T) {
		p, err := s.Create(ctx, &CreateProjectInput{
			Code: "proj-b",
			Name: "Project B",
		}, 200, 99)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.DeptID != 99 {
			t.Fatalf("dept_id=%d want 99", p.DeptID)
		}
	})

	t.Run("duplicate code", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateProjectInput{
			Code: "proj-a",
			Name: "Duplicate",
		}, 1, 1)
		if err == nil {
			t.Fatalf("expected error for duplicate code")
		}
	})
}

// ==================== Get ====================

func TestProjectService_Get(t *testing.T) {
	ctx := context.Background()
	s, r := newProjectService(t)

	p := &model.Project{Code: "get-test", Name: "GetTest", OwnerID: 1, Status: 1}
	if err := r.Project.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		got, err := s.Get(ctx, p.ID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.ID != p.ID {
			t.Fatalf("id=%d want %d", got.ID, p.ID)
		}
		if got.Name != "GetTest" {
			t.Fatalf("name=%s want GetTest", got.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := s.Get(ctx, 99999)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// ==================== Update ====================

func TestProjectService_Update(t *testing.T) {
	ctx := context.Background()
	s, r := newProjectService(t)

	p := &model.Project{Code: "upd-test", Name: "Old", Description: "old desc", OwnerID: 1, Status: 1, DefaultPipelineID: 1, CoverURL: "old.jpg"}
	if err := r.Project.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	t.Run("normal full update", func(t *testing.T) {
		newName := "New"
		newDesc := "new desc"
		newStatus := int8(0)
		newPipeline := int64(99)
		newCover := "new.jpg"
		updated, err := s.Update(ctx, p.ID, &UpdateProjectInput{
			Name:              &newName,
			Description:       &newDesc,
			Status:            &newStatus,
			DefaultPipelineID: &newPipeline,
			CoverURL:          &newCover,
		}, 200)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Name != "New" {
			t.Fatalf("name=%s want New", updated.Name)
		}
		if updated.Description != "new desc" {
			t.Fatalf("description=%s want 'new desc'", updated.Description)
		}
		if updated.Status != 0 {
			t.Fatalf("status=%d want 0", updated.Status)
		}
		if updated.DefaultPipelineID != 99 {
			t.Fatalf("default_pipeline_id=%d want 99", updated.DefaultPipelineID)
		}
		if updated.CoverURL != "new.jpg" {
			t.Fatalf("cover_url=%s want new.jpg", updated.CoverURL)
		}
		if updated.UpdatedBy != 200 {
			t.Fatalf("updated_by=%d want 200", updated.UpdatedBy)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := s.Update(ctx, 99999, &UpdateProjectInput{}, 1)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("partial update name only", func(t *testing.T) {
		partialName := "Partial"
		updated, err := s.Update(ctx, p.ID, &UpdateProjectInput{Name: &partialName}, 1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Name != "Partial" {
			t.Fatalf("name=%s want Partial", updated.Name)
		}
	})

	t.Run("partial update status only", func(t *testing.T) {
		newStatus := int8(1)
		updated, err := s.Update(ctx, p.ID, &UpdateProjectInput{Status: &newStatus}, 1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Status != 1 {
			t.Fatalf("status=%d want 1", updated.Status)
		}
	})
}

// ==================== Delete ====================

func TestProjectService_Delete(t *testing.T) {
	ctx := context.Background()
	s, r := newProjectService(t)

	p := &model.Project{Code: "del-test", Name: "DelTest", OwnerID: 1, Status: 1}
	if err := r.Project.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.Delete(ctx, p.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		_, err := r.Project.GetByID(ctx, p.ID)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := s.Delete(ctx, 99999)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

// ==================== ListMembers ====================

func TestProjectService_ListMembers(t *testing.T) {
	ctx := context.Background()
	s, r := newProjectService(t)

	p := &model.Project{Code: "lm-test", Name: "LMTest", OwnerID: 1, Status: 1}
	if err := r.Project.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	members := []model.ProjectMember{
		{ProjectID: p.ID, UserID: 10, RoleInProject: "editor"},
		{ProjectID: p.ID, UserID: 20, RoleInProject: "viewer"},
	}
	for _, m := range members {
		if err := r.Project.AddMember(ctx, &m); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}

	t.Run("normal", func(t *testing.T) {
		list, err := s.ListMembers(ctx, p.ID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("len=%d want 2", len(list))
		}
	})

	t.Run("empty", func(t *testing.T) {
		p2 := &model.Project{Code: "lm-empty", Name: "LMEmpty", OwnerID: 1, Status: 1}
		if err := r.Project.Create(ctx, p2); err != nil {
			t.Fatalf("create project: %v", err)
		}
		list, err := s.ListMembers(ctx, p2.ID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(list) != 0 {
			t.Fatalf("len=%d want 0", len(list))
		}
	})
}

// ==================== AddMember ====================

func TestProjectService_AddMember(t *testing.T) {
	ctx := context.Background()
	s, r := newProjectService(t)

	p := &model.Project{Code: "am-test", Name: "AMTest", OwnerID: 1, Status: 1}
	if err := r.Project.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.AddMember(ctx, p.ID, 100, "editor"); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		exists, err := r.Project.IsMember(ctx, p.ID, 100)
		if err != nil {
			t.Fatalf("check member: %v", err)
		}
		if !exists {
			t.Fatalf("member should exist")
		}
	})

	t.Run("default role", func(t *testing.T) {
		if err := s.AddMember(ctx, p.ID, 101, ""); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		list, err := r.Project.ListMembers(ctx, p.ID)
		if err != nil {
			t.Fatalf("list members: %v", err)
		}
		for _, m := range list {
			if m.UserID == 101 && m.RoleInProject != "editor" {
				t.Fatalf("role=%s want editor", m.RoleInProject)
			}
		}
	})

	t.Run("invalid project id", func(t *testing.T) {
		err := s.AddMember(ctx, 0, 100, "editor")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("invalid user id", func(t *testing.T) {
		err := s.AddMember(ctx, p.ID, 0, "editor")
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("project not found", func(t *testing.T) {
		err := s.AddMember(ctx, 99999, 100, "editor")
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("idempotent add", func(t *testing.T) {
		if err := s.AddMember(ctx, p.ID, 200, "editor"); err != nil {
			t.Fatalf("first add: %v", err)
		}
		if err := s.AddMember(ctx, p.ID, 200, "viewer"); err != nil {
			t.Fatalf("second add should be idempotent: %v", err)
		}
		list, err := r.Project.ListMembers(ctx, p.ID)
		if err != nil {
			t.Fatalf("list members: %v", err)
		}
		count := 0
		for _, m := range list {
			if m.UserID == 200 {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("duplicate member count=%d want 1", count)
		}
	})
}

// ==================== RemoveMember ====================

func TestProjectService_RemoveMember(t *testing.T) {
	ctx := context.Background()
	s, r := newProjectService(t)

	p := &model.Project{Code: "rm-test", Name: "RMTest", OwnerID: 1, Status: 1}
	if err := r.Project.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := r.Project.AddMember(ctx, &model.ProjectMember{ProjectID: p.ID, UserID: 100, RoleInProject: "editor"}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.RemoveMember(ctx, p.ID, 100); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		exists, err := r.Project.IsMember(ctx, p.ID, 100)
		if err != nil {
			t.Fatalf("check member: %v", err)
		}
		if exists {
			t.Fatalf("member should be removed")
		}
	})

	t.Run("invalid project id", func(t *testing.T) {
		err := s.RemoveMember(ctx, 0, 100)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("invalid user id", func(t *testing.T) {
		err := s.RemoveMember(ctx, p.ID, 0)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("remove non-existent member", func(t *testing.T) {
		err := s.RemoveMember(ctx, p.ID, 99999)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}
