package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/queue"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ===================== Helpers =====================

func newPipelineTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return newTestDB(t,
		&model.Pipeline{}, &model.PipelineRun{}, &model.StepRun{},
		&model.Project{},
	)
}

func mustCreatePipeline(t *testing.T, db *gorm.DB, p *model.Pipeline) {
	t.Helper()
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create pipeline: %v", err)
	}
}

func mustCreatePipelineRun(t *testing.T, db *gorm.DB, r *model.PipelineRun) {
	t.Helper()
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("create pipeline run: %v", err)
	}
}

func mustCreateStepRun(t *testing.T, db *gorm.DB, s *model.StepRun) {
	t.Helper()
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("create step run: %v", err)
	}
}

// ===================== PipelineService CRUD Tests =====================

func TestPipelineService_Create(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	p, err := svc.Create(ctx, &CreatePipelineInput{
		ProjectID:   1,
		Name:        "test-pipe",
		Description: "desc",
		DAG:         json.RawMessage(`{"nodes":[{"id":"n1","type":"noop"}]}`),
		IsTemplate:  0,
		Enabled:     1,
	}, 100)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("pipeline id should not be zero")
	}
	if p.Name != "test-pipe" {
		t.Errorf("name=%s want test-pipe", p.Name)
	}
	if p.Enabled != 1 {
		t.Errorf("enabled=%d want 1", p.Enabled)
	}
	if p.CreatedBy != 100 {
		t.Errorf("created_by=%d want 100", p.CreatedBy)
	}
}

func TestPipelineService_Create_DefaultEnabled(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	p, err := svc.Create(ctx, &CreatePipelineInput{
		ProjectID: 1,
		Name:      "pipe2",
		DAG:       json.RawMessage(`{"nodes":[{"id":"n1","type":"noop"}]}`),
		Enabled:   0, // zero value; service should default to 1
	}, 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Enabled != 1 {
		t.Errorf("enabled=%d want 1 (default)", p.Enabled)
	}
}

func TestPipelineService_Get(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	mustCreatePipeline(t, db, &model.Pipeline{Base: model.Base{ID: 1}, ProjectID: 1, Name: "p1", DAG: model.JSON(`{}`), Enabled: 1})

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	p, err := svc.Get(ctx, 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if p.Name != "p1" {
		t.Errorf("name=%s want p1", p.Name)
	}
}

func TestPipelineService_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	_, err := svc.Get(ctx, 999)
	if err == nil {
		t.Fatal("expected not found")
	}
	if !errors.Is(err, errcode.ErrNotFound) {
		t.Errorf("err=%v want ErrNotFound", err)
	}
}

func TestPipelineService_List(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	mustCreatePipeline(t, db, &model.Pipeline{Base: model.Base{ID: 1}, ProjectID: 1, Name: "p1", DAG: model.JSON(`{}`), Enabled: 1})
	mustCreatePipeline(t, db, &model.Pipeline{Base: model.Base{ID: 2}, ProjectID: 1, Name: "p2", DAG: model.JSON(`{}`), Enabled: 1})

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	list, total, err := svc.List(ctx, &repo.ListPipelinesQuery{Page: 1, PageSize: 10, ProjectID: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total=%d want 2", total)
	}
	if len(list) != 2 {
		t.Errorf("len(list)=%d want 2", len(list))
	}
}

func TestPipelineService_Update(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	mustCreatePipeline(t, db, &model.Pipeline{Base: model.Base{ID: 1}, ProjectID: 1, Name: "old", DAG: model.JSON(`{}`), Enabled: 1})

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	newName := "new"
	p, err := svc.Update(ctx, 1, &UpdatePipelineInput{Name: &newName})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if p.Name != "new" {
		t.Errorf("name=%s want new", p.Name)
	}
}

func TestPipelineService_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	newName := "new"
	_, err := svc.Update(ctx, 999, &UpdatePipelineInput{Name: &newName})
	if err == nil {
		t.Fatal("expected not found")
	}
	if !errors.Is(err, errcode.ErrNotFound) {
		t.Errorf("err=%v want ErrNotFound", err)
	}
}

func TestPipelineService_Delete(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	mustCreatePipeline(t, db, &model.Pipeline{Base: model.Base{ID: 1}, ProjectID: 1, Name: "p1", DAG: model.JSON(`{}`), Enabled: 1})

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	if err := svc.Delete(ctx, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var count int64
	db.Model(&model.Pipeline{}).Where("id = ?", 1).Count(&count)
	if count != 0 {
		t.Errorf("pipeline still exists after delete")
	}
}

// ===================== PipelineService Run Tests =====================

func TestPipelineService_Run_EnqueueFailure(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	mustCreatePipeline(t, db, &model.Pipeline{Base: model.Base{ID: 1}, ProjectID: 1, Name: "p1", DAG: model.JSON(`{"nodes":[]}`), Enabled: 1})

	// Use a real queue client pointing to an invalid address; Enqueue will fail with Redis error.
	// Since we cannot mock *queue.Client (concrete type) without modifying production code,
	// we verify the transaction rollback behavior when enqueue fails.
	tc := queue.NewClient("127.0.0.1:1", "", 0)

	svc := &pipelineService{r: r, db: db, tc: tc, hub: nil, registry: nil, log: zap.NewNop()}
	_, err := svc.Run(ctx, 1, map[string]any{"key": "val"}, map[string]any{})
	if err == nil {
		t.Fatal("expected error because Redis is unavailable")
	}

	// Transaction rolls back when enqueue fails, so no run record should be committed
	var count int64
	db.Model(&model.PipelineRun{}).Where("pipeline_id = ?", 1).Count(&count)
	if count != 0 {
		t.Errorf("expected no run record after rollback, got %d", count)
	}
}

func TestPipelineService_Run_PipelineNotFound(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	_, err := svc.Run(ctx, 999, map[string]any{}, map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing pipeline")
	}
}

func TestPipelineService_Run_EmptyDAG(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	// Use nil DAG (zero-length byte slice) to trigger the empty DAG check
	mustCreatePipeline(t, db, &model.Pipeline{Base: model.Base{ID: 1}, ProjectID: 1, Name: "p1", DAG: nil, Enabled: 1})

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	_, err := svc.Run(ctx, 1, map[string]any{}, map[string]any{})
	if err == nil {
		t.Fatal("expected error for empty dag")
	}
	if err.Error() != "pipeline.run: pipeline 1 has empty dag" {
		t.Errorf("err=%v", err)
	}
}

// ===================== PipelineRun / StepRun Query Tests =====================

func TestPipelineService_ListRuns(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	mustCreatePipeline(t, db, &model.Pipeline{Base: model.Base{ID: 1}, ProjectID: 1, Name: "p1", DAG: model.JSON(`{}`), Enabled: 1})
	mustCreatePipelineRun(t, db, &model.PipelineRun{PipelineID: 1, ProjectID: 1, Status: "succeeded"})
	mustCreatePipelineRun(t, db, &model.PipelineRun{PipelineID: 1, ProjectID: 1, Status: "failed"})

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	list, total, err := svc.ListRuns(ctx, 1, 1, 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if total != 2 {
		t.Errorf("total=%d want 2", total)
	}
	if len(list) != 2 {
		t.Errorf("len=%d want 2", len(list))
	}
}

func TestPipelineService_GetRun(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	mustCreatePipeline(t, db, &model.Pipeline{Base: model.Base{ID: 1}, ProjectID: 1, Name: "p1", DAG: model.JSON(`{}`), Enabled: 1})
	mustCreatePipelineRun(t, db, &model.PipelineRun{Base: model.Base{ID: 1}, PipelineID: 1, ProjectID: 1, Status: "succeeded"})

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	run, err := svc.GetRun(ctx, 1)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if run.Status != "succeeded" {
		t.Errorf("status=%s want succeeded", run.Status)
	}
}

func TestPipelineService_GetRun_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	_, err := svc.GetRun(ctx, 999)
	if err == nil {
		t.Fatal("expected not found")
	}
	if !errors.Is(err, errcode.ErrNotFound) {
		t.Errorf("err=%v want ErrNotFound", err)
	}
}

func TestPipelineService_ListSteps(t *testing.T) {
	ctx := context.Background()
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	mustCreatePipeline(t, db, &model.Pipeline{Base: model.Base{ID: 1}, ProjectID: 1, Name: "p1", DAG: model.JSON(`{}`), Enabled: 1})
	mustCreatePipelineRun(t, db, &model.PipelineRun{Base: model.Base{ID: 1}, PipelineID: 1, ProjectID: 1, Status: "succeeded"})
	mustCreateStepRun(t, db, &model.StepRun{RunID: 1, NodeID: "node-a", NodeType: "llm", Status: "succeeded"})
	mustCreateStepRun(t, db, &model.StepRun{RunID: 1, NodeID: "node-b", NodeType: "image", Status: "succeeded"})

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	steps, err := svc.ListSteps(ctx, 1)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 2 {
		t.Errorf("len=%d want 2", len(steps))
	}
}

// ===================== PipelineRegistry (mock) =====================

type mockPipelineRegistry struct {
	executeFn func(ctx context.Context, dagJSON []byte, input map[string]any, runID int64) (map[string]any, error)
}

func (m *mockPipelineRegistry) Execute(ctx context.Context, dagJSON []byte, input map[string]any, runID int64) (map[string]any, error) {
	return m.executeFn(ctx, dagJSON, input, runID)
}

// ===================== SetDeps Tests =====================

func TestPipelineService_SetDeps(t *testing.T) {
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	if svc.hub != nil {
		t.Error("hub should be nil initially")
	}
	if svc.registry != nil {
		t.Error("registry should be nil initially")
	}

	reg := &mockPipelineRegistry{}
	svc.SetDeps(nil, reg)
	if svc.registry != reg {
		t.Error("registry not set")
	}
}

// ===================== publish helper Tests =====================

func TestPipelineService_publish_NilHub(t *testing.T) {
	db := newPipelineTestDB(t)
	r := newTestRepos(db)

	svc := &pipelineService{r: r, db: db, tc: nil, hub: nil, registry: nil, log: zap.NewNop()}
	// should not panic
	svc.publish("pipeline:1", "progress", 0.5, "ok")
}
