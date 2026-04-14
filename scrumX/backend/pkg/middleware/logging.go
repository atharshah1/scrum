package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func LoggingMiddleware(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
			c.Set("X-Request-ID", requestID)
		}
		started := time.Now()
		err := c.Next()
		duration := time.Since(started)
		log.Info("http_request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", duration.Milliseconds(),
			"request_id", requestID,
		)
		if duration > 500*time.Millisecond {
			log.Warn("slow_request",
				"method", c.Method(),
				"path", c.Path(),
				"status", c.Response().StatusCode(),
				"duration_ms", duration.Milliseconds(),
				"request_id", requestID,
			)
		}
		return err
	}
}
