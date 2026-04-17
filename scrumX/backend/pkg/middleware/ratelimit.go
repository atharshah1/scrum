package middleware

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

type RateLimitKeyFunc func(c *fiber.Ctx) string

func RateLimitMiddleware(limit int, window time.Duration, redisClient *redis.Client) fiber.Handler {
	return RateLimitMiddlewareWithKey(limit, window, redisClient, nil)
}

func RateLimitMiddlewareWithKey(limit int, window time.Duration, redisClient *redis.Client, keyFn RateLimitKeyFunc) fiber.Handler {
	var (
		mu          sync.Mutex
		entries     = map[string]rateLimitEntry{}
		nextCleanup time.Time
	)

	localEval := func(key string, now time.Time) (count int, resetAt time.Time) {
		mu.Lock()
		defer mu.Unlock()
		if nextCleanup.IsZero() || !now.Before(nextCleanup) {
			for existingKey, existingEntry := range entries {
				if !now.Before(existingEntry.resetAt) {
					delete(entries, existingKey)
				}
			}
			nextCleanup = now.Add(window)
		}
		entry, ok := entries[key]
		if !ok || now.After(entry.resetAt) {
			entry = rateLimitEntry{count: 0, resetAt: now.Add(window)}
		}
		entry.count++
		entries[key] = entry
		return entry.count, entry.resetAt
	}

	redisEval := func(key string, now time.Time) (count int, resetAt time.Time, ok bool) {
		if redisClient == nil {
			return 0, time.Time{}, false
		}
		windowSeconds := int(window.Seconds())
		if windowSeconds <= 0 {
			return 0, time.Time{}, false
		}
		windowBucket := now.Unix() / int64(windowSeconds)
		redisKey := fmt.Sprintf("rate_limit:%d:%s", windowBucket, key)
		ctx := context.Background()
		total, err := redisClient.Incr(ctx, redisKey).Result()
		if err != nil {
			return 0, time.Time{}, false
		}
		if total == 1 {
			_ = redisClient.Expire(ctx, redisKey, window).Err()
		}
		resetUnix := (windowBucket + 1) * int64(windowSeconds)
		return int(total), time.Unix(resetUnix, 0).UTC(), true
	}

	return func(c *fiber.Ctx) error {
		if limit <= 0 || window <= 0 {
			return c.Next()
		}
		key := deriveRateLimitKey(c, keyFn)
		now := time.Now()
		count, resetAt, remote := redisEval(key, now)
		if !remote {
			count, resetAt = localEval(key, now)
		}
		remaining := limit - count

		c.Set("X-RateLimit-Limit", itoa(limit))
		if remaining < 0 {
			remaining = 0
		}
		c.Set("X-RateLimit-Remaining", itoa(remaining))
		c.Set("X-RateLimit-Reset", itoa(int(resetAt.Unix())))
		if count > limit {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"message": "rate limit exceeded"},
			})
		}
		return c.Next()
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func deriveRateLimitKey(c *fiber.Ctx, keyFn RateLimitKeyFunc) string {
	if keyFn != nil {
		if key := keyFn(c); key != "" {
			return key
		}
	}
	base := c.Route().Path
	if orgID, ok := MustOrgID(c); ok {
		if userID, userOK := MustUserID(c); userOK {
			return fmt.Sprintf("org:%s:user:%s:route:%s", orgID, userID, base)
		}
		return fmt.Sprintf("org:%s:ip:%s:route:%s", orgID, c.IP(), base)
	}
	return c.IP() + ":" + base
}
