package common

import "github.com/gofiber/fiber/v2"

func NotImplemented(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"message": "preview surface: endpoint scaffolded but not yet part of the proven issue/sync/conflict workflow",
	})
}
