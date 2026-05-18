//go:build wireinject
// +build wireinject

package main

import (
	"github.com/godfreygan/ai-script/backend/internal/conf"
	"github.com/google/wire"
	"go.uber.org/zap"
)

// InitializeWorker 是 worker 的 Wire 注入器。
func InitializeWorker(cfg *conf.Config, log *zap.Logger) (*WorkerDeps, error) {
	wire.Build(WorkerProviderSet)
	return nil, nil
}
