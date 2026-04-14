package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func ProtectMetrics(token string) fiber.Handler {
	token = strings.TrimSpace(token)
	return func(c *fiber.Ctx) error {
		ip := strings.TrimSpace(c.IP())
		if ip == "127.0.0.1" || ip == "::1" || ip == "::ffff:127.0.0.1" {
			return c.Next()
		}
		if token == "" {
			return c.SendStatus(fiber.StatusForbidden)
		}
		authHeader := strings.TrimSpace(c.Get("Authorization"))
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		provided := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Next()
	}
}
