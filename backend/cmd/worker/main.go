// Worker 是异步任务消费者:从 Redis (Asynq) 拉取任务,执行 LLM 调用/媒体合成等耗时操作,
// 通过 ws.Hub 的 Redis Pub/Sub 桥接把进度推送到 server 进程的 WebSocket 连接。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/adapter"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/conf"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/pipeline"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	pkgcasbin "git.myscrm.cn/ganqx01/ai-script/backend/pkg/casbin"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/crypto"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/jwt"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/logger"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/queue"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/storage"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/ws"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
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

	db, err := newDB(cfg)
	if err != nil {
		log.Fatal("init db failed", zap.Error(err))
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	enforcer, err := pkgcasbin.New(db, "./configs/rbac_model.conf")
	if err != nil {
		log.Warn("init casbin failed", zap.Error(err))
	}

	jwtMgr := jwt.NewManager(cfg.JWT.Secret, cfg.JWT.AccessExpiresIn, cfg.JWT.RefreshExpiresIn)
	cipher, err := crypto.New(cfg.Crypto.Key)
	if err != nil {
		log.Fatal("init cipher failed", zap.Error(err))
	}
	taskClient := queue.NewClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	registry := adapter.NewRegistry()

	store, err := storage.New(storage.FactoryConfig{
		Provider:   cfg.Storage.Provider,
		Endpoint:   cfg.Storage.Endpoint,
		Region:     cfg.Storage.Region,
		Bucket:     cfg.Storage.Bucket,
		AccessKey:  cfg.Storage.AccessKey,
		SecretKey:  cfg.Storage.SecretKey,
		PublicHost: cfg.Storage.PublicHost,
	})
	if err != nil {
		log.Fatal("init storage failed", zap.Error(err))
	}

	// hub 绑定 Redis,这样 worker 端 publish 的事件能被 server 端订阅到并推给 WS 客户端
	hub := ws.NewHub(log)
	hub.BindRedis(rdb, "")
	hubCtx, cancelHub := context.WithCancel(context.Background())
	go hub.Run(hubCtx)
	defer cancelHub()

	repos := repo.NewRepositories(db, rdb)
	services := service.NewServices(repos, jwtMgr, enforcer, cipher, registry, taskClient, hub, store, log)

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
	})
	runner := pipeline.NewRunner(nodeReg, repos, hub, log)

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
	handlerReg.Register(service.TaskPipelineRun, pipeline.NewAsynqHandler(repos, runner, log))

	q := queue.NewAsynqServer(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	q.RegisterHandlers(handlerReg)

	go func() {
		if err := q.Run(); err != nil {
			log.Fatal("worker crashed", zap.Error(err))
		}
	}()
	log.Info("worker started",
		zap.String("redis", cfg.Redis.Addr),
		zap.Int("redis_db", cfg.Redis.DB),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	q.Shutdown()
	cancelHub()
	log.Info("worker exited")
}

func newDB(cfg *conf.Config) (*gorm.DB, error) {
	if cfg.MySQL.DSN == "" {
		return nil, fmt.Errorf("mysql dsn empty")
	}
	db, err := gorm.Open(mysql.Open(cfg.MySQL.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}
