package api

import (
	"context"
	"time"

	"publishing-backend/db"

	"github.com/redis/go-redis/v9"
)

const (
	EventCacheHit        = "cache.hit"
	EventCacheMiss       = "cache.miss"
	EventCacheSet        = "cache.set"
	EventCacheInvalidate = "cache.invalidate"
	EventCacheError      = "cache.error"
)

func logCache(event, key, result string, err error) {
	args := []any{
		"event", event,
		"cache_key", key,
		"result", result,
	}
	if err != nil {
		args = append(args, "error", err.Error())
		Logger.Error(event, args...)
		return
	}
	Logger.Info(event, args...)
}

// CacheGet возвращает значение из Redis . При miss или ошибке — пустая строка и false.
func CacheGet(ctx context.Context, key string) (string, bool) {
	if db.Redis == nil {
		logCache(EventCacheError, key, "error", redis.ErrClosed)
		return "", false
	}

	val, err := db.Redis.Get(ctx, key).Result()
	if err == redis.Nil {
		logCache(EventCacheMiss, key, "miss", nil)
		return "", false
	}
	if err != nil {
		logCache(EventCacheError, key, "error", err)
		return "", false
	}

	logCache(EventCacheHit, key, "hit", nil)
	return val, true
}

// CacheSet записывает значение в Redis с TTL.
func CacheSet(ctx context.Context, key, value string, ttl time.Duration) {
	if db.Redis == nil {
		logCache(EventCacheError, key, "error", redis.ErrClosed)
		return
	}
	if err := db.Redis.Set(ctx, key, value, ttl).Err(); err != nil {
		logCache(EventCacheError, key, "error", err)
		return
	}
	logCache(EventCacheSet, key, "set", nil)
}

// CacheInvalidate удаляет ключ из Redis (delete on write).
func CacheInvalidate(ctx context.Context, key string) {
	if db.Redis == nil {
		logCache(EventCacheError, key, "error", redis.ErrClosed)
		return
	}
	if err := db.Redis.Del(ctx, key).Err(); err != nil {
		logCache(EventCacheError, key, "error", err)
		return
	}
	logCache(EventCacheInvalidate, key, "invalidate", nil)
}
