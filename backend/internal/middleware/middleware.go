package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/jwt"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			b := make([]byte, 12)
			if _, err := rand.Read(b); err != nil {
				id = fmt.Sprintf("%d", time.Now().UnixNano())
			} else {
				id = hex.EncodeToString(b)
			}
		}
		c.Set("request_id", id)
		c.Writer.Header().Set("X-Request-Id", id)
		c.Next()
	}
}

func AccessLog(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.Request.URL.Path
		query := c.Request.URL.Query()
		if query.Get("token") != "" {
			query.Set("token", "***")
			path = path + "?" + query.Encode()
		}

		log.Info("access",
			zap.String("rid", c.GetString("request_id")),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
			zap.Any("headers", sanitizeHeaders(c.Request.Header)),
		)
	}
}

func sanitizeHeaders(h map[string][]string) map[string][]string {
	if h == nil {
		return nil
	}
	out := make(map[string][]string, len(h))
	for k, v := range h {
		kl := strings.ToLower(k)
		switch kl {
		case "authorization", "x-api-token":
			out[k] = []string{"***"}
		case "cookie":
			out[k] = []string{sanitizeCookie(v)}
		default:
			if strings.Contains(kl, "secret") || strings.Contains(kl, "key") || strings.Contains(kl, "token") || strings.Contains(kl, "password") || strings.Contains(kl, "credential") {
				out[k] = []string{"***"}
			} else {
				out[k] = v
			}
		}
	}
	return out
}

func sanitizeCookie(v []string) string {
	if len(v) == 0 {
		return ""
	}
	s := strings.Join(v, "; ")
	parts := strings.Split(s, ";")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if idx := strings.Index(p, "="); idx > 0 {
			name := strings.ToLower(strings.TrimSpace(p[:idx]))
			if name == "session" || name == "token" || strings.Contains(name, "auth") {
				parts[i] = p[:idx+1] + "***"
			}
		}
	}
	return strings.Join(parts, "; ")
}

func JWTAuth(mgr *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if len(auth) < 8 || auth[:7] != "Bearer " {
			response.Fail(c, errcode.ErrUnauthorized)
			return
		}
		claims, err := mgr.Parse(auth[7:])
		if err != nil {
			response.Fail(c, errcode.ErrTokenInvalid.Wrap(err))
			return
		}
		c.Set("uid", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("dept_id", claims.DeptID)
		c.Set("roles", claims.Roles)
		c.Next()
	}
}

// WSAuth 与 JWTAuth 类似,但允许从 ?token= 取 token(浏览器 WebSocket 无法设置 Authorization 头)
func WSAuth(mgr *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := c.Query("token")
		if tok == "" {
			auth := c.GetHeader("Authorization")
			if len(auth) >= 8 && auth[:7] == "Bearer " {
				tok = auth[7:]
			}
		}
		if tok == "" {
			response.Fail(c, errcode.ErrUnauthorized)
			return
		}
		claims, err := mgr.Parse(tok)
		if err != nil {
			response.Fail(c, errcode.ErrTokenInvalid.Wrap(err))
			return
		}
		c.Set("uid", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("dept_id", claims.DeptID)
		c.Set("roles", claims.Roles)
		c.Next()
	}
}

// Recovery 捕获 panic,用 zap 结构化日志记录 rid + stack trace,
// 并返回带 request_id 的统一错误响应(修复 P0 B1/B2)
func Recovery(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				rid := c.GetString("request_id")

				path := c.Request.URL.Path
				query := c.Request.URL.Query()
				if query.Get("token") != "" {
					query.Set("token", "***")
					path = path + "?" + query.Encode()
				}

				log.Error("panic recovered",
					zap.String("rid", rid),
					zap.String("method", c.Request.Method),
					zap.String("path", path),
					zap.String("ip", c.ClientIP()),
					zap.Any("panic", rec),
					zap.String("stack", string(debug.Stack())),
				)
				response.Fail(c, errcode.ErrInternal.WithMsg(fmt.Sprintf("internal error (rid: %s)", rid)))
				c.Abort()
			}
		}()
		c.Next()
	}
}

// RBAC 一个轻量级 RBAC 中间件,资源/动作由路由元数据决定
// 这里给出框架;每个路由可用 c.Set("rbac_obj","project") + c.Set("rbac_act","read") 标记
func RBAC(e *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, _ := c.Get("rbac_obj")
		act, _ := c.Get("rbac_act")
		if obj == nil || act == nil {
			c.Next()
			return
		}
		roles, _ := c.Get("roles")
		rs, _ := roles.([]string)
		for _, role := range rs {
			ok, _ := e.Enforce(role, obj.(string), act.(string))
			if ok {
				c.Next()
				return
			}
		}
		response.Fail(c, errcode.ErrForbidden)
	}
}

// idempotencyWriter 捕获响应体与状态码，供幂等中间件缓存首次响应。
type idempotencyWriter struct {
	gin.ResponseWriter
	body   []byte
	status int
}

func (w *idempotencyWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *idempotencyWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

// Idempotency 基于 Redis 的幂等性中间件。
// 客户端通过 X-Idempotency-Key 请求头传递幂等键; 服务端在 TTL(5分钟) 内对相同键去重。
// 首次请求放行并缓存响应; 重复请求直接返回缓存的响应。
func Idempotency(rdb *redis.Client) gin.HandlerFunc {
	const ttl = 5 * time.Minute
	return func(c *gin.Context) {
		key := c.GetHeader("X-Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}
		if rdb == nil {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		cacheKey := fmt.Sprintf("idempotency:%s", key)
		val, err := rdb.Get(ctx, cacheKey).Result()
		if err == nil && val != "" {
			// 尝试解析缓存的响应并原样返回
			var cached struct {
				Status int    `json:"s"`
				Body   string `json:"b"`
			}
			if json.Unmarshal([]byte(val), &cached) == nil && cached.Status != 0 {
				c.Status(cached.Status)
				c.Writer.Write([]byte(cached.Body))
			} else {
				response.Fail(c, errcode.ErrConflict.WithMsg("重复请求,请使用新的 X-Idempotency-Key"))
			}
			c.Abort()
			return
		}
		// 首次请求: 先占位防止并发重复，再捕获响应
		_ = rdb.Set(ctx, cacheKey, `{"s":202,"b":""}`, ttl).Err()
		cw := &idempotencyWriter{ResponseWriter: c.Writer, status: 200}
		c.Writer = cw
		c.Next()
		// 缓存成功的响应（只缓存 2xx 与 4xx 业务错误，不缓存 5xx）
		if cw.status < 500 {
			cached, _ := json.Marshal(struct {
				Status int    `json:"s"`
				Body   string `json:"b"`
			}{Status: cw.status, Body: string(cw.body)})
			_ = rdb.Set(ctx, cacheKey, string(cached), ttl).Err()
		}
	}
}
