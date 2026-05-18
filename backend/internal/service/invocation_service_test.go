package service

import (
	"context"
	"testing"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
)

func newInvocationService(t *testing.T) (InvocationService, *repo.Repositories) {
	db := newTestDB(t, &model.ModelInvocation{})
	r := newTestRepos(db)
	return &invocationService{r: r, log: newNopLog()}, r
}

// ==================== Log ====================

func TestInvocationService_Log(t *testing.T) {
	ctx := context.Background()
	s, r := newInvocationService(t)

	t.Run("normal succeeded", func(t *testing.T) {
		ended := time.Now()
		s.Log(ctx, &LogParams{
			ModelID:      1,
			UserID:       2,
			DeptID:       3,
			ProjectID:    4,
			BizType:      "image_gen",
			BizRef:       "image:42",
			InputTokens:  100,
			OutputTokens: 200,
			Units:        4,
			DurationMs:   1500,
			Cost:         0.5,
			Status:       "succeeded",
			ErrorCode:    "",
			StartedAt:    time.Now().Add(-time.Second),
			EndedAt:      &ended,
		})

		list, total, err := r.Invocation.List(ctx, &repo.ListInvocationsQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("list invocations: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		inv := list[0]
		if inv.ModelID != 1 {
			t.Fatalf("model_id=%d want 1", inv.ModelID)
		}
		if inv.UserID != 2 {
			t.Fatalf("user_id=%d want 2", inv.UserID)
		}
		if inv.DeptID != 3 {
			t.Fatalf("dept_id=%d want 3", inv.DeptID)
		}
		if inv.ProjectID != 4 {
			t.Fatalf("project_id=%d want 4", inv.ProjectID)
		}
		if inv.BizType != "image_gen" {
			t.Fatalf("biz_type=%s want image_gen", inv.BizType)
		}
		if inv.BizRef != "image:42" {
			t.Fatalf("biz_ref=%s want image:42", inv.BizRef)
		}
		if inv.InputTokens != 100 {
			t.Fatalf("input_tokens=%d want 100", inv.InputTokens)
		}
		if inv.OutputTokens != 200 {
			t.Fatalf("output_tokens=%d want 200", inv.OutputTokens)
		}
		if inv.Units != 4 {
			t.Fatalf("units=%d want 4", inv.Units)
		}
		if inv.DurationMs != 1500 {
			t.Fatalf("duration_ms=%d want 1500", inv.DurationMs)
		}
		if inv.Cost != 0.5 {
			t.Fatalf("cost=%v want 0.5", inv.Cost)
		}
		if inv.Status != "succeeded" {
			t.Fatalf("status=%s want succeeded", inv.Status)
		}
		if inv.ErrorCode != "" {
			t.Fatalf("error_code should be empty")
		}
		if inv.EndedAt == nil {
			t.Fatalf("ended_at should not be nil")
		}
	})

	t.Run("default status and started_at", func(t *testing.T) {
		s.Log(ctx, &LogParams{
			ModelID: 10,
			UserID:  20,
			BizType: "text_gen",
		})

		list, total, err := r.Invocation.List(ctx, &repo.ListInvocationsQuery{UserID: 20, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("list invocations: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].Status != "succeeded" {
			t.Fatalf("status=%s want succeeded", list[0].Status)
		}
		if list[0].StartedAt.IsZero() {
			t.Fatalf("started_at should not be zero")
		}
	})

	t.Run("failed status", func(t *testing.T) {
		s.Log(ctx, &LogParams{
			ModelID:   5,
			UserID:    6,
			BizType:   "video_gen",
			Status:    "failed",
			ErrorCode: "ERR_TIMEOUT",
		})

		list, total, err := r.Invocation.List(ctx, &repo.ListInvocationsQuery{Status: "failed", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("list invocations: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].Status != "failed" {
			t.Fatalf("status=%s want failed", list[0].Status)
		}
		if list[0].ErrorCode != "ERR_TIMEOUT" {
			t.Fatalf("error_code=%s want ERR_TIMEOUT", list[0].ErrorCode)
		}
	})

	t.Run("nil service or repo", func(t *testing.T) {
		var nilService *invocationService
		// should not panic
		nilService.Log(ctx, &LogParams{ModelID: 1, UserID: 1, BizType: "test"})

		emptyService := &invocationService{r: nil, log: newNopLog()}
		emptyService.Log(ctx, &LogParams{ModelID: 1, UserID: 1, BizType: "test"})
	})
}

// ==================== List ====================

func TestInvocationService_List(t *testing.T) {
	ctx := context.Background()
	s, r := newInvocationService(t)

	// pre-seed invocations
	now := time.Now()
	seeds := []struct {
		modelID   int64
		userID    int64
		deptID    int64
		projectID int64
		bizType   string
		status    string
		cost      float64
	}{
		{1, 1, 1, 1, "image_gen", "succeeded", 0.5},
		{1, 1, 1, 1, "image_gen", "succeeded", 0.6},
		{2, 1, 1, 2, "video_gen", "failed", 0.0},
		{3, 2, 2, 1, "text_gen", "succeeded", 0.1},
	}
	for _, seed := range seeds {
		inv := &model.ModelInvocation{
			ModelID:   seed.modelID,
			UserID:    seed.userID,
			DeptID:    seed.deptID,
			ProjectID: seed.projectID,
			BizType:   seed.bizType,
			Status:    seed.status,
			Cost:      seed.cost,
			StartedAt: now,
		}
		if err := r.Invocation.Create(ctx, inv); err != nil {
			t.Fatalf("seed invocation: %v", err)
		}
	}

	t.Run("normal list", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 4 || len(list) != 4 {
			t.Fatalf("total=%d len=%d want 4,4", total, len(list))
		}
	})

	t.Run("filter by user_id", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{UserID: 1, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 3 {
			t.Fatalf("total=%d len=%d want 3,3", total, len(list))
		}
	})

	t.Run("filter by dept_id", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{DeptID: 2, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].UserID != 2 {
			t.Fatalf("user_id=%d want 2", list[0].UserID)
		}
	})

	t.Run("filter by project_id", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{ProjectID: 2, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].BizType != "video_gen" {
			t.Fatalf("biz_type=%s want video_gen", list[0].BizType)
		}
	})

	t.Run("filter by model_id", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{ModelID: 1, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 2 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 2,2", total, len(list))
		}
	})

	t.Run("filter by biz_type", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{BizType: "image_gen", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 2 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 2,2", total, len(list))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{Status: "failed", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
	})

	t.Run("filter by time range", func(t *testing.T) {
		from := now.Add(-time.Hour)
		to := now.Add(time.Hour)
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{From: &from, To: &to, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 4 || len(list) != 4 {
			t.Fatalf("total=%d len=%d want 4,4", total, len(list))
		}
	})

	t.Run("filter by time range no match", func(t *testing.T) {
		from := now.Add(-48 * time.Hour)
		to := now.Add(-24 * time.Hour)
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{From: &from, To: &to, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 0 || len(list) != 0 {
			t.Fatalf("total=%d len=%d want 0,0", total, len(list))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 4 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 4,2", total, len(list))
		}
		// ordered by id desc
		if list[0].ID < list[1].ID {
			t.Fatalf("should be ordered by id desc")
		}
	})

	t.Run("empty result", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListInvocationsQuery{UserID: 99999, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 0 || len(list) != 0 {
			t.Fatalf("total=%d len=%d want 0,0", total, len(list))
		}
	})

	t.Run("internal error wrapped", func(t *testing.T) {
		db := newTestDB(t, &model.ModelInvocation{})
		badR := newTestRepos(db)
		badS := &invocationService{r: badR, log: newNopLog()}

		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}

		_, _, err := badS.List(ctx, &repo.ListInvocationsQuery{Page: 1, PageSize: 10})
		if !isErr(err, errcode.ErrInternal) {
			t.Fatalf("want ErrInternal, got %v", err)
		}
	})
}

// ==================== Stats ====================

func TestInvocationService_Stats(t *testing.T) {
	ctx := context.Background()
	s, r := newInvocationService(t)

	now := time.Now()
	seeds := []struct {
		modelID      int64
		userID       int64
		deptID       int64
		projectID    int64
		bizType      string
		status       string
		inputTokens  int
		outputTokens int
		units        int
		cost         float64
	}{
		{1, 1, 1, 1, "image_gen", "succeeded", 100, 50, 2, 0.5},
		{1, 1, 1, 1, "image_gen", "succeeded", 200, 100, 4, 1.0},
		{2, 1, 1, 2, "video_gen", "failed", 0, 0, 0, 0.0},
		{3, 2, 2, 1, "text_gen", "succeeded", 50, 25, 1, 0.1},
	}
	for _, seed := range seeds {
		inv := &model.ModelInvocation{
			ModelID:      seed.modelID,
			UserID:       seed.userID,
			DeptID:       seed.deptID,
			ProjectID:    seed.projectID,
			BizType:      seed.bizType,
			Status:       seed.status,
			InputTokens:  seed.inputTokens,
			OutputTokens: seed.outputTokens,
			Units:        seed.units,
			Cost:         seed.cost,
			StartedAt:    now,
		}
		if err := r.Invocation.Create(ctx, inv); err != nil {
			t.Fatalf("seed invocation: %v", err)
		}
	}

	t.Run("stats all", func(t *testing.T) {
		stats, err := s.Stats(ctx, &repo.ListInvocationsQuery{})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if stats.Calls != 4 {
			t.Fatalf("calls=%d want 4", stats.Calls)
		}
		if stats.InputTokens != 350 {
			t.Fatalf("input_tokens=%d want 350", stats.InputTokens)
		}
		if stats.OutputTokens != 175 {
			t.Fatalf("output_tokens=%d want 175", stats.OutputTokens)
		}
		if stats.Units != 7 {
			t.Fatalf("units=%d want 7", stats.Units)
		}
		// cost sum: 0.5 + 1.0 + 0.0 + 0.1 = 1.6
		if stats.Cost != 1.6 {
			t.Fatalf("cost=%v want 1.6", stats.Cost)
		}
	})

	t.Run("stats filter by user_id", func(t *testing.T) {
		stats, err := s.Stats(ctx, &repo.ListInvocationsQuery{UserID: 1})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if stats.Calls != 3 {
			t.Fatalf("calls=%d want 3", stats.Calls)
		}
	})

	t.Run("stats filter by dept_id", func(t *testing.T) {
		stats, err := s.Stats(ctx, &repo.ListInvocationsQuery{DeptID: 2})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if stats.Calls != 1 {
			t.Fatalf("calls=%d want 1", stats.Calls)
		}
		if stats.Cost != 0.1 {
			t.Fatalf("cost=%v want 0.1", stats.Cost)
		}
	})

	t.Run("stats filter by project_id", func(t *testing.T) {
		stats, err := s.Stats(ctx, &repo.ListInvocationsQuery{ProjectID: 2})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if stats.Calls != 1 {
			t.Fatalf("calls=%d want 1", stats.Calls)
		}
		if stats.Cost != 0.0 {
			t.Fatalf("cost=%v want 0.0", stats.Cost)
		}
	})

	t.Run("stats filter by model_id", func(t *testing.T) {
		stats, err := s.Stats(ctx, &repo.ListInvocationsQuery{ModelID: 1})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if stats.Calls != 2 {
			t.Fatalf("calls=%d want 2", stats.Calls)
		}
		if stats.InputTokens != 300 {
			t.Fatalf("input_tokens=%d want 300", stats.InputTokens)
		}
	})

	t.Run("stats filter by biz_type", func(t *testing.T) {
		stats, err := s.Stats(ctx, &repo.ListInvocationsQuery{BizType: "image_gen"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if stats.Calls != 2 {
			t.Fatalf("calls=%d want 2", stats.Calls)
		}
		if stats.Units != 6 {
			t.Fatalf("units=%d want 6", stats.Units)
		}
	})

	t.Run("stats filter by status", func(t *testing.T) {
		stats, err := s.Stats(ctx, &repo.ListInvocationsQuery{Status: "failed"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if stats.Calls != 1 {
			t.Fatalf("calls=%d want 1", stats.Calls)
		}
	})

	t.Run("stats filter by time range", func(t *testing.T) {
		from := now.Add(-time.Hour)
		to := now.Add(time.Hour)
		stats, err := s.Stats(ctx, &repo.ListInvocationsQuery{From: &from, To: &to})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if stats.Calls != 4 {
			t.Fatalf("calls=%d want 4", stats.Calls)
		}
	})

	t.Run("stats filter by time range no match", func(t *testing.T) {
		from := now.Add(-48 * time.Hour)
		to := now.Add(-24 * time.Hour)
		stats, err := s.Stats(ctx, &repo.ListInvocationsQuery{From: &from, To: &to})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if stats.Calls != 0 {
			t.Fatalf("calls=%d want 0", stats.Calls)
		}
		if stats.Cost != 0.0 {
			t.Fatalf("cost=%v want 0.0", stats.Cost)
		}
	})

	t.Run("stats empty", func(t *testing.T) {
		stats, err := s.Stats(ctx, &repo.ListInvocationsQuery{UserID: 99999})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if stats.Calls != 0 {
			t.Fatalf("calls=%d want 0", stats.Calls)
		}
		if stats.Cost != 0.0 {
			t.Fatalf("cost=%v want 0.0", stats.Cost)
		}
	})

	t.Run("internal error wrapped", func(t *testing.T) {
		db := newTestDB(t, &model.ModelInvocation{})
		badR := newTestRepos(db)
		badS := &invocationService{r: badR, log: newNopLog()}

		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}

		_, err := badS.Stats(ctx, &repo.ListInvocationsQuery{})
		if !isErr(err, errcode.ErrInternal) {
			t.Fatalf("want ErrInternal, got %v", err)
		}
	})
}
