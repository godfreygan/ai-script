package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/adapter"
	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ===================== Mocks =====================

type mockQueueClient struct {
	enqueueFunc func(ctx context.Context, taskType string, payload []byte, opts ...asynq.Option) (string, error)
}

func (m *mockQueueClient) Enqueue(ctx context.Context, taskType string, payload []byte, opts ...asynq.Option) (string, error) {
	return m.enqueueFunc(ctx, taskType, payload, opts...)
}
func (m *mockQueueClient) EnqueueIn(ctx context.Context, taskType string, payload []byte, delay time.Duration) (string, error) {
	return m.Enqueue(ctx, taskType, payload, asynq.ProcessIn(delay))
}
func (m *mockQueueClient) Ping() error { return nil }

type mockStorage struct {
	putFunc func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error)
}

func (m *mockStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	return m.putFunc(ctx, key, r, size, contentType)
}
func (m *mockStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) { return nil, nil }
func (m *mockStorage) Delete(ctx context.Context, key string) error               { return nil }
func (m *mockStorage) SignURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	return "", nil
}

type mockAdapter struct {
	code       string
	mtype      adapter.ModelType
	generateFn func(ctx context.Context, req *adapter.Request) (*adapter.Response, error)
}

func (m *mockAdapter) Code() string            { return m.code }
func (m *mockAdapter) Type() adapter.ModelType { return m.mtype }
func (m *mockAdapter) Generate(ctx context.Context, req *adapter.Request) (*adapter.Response, error) {
	return m.generateFn(ctx, req)
}
func (m *mockAdapter) Healthcheck(ctx context.Context) error { return nil }

type mockModelService struct {
	getAdapterFn func(ctx context.Context, id int64) (adapter.Adapter, *model.Model, error)
}

func (m *mockModelService) GetAdapter(ctx context.Context, id int64) (adapter.Adapter, *model.Model, error) {
	return m.getAdapterFn(ctx, id)
}

// satisfy minimal interface so we can pass it where *ModelService is expected in tests
// (we use a wrapper approach below to avoid modifying production code)

type mockInvocationService struct {
	logCalled  int
	lastParams *LogParams
}

func (m *mockInvocationService) Log(ctx context.Context, p *LogParams) {
	m.logCalled++
	m.lastParams = p
}
func (m *mockInvocationService) List(ctx context.Context, q *repo.ListInvocationsQuery) ([]model.ModelInvocation, int64, error) {
	return nil, 0, nil
}
func (m *mockInvocationService) Stats(ctx context.Context, q *repo.ListInvocationsQuery) (*repo.InvocationStats, error) {
	return nil, nil
}

// ===================== Helpers =====================

func newGenerationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return newTestDB(t,
		&model.Script{}, &model.ScriptVersion{}, &model.Episode{},
		&model.EpisodePrompt{}, &model.Storyboard{}, &model.StoryboardStyle{},
		&model.Style{}, &model.Image{}, &model.ShortVideo{},
		&model.Model{}, &model.ModelInvocation{}, &model.Project{},
	)
}

func mustCreateModel(t *testing.T, db *gorm.DB, m *model.Model) {
	t.Helper()
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
}

func mustCreateProject(t *testing.T, db *gorm.DB, p *model.Project) {
	t.Helper()
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
}

func mustCreateEpisode(t *testing.T, db *gorm.DB, e *model.Episode) {
	t.Helper()
	if err := db.Create(e).Error; err != nil {
		t.Fatalf("create episode: %v", err)
	}
}

func mustCreatePrompt(t *testing.T, db *gorm.DB, p *model.EpisodePrompt) {
	t.Helper()
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create prompt: %v", err)
	}
}

func mustCreateStoryboard(t *testing.T, db *gorm.DB, s *model.Storyboard) {
	t.Helper()
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("create storyboard: %v", err)
	}
}

func mustCreateImage(t *testing.T, db *gorm.DB, img *model.Image) {
	t.Helper()
	if err := db.Create(img).Error; err != nil {
		t.Fatalf("create image: %v", err)
	}
}

// ===================== ImageService Tests =====================

func TestImageService_HandleGenerateTask_Success(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	mustCreateModel(t, db, &model.Model{Base: model.Base{ID: 1}, Code: "img-model", Type: "image", Enabled: 1})

	store := &mockStorage{
		putFunc: func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
			return "/uploads/" + key, nil
		},
	}

	svc := &imageService{r: r, tc: nil, hub: nil, store: store, log: zap.NewNop()}

	payload, _ := json.Marshal(imagePayload{
		StoryboardID: 1,
		ProjectID:    1,
		ModelID:      1,
		Prompt:       "a cat",
		UserID:       100,
		DeptID:       10,
	})
	task := asynq.NewTask(TaskImageGenerate, payload)

	// We need to use the actual handler signature; since modelSvc is an interface we wrap it
	handler := svc.HandleGenerateTask(&modelService{r: r, registry: adapter.NewRegistry(), log: zap.NewNop()}, &invocationService{r: r, log: zap.NewNop()})
	// But ModelService.GetAdapter will hit DB, so let's register adapter manually via registry
	reg := adapter.NewRegistry()
	reg.Register(&mockAdapter{
		code:  "img-model",
		mtype: adapter.TypeImage,
		generateFn: func(ctx context.Context, req *adapter.Request) (*adapter.Response, error) {
			return &adapter.Response{
				ImageURLs: []string{"https://cdn.example.com/img.png"},
				Raw:       map[string]any{"width": 512, "height": 768},
			}, nil
		},
	})
	ms2 := &modelService{r: r, registry: reg, log: zap.NewNop()}
	invSvc2 := &invocationService{r: r, log: zap.NewNop()}
	handler = svc.HandleGenerateTask(ms2, invSvc2)

	if err := handler(ctx, task); err != nil {
		t.Fatalf("handle task: %v", err)
	}

	var img model.Image
	if err := db.First(&img, "storyboard_id = ?", 1).Error; err != nil {
		t.Fatalf("image record not created: %v", err)
	}
	if img.Status != 2 {
		t.Errorf("status=%d want 2", img.Status)
	}
	if img.Width != 512 {
		t.Errorf("width=%d want 512", img.Width)
	}
	if img.Height != 768 {
		t.Errorf("height=%d want 768", img.Height)
	}
}

func TestImageService_HandleGenerateTask_AdapterUnavailable(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	// no model in DB -> GetAdapter returns not found

	svc := &imageService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	ms := &modelService{r: r, registry: adapter.NewRegistry(), log: zap.NewNop()}
	invSvc := &invocationService{r: r, log: zap.NewNop()}

	payload, _ := json.Marshal(imagePayload{StoryboardID: 1, ProjectID: 1, ModelID: 99, UserID: 1, DeptID: 1})
	task := asynq.NewTask(TaskImageGenerate, payload)

	handler := svc.HandleGenerateTask(ms, invSvc)
	err := handler(ctx, task)
	if err == nil {
		t.Fatal("expected error for missing adapter")
	}
	if !errors.Is(err, errcode.ErrNotFound) {
		t.Errorf("err=%v want ErrNotFound", err)
	}
}

func TestImageService_HandleGenerateTask_ModelTypeMismatch(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	mustCreateModel(t, db, &model.Model{Base: model.Base{ID: 1}, Code: "txt-model", Type: "text", Enabled: 1})

	reg := adapter.NewRegistry()
	reg.Register(&mockAdapter{code: "txt-model", mtype: adapter.TypeText})
	ms := &modelService{r: r, registry: reg, log: zap.NewNop()}
	invSvc := &invocationService{r: r, log: zap.NewNop()}

	svc := &imageService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	payload, _ := json.Marshal(imagePayload{StoryboardID: 1, ProjectID: 1, ModelID: 1, UserID: 1, DeptID: 1})
	task := asynq.NewTask(TaskImageGenerate, payload)

	handler := svc.HandleGenerateTask(ms, invSvc)
	err := handler(ctx, task)
	if err == nil {
		t.Fatal("expected error for model type mismatch")
	}
	if err.Error() != "image.generate requires an image model" {
		t.Errorf("err=%v", err)
	}
}

func TestImageService_HandleGenerateTask_ModelCallFailed(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	mustCreateModel(t, db, &model.Model{Base: model.Base{ID: 1}, Code: "img-model", Type: "image", Enabled: 1})

	reg := adapter.NewRegistry()
	reg.Register(&mockAdapter{
		code:  "img-model",
		mtype: adapter.TypeImage,
		generateFn: func(ctx context.Context, req *adapter.Request) (*adapter.Response, error) {
			return nil, errors.New("upstream timeout")
		},
	})
	ms := &modelService{r: r, registry: reg, log: zap.NewNop()}
	invSvc := &invocationService{r: r, log: zap.NewNop()}

	svc := &imageService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	payload, _ := json.Marshal(imagePayload{StoryboardID: 1, ProjectID: 1, ModelID: 1, UserID: 1, DeptID: 1})
	task := asynq.NewTask(TaskImageGenerate, payload)

	handler := svc.HandleGenerateTask(ms, invSvc)
	err := handler(ctx, task)
	if err == nil {
		t.Fatal("expected error for model call failure")
	}
	if err.Error() != "upstream timeout" {
		t.Errorf("err=%v want upstream timeout", err)
	}
}

func TestImageService_HandleGenerateTask_EmptyImageURL(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	mustCreateModel(t, db, &model.Model{Base: model.Base{ID: 1}, Code: "img-model", Type: "image", Enabled: 1})

	reg := adapter.NewRegistry()
	reg.Register(&mockAdapter{
		code:  "img-model",
		mtype: adapter.TypeImage,
		generateFn: func(ctx context.Context, req *adapter.Request) (*adapter.Response, error) {
			return &adapter.Response{ImageURLs: []string{}}, nil
		},
	})
	ms := &modelService{r: r, registry: reg, log: zap.NewNop()}
	invSvc := &invocationService{r: r, log: zap.NewNop()}

	svc := &imageService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	payload, _ := json.Marshal(imagePayload{StoryboardID: 1, ProjectID: 1, ModelID: 1, UserID: 1, DeptID: 1})
	task := asynq.NewTask(TaskImageGenerate, payload)

	handler := svc.HandleGenerateTask(ms, invSvc)
	err := handler(ctx, task)
	if err == nil {
		t.Fatal("expected error for empty image url")
	}
	if err.Error() != "empty image response" {
		t.Errorf("err=%v", err)
	}
}

// ===================== ShortVideoService Tests =====================

func TestShortVideoService_HandleGenerateTask_Success(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	mustCreateModel(t, db, &model.Model{Base: model.Base{ID: 1}, Code: "vid-model", Type: "video", Enabled: 1})

	reg := adapter.NewRegistry()
	reg.Register(&mockAdapter{
		code:  "vid-model",
		mtype: adapter.TypeVideo,
		generateFn: func(ctx context.Context, req *adapter.Request) (*adapter.Response, error) {
			return &adapter.Response{
				VideoURLs:  []string{"https://cdn.example.com/vid.mp4"},
				DurationMs: 5000,
				Raw:        map[string]any{"width": 720, "height": 1280},
			}, nil
		},
	})
	ms := &modelService{r: r, registry: reg, log: zap.NewNop()}
	invSvc := &invocationService{r: r, log: zap.NewNop()}

	store := &mockStorage{
		putFunc: func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
			return "/uploads/" + key, nil
		},
	}

	svc := &shortVideoService{r: r, tc: nil, hub: nil, store: store, log: zap.NewNop()}
	payload, _ := json.Marshal(shortVideoPayload{
		StoryboardID: 1,
		ProjectID:    1,
		ModelID:      1,
		Prompt:       "dancing cat",
		UserID:       100,
		DeptID:       10,
	})
	task := asynq.NewTask(TaskVideoGenerate, payload)

	handler := svc.HandleGenerateTask(ms, invSvc)
	if err := handler(ctx, task); err != nil {
		t.Fatalf("handle task: %v", err)
	}

	var sv model.ShortVideo
	if err := db.First(&sv, "storyboard_id = ?", 1).Error; err != nil {
		t.Fatalf("short video not created: %v", err)
	}
	if sv.Status != "succeeded" {
		t.Errorf("status=%s want succeeded", sv.Status)
	}
	if sv.DurationMs != 5000 {
		t.Errorf("duration_ms=%d want 5000", sv.DurationMs)
	}
	if sv.Width != 720 {
		t.Errorf("width=%d want 720", sv.Width)
	}
	if sv.Height != 1280 {
		t.Errorf("height=%d want 1280", sv.Height)
	}
}

func TestShortVideoService_HandleGenerateTask_AdapterUnavailable(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	// model missing

	svc := &shortVideoService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	ms := &modelService{r: r, registry: adapter.NewRegistry(), log: zap.NewNop()}
	invSvc := &invocationService{r: r, log: zap.NewNop()}

	payload, _ := json.Marshal(shortVideoPayload{StoryboardID: 1, ProjectID: 1, ModelID: 99, UserID: 1, DeptID: 1})
	task := asynq.NewTask(TaskVideoGenerate, payload)

	handler := svc.HandleGenerateTask(ms, invSvc)
	err := handler(ctx, task)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errcode.ErrNotFound) {
		t.Errorf("err=%v want ErrNotFound", err)
	}
}

func TestShortVideoService_HandleGenerateTask_ModelTypeMismatch(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	mustCreateModel(t, db, &model.Model{Base: model.Base{ID: 1}, Code: "txt-model", Type: "text", Enabled: 1})

	reg := adapter.NewRegistry()
	reg.Register(&mockAdapter{code: "txt-model", mtype: adapter.TypeText})
	ms := &modelService{r: r, registry: reg, log: zap.NewNop()}
	invSvc := &invocationService{r: r, log: zap.NewNop()}

	svc := &shortVideoService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	payload, _ := json.Marshal(shortVideoPayload{StoryboardID: 1, ProjectID: 1, ModelID: 1, UserID: 1, DeptID: 1})
	task := asynq.NewTask(TaskVideoGenerate, payload)

	handler := svc.HandleGenerateTask(ms, invSvc)
	err := handler(ctx, task)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "video.generate requires a video model" {
		t.Errorf("err=%v", err)
	}
}

func TestShortVideoService_HandleGenerateTask_ModelCallFailed(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	mustCreateModel(t, db, &model.Model{Base: model.Base{ID: 1}, Code: "vid-model", Type: "video", Enabled: 1})

	reg := adapter.NewRegistry()
	reg.Register(&mockAdapter{
		code:  "vid-model",
		mtype: adapter.TypeVideo,
		generateFn: func(ctx context.Context, req *adapter.Request) (*adapter.Response, error) {
			return nil, errors.New("upstream timeout")
		},
	})
	ms := &modelService{r: r, registry: reg, log: zap.NewNop()}
	invSvc := &invocationService{r: r, log: zap.NewNop()}

	svc := &shortVideoService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	payload, _ := json.Marshal(shortVideoPayload{StoryboardID: 1, ProjectID: 1, ModelID: 1, UserID: 1, DeptID: 1})
	task := asynq.NewTask(TaskVideoGenerate, payload)

	handler := svc.HandleGenerateTask(ms, invSvc)
	err := handler(ctx, task)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "upstream timeout" {
		t.Errorf("err=%v want upstream timeout", err)
	}

	// verify placeholder record status updated to failed
	var sv model.ShortVideo
	if dbErr := db.First(&sv, "storyboard_id = ?", 1).Error; dbErr != nil {
		t.Fatalf("placeholder record missing: %v", dbErr)
	}
	if sv.Status != "failed" {
		t.Errorf("status=%s want failed", sv.Status)
	}
}

func TestShortVideoService_HandleGenerateTask_EmptyVideoURL(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	mustCreateModel(t, db, &model.Model{Base: model.Base{ID: 1}, Code: "vid-model", Type: "video", Enabled: 1})

	reg := adapter.NewRegistry()
	reg.Register(&mockAdapter{
		code:  "vid-model",
		mtype: adapter.TypeVideo,
		generateFn: func(ctx context.Context, req *adapter.Request) (*adapter.Response, error) {
			return &adapter.Response{VideoURLs: []string{}}, nil
		},
	})
	ms := &modelService{r: r, registry: reg, log: zap.NewNop()}
	invSvc := &invocationService{r: r, log: zap.NewNop()}

	svc := &shortVideoService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	payload, _ := json.Marshal(shortVideoPayload{StoryboardID: 1, ProjectID: 1, ModelID: 1, UserID: 1, DeptID: 1})
	task := asynq.NewTask(TaskVideoGenerate, payload)

	handler := svc.HandleGenerateTask(ms, invSvc)
	err := handler(ctx, task)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "empty video response" {
		t.Errorf("err=%v", err)
	}
}

// ===================== StoryboardService Tests =====================

func TestStoryboardService_HandleGenerateTask_Success(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateEpisode(t, db, &model.Episode{Base: model.Base{ID: 1}, ScriptID: 1, EpNo: 1, Title: "E1"})
	promptJSON := `{"summary":"test","shots":[{"shot_no":1,"shot_type":"wide","camera":"static","scene":"street","characters":["A"],"action":"walk","dialogue":"hi","duration_sec":10,"image_prompt":"img","video_prompt":"vid"}]}`
	mustCreatePrompt(t, db, &model.EpisodePrompt{EpisodeID: 1, IsCurrent: 1, Content: model.JSON(promptJSON)})

	svc := &storyboardService{r: r, tc: nil, hub: nil, log: zap.NewNop()}
	payload, _ := json.Marshal(storyboardPayload{EpisodeID: 1, ModelID: 1, UserID: 1})
	task := asynq.NewTask(TaskStoryboardGenerate, payload)

	ms := &modelService{r: r, registry: adapter.NewRegistry(), log: zap.NewNop()}
	handler := svc.HandleGenerateTask(ms)
	if err := handler(ctx, task); err != nil {
		t.Fatalf("handle task: %v", err)
	}

	var list []model.Storyboard
	if err := db.Where("episode_id = ?", 1).Find(&list).Error; err != nil {
		t.Fatalf("list storyboard: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expect 1 storyboard, got %d", len(list))
	}
	if list[0].ShotNo != 1 {
		t.Errorf("shot_no=%d want 1", list[0].ShotNo)
	}
	if list[0].ShotType != "wide" {
		t.Errorf("shot_type=%s want wide", list[0].ShotType)
	}
	if list[0].DurationSec != 10 {
		t.Errorf("duration_sec=%d want 10", list[0].DurationSec)
	}
}

func TestStoryboardService_HandleGenerateTask_NoCurrentPrompt(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateEpisode(t, db, &model.Episode{Base: model.Base{ID: 1}, ScriptID: 1, EpNo: 1})
	// no prompt

	svc := &storyboardService{r: r, tc: nil, hub: nil, log: zap.NewNop()}
	payload, _ := json.Marshal(storyboardPayload{EpisodeID: 1, ModelID: 1, UserID: 1})
	task := asynq.NewTask(TaskStoryboardGenerate, payload)

	ms := &modelService{r: r, registry: adapter.NewRegistry(), log: zap.NewNop()}
	handler := svc.HandleGenerateTask(ms)
	err := handler(ctx, task)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestStoryboardService_HandleGenerateTask_NoShots(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateEpisode(t, db, &model.Episode{Base: model.Base{ID: 1}, ScriptID: 1, EpNo: 1})
	promptJSON := `{"summary":"test","shots":[]}`
	mustCreatePrompt(t, db, &model.EpisodePrompt{EpisodeID: 1, IsCurrent: 1, Content: model.JSON(promptJSON)})

	svc := &storyboardService{r: r, tc: nil, hub: nil, log: zap.NewNop()}
	payload, _ := json.Marshal(storyboardPayload{EpisodeID: 1, ModelID: 1, UserID: 1})
	task := asynq.NewTask(TaskStoryboardGenerate, payload)

	ms := &modelService{r: r, registry: adapter.NewRegistry(), log: zap.NewNop()}
	handler := svc.HandleGenerateTask(ms)
	err := handler(ctx, task)
	if err == nil {
		t.Fatal("expected error for empty shots")
	}
	if err.Error() != "prompt has no shots" {
		t.Errorf("err=%v", err)
	}
}

// ===================== StoryboardService CRUD Tests =====================

func TestStoryboardService_BulkSave(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateEpisode(t, db, &model.Episode{Base: model.Base{ID: 1}, ScriptID: 1, EpNo: 1})
	mustCreateStoryboard(t, db, &model.Storyboard{EpisodeID: 1, ShotNo: 1, SceneDesc: "old"})

	svc := &storyboardService{r: r, tc: nil, hub: nil, log: zap.NewNop()}
	if err := svc.BulkSave(ctx, 1, []model.Storyboard{
		{ShotNo: 2, SceneDesc: "new1"},
		{ShotNo: 3, SceneDesc: "new2"},
	}); err != nil {
		t.Fatalf("bulk save: %v", err)
	}

	var list []model.Storyboard
	if err := db.Where("episode_id = ?", 1).Find(&list).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expect 2 storyboards after bulk save, got %d", len(list))
	}
}

func TestStoryboardService_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	svc := &storyboardService{r: r, tc: nil, hub: nil, log: zap.NewNop()}
	_, err := svc.Get(ctx, 999)
	if err == nil {
		t.Fatal("expected not found")
	}
}

// ===================== ImageService CRUD Tests =====================

func TestImageService_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	svc := &imageService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	_, err := svc.Get(ctx, 999)
	if err == nil {
		t.Fatal("expected not found")
	}
}

func TestImageService_Delete(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateImage(t, db, &model.Image{Base: model.Base{ID: 1}, ProjectID: 1, StoryboardID: 1, URL: "http://x"})

	svc := &imageService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	if err := svc.Delete(ctx, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var count int64
	db.Model(&model.Image{}).Where("id = ?", 1).Count(&count)
	if count != 0 {
		t.Errorf("image still exists after delete")
	}
}

// ===================== ShortVideoService CRUD Tests =====================

func TestShortVideoService_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	svc := &shortVideoService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	_, err := svc.Get(ctx, 999)
	if err == nil {
		t.Fatal("expected not found")
	}
}

func TestShortVideoService_Delete(t *testing.T) {
	ctx := context.Background()
	db := newGenerationTestDB(t)
	r := newTestRepos(db)

	mustCreateProject(t, db, &model.Project{Base: model.Base{ID: 1}, Name: "p1"})
	mustCreateStoryboard(t, db, &model.Storyboard{Base: model.Base{ID: 1}, EpisodeID: 1, ShotNo: 1})
	mustCreateModel(t, db, &model.Model{Base: model.Base{ID: 1}, Code: "vid-model", Type: "video", Enabled: 1})

	// create a short video record directly
	sv := &model.ShortVideo{Base: model.Base{ID: 1}, ProjectID: 1, StoryboardID: 1, VideoURL: "http://x", Status: "succeeded"}
	if err := db.Create(sv).Error; err != nil {
		t.Fatalf("create sv: %v", err)
	}

	svc := &shortVideoService{r: r, tc: nil, hub: nil, store: nil, log: zap.NewNop()}
	if err := svc.Delete(ctx, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var count int64
	db.Model(&model.ShortVideo{}).Where("id = ?", 1).Count(&count)
	if count != 0 {
		t.Errorf("short video still exists after delete")
	}
}

// ===================== persistRemoteAsset Tests =====================

func TestPersistRemoteAsset_NilStore(t *testing.T) {
	ctx := context.Background()
	url := persistRemoteAsset(ctx, nil, "images", "https://example.com/img.png", zap.NewNop())
	if url != "https://example.com/img.png" {
		t.Errorf("url=%s want original", url)
	}
}

func TestPersistRemoteAsset_EmptyURL(t *testing.T) {
	ctx := context.Background()
	store := &mockStorage{putFunc: func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
		return "", nil
	}}
	url := persistRemoteAsset(ctx, store, "images", "", zap.NewNop())
	if url != "" {
		t.Errorf("url=%s want empty", url)
	}
}

func TestPersistRemoteAsset_LocalPath(t *testing.T) {
	ctx := context.Background()
	store := &mockStorage{}
	url := persistRemoteAsset(ctx, store, "images", "/uploads/images/x.png", zap.NewNop())
	if url != "/uploads/images/x.png" {
		t.Errorf("url=%s want local path", url)
	}
}

// ===================== buildStyledPrompt Tests =====================

func TestBuildStyledPrompt(t *testing.T) {
	base := "a cat"
	st := &model.Style{ArtStyle: "oil", ColorTone: "warm", Lighting: "soft", Description: "detailed"}
	got := buildStyledPrompt(base, st)
	want := "a cat, art_style: oil, color_tone: warm, lighting: soft, style_note: detailed"
	if got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestBuildStyledPrompt_NoStyle(t *testing.T) {
	base := "a cat"
	st := &model.Style{}
	got := buildStyledPrompt(base, st)
	if got != base {
		t.Errorf("got=%q want=%q", got, base)
	}
}

// ===================== toIntFromAny / max1 Tests =====================

func TestToIntFromAny(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{int(42), 42},
		{int64(42), 42},
		{float64(42.9), 42},
		{"42", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := toIntFromAny(c.in); got != c.want {
			t.Errorf("toIntFromAny(%v)=%d want %d", c.in, got, c.want)
		}
	}
}

func TestMax1(t *testing.T) {
	if max1(0) != 1 {
		t.Errorf("max1(0)=%d want 1", max1(0))
	}
	if max1(5) != 5 {
		t.Errorf("max1(5)=%d want 5", max1(5))
	}
	if max1(-3) != 1 {
		t.Errorf("max1(-3)=%d want 1", max1(-3))
	}
}
