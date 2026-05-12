package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/jwt"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			b := make([]byte, 12)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
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
		log.Info("access",
			zap.String("rid", c.GetString("request_id")),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
		)
	}
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
