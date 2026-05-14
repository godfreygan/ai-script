package pipeline

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- HandlerRegistry tests ----------

// TestHandlerRegistry_NewIsEmpty 验证新注册表为空
func TestHandlerRegistry_NewIsEmpty(t *testing.T) {
	reg := NewHandlerRegistry()
	assert.NotNil(t, reg)
	assert.Empty(t, reg.Handlers())
}

// TestHandlerRegistry_RegisterAndGet 验证注册后可通过 Handlers() 读取
func TestHandlerRegistry_RegisterAndGet(t *testing.T) {
	reg := NewHandlerRegistry()
	called := false
	reg.Register("task.foo", func(ctx context.Context, t *asynq.Task) error {
		called = true
		return nil
	})

	handlers := reg.Handlers()
	require.Len(t, handlers, 1)
	assert.Contains(t, handlers, "task.foo")

	// 执行验证
	err := handlers["task.foo"](context.Background(), asynq.NewTask("task.foo", []byte("{}")))
	require.NoError(t, err)
	assert.True(t, called)
}

// TestHandlerRegistry_RegisterMultiple 验证注册多个 handler
func TestHandlerRegistry_RegisterMultiple(t *testing.T) {
	reg := NewHandlerRegistry()
	for _, name := range []string{"a", "b", "c", "d"} {
		name := name
		reg.Register(name, func(ctx context.Context, t *asynq.Task) error {
			return nil
		})
	}
	assert.Len(t, reg.Handlers(), 4)
}

// TestHandlerRegistry_Overwrite 验证同名 handler 被覆盖
func TestHandlerRegistry_Overwrite(t *testing.T) {
	reg := NewHandlerRegistry()
	firstCalled := false
	secondCalled := false

	reg.Register("task.x", func(ctx context.Context, t *asynq.Task) error {
		firstCalled = true
		return nil
	})
	reg.Register("task.x", func(ctx context.Context, t *asynq.Task) error {
		secondCalled = true
		return nil
	})

	handlers := reg.Handlers()
	require.Len(t, handlers, 1)
	handlers["task.x"](context.Background(), asynq.NewTask("task.x", nil))
	assert.False(t, firstCalled)
	assert.True(t, secondCalled)
}

// TestHandlerRegistry_HandlersReturnsCopy 验证 Handlers() 返回的是引用(当前实现为直接返回 map)
func TestHandlerRegistry_HandlersReturnsCopy(t *testing.T) {
	reg := NewHandlerRegistry()
	reg.Register("a", func(ctx context.Context, t *asynq.Task) error { return nil })

	h1 := reg.Handlers()
	h1["b"] = func(ctx context.Context, t *asynq.Task) error { return nil }

	// Handlers() 返回深拷贝,修改不应影响原注册表
	h2 := reg.Handlers()
	assert.NotContains(t, h2, "b")
}

// ---------- RegisterDefaults tests ----------

// TestRegisterDefaults_CoversAllNodes 验证默认注册覆盖所有节点类型
func TestRegisterDefaults_CoversAllNodes(t *testing.T) {
	reg := NewHandlerRegistry()
	RegisterDefaults(reg)

	handlers := reg.Handlers()
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
		TaskPipelineRun,
	}

	for _, name := range expected {
		assert.Contains(t, handlers, name, "missing default handler for %s", name)
	}
	assert.Len(t, handlers, len(expected))
}

// TestRegisterDefaults_NoPanicOnDoubleCall 验证重复调用不 panic
func TestRegisterDefaults_NoPanicOnDoubleCall(t *testing.T) {
	reg := NewHandlerRegistry()
	RegisterDefaults(reg)
	RegisterDefaults(reg) // 不应 panic
	assert.Len(t, reg.Handlers(), 12)
}

// TestNoopHandler_ReturnsNil 验证 NoopHandler 总是返回 nil
func TestNoopHandler_ReturnsNil(t *testing.T) {
	for _, name := range []string{"foo", "bar", TaskPipelineRun} {
		name := name
		t.Run(name, func(t *testing.T) {
			h := NoopHandler(name)
			err := h(context.Background(), asynq.NewTask(name, []byte(`{"test":true}`)))
			assert.NoError(t, err)
		})
	}
}

// TestNoopHandler_IgnoresPayload 验证 NoopHandler 忽略 payload 内容
func TestNoopHandler_IgnoresPayload(t *testing.T) {
	h := NoopHandler("ignore.me")
	err := h(context.Background(), asynq.NewTask("ignore.me", []byte("not valid json")))
	assert.NoError(t, err)

	err = h(context.Background(), asynq.NewTask("ignore.me", nil))
	assert.NoError(t, err)
}
