package server

import (
	"github.com/godfreygan/ai-script/backend/internal/adapter"
	"github.com/godfreygan/ai-script/backend/internal/conf"
	"github.com/godfreygan/ai-script/backend/internal/handler"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/internal/service"
	pkgcasbin "github.com/godfreygan/ai-script/backend/pkg/casbin"
	"github.com/godfreygan/ai-script/backend/pkg/crypto"
	"github.com/godfreygan/ai-script/backend/pkg/jwt"
	"github.com/godfreygan/ai-script/backend/pkg/queue"
	"github.com/godfreygan/ai-script/backend/pkg/storage"
	"github.com/godfreygan/ai-script/backend/pkg/ws"
	"github.com/casbin/casbin/v2"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ProviderSet aggregates all server-level providers for Wire.
var ProviderSet = wire.NewSet(
	newDB,
	newRedis,
	ProvideCasbinModelPath,
	ProvideCasbin,
	ProvideJWTManager,
	ProvideCipher,
	ProvideTaskClient,
	ProvideRegistry,
	ProvideStorage,
	ProvideHub,
	repo.ProviderSet,
	service.ProviderSet,
	handler.ProviderSet,
	NewApp,
	newRouter,
)

// ProvideCasbinModelPath returns the hard-coded Casbin model configuration path.
func ProvideCasbinModelPath() string {
	return "./configs/rbac_model.conf"
}

// ProvideCasbin builds the Casbin enforcer.
func ProvideCasbin(db *gorm.DB, modelPath string) (*casbin.Enforcer, error) {
	return pkgcasbin.New(db, modelPath)
}

// ProvideJWTManager builds the JWT manager from config.
func ProvideJWTManager(cfg *conf.Config) *jwt.Manager {
	return jwt.NewManager(cfg.JWT.Secret, cfg.JWT.AccessExpiresIn, cfg.JWT.RefreshExpiresIn)
}

// ProvideCipher builds the crypto cipher from config.
// Validate() already decodes CRYPTO_KEY_BASE64 into Crypto.Key.
func ProvideCipher(cfg *conf.Config) (*crypto.Cipher, error) {
	return crypto.NewFromBytes([]byte(cfg.Crypto.Key))
}

// ProvideTaskClient builds the Asynq task client from config.
func ProvideTaskClient(cfg *conf.Config) queue.TaskClient {
	return queue.NewClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
}

// ProvideRegistry builds a new adapter registry.
func ProvideRegistry() *adapter.Registry {
	return adapter.NewRegistry()
}

// ProvideStorage builds the storage backend from config.
func ProvideStorage(cfg *conf.Config) (storage.Storage, error) {
	return storage.New(storage.FactoryConfig{
		Provider:   cfg.Storage.Provider,
		Endpoint:   cfg.Storage.Endpoint,
		Region:     cfg.Storage.Region,
		Bucket:     cfg.Storage.Bucket,
		AccessKey:  cfg.Storage.AccessKey,
		SecretKey:  cfg.Storage.SecretKey,
		PublicHost: cfg.Storage.PublicHost,
	})
}

// ProvideHub builds the WebSocket hub and binds Redis Pub/Sub.
func ProvideHub(cfg *conf.Config, log *zap.Logger, rdb *redis.Client) *ws.Hub {
	hub := ws.NewHub(log, cfg.App.Origins)
	hub.BindRedis(rdb, "")
	return hub
}
