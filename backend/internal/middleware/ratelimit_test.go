package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func setupMiniredis(t *testing.T) *miniredis.Miniredis {
	s := miniredis.RunT(t)
	return s
}

func setupGinWithRedis(s *miniredis.Miniredis) (*gin.Context, *httptest.ResponseRecorder, *redis.Client) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return c, w, rdb
}

func TestRateLimit_NoUID(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)

	handler := RateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRateLimit_UIDInt64(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)
	c.Set("uid", int64(42))

	handler := RateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code == http.StatusTooManyRequests {
		t.Errorf("unexpected rate limit, body: %s", w.Body.String())
	}
}

func TestRateLimit_UIDInt(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)
	c.Set("uid", int(42))

	handler := RateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code == http.StatusTooManyRequests {
		t.Errorf("unexpected rate limit, body: %s", w.Body.String())
	}
}

func TestRateLimit_UIDFloat64(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)
	c.Set("uid", float64(42))

	handler := RateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code == http.StatusTooManyRequests {
		t.Errorf("unexpected rate limit, body: %s", w.Body.String())
	}
}

func TestRateLimit_UIDString(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)
	c.Set("uid", "42")

	handler := RateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code == http.StatusTooManyRequests {
		t.Errorf("unexpected rate limit, body: %s", w.Body.String())
	}
}

func TestRateLimit_UIDStringInvalid(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)
	c.Set("uid", "not-a-number")

	handler := RateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRateLimit_UIDUnsupportedType(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)
	c.Set("uid", struct{}{})

	handler := RateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRateLimit_ExceedBurst(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)
	c.Set("uid", int64(99))

	handler := RateLimit(rdb, zap.NewNop())

	// burst = 5, so first 5 should pass, 6th should be rate limited
	for i := 0; i < 6; i++ {
		c, w, rdb = setupGinWithRedis(s)
		c.Set("uid", int64(99))
		handler(c)
	}

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d (rate limited)", w.Code, http.StatusTooManyRequests)
	}
}

func TestIPRateLimit_Allowed(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)
	c.Request.RemoteAddr = "192.168.1.1:12345"

	handler := IPRateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code == http.StatusTooManyRequests {
		t.Errorf("unexpected rate limit, body: %s", w.Body.String())
	}
}

func TestIPRateLimit_WithXForwardedFor(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)
	c.Request.Header.Set("X-Forwarded-For", "10.0.0.1")

	handler := IPRateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code == http.StatusTooManyRequests {
		t.Errorf("unexpected rate limit, body: %s", w.Body.String())
	}
}

func TestIPRateLimit_EmptyIP(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)
	c.Request.RemoteAddr = ":12345"

	handler := IPRateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code == http.StatusTooManyRequests {
		t.Errorf("unexpected rate limit, body: %s", w.Body.String())
	}
}

func TestGlobalRateLimit_Allowed(t *testing.T) {
	s := setupMiniredis(t)
	c, w, rdb := setupGinWithRedis(s)

	handler := GlobalRateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code == http.StatusTooManyRequests {
		t.Errorf("unexpected rate limit, body: %s", w.Body.String())
	}
}

func TestGlobalRateLimit_ReleaseUsesBackgroundContext(t *testing.T) {
	s := setupMiniredis(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(GlobalRateLimit(rdb, zap.NewNop()))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	reqCtx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(reqCtx, "GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	cancel()

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got, _ := s.Get("ratelimit:global:concurrent"); got != "" {
		t.Fatalf("expected global concurrent counter released, got %q", got)
	}
}

func TestGlobalRateLimit_ExceedMaxConcurrent(t *testing.T) {
	s := setupMiniredis(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	// Set the global counter above maxConcurrent (1000)
	s.Set("ratelimit:global:concurrent", "1001")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	handler := GlobalRateLimit(rdb, zap.NewNop())
	handler(c)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}
