package middleware

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// redisOpCtx 限流计数释放不得绑定请求 ctx：客户端超时/断连会取消 ctx，导致 DECR 失败、并发计数泄漏。
func redisOpCtx() context.Context {
	return context.Background()
}

// RateLimit 基于 Redis 令牌桶的按用户限流中间件。
// 速率: 每分钟 2 个令牌, 桶容量(burst): 5。
// 超限返回 HTTP 429。
func RateLimit(rdb *redis.Client) gin.HandlerFunc {
	const (
		ratePerMin = 2
		burst      = 5
		windowSec  = 60
	)

	// Lua 脚本: 原子化令牌桶逻辑
	script := redis.NewScript(`
		local key = KEYS[1]
		local burst = tonumber(ARGV[1])
		local rate = tonumber(ARGV[2])
		local window = tonumber(ARGV[3])
		local now = tonumber(ARGV[4])

		local data = redis.call("HMGET", key, "tokens", "last")
		local tokens = burst
		local last = now

		if data[1] and data[2] then
			tokens = tonumber(data[1])
			last = tonumber(data[2])
		end

		local elapsed = math.max(0, now - last)
		tokens = math.min(burst, tokens + elapsed * rate / window)

		if tokens < 1 then
			redis.call("HSET", key, "tokens", tokens, "last", last)
			redis.call("EXPIRE", key, window * 2)
			return 0
		end

		tokens = tokens - 1
		redis.call("HSET", key, "tokens", tokens, "last", now)
		redis.call("EXPIRE", key, window * 2)
		return 1
	`)

	return func(c *gin.Context) {
		uidVal, exists := c.Get("uid")
		if !exists {
			response.Fail(c, errcode.ErrUnauthorized)
			c.Abort()
			return
		}
		var uid int64
		switch v := uidVal.(type) {
		case int64:
			uid = v
		case int:
			uid = int64(v)
		case float64:
			uid = int64(v)
		case string:
			if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
				uid = parsed
			} else {
				response.Fail(c, errcode.ErrUnauthorized)
				c.Abort()
				return
			}
		default:
			response.Fail(c, errcode.ErrUnauthorized)
			c.Abort()
			return
		}

		key := fmt.Sprintf("ratelimit:user:%d", uid)
		now := time.Now().Unix()

		allowed, err := script.Run(c.Request.Context(), rdb, []string{key},
			strconv.Itoa(burst),
			strconv.Itoa(ratePerMin),
			strconv.Itoa(windowSec),
			strconv.FormatInt(now, 10),
		).Int()

		if err != nil {
			c.Next()
			return
		}
		if allowed == 0 {
			response.Fail(c, errcode.ErrRateLimit)
			c.Abort()
			return
		}
		c.Next()
	}
}

// clientIP 从请求中提取客户端真实 IP，优先读取 X-Forwarded-For / X-Real-Ip。
func clientIP(c *gin.Context) string {
	// 优先 X-Forwarded-For，取第一个合法 IP
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		if len(xff) > 500 {
			xff = xff[:500]
		}
		for _, part := range strings.Split(xff, ",") {
			ip := strings.TrimSpace(part)
			if ip != "" && net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	if xri := c.GetHeader("X-Real-Ip"); xri != "" {
		ip := strings.TrimSpace(xri)
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	// gin 的 ClientIP() 会处理 RemoteAddr，但在有代理时可能不准确，
	// 这里作为兜底
	if ip := c.ClientIP(); ip != "" {
		return ip
	}
	// 极端兜底
	host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
	return host
}

// IPRateLimit 基于客户端 IP 的限流中间件（令牌桶）。
// 默认速率: 100 请求/分钟, burst: 150。
// 超限返回 HTTP 429。
func IPRateLimit(rdb *redis.Client) gin.HandlerFunc {
	const (
		ratePerMin = 100
		burst      = 150
		windowSec  = 60
	)

	script := redis.NewScript(`
		local key = KEYS[1]
		local burst = tonumber(ARGV[1])
		local rate = tonumber(ARGV[2])
		local window = tonumber(ARGV[3])
		local now = tonumber(ARGV[4])

		local data = redis.call("HMGET", key, "tokens", "last")
		local tokens = burst
		local last = now

		if data[1] and data[2] then
			tokens = tonumber(data[1])
			last = tonumber(data[2])
		end

		local elapsed = math.max(0, now - last)
		tokens = math.min(burst, tokens + elapsed * rate / window)

		if tokens < 1 then
			redis.call("HSET", key, "tokens", tokens, "last", last)
			redis.call("EXPIRE", key, window * 2)
			return 0
		end

		tokens = tokens - 1
		redis.call("HSET", key, "tokens", tokens, "last", now)
		redis.call("EXPIRE", key, window * 2)
		return 1
	`)

	return func(c *gin.Context) {
		ip := clientIP(c)
		if ip == "" {
			ip = "unknown"
		}
		// 对 IPv6 地址中的冒号做替换，避免 Redis key 问题
		key := fmt.Sprintf("ratelimit:ip:%s", strings.ReplaceAll(ip, ":", "_"))
		now := time.Now().Unix()

		allowed, err := script.Run(c.Request.Context(), rdb, []string{key},
			strconv.Itoa(burst),
			strconv.Itoa(ratePerMin),
			strconv.Itoa(windowSec),
			strconv.FormatInt(now, 10),
		).Int()

		if err != nil {
			// Redis 不可用时 fail-open，避免整站不可用
			c.Next()
			return
		}
		if allowed == 0 {
			response.Fail(c, errcode.ErrRateLimit.WithMsg("IP 请求过于频繁，请稍后再试"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// GlobalRateLimit 全局并发请求数限制中间件。
// 使用 Redis INCR + EXPIRE 实现滑动窗口计数器，控制最大并发请求数。
// 默认最大并发: 1000。
// 超限返回 HTTP 429。
func GlobalRateLimit(rdb *redis.Client) gin.HandlerFunc {
	const (
		maxConcurrent = 1000
		windowSec     = 60
	)

	// Lua 脚本：原子化增加计数并设置过期时间
	// KEYS[1] = key, ARGV[1] = window
	// 返回当前计数
	acquireScript := redis.NewScript(`
		local key = KEYS[1]
		local window = tonumber(ARGV[1])

		local count = redis.call("INCR", key)
		if count == 1 then
			redis.call("EXPIRE", key, window)
		end
		return count
	`)

	// Lua 脚本：原子化减少计数
	releaseScript := redis.NewScript(`
		local key = KEYS[1]
		local count = redis.call("GET", key)
		if count then
			count = tonumber(count)
			if count > 1 then
				redis.call("DECR", key)
			else
				redis.call("DEL", key)
			end
		end
		return 0
	`)

	return func(c *gin.Context) {
		key := "ratelimit:global:concurrent"

		count, err := acquireScript.Run(c.Request.Context(), rdb, []string{key},
			strconv.Itoa(windowSec),
		).Int()

		if err != nil {
			c.Next()
			return
		}
		if count > maxConcurrent {
			_ = releaseScript.Run(redisOpCtx(), rdb, []string{key}).Err()
			response.Fail(c, errcode.ErrRateLimit.WithMsg("服务器繁忙，请稍后再试"))
			c.Abort()
			return
		}

		defer func() {
			_ = releaseScript.Run(redisOpCtx(), rdb, []string{key}).Err()
		}()

		c.Next()
	}
}
