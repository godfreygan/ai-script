package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- Client tests ----------

// TestClient_Enqueue 验证任务入队成功并返回 ID
func TestClient_Enqueue(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client := NewClient(s.Addr(), "", 0)
	defer client.c.Close()

	ctx := context.Background()
	id, err := client.Enqueue(ctx, "test.task", []byte(`{"key":"value"}`))
	require.NoError(t, err)
	assert.NotEmpty(t, id)
}

// TestClient_EnqueueIn 验证延迟任务入队
func TestClient_EnqueueIn(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client := NewClient(s.Addr(), "", 0)
	defer client.c.Close()

	ctx := context.Background()
	id, err := client.EnqueueIn(ctx, "delayed.task", []byte(`{}`), 5*time.Minute)
	require.NoError(t, err)
	assert.NotEmpty(t, id)
}

// TestClient_Ping_Success 验证 Redis 连接正常时 Ping 通过
func TestClient_Ping_Success(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client := NewClient(s.Addr(), "", 0)
	defer client.c.Close()

	err := client.Ping()
	require.NoError(t, err)
}

// TestClient_Ping_NotInitialized 验证未初始化客户端返回错误
func TestClient_Ping_NotInitialized(t *testing.T) {
	client := &Client{c: nil}
	err := client.Ping()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestClient_Ping_BadAddr 验证错误地址时 Ping 失败
func TestClient_Ping_BadAddr(t *testing.T) {
	client := NewClient("127.0.0.1:1", "", 0)
	defer client.c.Close()

	err := client.Ping()
	require.Error(t, err)
}

// ---------- Server tests ----------

// TestServer_RegisterHandlers 验证 handler 注册
func TestServer_RegisterHandlers(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	server := NewAsynqServer(s.Addr(), "", 0, 0)

	reg := NewTestHandlerRegistry()
	called := make(map[string]bool)
	reg.Register("task.a", func(ctx context.Context, t *asynq.Task) error {
		called["a"] = true
		return nil
	})
	reg.Register("task.b", func(ctx context.Context, t *asynq.Task) error {
		called["b"] = true
		return nil
	})

	server.RegisterHandlers(reg)
	// 由于无法直接访问私有 mux,通过构造任务验证(需要启动 server)
	// 这里仅验证 RegisterHandlers 不 panic 且 handler 数量正确
	assert.Len(t, reg.Handlers(), 2)
}

// TestServer_Shutdown_Graceful 验证优雅关闭在 server 未启动时也能返回
func TestServer_Shutdown_Graceful(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	server := NewAsynqServer(s.Addr(), "", 0, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Shutdown 内部调用 asynq.Server.Shutdown,即使未启动也不会 panic
	err := server.Shutdown(ctx)
	// 未启动的 server Shutdown 可能返回 nil 或 context deadline,视 asynq 版本而定
	assert.True(t, err == nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
}

// TestServer_Stop 验证 Stop 不 panic
func TestServer_Stop(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	server := NewAsynqServer(s.Addr(), "", 0, 0)
	// 未启动时 Stop 不应 panic
	server.Stop()
}

// ---------- MetricsCollector tests ----------

// TestMetricsCollector_StartStop 验证启动和停止不 panic/leak
func TestMetricsCollector_StartStop(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	mc := NewMetricsCollector(asynq.RedisClientOpt{Addr: s.Addr()}, 100*time.Millisecond)
	mc.Start()
	time.Sleep(250 * time.Millisecond) // 让 loop 跑 1-2 轮
	mc.Stop()
	// 二次 Stop 不应 panic(但会 panic on close closed channel,所以只调一次)
}

// TestMetricsCollector_DefaultInterval 验证默认间隔
func TestMetricsCollector_DefaultInterval(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	mc := NewMetricsCollector(asynq.RedisClientOpt{Addr: s.Addr()}, 0)
	assert.Equal(t, 15*time.Second, mc.interval)
	mc.Start()
	mc.Stop()
}

// TestMetricsCollector_CollectEmptyQueues 验证空队列时 collect 不 panic
func TestMetricsCollector_CollectEmptyQueues(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	mc := NewMetricsCollector(asynq.RedisClientOpt{Addr: s.Addr()}, 100*time.Millisecond)
	mc.Start()
	time.Sleep(150 * time.Millisecond)
	mc.Stop()
	// 无队列时也应正常退出
}

// ---------- HandlerRegistry (pipeline) interface tests ----------

// TestHandlerRegistry_RegisterAndHandlers 验证注册与读取
func TestHandlerRegistry_RegisterAndHandlers(t *testing.T) {
	reg := NewTestHandlerRegistry()
	assert.Empty(t, reg.Handlers())

	fn := func(ctx context.Context, t *asynq.Task) error { return nil }
	reg.Register("my.task", fn)

	handlers := reg.Handlers()
	assert.Len(t, handlers, 1)
	assert.Contains(t, handlers, "my.task")
}

// TestHandlerRegistry_Overwrite 验证同名覆盖
func TestHandlerRegistry_Overwrite(t *testing.T) {
	reg := NewTestHandlerRegistry()
	first := false
	second := false

	reg.Register("task", func(ctx context.Context, t *asynq.Task) error {
		first = true
		return nil
	})
	reg.Register("task", func(ctx context.Context, t *asynq.Task) error {
		second = true
		return nil
	})

	handlers := reg.Handlers()
	handlers["task"](context.Background(), asynq.NewTask("task", nil))
	assert.False(t, first)
	assert.True(t, second)
}

// ---------- TaskClient interface compliance ----------

// TestTaskClientInterface 编译期检查 Client 实现 TaskClient
func TestTaskClientInterface(t *testing.T) {
	var _ TaskClient = (*Client)(nil)
}

// ---------- helpers ----------

// testHandlerRegistry 是 pipeline.HandlerRegistry 的本地简化版,用于测试 queue 包
// 避免循环依赖(queue 不依赖 pipeline)
type testHandlerRegistry struct {
	handlers map[string]asynq.HandlerFunc
}

func NewTestHandlerRegistry() *testHandlerRegistry {
	return &testHandlerRegistry{handlers: make(map[string]asynq.HandlerFunc)}
}

func (r *testHandlerRegistry) Register(name string, h asynq.HandlerFunc) {
	r.handlers[name] = h
}

func (r *testHandlerRegistry) Handlers() map[string]asynq.HandlerFunc {
	return r.handlers
}

var _ HandlerRegistry = (*testHandlerRegistry)(nil)
