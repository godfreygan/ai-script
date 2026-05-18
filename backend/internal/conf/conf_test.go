package conf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildMySQLDSNFromEnv(t *testing.T) {
	t.Setenv("MYSQL_USER", "ai_script")
	t.Setenv("MYSQL_PASSWORD", "secret")
	t.Setenv("MYSQL_HOST", "127.0.0.1")
	t.Setenv("MYSQL_PORT", "3306")
	t.Setenv("MYSQL_DATABASE", "ai_script")
	t.Setenv("MYSQL_PARAMS", "charset=utf8mb4&parseTime=True&loc=Local")

	dsn := buildMySQLDSNFromEnv()
	if dsn == "" {
		t.Fatal("expected non-empty DSN")
	}
	if want := "ai_script:secret@tcp(127.0.0.1:3306)/ai_script"; dsn[:len(want)] != want {
		t.Fatalf("DSN prefix = %q, want prefix %q", dsn, want)
	}
}

func TestDiscoverEnvFiles_WalksUpward(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "backend", "cmd", "server")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("JWT_SECRET=root\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "backend", ".env"), []byte("JWT_SECRET=backend\n"), 0o600))

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(sub))
	t.Cleanup(func() { _ = os.Chdir(wd) })

	paths := discoverEnvFiles()
	if len(paths) < 2 {
		t.Fatalf("expected at least 2 .env files, got %v", paths)
	}
	wantLast, _ := filepath.EvalSymlinks(filepath.Join(dir, "backend", ".env"))
	gotLast, _ := filepath.EvalSymlinks(paths[len(paths)-1])
	if gotLast != wantLast {
		t.Fatalf("last .env = %q, want %q (all: %v)", gotLast, wantLast, paths)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("APP_NAME", "test-app")
	t.Setenv("APP_ENV", "dev")
	t.Setenv("MYSQL_USER", "ai_script")
	t.Setenv("MYSQL_PASSWORD", "secret")
	t.Setenv("MYSQL_HOST", "127.0.0.1")
	t.Setenv("MYSQL_PORT", "3306")
	t.Setenv("MYSQL_DATABASE", "ai_script")
	t.Setenv("REDIS_HOST", "127.0.0.1")
	t.Setenv("REDIS_PORT", "6379")
	t.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-that-is-32chars")
	t.Setenv("CRYPTO_KEY", "this-is-32-byte-crypto-key-12345")
	t.Setenv("OSS_PROVIDER", "local")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Name != "test-app" {
		t.Fatalf("app name = %q", cfg.App.Name)
	}
}

func TestApplyEnvDerivedConfig_OverridesStaleMYSQL_DSN(t *testing.T) {
	t.Setenv("MYSQL_USER", "ai_script")
	t.Setenv("MYSQL_PASSWORD", "newpass")
	t.Setenv("MYSQL_HOST", "mysql")
	t.Setenv("MYSQL_PORT", "3306")
	t.Setenv("MYSQL_DATABASE", "ai_script")
	t.Setenv("MYSQL_DSN", "wrong:wrong@tcp(127.0.0.1:3306)/ai_script")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6379")

	var c Config
	c.MySQL.DSN = "wrong:wrong@tcp(127.0.0.1:3306)/ai_script"
	c.Redis.Addr = "127.0.0.1:6379"
	applyEnvDerivedConfig(&c)

	if c.MySQL.DSN == "" || c.MySQL.DSN == "wrong:wrong@tcp(127.0.0.1:3306)/ai_script" {
		t.Fatalf("expected DSN rebuilt from MYSQL_USER, got %q", c.MySQL.DSN)
	}
	if c.Redis.Addr != "redis:6379" {
		t.Fatalf("redis addr = %q, want redis:6379", c.Redis.Addr)
	}
}

func TestValidate_RejectsMySQLUserRoot(t *testing.T) {
	t.Setenv("MYSQL_USER", "root")
	cfg := Config{
		App:     AppConfig{Env: "local"},
		JWT:     JWTConfig{Secret: "this-is-a-very-long-secret-key-that-is-32chars"},
		MySQL:   MySQLConfig{DSN: "root:pass@tcp(127.0.0.1:3306)/db"},
		Redis:   RedisConfig{Addr: "127.0.0.1:6379"},
		Storage: StorageConfig{Provider: "local"},
		Crypto:  CryptoConfig{Key: "this-is-32-byte-crypto-key-12345"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for MYSQL_USER=root")
	}
	_ = os.Unsetenv("MYSQL_USER")
}

func TestValidate(t *testing.T) {
	goodSecret := "this-is-a-very-long-secret-key-that-is-32chars"
	goodKey := "this-is-32-byte-crypto-key-12345"

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "all valid",
			cfg: Config{
				App:     AppConfig{Env: "dev"},
				JWT:     JWTConfig{Secret: goodSecret},
				MySQL:   MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: goodKey},
			},
			wantErr: false,
		},
		{
			name: "jwt secret empty",
			cfg: Config{
				App:     AppConfig{Env: "dev"},
				JWT:     JWTConfig{Secret: ""},
				MySQL:   MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: goodKey},
			},
			wantErr: true,
			errMsg:  "jwt.secret is empty",
		},
		{
			name: "jwt secret too short",
			cfg: Config{
				App:     AppConfig{Env: "dev"},
				JWT:     JWTConfig{Secret: "short"},
				MySQL:   MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: goodKey},
			},
			wantErr: true,
			errMsg:  "jwt.secret too short",
		},
		{
			name: "jwt secret placeholder",
			cfg: Config{
				App:     AppConfig{Env: "dev"},
				JWT:     JWTConfig{Secret: "please-change-me"},
				MySQL:   MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: goodKey},
			},
			wantErr: true,
			errMsg:  "placeholder",
		},
		{
			name: "crypto key empty",
			cfg: Config{
				App:     AppConfig{Env: "dev"},
				JWT:     JWTConfig{Secret: goodSecret},
				MySQL:   MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: ""},
			},
			wantErr: true,
			errMsg:  "crypto.key is empty",
		},
		{
			name: "crypto key too short",
			cfg: Config{
				App:     AppConfig{Env: "dev"},
				JWT:     JWTConfig{Secret: goodSecret},
				MySQL:   MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: "short"},
			},
			wantErr: true,
			errMsg:  "crypto.key length is 5 bytes",
		},
		{
			name: "mysql dsn empty",
			cfg: Config{
				App:     AppConfig{Env: "dev"},
				JWT:     JWTConfig{Secret: goodSecret},
				MySQL:   MySQLConfig{DSN: ""},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: goodKey},
			},
			wantErr: true,
			errMsg:  "mysql.dsn is empty",
		},
		{
			name: "mysql dsn no password",
			cfg: Config{
				App:     AppConfig{Env: "dev"},
				JWT:     JWTConfig{Secret: goodSecret},
				MySQL:   MySQLConfig{DSN: "user@tcp(localhost:3306)/db"},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: goodKey},
			},
			wantErr: true,
			errMsg:  "mysql.dsn missing password",
		},
		{
			name: "mysql dsn empty password",
			cfg: Config{
				App:     AppConfig{Env: "dev"},
				JWT:     JWTConfig{Secret: goodSecret},
				MySQL:   MySQLConfig{DSN: "user:@tcp(localhost:3306)/db"},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: goodKey},
			},
			wantErr: true,
			errMsg:  "mysql.dsn missing password",
		},
		{
			name: "prod no origins",
			cfg: Config{
				App:     AppConfig{Env: "prod", Origins: []string{}},
				JWT:     JWTConfig{Secret: goodSecret},
				MySQL:   MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: goodKey},
			},
			wantErr: true,
			errMsg:  "app.origins is empty in prod",
		},
		{
			name: "prod with origins ok",
			cfg: Config{
				App:     AppConfig{Env: "prod", Origins: []string{"https://example.com"}},
				JWT:     JWTConfig{Secret: goodSecret},
				MySQL:   MySQLConfig{DSN: "user:pass@tcp(localhost:3306)/db"},
				Redis:   RedisConfig{Addr: "localhost:6379"},
				Storage: StorageConfig{Provider: "local"},
				Crypto:  CryptoConfig{Key: goodKey},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMsg)
				}
				if !contains(err.Error(), tt.errMsg) {
					t.Fatalf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(substr) <= len(s) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
