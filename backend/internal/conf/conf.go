package conf

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

type Config struct {
	App          AppConfig
	JWT          JWTConfig
	MySQL        MySQLConfig
	Redis        RedisConfig
	Queue        QueueConfig
	Storage      StorageConfig
	Crypto       CryptoConfig
	ModelGateway ModelGatewayConfig
	Migrate      MigrateConfig
	Timeouts     TimeoutsConfig
}

type MigrateConfig struct {
	Mode   string // auto / migrate / off
	Source string
	DSN    string
}

type AppConfig struct {
	Name     string
	Env      string
	Port     int
	LogLevel string
	Origins  []string
}

type JWTConfig struct {
	Secret           string
	AccessExpiresIn  int
	RefreshExpiresIn int
}

type MySQLConfig struct {
	DSN             string
	MaxIdle         int
	MaxOpen         int
	ConnMaxLifetime int
	ConnMaxIdleTime int
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	PoolTimeout  int
}

type QueueConfig struct {
	Concurrency int
}

type StorageConfig struct {
	Provider   string
	Endpoint   string
	Region     string
	Bucket     string
	AccessKey  string
	SecretKey  string
	PublicHost string
}

type CryptoConfig struct {
	Key       string
	KeyBase64 string
}

type ModelGatewayConfig struct {
	URL string
	Key string
}

type TimeoutsConfig struct {
	ImageGen     int
	VideoGen     int
	VideoCompose int
	ModelHealth  int
	PipelineRun  int
}

// Load 从 .env 与环境变量加载配置（唯一配置来源，不使用 config.yaml）。
func Load() (*Config, error) {
	loadEnvFiles()
	c := loadFromEnv()
	applyEnvDerivedConfig(c)
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func loadEnvFiles() {
	for _, p := range discoverEnvFiles() {
		_ = godotenv.Overload(p)
	}
}

// discoverEnvFiles 从当前工作目录向上查找 .env（先加载仓库根，后加载子目录，后者覆盖前者）。
func discoverEnvFiles() []string {
	seen := make(map[string]struct{})
	var found []string

	add := func(dir string) {
		p := filepath.Join(dir, ".env")
		if _, ok := seen[p]; ok {
			return
		}
		if _, err := os.Stat(p); err != nil {
			return
		}
		seen[p] = struct{}{}
		found = append(found, p)
	}

	if wd, err := os.Getwd(); err == nil {
		dir := wd
		var upward []string
		for {
			upward = append(upward, dir)
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
		for i := len(upward) - 1; i >= 0; i-- {
			add(upward[i])
		}
	}

	if exe, err := os.Executable(); err == nil {
		add(filepath.Dir(exe))
		add(filepath.Join(filepath.Dir(exe), ".."))
	}

	return found
}

func loadFromEnv() *Config {
	c := &Config{
		App: AppConfig{
			Name:     envStr("APP_NAME", "ai-script"),
			Env:      envStr("APP_ENV", "local"),
			Port:     envInt("APP_PORT", 8080),
			LogLevel: envStr("APP_LOG_LEVEL", "debug"),
			Origins:  splitCSV(envStr("APP_ORIGINS", "http://localhost")),
		},
		JWT: JWTConfig{
			Secret:           strings.TrimSpace(os.Getenv("JWT_SECRET")),
			AccessExpiresIn:  envIntFirst(7200, "JWT_EXPIRES_IN", "JWT_ACCESS_EXPIRES_IN"),
			RefreshExpiresIn: envInt("JWT_REFRESH_EXPIRES_IN", 604800),
		},
		MySQL: MySQLConfig{
			DSN:             strings.TrimSpace(os.Getenv("MYSQL_DSN")),
			MaxIdle:         envInt("MYSQL_MAX_IDLE", 10),
			MaxOpen:         envInt("MYSQL_MAX_OPEN", 100),
			ConnMaxLifetime: envInt("MYSQL_CONN_MAX_LIFETIME", 3600),
			ConnMaxIdleTime: envInt("MYSQL_CONN_MAX_IDLE_TIME", 1800),
		},
		Redis: RedisConfig{
			Addr:         strings.TrimSpace(os.Getenv("REDIS_ADDR")),
			Password:     os.Getenv("REDIS_PASSWORD"),
			DB:           envInt("REDIS_DB", 0),
			PoolSize:     envInt("REDIS_POOL_SIZE", 20),
			MinIdleConns: envInt("REDIS_MIN_IDLE_CONNS", 5),
			PoolTimeout:  envInt("REDIS_POOL_TIMEOUT", 5),
		},
		Queue: QueueConfig{
			Concurrency: envInt("QUEUE_CONCURRENCY", 16),
		},
		Storage: StorageConfig{
			Provider: envStr("OSS_PROVIDER", "local"),
			Endpoint: os.Getenv("OSS_ENDPOINT"),
			Region:   envStr("OSS_REGION", ""),
			Bucket: resolveLocalBucket(
				envStr("OSS_PROVIDER", "local"),
				envStr("OSS_BUCKET", "/data/uploads"),
			),
			AccessKey:  os.Getenv("OSS_ACCESS_KEY"),
			SecretKey:  os.Getenv("OSS_SECRET_KEY"),
			PublicHost: os.Getenv("OSS_PUBLIC_HOST"),
		},
		Crypto: CryptoConfig{
			Key:       os.Getenv("CRYPTO_KEY"),
			KeyBase64: strings.TrimSpace(os.Getenv("CRYPTO_KEY_BASE64")),
		},
		ModelGateway: ModelGatewayConfig{
			URL: os.Getenv("MODEL_GATEWAY_URL"),
			Key: os.Getenv("MODEL_GATEWAY_KEY"),
		},
		Migrate: MigrateConfig{
			Mode:   envStr("MIGRATE_MODE", "auto"),
			Source: os.Getenv("MIGRATE_SOURCE"),
			DSN:    os.Getenv("MIGRATE_DSN"),
		},
		Timeouts: TimeoutsConfig{
			ImageGen:     envInt("TIMEOUTS_IMAGE_GEN_SECONDS", 120),
			VideoGen:     envInt("TIMEOUTS_VIDEO_GEN_SECONDS", 120),
			VideoCompose: envInt("TIMEOUTS_VIDEO_COMPOSE_SECONDS", 300),
			ModelHealth:  envInt("TIMEOUTS_MODEL_HEALTH_SECONDS", 30),
			PipelineRun:  envInt("TIMEOUTS_PIPELINE_RUN_SECONDS", 600),
		},
	}
	return c
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := parseIntEnv(v)
	if err != nil {
		return def
	}
	return n
}

func envIntFirst(def int, keys ...string) int {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			n, err := parseIntEnv(v)
			if err == nil {
				return n
			}
		}
	}
	return def
}

// applyEnvDerivedConfig 用 MYSQL_* / REDIS_* 分段变量覆盖 DSN/Addr。
func applyEnvDerivedConfig(c *Config) {
	if dsn := buildMySQLDSNFromEnv(); dsn != "" {
		c.MySQL.DSN = dsn
	}
	if addr := buildRedisAddrFromEnv(); addr != "" {
		c.Redis.Addr = addr
	}
	if v := strings.TrimSpace(os.Getenv("REDIS_DB")); v != "" {
		if db, err := parseIntEnv(v); err == nil {
			c.Redis.DB = db
		}
	}
}

func buildMySQLDSNFromEnv() string {
	user := strings.TrimSpace(os.Getenv("MYSQL_USER"))
	db := strings.TrimSpace(os.Getenv("MYSQL_DATABASE"))
	if user == "" || db == "" {
		return ""
	}
	host := envStr("MYSQL_HOST", "127.0.0.1")
	port := envStr("MYSQL_PORT", "3306")
	params := envStr("MYSQL_PARAMS", "charset=utf8mb4&parseTime=True&loc=Local")
	cfg := mysql.Config{
		User:                 user,
		Passwd:               os.Getenv("MYSQL_PASSWORD"),
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(host, port),
		DBName:               db,
		AllowNativePasswords: true,
		ParseTime:            true,
		Loc:                  nil,
	}
	if params != "" {
		cfg.Params = parseMySQLParams(params)
	}
	return cfg.FormatDSN()
}

func parseMySQLParams(raw string) map[string]string {
	out := make(map[string]string)
	for _, part := range strings.Split(raw, "&") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

func buildRedisAddrFromEnv() string {
	host := strings.TrimSpace(os.Getenv("REDIS_HOST"))
	if host == "" {
		return ""
	}
	port := envStr("REDIS_PORT", "6379")
	return net.JoinHostPort(host, port)
}

func envStr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func parseIntEnv(v string) (int, error) {
	var n int
	_, err := fmt.Sscanf(v, "%d", &n)
	return n, err
}

// Validate 关键配置 fail-fast。
func (c *Config) Validate() error {
	if c.JWT.Secret == "" {
		hint := "set JWT_SECRET in .env (repo root or backend/) or export it before start"
		if len(discoverEnvFiles()) == 0 {
			hint += "; no .env file found — Docker 需在 compose 中配置 env_file: .env 或注入 JWT_SECRET"
		}
		return fmt.Errorf("conf: jwt.secret is empty — %s", hint)
	}
	if strings.Contains(c.JWT.Secret, "change-me") || strings.Contains(c.JWT.Secret, "default") {
		return fmt.Errorf("conf: jwt.secret still uses placeholder value — set JWT_SECRET to a real random string >=32 chars")
	}
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("conf: jwt.secret too short (%d), require >=32 chars", len(c.JWT.Secret))
	}

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

	if c.MySQL.DSN == "" {
		return fmt.Errorf("conf: mysql.dsn is empty — set MYSQL_HOST/MYSQL_PORT/MYSQL_USER/MYSQL_PASSWORD/MYSQL_DATABASE or MYSQL_DSN")
	}
	if u := strings.TrimSpace(os.Getenv("MYSQL_USER")); strings.EqualFold(u, "root") {
		return fmt.Errorf("conf: MYSQL_USER must not be \"root\" — set an application user via MYSQL_USER in .env; root is only for MYSQL_ROOT_PASSWORD in the MySQL container")
	}
	if !hasDSNPassword(c.MySQL.DSN) {
		return fmt.Errorf("conf: mysql.dsn missing password — ensure DSN contains a password segment")
	}

	if c.Redis.Addr == "" {
		return fmt.Errorf("conf: redis.addr is empty — set REDIS_HOST/REDIS_PORT or REDIS_ADDR")
	}

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

	if c.App.Env == "prod" && len(c.App.Origins) == 0 {
		return fmt.Errorf("conf: app.origins is empty in prod — set APP_ORIGINS to prevent CORS wildcard")
	}

	if c.ModelGateway.URL != "" && c.ModelGateway.Key == "" {
		return fmt.Errorf("conf: model_gateway.key is empty while url is set — set MODEL_GATEWAY_KEY")
	}

	return nil
}

func hasDSNPassword(dsn string) bool {
	re := regexp.MustCompile(`^[^:]+:([^@]+)@`)
	matches := re.FindStringSubmatch(dsn)
	if len(matches) < 2 {
		return false
	}
	return strings.TrimSpace(matches[1]) != ""
}
