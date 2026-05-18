package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/adapter"
	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/storage"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------- test fixtures ----------

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if sqlDB, derr := db.DB(); derr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	return db
}

func migrateAll(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&model.User{}, &model.Department{}, &model.Role{}, &model.Project{},
		&model.Script{}, &model.Episode{}, &model.EpisodePrompt{},
		&model.Storyboard{}, &model.Style{}, &model.StoryboardStyle{},
		&model.Image{}, &model.ShortVideo{}, &model.FullVideo{},
		&model.ReviewFlow{}, &model.ReviewNode{}, &model.ReviewRecord{}, &model.ReviewNodeRecord{},
		&model.Pipeline{}, &model.PipelineRun{}, &model.StepRun{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
}

func newTestDeps(t *testing.T, db *gorm.DB, store storage.Storage) *DefaultDeps {
	t.Helper()
	return &DefaultDeps{
		Repos: repo.NewRepositories(db, nil),
		GetAdapter: func(ctx context.Context, modelID int64) (adapter.Adapter, *model.Model, error) {
			return nil, nil, errors.New("no adapter in test")
		},
		Store: store,
	}
}

// noopPublisher 用于测试的安全 Publisher
func noopPublisher(float64, string) {}

// mockTextAdapter 返回固定文本的 text adapter

type mockTextAdapter struct {
	code     string
	response string
	err      error
}

func (m *mockTextAdapter) Code() string                          { return m.code }
func (m *mockTextAdapter) Type() adapter.ModelType               { return adapter.TypeText }
func (m *mockTextAdapter) Healthcheck(ctx context.Context) error { return nil }
func (m *mockTextAdapter) Generate(ctx context.Context, req *adapter.Request) (*adapter.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &adapter.Response{Texts: []string{m.response}}, nil
}

// mockStorage 内存存储实现

type mockStorage struct {
	files map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{files: make(map[string][]byte)}
}

func (m *mockStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	m.files[key] = data
	return fmt.Sprintf("http://mock/%s", key), nil
}
func (m *mockStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	data, ok := m.files[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}
func (m *mockStorage) Delete(ctx context.Context, key string) error {
	delete(m.files, key)
	return nil
}
func (m *mockStorage) SignURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	return fmt.Sprintf("http://mock/%s?signed=1", key), nil
}

// ---------- newScriptSplit tests ----------

func TestNewScriptSplit_MissingScriptID(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	deps := newTestDeps(t, db, nil)
	h := newScriptSplit(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing script_id")
}

func TestNewScriptSplit_MissingModelID(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	deps := newTestDeps(t, db, nil)
	h := newScriptSplit(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"script_id": 1}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing model_id")
}

func TestNewScriptSplit_ScriptNotFound(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	deps := newTestDeps(t, db, nil)
	h := newScriptSplit(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"script_id": 999, "model_id": 1}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load script")
}

func TestNewScriptSplit_AdapterError(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)
	sc := &model.Script{ProjectID: 1, Name: "test", RawText: "some text"}
	require.NoError(t, repos.Script.Create(context.Background(), sc))

	deps := &DefaultDeps{
		Repos: repos,
		GetAdapter: func(ctx context.Context, modelID int64) (adapter.Adapter, *model.Model, error) {
			return nil, nil, errors.New("adapter down")
		},
	}
	h := newScriptSplit(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"script_id": sc.ID, "model_id": 1}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get adapter")
}

func TestNewScriptSplit_WrongModelType(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)
	sc := &model.Script{ProjectID: 1, Name: "test", RawText: "some text"}
	require.NoError(t, repos.Script.Create(context.Background(), sc))

	deps := &DefaultDeps{
		Repos: repos,
		GetAdapter: func(ctx context.Context, modelID int64) (adapter.Adapter, *model.Model, error) {
			return &mockImageAdapter{}, nil, nil
		},
	}
	h := newScriptSplit(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"script_id": sc.ID, "model_id": 1}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires text model")
}

func TestNewScriptSplit_Success(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)
	sc := &model.Script{ProjectID: 1, Name: "test", RawText: "Episode 1 content. Episode 2 content."}
	require.NoError(t, repos.Script.Create(context.Background(), sc))

	llmOut := `[{"ep_no":1,"title":"First","summary":"sum1","raw_segment":"seg1"},{"ep_no":2,"title":"Second","summary":"sum2","raw_segment":"seg2"}]`
	deps := &DefaultDeps{
		Repos: repos,
		GetAdapter: func(ctx context.Context, modelID int64) (adapter.Adapter, *model.Model, error) {
			return &mockTextAdapter{code: "gpt", response: llmOut}, nil, nil
		},
	}
	h := newScriptSplit(deps)
	out, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"script_id": sc.ID, "model_id": 1}, Publisher: noopPublisher})
	require.NoError(t, err)
	assert.Equal(t, sc.ID, out["script_id"])
	assert.Equal(t, int(2), out["episode_count"])

	// 验证数据库
	eps, err := repos.Episode.ListByScript(context.Background(), sc.ID)
	require.NoError(t, err)
	require.Len(t, eps, 2)
	assert.Equal(t, "First", eps[0].Title)
	assert.Equal(t, "Second", eps[1].Title)

	// 验证 script status
	updated, err := repos.Script.Get(context.Background(), sc.ID)
	require.NoError(t, err)
	assert.Equal(t, int8(3), updated.Status)
}

func TestNewScriptSplit_ParseEpisodes_EmptyResponse(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)
	sc := &model.Script{ProjectID: 1, Name: "test", RawText: "text"}
	require.NoError(t, repos.Script.Create(context.Background(), sc))

	deps := &DefaultDeps{
		Repos: repos,
		GetAdapter: func(ctx context.Context, modelID int64) (adapter.Adapter, *model.Model, error) {
			return &mockTextAdapter{code: "gpt", response: "[]"}, nil, nil
		},
	}
	h := newScriptSplit(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"script_id": sc.ID, "model_id": 1}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no episodes parsed")
}

func TestNewScriptSplit_ParamsOverride(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)
	sc := &model.Script{ProjectID: 1, Name: "test", RawText: "text"}
	require.NoError(t, repos.Script.Create(context.Background(), sc))

	var capturedReq *adapter.Request
	deps := &DefaultDeps{
		Repos: repos,
		GetAdapter: func(ctx context.Context, modelID int64) (adapter.Adapter, *model.Model, error) {
			return &mockTextAdapterCapture{code: "gpt", onGenerate: func(req *adapter.Request) {
				capturedReq = req
			}}, nil, nil
		},
	}
	h := newScriptSplit(deps)
	_, err := h(
		&NodeContext{
			Ctx:       context.Background(),
			Input:     map[string]any{"script_id": sc.ID, "model_id": 1},
			Params:    map[string]any{"episode_count": 5, "target_chars": 500, "temperature": 0.9},
			Publisher: noopPublisher,
		})
	require.NoError(t, err)
	require.NotNil(t, capturedReq)
	assert.Equal(t, 0.9, capturedReq.Params["temperature"])
}

// mockTextAdapterCapture 捕获请求参数

type mockTextAdapterCapture struct {
	code       string
	onGenerate func(req *adapter.Request)
}

func (m *mockTextAdapterCapture) Code() string                          { return m.code }
func (m *mockTextAdapterCapture) Type() adapter.ModelType               { return adapter.TypeText }
func (m *mockTextAdapterCapture) Healthcheck(ctx context.Context) error { return nil }
func (m *mockTextAdapterCapture) Generate(ctx context.Context, req *adapter.Request) (*adapter.Response, error) {
	if m.onGenerate != nil {
		m.onGenerate(req)
	}
	return &adapter.Response{Texts: []string{`[{"ep_no":1,"title":"T","summary":"S","raw_segment":"R"}]`}}, nil
}

// mockImageAdapter 返回 image 类型(用于类型不匹配测试)

type mockImageAdapter struct{}

func (m *mockImageAdapter) Code() string                          { return "img" }
func (m *mockImageAdapter) Type() adapter.ModelType               { return adapter.TypeImage }
func (m *mockImageAdapter) Healthcheck(ctx context.Context) error { return nil }
func (m *mockImageAdapter) Generate(ctx context.Context, req *adapter.Request) (*adapter.Response, error) {
	return nil, errors.New("not impl")
}

// ---------- newImageUpload tests ----------

func TestNewImageUpload_MissingImageID(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	deps := newTestDeps(t, db, newMockStorage())
	h := newImageUpload(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing image_id")
}

func TestNewImageUpload_NoStorage(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	deps := newTestDeps(t, db, nil)
	h := newImageUpload(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"image_id": 1}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "storage not configured")
}

func TestNewImageUpload_ImageNotFound(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	deps := newTestDeps(t, db, newMockStorage())
	h := newImageUpload(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"image_id": 999}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load image")
}

func TestNewImageUpload_EmptyURL(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)
	img := &model.Image{ProjectID: 1, StoryboardID: 1, URL: ""}
	require.NoError(t, repos.Image.Create(context.Background(), img))

	deps := newTestDeps(t, db, newMockStorage())
	h := newImageUpload(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"image_id": img.ID}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image has no URL")
}

func TestNewImageUpload_Success(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)
	img := &model.Image{ProjectID: 1, StoryboardID: 1, URL: "http://example.com/image.png", Status: 1}
	require.NoError(t, repos.Image.Create(context.Background(), img))

	deps := newTestDeps(t, db, newMockStorage())
	h := newImageUpload(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"image_id": img.ID}, Publisher: noopPublisher})
	// 因为 URL 不可达,会返回下载错误(可能是 "download failed" 或 "bad status")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload")
}

// ---------- newReviewSubmit tests ----------

func TestNewReviewSubmit_MissingFullVideoID(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	deps := newTestDeps(t, db, nil)
	h := newReviewSubmit(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing full_video_id")
}

func TestNewReviewSubmit_FullVideoNotFound(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	deps := newTestDeps(t, db, nil)
	h := newReviewSubmit(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"full_video_id": 999}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load full_video")
}

func TestNewReviewSubmit_NoDefaultFlow(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)
	fv := &model.FullVideo{ProjectID: 1, Name: "test", Status: "draft"}
	require.NoError(t, repos.Full.Create(context.Background(), fv))

	deps := newTestDeps(t, db, nil)
	h := newReviewSubmit(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"full_video_id": fv.ID}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no default review flow")
}

func TestNewReviewSubmit_Success(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)
	fv := &model.FullVideo{ProjectID: 1, Name: "test", Status: "draft"}
	require.NoError(t, repos.Full.Create(context.Background(), fv))

	flow := &model.ReviewFlow{Name: "default", TargetType: "full_video", Enabled: 1, IsDefault: 1}
	require.NoError(t, repos.DB.Create(flow).Error)

	deps := newTestDeps(t, db, nil)
	h := newReviewSubmit(deps)
	out, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"full_video_id": fv.ID}, Publisher: noopPublisher})
	require.NoError(t, err)
	assert.Equal(t, fv.ID, out["full_video_id"])
	assert.Greater(t, out["review_record_id"], int64(0))
	assert.Equal(t, flow.ID, out["flow_id"])

	// 验证 full_video 状态变为 reviewing
	updated, err := repos.Full.Get(context.Background(), fv.ID)
	require.NoError(t, err)
	assert.Equal(t, "reviewing", updated.Status)
}

// ---------- newHumanApprove tests ----------

func TestNewHumanApprove_MissingFullVideoID(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	deps := newTestDeps(t, db, nil)
	h := newHumanApprove(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing full_video_id")
}

func TestNewHumanApprove_NoReviewRecord(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	deps := newTestDeps(t, db, nil)
	h := newHumanApprove(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"full_video_id": 999}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no review record")
}

func TestNewHumanApprove_NotApproved(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)

	rec := &model.ReviewRecord{TargetType: "full_video", TargetID: 1, FlowID: 1, Status: "pending", CurrentStep: 1}
	require.NoError(t, repos.Review.CreateRecord(context.Background(), rec))

	deps := newTestDeps(t, db, nil)
	h := newHumanApprove(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"full_video_id": 1}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not approved")
	assert.Contains(t, err.Error(), "pending")
}

func TestNewHumanApprove_Rejected(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)

	rec := &model.ReviewRecord{TargetType: "full_video", TargetID: 1, FlowID: 1, Status: "rejected", CurrentStep: 1}
	require.NoError(t, repos.Review.CreateRecord(context.Background(), rec))

	deps := newTestDeps(t, db, nil)
	h := newHumanApprove(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"full_video_id": 1}, Publisher: noopPublisher})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not approved")
	assert.Contains(t, err.Error(), "rejected")
}

func TestNewHumanApprove_Success(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)

	rec := &model.ReviewRecord{TargetType: "full_video", TargetID: 1, FlowID: 1, Status: "approved", CurrentStep: 2}
	require.NoError(t, repos.Review.CreateRecord(context.Background(), rec))

	deps := newTestDeps(t, db, nil)
	h := newHumanApprove(deps)
	out, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"full_video_id": 1}, Publisher: noopPublisher})
	require.NoError(t, err)
	assert.Equal(t, int64(1), out["full_video_id"])
	assert.Equal(t, rec.ID, out["review_record_id"])
	assert.Equal(t, true, out["approved"])
}

func TestNewHumanApprove_FindLatestNonPending(t *testing.T) {
	// 当没有 pending 记录时,查找最新的任意记录
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)

	rec := &model.ReviewRecord{TargetType: "full_video", TargetID: 1, FlowID: 1, Status: "approved", CurrentStep: 2}
	require.NoError(t, repos.Review.CreateRecord(context.Background(), rec))

	deps := newTestDeps(t, db, nil)
	h := newHumanApprove(deps)
	// GetActiveRecord 查找 pending,找不到会 fallback 到最新记录
	out, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"full_video_id": 1}, Publisher: noopPublisher})
	require.NoError(t, err)
	assert.Equal(t, true, out["approved"])
}

// ---------- helper tests ----------

func TestBuildSplitPrompt(t *testing.T) {
	p := buildSplitPrompt("hello world", 3, 500)
	assert.Contains(t, p, "3 集")
	assert.Contains(t, p, "500-700 字")
	assert.Contains(t, p, "hello world")
	assert.Contains(t, p, "ep_no")
}

func TestParseEpisodes_Success(t *testing.T) {
	json := `[{"ep_no":1,"title":"T1","summary":"S1","raw_segment":"R1"},{"ep_no":2,"title":"T2","summary":"S2","raw_segment":"R2"}]`
	eps, err := parseEpisodes(json)
	require.NoError(t, err)
	require.Len(t, eps, 2)
	assert.Equal(t, 1, eps[0].EpNo)
	assert.Equal(t, "T1", eps[0].Title)
	assert.Equal(t, "S1", eps[0].Summary)
	assert.Equal(t, "R1", eps[0].RawSegment)
}

func TestParseEpisodes_WithFence(t *testing.T) {
	json := "```json\n[{\"ep_no\":1,\"title\":\"T\",\"summary\":\"S\",\"raw_segment\":\"R\"}]\n```"
	eps, err := parseEpisodes(json)
	require.NoError(t, err)
	require.Len(t, eps, 1)
}

func TestParseEpisodes_NoArray(t *testing.T) {
	_, err := parseEpisodes("no json here")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no JSON array")
}

func TestParseEpisodes_InvalidJSON(t *testing.T) {
	_, err := parseEpisodes("[not valid json]")
	require.Error(t, err)
}

func TestStripJSONFence(t *testing.T) {
	assert.Equal(t, "{\"a\":1}", stripJSONFence("```json\n{\"a\":1}\n```"))
	assert.Equal(t, "{\"a\":1}", stripJSONFence("```\n{\"a\":1}\n```"))
	assert.Equal(t, `{"a":1}`, stripJSONFence(`{"a":1}`))
}

func TestIntFromInput(t *testing.T) {
	m := map[string]any{
		"a": int64(42),
		"b": int(43),
		"c": float64(44),
		"d": "45",
		"e": "not a number",
	}
	assert.Equal(t, int64(42), intFromInput(m, "a"))
	assert.Equal(t, int64(43), intFromInput(m, "b"))
	assert.Equal(t, int64(44), intFromInput(m, "c"))
	assert.Equal(t, int64(45), intFromInput(m, "d"))
	assert.Equal(t, int64(0), intFromInput(m, "e"))
	assert.Equal(t, int64(0), intFromInput(m, "missing"))
}

func TestStrFromInput(t *testing.T) {
	m := map[string]any{"a": "hello", "b": 123}
	assert.Equal(t, "hello", strFromInput(m, "a"))
	assert.Equal(t, "", strFromInput(m, "b"))
	assert.Equal(t, "", strFromInput(m, "missing"))
}

func TestBase64Bytes(t *testing.T) {
	assert.Equal(t, "<10 bytes>", bytesPlaceholder(make([]byte, 10)))
}

func TestPersistRemoteAsset_StoreNil(t *testing.T) {
	url := persistRemoteAsset(context.Background(), nil, "images", "http://example.com/img.jpg")
	assert.Equal(t, "http://example.com/img.jpg", url)
}

func TestPersistRemoteAsset_LocalPath(t *testing.T) {
	store := newMockStorage()
	url := persistRemoteAsset(context.Background(), store, "images", "/uploads/local.jpg")
	assert.Equal(t, "/uploads/local.jpg", url)
}

func TestPersistRemoteAsset_BadURL(t *testing.T) {
	store := newMockStorage()
	url := persistRemoteAsset(context.Background(), store, "images", "http://127.0.0.1:1/nope")
	// 下载失败,退化到原 URL
	assert.Equal(t, "http://127.0.0.1:1/nope", url)
}

// ---------- integration: RegisterDefaultNodeHandlers ----------

func TestRegisterDefaultNodeHandlers_CoversAll(t *testing.T) {
	reg := NewNodeHandlerRegistry()
	deps := &DefaultDeps{
		Repos:      nil,
		GetAdapter: nil,
		Store:      nil,
	}
	RegisterDefaultNodeHandlers(reg, deps)

	expected := []string{
		NodeScriptSplit,
		NodePromptGenerate,
		NodeStoryboardGenerate,
		NodeStyleApply,
		NodeImageGenerate,
		NodeImageUpload,
		NodeVideoGenerate,
		NodeAudioTTS,
		NodeVideoCompose,
		NodeReviewSubmit,
		NodeHumanApprove,
	}
	for _, name := range expected {
		_, ok := reg.Get(name)
		assert.True(t, ok, "missing handler for %s", name)
	}
}

// ---------- edge: newScriptSplit with model_id from NodeContext ----------

func TestNewScriptSplit_ModelIDFromNodeContext(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)
	repos := repo.NewRepositories(db, nil)
	sc := &model.Script{ProjectID: 1, Name: "test", RawText: "text"}
	require.NoError(t, repos.Script.Create(context.Background(), sc))

	var calledModelID int64
	deps := &DefaultDeps{
		Repos: repos,
		GetAdapter: func(ctx context.Context, modelID int64) (adapter.Adapter, *model.Model, error) {
			calledModelID = modelID
			return &mockTextAdapter{code: "gpt", response: `[{"ep_no":1,"title":"T","summary":"S","raw_segment":"R"}]`}, nil, nil
		},
	}
	h := newScriptSplit(deps)
	_, err := h(
		&NodeContext{Ctx: context.Background(), Input: map[string]any{"script_id": sc.ID}, ModelID: 42, Publisher: noopPublisher})
	require.NoError(t, err)
	assert.Equal(t, int64(42), calledModelID)
}
