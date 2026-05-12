package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int64    `json:"uid"`
	Username string   `json:"un"`
	DeptID   int64    `json:"did"`
	Roles    []string `json:"r"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret           []byte
	accessExpiresIn  time.Duration
	refreshExpiresIn time.Duration
}

func NewManager(secret string, accessSec, refreshSec int) *Manager {
	return &Manager{
		secret:           []byte(secret),
		accessExpiresIn:  time.Duration(accessSec) * time.Second,
		refreshExpiresIn: time.Duration(refreshSec) * time.Second,
	}
}

func (m *Manager) Issue(c *Claims) (access, refresh string, err error) {
	now := time.Now()

	c.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiresIn)),
	}
	access, err = jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(m.secret)
	if err != nil {
		return
	}

	rc := *c
	rc.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpiresIn)),
		Subject:   "refresh",
	}
	refresh, err = jwt.NewWithClaims(jwt.SigningMethodHS256, &rc).SignedString(m.secret)
	return
}

func (m *Manager) Parse(token string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}
