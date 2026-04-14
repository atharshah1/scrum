package middleware

import (
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/pkg/observability"
	"github.com/gofiber/fiber/v2"
)

func MetricsMiddleware(metrics *observability.Metrics) fiber.Handler {
	return func(c *fiber.Ctx) error {
		started := time.Now()
		err := c.Next()
		if metrics != nil {
			metrics.ObserveRequest(c.Method(), c.Route().Path, c.Response().StatusCode(), time.Since(started))
		}
		return err
	}
}
