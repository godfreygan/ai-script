package conf

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	App          AppConfig          `mapstructure:"app"`
	JWT          JWTConfig          `mapstructure:"jwt"`
	MySQL        MySQLConfig        `mapstructure:"mysql"`
	Redis        RedisConfig        `mapstructure:"redis"`
	Queue        QueueConfig        `mapstructure:"queue"`
	Storage      StorageConfig      `mapstructure:"storage"`
	Crypto       CryptoConfig       `mapstructure:"crypto"`
	ModelGateway ModelGatewayConfig `mapstructure:"model_gateway"`
	Migrate      MigrateConfig      `mapstructure:"migrate"`
	Timeouts     TimeoutsConfig     `mapstructure:"timeouts"`
}

type MigrateConfig struct {
	Mode   string `mapstructure:"mode"`   // auto / migrate / off
	Source string `mapstructure:"source"` // e.g. "file://./migrations"
	DSN    string `mapstructure:"dsn"`    // 覆盖 mysql.dsn,可选
}

type AppConfig struct {
	Name     string   `mapstructure:"name"`
	Env      string   `mapstructure:"env"`
	Port     int      `mapstructure:"port"`
	LogLevel string   `mapstructure:"log_level"`
	Origins  []string `mapstructure:"origins"`
}

type JWTConfig struct {
	Secret           string `mapstructure:"secret"`
	AccessExpiresIn  int    `mapstructure:"access_expires_in"`
	RefreshExpiresIn int    `mapstructure:"refresh_expires_in"`
}

type MySQLConfig struct {
	DSN             string `mapstructure:"dsn"`
	MaxIdle         int    `mapstructure:"max_idle"`
	MaxOpen         int    `mapstructure:"max_open"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime_seconds"`
	ConnMaxIdleTime int    `mapstructure:"conn_max_idle_time_seconds"`
}

type RedisConfig struct {
	Addr         string `mapstructure:"addr"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
	PoolTimeout  int    `mapstructure:"pool_timeout_seconds"`
}

type QueueConfig struct {
	Concurrency int `mapstructure:"concurrency"`
}

type StorageConfig struct {
	Provider   string `mapstructure:"provider"`
	Endpoint   string `mapstructure:"endpoint"`
	Region     string `mapstructure:"region"`
	Bucket     string `mapstructure:"bucket"`
	AccessKey  string `mapstructure:"access_key"`
	SecretKey  string `mapstructure:"secret_key"`
	PublicHost string `mapstructure:"public_host"`
}

type CryptoConfig struct {
	Key       string `mapstructure:"key"`
	KeyBase64 string `mapstructure:"key_base64"`
}

type ModelGatewayConfig struct {
	URL string `mapstructure:"url"`
	Key string `mapstructure:"key"`
}

type TimeoutsConfig struct {
	ImageGen     int `mapstructure:"image_gen_seconds"`
	VideoGen     int `mapstructure:"video_gen_seconds"`
	VideoCompose int `mapstructure:"video_compose_seconds"`
	ModelHealth  int `mapstructure:"model_health_seconds"`
	PipelineRun  int `mapstructure:"pipeline_run_seconds"`
}

// Load 读取 yaml + env 覆盖
func Load(path string) (*Config, error) {
	_ = godotenv.Load()

	v := viper.New()
	v.SetConfigFile(path)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if _, err := os.Stat(path); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	expandEnv(v)

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate 关键配置 fail-fast — 防止默认 secret / 空 key 进入生产。
// 修复 P0 #3 — 原 Load 无校验, .env 漏配 / 沿用 please-change-me 也能起服务。
func (c *Config) Validate() error {
	// 1. JWT.Secret 不能为空且长度 >= 32
	if c.JWT.Secret == "" {
		return fmt.Errorf("conf: jwt.secret is empty — set JWT_SECRET")
	}
	if strings.Contains(c.JWT.Secret, "change-me") || strings.Contains(c.JWT.Secret, "default") {
		return fmt.Errorf("conf: jwt.secret still uses placeholder value — set JWT_SECRET to a real random string >=32 chars")
	}
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("conf: jwt.secret too short (%d), require >=32 chars", len(c.JWT.Secret))
	}

	// 2. Crypto.Key 处理：优先 CRYPTO_KEY，若为空则尝试从 CRYPTO_KEY_BASE64 解码
	if c.Crypto.Key == "" && c.Crypto.KeyBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(c.Crypto.KeyBase64)
		if err != nil {
			return fmt.Errorf("conf: crypto.key_base64 invalid base64: %w", err)
		}
		if len(decoded) != 32 {
			return fmt.Errorf("conf: crypto.key_base64 decoded length is %d bytes, require 32 bytes (AES-256)", len(decoded))
		}
		c.Crypto.Key = string(decoded)
	}
	if c.Crypto.Key == "" {
		return fmt.Errorf("conf: crypto.key is empty — set CRYPTO_KEY or CRYPTO_KEY_BASE64")
	}
	if len(c.Crypto.Key) != 32 {
		hint := ""
		if len(c.Crypto.Key) == 44 {
			hint = " — did you put a base64-encoded key into CRYPTO_KEY? Use CRYPTO_KEY_BASE64 instead"
		}
		return fmt.Errorf("conf: crypto.key length is %d bytes, require exactly 32 bytes (AES-256)%s", len(c.Crypto.Key), hint)
	}

	// 3. MySQL.DSN 必须包含密码（不为空）
	if c.MySQL.DSN == "" {
		return fmt.Errorf("conf: mysql.dsn is empty — set MYSQL_DSN")
	}
	if !hasDSNPassword(c.MySQL.DSN) {
		return fmt.Errorf("conf: mysql.dsn missing password — ensure DSN contains a password segment")
	}

	// 4. Redis.Addr 非空
	if c.Redis.Addr == "" {
		return fmt.Errorf("conf: redis.addr is empty — set REDIS_ADDR")
	}

	// 5. Storage 配置完整性
	if c.Storage.Provider == "" {
		return fmt.Errorf("conf: storage.provider is empty — set OSS_PROVIDER")
	}
	if c.Storage.Provider != "local" {
		if c.Storage.Endpoint == "" {
			return fmt.Errorf("conf: storage.endpoint is empty — set OSS_ENDPOINT")
		}
		if c.Storage.Bucket == "" {
			return fmt.Errorf("conf: storage.bucket is empty — set OSS_BUCKET")
		}
		if c.Storage.AccessKey == "" {
			return fmt.Errorf("conf: storage.access_key is empty — set OSS_ACCESS_KEY")
		}
		if c.Storage.SecretKey == "" {
			return fmt.Errorf("conf: storage.secret_key is empty — set OSS_SECRET_KEY")
		}
	}

	// 6. 生产环境必须设置 app.origins
	if c.App.Env == "prod" && len(c.App.Origins) == 0 {
		return fmt.Errorf("conf: app.origins is empty in prod — set APP_ORIGINS to prevent CORS wildcard")
	}

	// 7. ModelGateway 配置了 URL 时必须同时配置 Key
	if c.ModelGateway.URL != "" && c.ModelGateway.Key == "" {
		return fmt.Errorf("conf: model_gateway.key is empty while url is set — set MODEL_GATEWAY_KEY")
	}

	return nil
}

// hasDSNPassword 检查 MySQL DSN 中是否包含非空密码。
// DSN 格式: user:password@tcp(host:port)/dbname?params
func hasDSNPassword(dsn string) bool {
	// 匹配 user:password@ 格式，密码段不为空
	re := regexp.MustCompile(`^[^:]+:([^@]+)@`)
	matches := re.FindStringSubmatch(dsn)
	if len(matches) < 2 {
		return false
	}
	return strings.TrimSpace(matches[1]) != ""
}

// expandEnv 把 ${VAR:default} 展开
func expandEnv(v *viper.Viper) {
	for _, key := range v.AllKeys() {
		val := v.GetString(key)
		v.Set(key, os.ExpandEnv(val))
	}
}
