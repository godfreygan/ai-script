package service

import (
	"context"
	"testing"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
)

func newAuditService(t *testing.T) (AuditService, *repo.Repositories) {
	db := newTestDB(t, &model.AuditLog{})
	r := newTestRepos(db)
	return &auditService{r: r, log: newNopLog()}, r
}

// ==================== Log ====================

func TestAuditService_Log(t *testing.T) {
	ctx := context.Background()
	s, r := newAuditService(t)

	t.Run("normal with before and after", func(t *testing.T) {
		before := map[string]any{"name": "old"}
		after := map[string]any{"name": "new"}

		s.Log(ctx, &LogAuditParams{
			UserID:       1,
			Action:       "update",
			ResourceType: "user",
			ResourceID:   "42",
			Before:       before,
			After:        after,
			IP:           "127.0.0.1",
			UA:           "Mozilla/5.0",
			RequestID:    "req-123",
		})

		// verify persisted
		list, total, err := r.Audit.List(ctx, &repo.ListAuditQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("list audit logs: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		entry := list[0]
		if entry.UserID != 1 {
			t.Fatalf("user_id=%d want 1", entry.UserID)
		}
		if entry.Action != "update" {
			t.Fatalf("action=%s want update", entry.Action)
		}
		if entry.ResourceType != "user" {
			t.Fatalf("resource_type=%s want user", entry.ResourceType)
		}
		if entry.ResourceID != "42" {
			t.Fatalf("resource_id=%s want 42", entry.ResourceID)
		}
		if entry.IP != "127.0.0.1" {
			t.Fatalf("ip=%s want 127.0.0.1", entry.IP)
		}
		if entry.UA != "Mozilla/5.0" {
			t.Fatalf("ua=%s want Mozilla/5.0", entry.UA)
		}
		if entry.RequestID != "req-123" {
			t.Fatalf("request_id=%s want req-123", entry.RequestID)
		}
		if entry.Before == nil {
			t.Fatalf("before should not be nil")
		}
		if entry.After == nil {
			t.Fatalf("after should not be nil")
		}
	})

	t.Run("nil params", func(t *testing.T) {
		// should not panic and not write anything
		s.Log(ctx, nil)
		list, total, err := r.Audit.List(ctx, &repo.ListAuditQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("list audit logs: %v", err)
		}
		// previous subtest wrote 1 record
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
	})

	t.Run("no before after", func(t *testing.T) {
		s.Log(ctx, &LogAuditParams{
			UserID:       2,
			Action:       "delete",
			ResourceType: "project",
			ResourceID:   "99",
		})

		list, total, err := r.Audit.List(ctx, &repo.ListAuditQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("list audit logs: %v", err)
		}
		if total != 2 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 2,2", total, len(list))
		}
		// ordered by id desc, so the newest is first
		if list[0].UserID != 2 {
			t.Fatalf("user_id=%d want 2", list[0].UserID)
		}
		if list[0].Before != nil {
			t.Fatalf("before should be nil")
		}
		if list[0].After != nil {
			t.Fatalf("after should be nil")
		}
	})

	t.Run("unmarshalable before", func(t *testing.T) {
		// channel cannot be marshaled to JSON
		s.Log(ctx, &LogAuditParams{
			UserID:       3,
			Action:       "create",
			ResourceType: "test",
			ResourceID:   "1",
			Before:       make(chan int),
		})

		// should still persist with nil Before
		list, total, err := r.Audit.List(ctx, &repo.ListAuditQuery{UserID: 3, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("list audit logs: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].Before != nil {
			t.Fatalf("before should be nil for unmarshalable value")
		}
	})

	t.Run("unmarshalable after", func(t *testing.T) {
		// channel cannot be marshaled to JSON
		s.Log(ctx, &LogAuditParams{
			UserID:       4,
			Action:       "create",
			ResourceType: "test",
			ResourceID:   "1",
			After:        make(chan int),
		})

		// should still persist with nil After
		list, total, err := r.Audit.List(ctx, &repo.ListAuditQuery{UserID: 4, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("list audit logs: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].After != nil {
			t.Fatalf("after should be nil for unmarshalable value")
		}
	})

	t.Run("nil service or repo", func(t *testing.T) {
		// AuditService.Log does NOT have nil guard for s == nil or s.r == nil,
		// so calling on nil receiver or nil repo panics.
		// This test documents the current behavior.
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic for nil repo")
			}
		}()
		emptyService := &auditService{r: nil, log: newNopLog()}
		emptyService.Log(ctx, &LogAuditParams{UserID: 1, Action: "test"})
	})
}

// ==================== List ====================

func TestAuditService_List(t *testing.T) {
	ctx := context.Background()
	s, r := newAuditService(t)

	// pre-seed audit logs
	actions := []string{"create", "update", "delete", "update"}
	resourceTypes := []string{"user", "user", "project", "role"}
	userIDs := []int64{1, 1, 2, 1}
	for i := 0; i < 4; i++ {
		entry := &model.AuditLog{
			UserID:       userIDs[i],
			Action:       actions[i],
			ResourceType: resourceTypes[i],
			ResourceID:   "res-1",
			CreatedAt:    time.Now(),
		}
		if err := r.Audit.Create(ctx, entry); err != nil {
			t.Fatalf("seed audit log: %v", err)
		}
	}

	t.Run("normal list", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListAuditQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 4 || len(list) != 4 {
			t.Fatalf("total=%d len=%d want 4,4", total, len(list))
		}
	})

	t.Run("filter by user_id", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListAuditQuery{UserID: 1, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 3 {
			t.Fatalf("total=%d len=%d want 3,3", total, len(list))
		}
	})

	t.Run("filter by resource_type", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListAuditQuery{ResourceType: "project", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].Action != "delete" {
			t.Fatalf("action=%s want delete", list[0].Action)
		}
	})

	t.Run("filter by action", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListAuditQuery{Action: "update", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 2 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 2,2", total, len(list))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListAuditQuery{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 4 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 4,2", total, len(list))
		}
		// ordered by id desc, so first page should have ids 4 and 3
		if list[0].ID < list[1].ID {
			t.Fatalf("should be ordered by id desc")
		}
	})

	t.Run("empty result", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListAuditQuery{UserID: 99999, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 0 || len(list) != 0 {
			t.Fatalf("total=%d len=%d want 0,0", total, len(list))
		}
	})

	t.Run("internal error wrapped", func(t *testing.T) {
		// use a closed/bad db to trigger error
		db := newTestDB(t, &model.AuditLog{})
		badR := newTestRepos(db)
		badS := &auditService{r: badR, log: newNopLog()}

		// close underlying sql db to cause errors
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}

		_, _, err := badS.List(ctx, &repo.ListAuditQuery{Page: 1, PageSize: 10})
		if !isErr(err, errcode.ErrInternal) {
			t.Fatalf("want ErrInternal, got %v", err)
		}
	})
}
