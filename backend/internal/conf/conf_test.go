package conf

import (
	"testing"
)

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
