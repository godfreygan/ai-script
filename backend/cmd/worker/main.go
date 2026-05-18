// Worker 是异步任务消费者:从 Redis (Asynq) 拉取任务,执行 LLM 调用/媒体合成等耗时操作,
// 通过 ws.Hub 的 Redis Pub/Sub 桥接把进度推送到 server 进程的 WebSocket 连接。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/conf"
	"github.com/godfreygan/ai-script/backend/internal/pipeline"
	"github.com/godfreygan/ai-script/backend/internal/service"
	"github.com/godfreygan/ai-script/backend/pkg/logger"
	"github.com/godfreygan/ai-script/backend/pkg/metrics"
	"github.com/godfreygan/ai-script/backend/pkg/queue"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

func main() {
	cfg, err := conf.Load("./configs/config.yaml")
	if err != nil {
		panic(fmt.Sprintf("load config failed: %v", err))
	}
	log, err := logger.New(cfg.App.LogLevel, cfg.App.Env)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	deps, err := InitializeWorker(cfg, log)
	if err != nil {
		log.Fatal("init worker deps failed", zap.Error(err))
	}
	rdb := deps.RDB
	hub := deps.Hub
	repos := deps.Repos
	services := deps.Services
	store := deps.Store

	hubCtx, cancelHub := context.WithCancel(context.Background())
	go hub.Run(hubCtx)
	defer cancelHub()

	// 加载所有 enabled 模型 adapter
	loadCtx, lcancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := services.Model.LoadAllAdapters(loadCtx); err != nil {
		log.Warn("worker: load adapters failed", zap.Error(err))
	}
	lcancel()

	// DAG runner:节点处理器注册表 + Runner,worker 同进程内执行 pipeline 节点
	nodeReg := pipeline.NewNodeHandlerRegistry()
	pipeline.RegisterDefaultNodeHandlers(nodeReg, &pipeline.DefaultDeps{
		Repos:      repos,
		GetAdapter: services.Model.GetAdapter,
		Store:      store,
	})
	runner := pipeline.NewRunner(nodeReg, repos, hub, log, pipeline.WithMaxConcurrency(10))

	// 注入 Full / Pipeline 运行期依赖(server 端 NewServices 时无法拿到 runner)
	services.Full.SetDeps(hub, store, services.Model, services.Invoke)
	services.Pipeline.SetDeps(hub, runner)

	// 构造 asynq 处理器注册表:先填默认 noop,再用真实实现覆盖
	handlerReg := pipeline.NewHandlerRegistry()
	pipeline.RegisterDefaults(handlerReg)
	handlerReg.Register(service.TaskScriptSplit, services.Script.HandleSplitTask(services.Model))
	handlerReg.Register(service.TaskPromptGenerate, services.Prompt.HandleGenerateTask(services.Model))
	handlerReg.Register(service.TaskStoryboardGenerate, services.Story.HandleGenerateTask(services.Model))
	handlerReg.Register(service.TaskImageGenerate, services.Image.HandleGenerateTask(services.Model, services.Invoke))
	handlerReg.Register(service.TaskVideoGenerate, services.Short.HandleGenerateTask(services.Model, services.Invoke))
	handlerReg.Register(service.TaskVideoCompose, services.Full.HandleComposeTask())
	handlerReg.Register(service.TaskPipelineRun, services.Pipeline.HandleRunTask())

	q := queue.NewAsynqServer(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, cfg.Queue.Concurrency)
	q.RegisterHandlers(wrapHandlersWithMetrics(handlerReg))

	// 启动队列深度采集器
	collector := queue.NewMetricsCollector(
		asynq.RedisClientOpt{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB},
		15*time.Second,
	)
	collector.Start()
	defer collector.Stop()

	// 记录 worker 并发数
	metrics.WorkerRunning.Set(16)

	go func() {
		if err := q.Run(); err != nil {
			log.Fatal("worker crashed", zap.Error(err))
		}
	}()

	// 启动 Worker HTTP 探针:供 K8s liveness/readiness 探测 + Prometheus 指标
	healthPort := cfg.App.Port + 1000
	mux := http.NewServeMux()
	healthSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", healthPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	mux.HandleFunc("/healthz/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/healthz/ready", func(w http.ResponseWriter, r *http.Request) {
		if rdb != nil {
			if err := rdb.Ping(r.Context()).Err(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("redis_unavailable"))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(metrics.FormatPrometheus()))
	})
	go func() {
		log.Info("worker health server started", zap.Int("port", healthPort))
		if err := healthSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("health server crashed", zap.Error(err))
		}
	}()

	log.Info("worker started",
		zap.String("redis", cfg.Redis.Addr),
		zap.Int("redis_db", cfg.Redis.DB),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit

	log.Info("worker shutting down...")
	// 修复 P0 F3 — 优雅关闭:先停 asynq(等任务完成或超时),再关 hub
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := q.Shutdown(ctx); err != nil {
		log.Error("worker forced shutdown", zap.Error(err))
	}
	// 关闭 health server
	hcancel, hcancelCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer hcancelCancel()
	_ = healthSrv.Shutdown(hcancel)
	cancelHub()
	metrics.WorkerRunning.Set(0)
	log.Info("worker exited")
}

// wrapHandlersWithMetrics 为每个 handler 包装指标记录:处理前后计数 + 耗时 histogram。
func wrapHandlersWithMetrics(reg *pipeline.HandlerRegistry) *pipeline.HandlerRegistry {
	wrapped := pipeline.NewHandlerRegistry()
	for taskType, h := range reg.Handlers() {
		tt := taskType
		fn := h
		wrapped.Register(tt, func(ctx context.Context, t *asynq.Task) error {
			start := time.Now()
			err := fn(ctx, t)
			dur := time.Since(start)
			status := "success"
			if err != nil {
				status = "failure"
			}
			metrics.RecordTaskProcessed(tt, status)
			metrics.RecordTaskLatency(tt, dur)
			return err
		})
	}
	return wrapped
}

