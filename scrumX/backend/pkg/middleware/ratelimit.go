package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

func RateLimitMiddleware(limit int, window time.Duration) fiber.Handler {
	var (
		mu          sync.Mutex
		entries     = map[string]rateLimitEntry{}
		nextCleanup time.Time
	)
	return func(c *fiber.Ctx) error {
		if limit <= 0 || window <= 0 {
			return c.Next()
		}
		key := c.IP() + ":" + c.Route().Path
		now := time.Now()
		mu.Lock()
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
		remaining := limit - entry.count
		resetAt := entry.resetAt
		mu.Unlock()

		c.Set("X-RateLimit-Limit", itoa(limit))
		if remaining < 0 {
			remaining = 0
		}
		c.Set("X-RateLimit-Remaining", itoa(remaining))
		c.Set("X-RateLimit-Reset", itoa(int(resetAt.Unix())))
		if entry.count > limit {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{"message": "rate limit exceeded"},
			})
		}
		return c.Next()
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
