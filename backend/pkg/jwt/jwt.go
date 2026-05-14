package jwt

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// Key 表示一个带 ID 的签名密钥,用于密钥轮换。
type Key struct {
	ID     string
	Secret []byte
}

// RevocationChecker 是 JWT 黑名单/撤销检查接口。
type RevocationChecker interface {
	IsRevoked(jti string) bool
}

// noopRevocationChecker 是一个空实现,默认不检查撤销状态。
type noopRevocationChecker struct{}

func (n *noopRevocationChecker) IsRevoked(string) bool { return false }

// RedisRevocationChecker 基于 Redis 的 JWT 黑名单检查器。
type RedisRevocationChecker struct {
	RDB *redis.Client
}

func (r *RedisRevocationChecker) IsRevoked(jti string) bool {
	if r.RDB == nil || jti == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	key := fmt.Sprintf("token:blacklist:%s", jti)
	exists, err := r.RDB.Exists(ctx, key).Result()
	return err == nil && exists > 0
}

// Claims JWT 声明
type Claims struct {
	UserID   int64    `json:"uid"`
	Username string   `json:"un"`
	DeptID   int64    `json:"did"`
	Roles    []string `json:"r"`
	jwt.RegisteredClaims
}

// Manager JWT 管理器,支持多密钥(kid)与黑名单检查。
type Manager struct {
	mu               sync.RWMutex
	keys             map[string]Key      // kid -> Key
	defaultKeyID     string              // 当前默认签名密钥 ID
	accessExpiresIn  time.Duration
	refreshExpiresIn time.Duration
	revocation       RevocationChecker
}

// NewManager 创建 JWT 管理器。secret 为初始默认密钥,accessSec/refreshSec 为秒数。
func NewManager(secret string, accessSec, refreshSec int) *Manager {
	kid := "default"
	return &Manager{
		keys: map[string]Key{
			kid: {ID: kid, Secret: []byte(secret)},
		},
		defaultKeyID:     kid,
		accessExpiresIn:  time.Duration(accessSec) * time.Second,
		refreshExpiresIn: time.Duration(refreshSec) * time.Second,
		revocation:       &noopRevocationChecker{},
	}
}

// SetRevocationChecker 设置黑名单检查器(如 Redis 实现)。
func (m *Manager) SetRevocationChecker(r RevocationChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r != nil {
		m.revocation = r
	}
}

// AddKey 添加一个新密钥并设为默认(用于密钥轮换)。
func (m *Manager) AddKey(k Key) error {
	if k.ID == "" {
		return errors.New("jwt: key id cannot be empty")
	}
	if len(k.Secret) < 32 {
		return errors.New("jwt: secret too short, require >= 32 bytes")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys[k.ID] = k
	m.defaultKeyID = k.ID
	return nil
}

// RemoveKey 移除指定密钥(不能移除当前默认密钥)。
func (m *Manager) RemoveKey(kid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if kid == m.defaultKeyID {
		return errors.New("jwt: cannot remove default key")
	}
	delete(m.keys, kid)
	return nil
}

// Issue 签发 access token 与 refresh token。
func (m *Manager) Issue(c *Claims) (access, refresh string, err error) {
	m.mu.RLock()
	key := m.keys[m.defaultKeyID]
	kid := m.defaultKeyID
	m.mu.RUnlock()

	now := time.Now()
	jti := mustJTI()

	// Access token
	c.RegisteredClaims = jwt.RegisteredClaims{
		ID:        jti,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiresIn)),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	accessToken.Header["kid"] = kid
	access, err = accessToken.SignedString(key.Secret)
	if err != nil {
		return
	}

	// Refresh token
	rc := *c
	rc.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpiresIn)),
		Subject:   "refresh",
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, &rc)
	refreshToken.Header["kid"] = kid
	refresh, err = refreshToken.SignedString(key.Secret)
	return
}

// Parse 解析并校验 token,同时检查黑名单。
func (m *Manager) Parse(token string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		// 从 header 提取 kid,支持密钥轮换
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			kid = m.defaultKeyID
		}
		m.mu.RLock()
		key, ok := m.keys[kid]
		m.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("jwt: unknown key id %q", kid)
		}
		return key.Secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	// 黑名单检查:若 token 有 jti 且在黑名单中,则拒绝
	if c.ID != "" && m.revocation.IsRevoked(c.ID) {
		return nil, errors.New("token has been revoked")
	}
	return c, nil
}

func mustJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 回退到时间戳+纳秒，保证唯一性
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
