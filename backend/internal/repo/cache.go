package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// CacheRepo 通用缓存包装器，为 repo 层提供 Redis 缓存能力
type CacheRepo struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewCacheRepo 创建 CacheRepo 实例
func NewCacheRepo(db *gorm.DB, rdb *redis.Client) *CacheRepo {
	return &CacheRepo{db: db, rdb: rdb}
}

// LoaderFunc 缓存未命中时从数据库加载数据的函数签名
type LoaderFunc[T any] func(ctx context.Context) (T, error)

// Get 通用缓存读取方法：先读 Redis，未命中则调用 loader 从数据库加载并写入缓存
func Get[T any](ctx context.Context, rdb *redis.Client, key string, loader LoaderFunc[T], ttl time.Duration) (T, error) {
	var zero T
	if rdb == nil {
		return loader(ctx)
	}

	// 尝试从 Redis 读取
	val, err := rdb.Get(ctx, key).Result()
	if err == nil {
		var result T
		if err := json.Unmarshal([]byte(val), &result); err == nil {
			return result, nil
		}
		// 反序列化失败，继续走 loader
	} else if err != redis.Nil {
		// Redis 出错但不阻断，降级到数据库
	}

	// 缓存未命中，从数据库加载
	result, err := loader(ctx)
	if err != nil {
		return zero, err
	}

	// 写入 Redis（忽略写入错误，不影响主流程）
	if data, err := json.Marshal(result); err == nil {
		rdb.Set(ctx, key, data, ttl)
	}

	return result, nil
}

// Delete 删除指定缓存键
func Delete(ctx context.Context, rdb *redis.Client, key string) error {
	if rdb == nil {
		return nil
	}
	return rdb.Del(ctx, key).Err()
}

// DeletePattern 按模式删除缓存键
func DeletePattern(ctx context.Context, rdb *redis.Client, pattern string) error {
	if rdb == nil {
		return nil
	}
	iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		return rdb.Del(ctx, keys...).Err()
	}
	return nil
}

// cacheKey 生成统一格式的缓存键：{resource}:{identifier}
func cacheKey(resource, identifier string) string {
	return fmt.Sprintf("%s:%s", resource, identifier)
}
