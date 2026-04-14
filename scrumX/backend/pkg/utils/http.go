package utils

import "github.com/gofiber/fiber/v2"

func JSONError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"message": message,
		},
	})
}

func JSONSuccess(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"data":    data,
	})
}

func JSONList(c *fiber.Ctx, items any, page, limit, total int) error {
	return c.JSON(fiber.Map{
		"success": true,
		"data":    items,
		"meta": fiber.Map{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}
