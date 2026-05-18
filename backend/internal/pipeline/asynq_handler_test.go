package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"

	"github.com/glebarez/sqlite"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestReposForAsynq 创建 in-memory sqlite 并 migrate PipelineRun / StepRun / Pipeline 表
func newTestReposForAsynq(t *testing.T) *repo.Repositories {
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
	if err := db.AutoMigrate(
		&model.PipelineRun{},
		&model.StepRun{},
		&model.Pipeline{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return repo.NewRepositories(db, nil)
}

// TestNewAsynqHandler_InvalidPayload 验证非法 JSON payload 返回错误
func TestNewAsynqHandler_InvalidPayload(t *testing.T) {
	repos := newTestReposForAsynq(t)
	runner := NewRunner(NewNodeHandlerRegistry(), repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	task := asynq.NewTask(TaskPipelineRun, []byte("not json"))
	err := handler(context.Background(), task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode pipeline.run payload")
}

// TestNewAsynqHandler_MissingRunID 验证缺少 run_id 返回错误
func TestNewAsynqHandler_MissingRunID(t *testing.T) {
	repos := newTestReposForAsynq(t)
	runner := NewRunner(NewNodeHandlerRegistry(), repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{RunID: 0, PipelineID: 1})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(context.Background(), task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing run_id")
}

// TestNewAsynqHandler_MissingPipelineID 验证缺少 pipeline_id 返回错误
func TestNewAsynqHandler_MissingPipelineID(t *testing.T) {
	repos := newTestReposForAsynq(t)
	runner := NewRunner(NewNodeHandlerRegistry(), repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{RunID: 1, PipelineID: 0})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(context.Background(), task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing run_id or pipeline_id")
}

// TestNewAsynqHandler_RunNotFound 验证 run 不存在时返回错误
func TestNewAsynqHandler_RunNotFound(t *testing.T) {
	repos := newTestReposForAsynq(t)
	runner := NewRunner(NewNodeHandlerRegistry(), repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{RunID: 999, PipelineID: 1})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(context.Background(), task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load run")
}

// TestNewAsynqHandler_IdempotentSucceeded 验证已成功的 run 被幂等跳过
func TestNewAsynqHandler_IdempotentSucceeded(t *testing.T) {
	repos := newTestReposForAsynq(t)
	ctx := context.Background()

	// 预创建 pipeline
	pl := &model.Pipeline{Name: "test", DAG: model.JSON(`{"nodes":[{"id":"a","type":"noop"}]}`)}
	require.NoError(t, repos.Pipeline.Create(ctx, pl))

	// 预创建 run 并标记为 succeeded
	run := &model.PipelineRun{PipelineID: pl.ID, Status: "succeeded"}
	require.NoError(t, repos.Pipeline.CreateRun(ctx, run))

	runner := NewRunner(NewNodeHandlerRegistry(), repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{RunID: run.ID, PipelineID: pl.ID})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(ctx, task)
	require.NoError(t, err) // 幂等返回 nil
}

// TestNewAsynqHandler_IdempotentFailed 验证已失败的 run 被幂等跳过
func TestNewAsynqHandler_IdempotentFailed(t *testing.T) {
	repos := newTestReposForAsynq(t)
	ctx := context.Background()

	pl := &model.Pipeline{Name: "test", DAG: model.JSON(`{"nodes":[{"id":"a","type":"noop"}]}`)}
	require.NoError(t, repos.Pipeline.Create(ctx, pl))

	run := &model.PipelineRun{PipelineID: pl.ID, Status: "failed"}
	require.NoError(t, repos.Pipeline.CreateRun(ctx, run))

	runner := NewRunner(NewNodeHandlerRegistry(), repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{RunID: run.ID, PipelineID: pl.ID})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(ctx, task)
	require.NoError(t, err)
}

// TestNewAsynqHandler_PipelineNotFound 验证 pipeline 不存在时标记 run 为 failed
func TestNewAsynqHandler_PipelineNotFound(t *testing.T) {
	repos := newTestReposForAsynq(t)
	ctx := context.Background()

	// 预创建 run
	run := &model.PipelineRun{PipelineID: 9999, Status: "queued"}
	require.NoError(t, repos.Pipeline.CreateRun(ctx, run))

	runner := NewRunner(NewNodeHandlerRegistry(), repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{RunID: run.ID, PipelineID: 9999})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(ctx, task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load pipeline")

	// 验证 run 被标记为 failed
	updated, err := repos.Pipeline.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", updated.Status)
}

// TestNewAsynqHandler_EmptyDAG 验证空 DAG 时标记 run 为 failed
func TestNewAsynqHandler_EmptyDAG(t *testing.T) {
	repos := newTestReposForAsynq(t)
	ctx := context.Background()

	pl := &model.Pipeline{Name: "empty", DAG: model.JSON("")}
	require.NoError(t, repos.Pipeline.Create(ctx, pl))

	run := &model.PipelineRun{PipelineID: pl.ID, Status: "queued"}
	require.NoError(t, repos.Pipeline.CreateRun(ctx, run))

	runner := NewRunner(NewNodeHandlerRegistry(), repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{RunID: run.ID, PipelineID: pl.ID})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(ctx, task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty dag")

	updated, err := repos.Pipeline.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", updated.Status)
}

// TestNewAsynqHandler_QueuedToRunning 验证 queued 状态被转为 running
func TestNewAsynqHandler_QueuedToRunning(t *testing.T) {
	repos := newTestReposForAsynq(t)
	ctx := context.Background()

	// 注册一个简单 noop handler
	reg := NewNodeHandlerRegistry()
	reg.Register("noop", func(nc *NodeContext) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	})

	pl := &model.Pipeline{Name: "test", DAG: model.JSON(`{"nodes":[{"id":"a","type":"noop"}]}`)}
	require.NoError(t, repos.Pipeline.Create(ctx, pl))

	run := &model.PipelineRun{PipelineID: pl.ID, Status: "queued"}
	require.NoError(t, repos.Pipeline.CreateRun(ctx, run))

	runner := NewRunner(reg, repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{RunID: run.ID, PipelineID: pl.ID})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(ctx, task)
	require.NoError(t, err)

	updated, err := repos.Pipeline.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", updated.Status)
}

// TestNewAsynqHandler_RunnerFailure 验证 runner 失败时 run 被标记为 failed
func TestNewAsynqHandler_RunnerFailure(t *testing.T) {
	repos := newTestReposForAsynq(t)
	ctx := context.Background()

	reg := NewNodeHandlerRegistry()
	reg.Register("fail", func(nc *NodeContext) (map[string]any, error) {
		return nil, errors.New("node boom")
	})

	pl := &model.Pipeline{Name: "test", DAG: model.JSON(`{"nodes":[{"id":"a","type":"fail"}]}`)}
	require.NoError(t, repos.Pipeline.Create(ctx, pl))

	run := &model.PipelineRun{PipelineID: pl.ID, Status: "queued"}
	require.NoError(t, repos.Pipeline.CreateRun(ctx, run))

	runner := NewRunner(reg, repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{RunID: run.ID, PipelineID: pl.ID})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(ctx, task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node boom")

	updated, err := repos.Pipeline.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", updated.Status)
}

// TestNewAsynqHandler_InputOverrides 验证 input 与 overrides 合并逻辑
func TestNewAsynqHandler_InputOverrides(t *testing.T) {
	repos := newTestReposForAsynq(t)
	ctx := context.Background()

	var capturedInput map[string]any
	reg := NewNodeHandlerRegistry()
	reg.Register("capture", func(nc *NodeContext) (map[string]any, error) {
		capturedInput = nc.Input
		return map[string]any{}, nil
	})

	pl := &model.Pipeline{Name: "test", DAG: model.JSON(`{"nodes":[{"id":"a","type":"capture"}]}`)}
	require.NoError(t, repos.Pipeline.Create(ctx, pl))

	run := &model.PipelineRun{PipelineID: pl.ID, Status: "queued"}
	require.NoError(t, repos.Pipeline.CreateRun(ctx, run))

	runner := NewRunner(reg, repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{
		RunID:      run.ID,
		PipelineID: pl.ID,
		Input:      map[string]any{"base": "from_input", "shared": "input_val"},
		Overrides:  map[string]any{"override": "from_override", "shared": "override_val"},
	})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(ctx, task)
	require.NoError(t, err)

	require.NotNil(t, capturedInput)
	assert.Equal(t, "from_input", capturedInput["base"])
	assert.Equal(t, "from_override", capturedInput["override"])
	// overrides 应覆盖 input 中同名 key
	assert.Equal(t, "override_val", capturedInput["shared"])
}

// TestNewAsynqHandler_OutputSaved 验证 runner 输出被写回 run.output
func TestNewAsynqHandler_OutputSaved(t *testing.T) {
	repos := newTestReposForAsynq(t)
	ctx := context.Background()

	reg := NewNodeHandlerRegistry()
	reg.Register("produce", func(nc *NodeContext) (map[string]any, error) {
		return map[string]any{"result": 42}, nil
	})

	pl := &model.Pipeline{Name: "test", DAG: model.JSON(`{"nodes":[{"id":"a","type":"produce"}]}`)}
	require.NoError(t, repos.Pipeline.Create(ctx, pl))

	run := &model.PipelineRun{PipelineID: pl.ID, Status: "queued"}
	require.NoError(t, repos.Pipeline.CreateRun(ctx, run))

	runner := NewRunner(reg, repos, nil, zap.NewNop())
	handler := NewAsynqHandler(repos, runner, zap.NewNop())

	payload, _ := json.Marshal(RunPayload{RunID: run.ID, PipelineID: pl.ID})
	task := asynq.NewTask(TaskPipelineRun, payload)
	err := handler(ctx, task)
	require.NoError(t, err)

	updated, err := repos.Pipeline.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", updated.Status)
	assert.NotNil(t, updated.Output)
	var out map[string]any
	require.NoError(t, json.Unmarshal(updated.Output, &out))
	nodes, ok := out["nodes"].(map[string]any)
	require.True(t, ok)
	aOut, ok := nodes["a"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(42), aOut["result"])
}
