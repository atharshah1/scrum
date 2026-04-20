package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func OrgContextMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if existing := c.Locals(string(orgIDKey)); existing != nil {
			return c.Next()
		}
		orgHeader := c.Get("X-Org-ID")
		if orgHeader != "" {
			orgID, err := uuid.Parse(orgHeader)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid org id"})
			}
			SetOrgID(c, orgID)
			return c.Next()
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing X-Org-ID header"})
	}
}

func MustOrgID(c *fiber.Ctx) (uuid.UUID, bool) {
	value := c.Locals(string(orgIDKey))
	orgID, ok := value.(uuid.UUID)
	return orgID, ok
}

func MustUserID(c *fiber.Ctx) (uuid.UUID, bool) {
	value := c.Locals(string(userIDKey))
	userID, ok := value.(uuid.UUID)
	return userID, ok
}

func Role(c *fiber.Ctx) string {
	if value, ok := c.Locals(string(roleKey)).(string); ok {
		return value
	}
	return "Viewer"
}
