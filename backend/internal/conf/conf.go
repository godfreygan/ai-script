package conf

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App           AppConfig           `mapstructure:"app"`
	JWT           JWTConfig           `mapstructure:"jwt"`
	MySQL         MySQLConfig         `mapstructure:"mysql"`
	Redis         RedisConfig         `mapstructure:"redis"`
	Storage       StorageConfig       `mapstructure:"storage"`
	Crypto        CryptoConfig        `mapstructure:"crypto"`
	ModelGateway  ModelGatewayConfig  `mapstructure:"model_gateway"`
}

type AppConfig struct {
	Name     string `mapstructure:"name"`
	Env      string `mapstructure:"env"`
	Port     int    `mapstructure:"port"`
	LogLevel string `mapstructure:"log_level"`
}

type JWTConfig struct {
	Secret            string `mapstructure:"secret"`
	AccessExpiresIn   int    `mapstructure:"access_expires_in"`
	RefreshExpiresIn  int    `mapstructure:"refresh_expires_in"`
}

type MySQLConfig struct {
	DSN     string `mapstructure:"dsn"`
	MaxIdle int    `mapstructure:"max_idle"`
	MaxOpen int    `mapstructure:"max_open"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
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
	Key string `mapstructure:"key"`
}

type ModelGatewayConfig struct {
	URL string `mapstructure:"url"`
	Key string `mapstructure:"key"`
}

// Load 读取 yaml + env 覆盖
func Load(path string) (*Config, error) {
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
	if c.JWT.Secret == "" {
		return fmt.Errorf("conf: jwt.secret is empty — set JWT_SECRET")
	}
	if strings.Contains(c.JWT.Secret, "change-me") {
		return fmt.Errorf("conf: jwt.secret still uses default 'change-me' placeholder — set JWT_SECRET to a real random string >=32 chars")
	}
	if c.App.Env == "prod" && len(c.JWT.Secret) < 32 {
		return fmt.Errorf("conf: jwt.secret too short (%d) in prod, require >=32 chars", len(c.JWT.Secret))
	}
	if c.Crypto.Key == "" {
		return fmt.Errorf("conf: crypto.key is empty — set CRYPTO_KEY (16/24/32 bytes for AES)")
	}
	keyLen := len(c.Crypto.Key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return fmt.Errorf("conf: crypto.key length=%d invalid for AES (must be 16/24/32 bytes)", keyLen)
	}
	if c.MySQL.DSN == "" {
		return fmt.Errorf("conf: mysql.dsn is empty — set MYSQL_DSN")
	}
	return nil
}

// expandEnv 把 ${VAR:default} 展开
func expandEnv(v *viper.Viper) {
	for _, key := range v.AllKeys() {
		val := v.GetString(key)
		v.Set(key, os.ExpandEnv(val))
	}
}
