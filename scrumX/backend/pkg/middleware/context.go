package middleware

import "github.com/gofiber/fiber/v2"
import "github.com/google/uuid"

type contextKey string

const (
	userIDKey   contextKey = "userID"
	orgIDKey    contextKey = "orgID"
	roleKey     contextKey = "role"
	clientIDKey contextKey = "clientID"
	scopesKey   contextKey = "scopes"
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

func SetClientID(c *fiber.Ctx, clientID string) {
	c.Locals(string(clientIDKey), clientID)
}

func SetScopes(c *fiber.Ctx, scopes []string) {
	c.Locals(string(scopesKey), scopes)
}

func ClientID(c *fiber.Ctx) string {
	if value, ok := c.Locals(string(clientIDKey)).(string); ok {
		return value
	}
	return ""
}

func Scopes(c *fiber.Ctx) []string {
	if value, ok := c.Locals(string(scopesKey)).([]string); ok {
		return value
	}
	return nil
}
