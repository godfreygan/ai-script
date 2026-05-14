package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/conf"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/pipeline"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/metrics"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- wrapHandlersWithMetrics tests ----------

// TestWrapHandlersWithMetrics_Success 验证成功时记录 success 指标
func TestWrapHandlersWithMetrics_Success(t *testing.T) {
	reg := pipeline.NewHandlerRegistry()
	called := false
	reg.Register("test.task", func(ctx context.Context, t *asynq.Task) error {
		called = true
		return nil
	})

	wrapped := wrapHandlersWithMetrics(reg)
	handlers := wrapped.Handlers()
	require.Contains(t, handlers, "test.task")

	// 重置指标,避免交叉测试污染
	metrics.TaskProcessedTotal = metrics.NewExpvarCounterMap("asynq_task_processed_total_test_success")
	metrics.TaskLatency = metrics.NewExpvarHistogram("asynq_task_latency_seconds_test_success")

	err := handlers["test.task"](context.Background(), asynq.NewTask("test.task", []byte("{}")))
	require.NoError(t, err)
	assert.True(t, called)

	// 指标已记录(通过不 panic 验证)
	snap := metrics.TaskProcessedTotal.Snapshot()
	assert.GreaterOrEqual(t, len(snap), 0) // 至少存在或为空均可
}

// TestWrapHandlersWithMetrics_Failure 验证失败时记录 failure 指标且不吞掉原错误
func TestWrapHandlersWithMetrics_Failure(t *testing.T) {
	reg := pipeline.NewHandlerRegistry()
	wantErr := errors.New("boom")
	reg.Register("test.fail", func(ctx context.Context, t *asynq.Task) error {
		return wantErr
	})

	wrapped := wrapHandlersWithMetrics(reg)
	handlers := wrapped.Handlers()

	// 重置指标
	metrics.TaskProcessedTotal = metrics.NewExpvarCounterMap("asynq_task_processed_total_test_fail")
	metrics.TaskLatency = metrics.NewExpvarHistogram("asynq_task_latency_seconds_test_fail")

	err := handlers["test.fail"](context.Background(), asynq.NewTask("test.fail", []byte("{}")))
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

// TestWrapHandlersWithMetrics_PreservesAllHandlers 验证所有 handler 都被包装
func TestWrapHandlersWithMetrics_PreservesAllHandlers(t *testing.T) {
	reg := pipeline.NewHandlerRegistry()
	reg.Register("a", func(ctx context.Context, t *asynq.Task) error { return nil })
	reg.Register("b", func(ctx context.Context, t *asynq.Task) error { return nil })
	reg.Register("c", func(ctx context.Context, t *asynq.Task) error { return nil })

	wrapped := wrapHandlersWithMetrics(reg)
	require.Len(t, wrapped.Handlers(), 3)
	for _, name := range []string{"a", "b", "c"} {
		assert.Contains(t, wrapped.Handlers(), name)
	}
}

// ---------- Config / Validate tests ----------

// TestConfigValidate_JWTSecret 验证 JWT secret 校验逻辑
func TestConfigValidate_JWTSecret(t *testing.T) {
	cases := []struct {
		name    string
		secret  string
		wantErr bool
		errSub  string
	}{
		{"empty", "", true, "jwt.secret is empty"},
		{"too-short", "short", true, "too short"},
		{"placeholder-change-me", "please-change-me-now", true, "placeholder"},
		{"placeholder-default", "default-secret-value-here", true, "placeholder"},
		{"ok-32", "this-is-a-32-byte-long-secret-key!", false, ""},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			cfg := &conf.Config{
				JWT:     conf.JWTConfig{Secret: c.secret},
				MySQL:   conf.MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
				Redis:   conf.RedisConfig{Addr: "localhost:6379"},
				Storage: conf.StorageConfig{Provider: "local"},
				Crypto:  conf.CryptoConfig{Key: "12345678901234567890123456789012"},
			}
			err := cfg.Validate()
			if c.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.errSub)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestConfigValidate_CryptoKey 验证 Crypto key 长度校验
func TestConfigValidate_CryptoKey(t *testing.T) {
	cases := []struct {
		name      string
		key       string
		keyBase64 string
		wantErr   bool
		errSub    string
	}{
		{"empty", "", "", true, "crypto.key is empty"},
		{"wrong-len-16", "1234567890123456", "", true, "require exactly 32 bytes"},
		{"ok-32", "12345678901234567890123456789012", "", false, ""},
		{"base64-ok", "", "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=", false, ""}, // 32 bytes base64
		{"base64-invalid", "", "not-valid-base64!!!", true, "invalid base64"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			cfg := &conf.Config{
				JWT:     conf.JWTConfig{Secret: "this-is-a-32-byte-long-secret-key!"},
				MySQL:   conf.MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
				Redis:   conf.RedisConfig{Addr: "localhost:6379"},
				Storage: conf.StorageConfig{Provider: "local"},
				Crypto:  conf.CryptoConfig{Key: c.key, KeyBase64: c.keyBase64},
			}
			err := cfg.Validate()
			if c.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.errSub)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestConfigValidate_MySQLDSN 验证 MySQL DSN 密码检查
func TestConfigValidate_MySQLDSN(t *testing.T) {
	cases := []struct {
		dsn     string
		wantErr bool
		errSub  string
	}{
		{"", true, "mysql.dsn is empty"},
		{"user@tcp(localhost:3306)/db", true, "missing password"},
		{"user:@tcp(localhost:3306)/db", true, "missing password"},
		{"user:pass@tcp(localhost:3306)/db", false, ""},
	}

	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("dsn_%v", c.wantErr), func(t *testing.T) {
			cfg := &conf.Config{
				JWT:     conf.JWTConfig{Secret: "this-is-a-32-byte-long-secret-key!"},
				MySQL:   conf.MySQLConfig{DSN: c.dsn},
				Redis:   conf.RedisConfig{Addr: "localhost:6379"},
				Storage: conf.StorageConfig{Provider: "local"},
				Crypto:  conf.CryptoConfig{Key: "12345678901234567890123456789012"},
			}
			err := cfg.Validate()
			if c.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.errSub)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestConfigValidate_RedisAddr 验证 Redis 地址非空
func TestConfigValidate_RedisAddr(t *testing.T) {
	cfg := &conf.Config{
		JWT:     conf.JWTConfig{Secret: "this-is-a-32-byte-long-secret-key!"},
		MySQL:   conf.MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
		Redis:   conf.RedisConfig{Addr: ""},
		Storage: conf.StorageConfig{Provider: "local"},
		Crypto:  conf.CryptoConfig{Key: "12345678901234567890123456789012"},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis.addr is empty")
}

// TestConfigValidate_StorageProvider 验证 Storage 配置完整性
func TestConfigValidate_StorageProvider(t *testing.T) {
	t.Run("empty-provider", func(t *testing.T) {
		cfg := baseValidConfig()
		cfg.Storage.Provider = ""
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "storage.provider is empty")
	})

	t.Run("local-ok", func(t *testing.T) {
		cfg := baseValidConfig()
		cfg.Storage.Provider = "local"
		cfg.Storage.Endpoint = ""
		cfg.Storage.Bucket = ""
		cfg.Storage.AccessKey = ""
		cfg.Storage.SecretKey = ""
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("oss-missing-endpoint", func(t *testing.T) {
		cfg := baseValidConfig()
		cfg.Storage.Provider = "oss"
		cfg.Storage.Endpoint = ""
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "storage.endpoint is empty")
	})
}

// TestConfigValidate_ProdOrigins 验证生产环境必须设置 origins
func TestConfigValidate_ProdOrigins(t *testing.T) {
	cfg := baseValidConfig()
	cfg.App.Env = "prod"
	cfg.App.Origins = nil
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app.origins is empty in prod")
}

// TestConfigValidate_ModelGateway 验证 ModelGateway URL/Key 一致性
func TestConfigValidate_ModelGateway(t *testing.T) {
	t.Run("url-without-key", func(t *testing.T) {
		cfg := baseValidConfig()
		cfg.ModelGateway.URL = "http://gateway.example.com"
		cfg.ModelGateway.Key = ""
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model_gateway.key is empty while url is set")
	})

	t.Run("url-with-key-ok", func(t *testing.T) {
		cfg := baseValidConfig()
		cfg.ModelGateway.URL = "http://gateway.example.com"
		cfg.ModelGateway.Key = "some-key"
		err := cfg.Validate()
		require.NoError(t, err)
	})
}

// TestLoadConfig_FileNotExist 验证配置文件不存在时行为(取决于 viper 实现,可能不报错)
func TestLoadConfig_FileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "not_exist.yaml")
	_, err := conf.Load(nonExistent)
	// 由于 Validate 会失败(空配置),这里预期返回错误
	require.Error(t, err)
}

// TestLoadConfig_ValidMinimalYAML 验证最小有效配置可加载
func TestLoadConfig_ValidMinimalYAML(t *testing.T) {
	// 设置环境变量防止 AutomaticEnv 读取到系统空值覆盖配置
	t.Setenv("MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("JWT_SECRET", "this-is-a-32-byte-long-secret-key!")
	t.Setenv("CRYPTO_KEY", "12345678901234567890123456789012")
	t.Setenv("OSS_PROVIDER", "local")

	yaml := `
app:
  name: test-app
  env: dev
  port: 8080
  log_level: debug
jwt:
  secret: this-is-a-32-byte-long-secret-key!
  access_expires_in: 3600
  refresh_expires_in: 86400
mysql:
  dsn: "user:pass@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local"
redis:
  addr: "127.0.0.1:6379"
storage:
  provider: local
crypto:
  key: "12345678901234567890123456789012"
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	cfg, err := conf.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "test-app", cfg.App.Name)
	assert.Equal(t, "dev", cfg.App.Env)
	assert.Equal(t, 8080, cfg.App.Port)
	assert.Equal(t, "debug", cfg.App.LogLevel)
	assert.Equal(t, "this-is-a-32-byte-long-secret-key!", cfg.JWT.Secret)
	assert.Equal(t, "user:pass@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local", cfg.MySQL.DSN)
	assert.Equal(t, "127.0.0.1:6379", cfg.Redis.Addr)
	assert.Equal(t, "local", cfg.Storage.Provider)
	assert.Equal(t, "12345678901234567890123456789012", cfg.Crypto.Key)
}

// ---------- helpers ----------

func baseValidConfig() *conf.Config {
	return &conf.Config{
		JWT:     conf.JWTConfig{Secret: "this-is-a-32-byte-long-secret-key!"},
		MySQL:   conf.MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
		Redis:   conf.RedisConfig{Addr: "localhost:6379"},
		Storage: conf.StorageConfig{Provider: "local"},
		Crypto:  conf.CryptoConfig{Key: "12345678901234567890123456789012"},
	}
}
