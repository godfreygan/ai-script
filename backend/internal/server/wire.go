//go:build wireinject
// +build wireinject

package server

import (
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/conf"
	"github.com/google/wire"
	"go.uber.org/zap"
)

// InitializeApp is the Wire injector for assembling the HTTP server application.
func InitializeApp(cfg *conf.Config, log *zap.Logger) (*App, error) {
	wire.Build(ProviderSet)
	return nil, nil
}
