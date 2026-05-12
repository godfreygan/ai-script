package queue

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
)

// Client 用于业务侧投递任务
type Client struct {
	c *asynq.Client
}

func NewClient(addr, password string, db int) *Client {
	return &Client{
		c: asynq.NewClient(asynq.RedisClientOpt{Addr: addr, Password: password, DB: db}),
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

// Server 用于 worker
type Server struct {
	s   *asynq.Server
	mux *asynq.ServeMux
}

func NewAsynqServer(addr, password string, db int) *Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: addr, Password: password, DB: db},
		asynq.Config{
			Concurrency: 16,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
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

func (s *Server) Run() error           { return s.s.Run(s.mux) }
func (s *Server) Shutdown()            { s.s.Shutdown() }
