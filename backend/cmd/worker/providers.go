package main

import (
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/adapter"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/server"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/storage"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/ws"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// WorkerDeps 组装 worker 所需的基础设施依赖。
type WorkerDeps struct {
	DB       *gorm.DB
	RDB      *redis.Client
	Repos    *repo.Repositories
	Services *service.Services
	Registry *adapter.Registry
	Store    storage.Storage
	Hub      *ws.Hub
}

// WorkerProviderSet 暴露 worker 级 Wire Provider。
var WorkerProviderSet = wire.NewSet(
	server.NewDB,
	server.NewRedis,
	server.ProvideCasbinModelPath,
	server.ProvideCasbin,
	server.ProvideJWTManager,
	server.ProvideCipher,
	server.ProvideTaskClient,
	server.ProvideRegistry,
	server.ProvideStorage,
	server.ProvideHub,
	repo.NewRepositories,
	service.NewServices,
	wire.Struct(new(WorkerDeps), "*"),
)
