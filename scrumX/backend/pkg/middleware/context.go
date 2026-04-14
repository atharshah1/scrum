package middleware

import "github.com/gofiber/fiber/v2"
import "github.com/google/uuid"

type contextKey string

const (
	userIDKey contextKey = "userID"
	orgIDKey  contextKey = "orgID"
	roleKey   contextKey = "role"
)

func SetUserID(c *fiber.Ctx, id uuid.UUID) {
	c.Locals(string(userIDKey), id)
}

func SetOrgID(c *fiber.Ctx, id uuid.UUID) {
	c.Locals(string(orgIDKey), id)
}

func SetRole(c *fiber.Ctx, role string) {
	c.Locals(string(roleKey), role)
}
