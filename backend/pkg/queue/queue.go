package queue

import (
	"context"
	"errors"
	"time"

	"github.com/godfreygan/ai-script/backend/pkg/metrics"
	"github.com/hibiken/asynq"
)

// TaskClient 是投递任务的最小接口，便于测试 mock
type TaskClient interface {
	Enqueue(ctx context.Context, taskType string, payload []byte, opts ...asynq.Option) (string, error)
	EnqueueIn(ctx context.Context, taskType string, payload []byte, delay time.Duration) (string, error)
	Ping() error
}

var _ TaskClient = (*Client)(nil)

// Client 用于业务侧投递任务
type Client struct {
	c        *asynq.Client
	redisOpt asynq.RedisConnOpt
}

func NewClient(addr, password string, db int) *Client {
	opt := asynq.RedisClientOpt{Addr: addr, Password: password, DB: db}
	return &Client{
		c:        asynq.NewClient(opt),
		redisOpt: opt,
	}
}

func (c *Client) Enqueue(ctx context.Context, taskType string, payload []byte, opts ...asynq.Option) (string, error) {
	task := asynq.NewTask(taskType, payload)
	info, err := c.c.EnqueueContext(ctx, task, opts...)
	if err != nil {
		return "", err
	}
	return info.ID, nil
}

func (c *Client) EnqueueIn(ctx context.Context, taskType string, payload []byte, delay time.Duration) (string, error) {
	return c.Enqueue(ctx, taskType, payload, asynq.ProcessIn(delay))
}

// NoopTaskClient 是一个空实现的 TaskClient,用于测试或无需投递任务的场景。
type NoopTaskClient struct{}

func (n *NoopTaskClient) Enqueue(ctx context.Context, taskType string, payload []byte, opts ...asynq.Option) (string, error) {
	return "", nil
}
func (n *NoopTaskClient) EnqueueIn(ctx context.Context, taskType string, payload []byte, delay time.Duration) (string, error) {
	return "", nil
}
func (n *NoopTaskClient) Ping() error { return nil }

// Ping 探测 asynq 底层 Redis 连接是否可用
func (c *Client) Ping() error {
	if c.c == nil {
		return errors.New("queue client not initialized")
	}
	// 用 asynq.Inspector 验证连接
	inspector := asynq.NewInspector(c.redisOpt)
	defer inspector.Close()
	_, err := inspector.Queues()
	return err
}

// Server 用于 worker
type Server struct {
	s   *asynq.Server
	mux *asynq.ServeMux
}

// NewAsynqServer 创建 asynq server。concurrency <= 0 时使用默认值 16。
func NewAsynqServer(addr, password string, db int, concurrency int) *Server {
	if concurrency <= 0 {
		concurrency = 16
	}
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: addr, Password: password, DB: db},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			// 修复 P0 F3 — 优雅关闭超时:正在执行的任务超过 60s 则强制终止
			ShutdownTimeout: 60 * time.Second,
		},
	)
	return &Server{s: srv, mux: asynq.NewServeMux()}
}

// HandlerRegistry 是 pipeline 节点的处理器注册中心
type HandlerRegistry interface {
	Handlers() map[string]asynq.HandlerFunc
}

func (s *Server) RegisterHandlers(r HandlerRegistry) {
	for k, v := range r.Handlers() {
		s.mux.HandleFunc(k, v)
	}
}

func (s *Server) Run() error { return s.s.Run(s.mux) }

// Shutdown 封装 asynq 关闭,支持外部 context 超时控制 — 修复 P0 F3
func (s *Server) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.s.Shutdown()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop 立即停止 asynq server,不等待任务完成
func (s *Server) Stop() {
	s.s.Stop()
}

// MetricsCollector 定期通过 asynq.Inspector 采集队列统计并更新到 metrics。
type MetricsCollector struct {
	inspector *asynq.Inspector
	interval  time.Duration
	stopCh    chan struct{}
}

// NewMetricsCollector 创建队列指标采集器,interval 为采集周期(建议 10-30s)。
func NewMetricsCollector(redisOpt asynq.RedisConnOpt, interval time.Duration) *MetricsCollector {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &MetricsCollector{
		inspector: asynq.NewInspector(redisOpt),
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

// Start 启动后台定时采集 goroutine。
func (mc *MetricsCollector) Start() {
	go mc.loop()
}

// Stop 停止采集循环。
func (mc *MetricsCollector) Stop() {
	close(mc.stopCh)
	mc.inspector.Close()
}

func (mc *MetricsCollector) loop() {
	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()

	mc.collect()
	for {
		select {
		case <-ticker.C:
			mc.collect()
		case <-mc.stopCh:
			return
		}
	}
}

func (mc *MetricsCollector) collect() {
	queues, err := mc.inspector.Queues()
	if err != nil {
		return
	}
	for _, q := range queues {
		info, err := mc.inspector.GetQueueInfo(q)
		if err != nil {
			continue
		}
		metrics.QueueDepth.Set(map[string]string{"queue": q}, int64(info.Size))
	}
}
