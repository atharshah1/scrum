package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func RequireScopes(required ...string) fiber.Handler {
	requiredSet := map[string]struct{}{}
	for _, scope := range required {
		scope = strings.TrimSpace(strings.ToLower(scope))
		if scope == "" {
			continue
		}
		requiredSet[scope] = struct{}{}
	}
	return func(c *fiber.Ctx) error {
		if len(requiredSet) == 0 {
			return c.Next()
		}
		clientID := strings.TrimSpace(ClientID(c))
		tokenScopes := Scopes(c)
		if clientID == "" && len(tokenScopes) == 0 {
			return c.Next()
		}
		available := map[string]struct{}{}
		for _, scope := range tokenScopes {
			scope = strings.TrimSpace(strings.ToLower(scope))
			if scope == "" {
				continue
			}
			available[scope] = struct{}{}
		}
		for scope := range requiredSet {
			if _, ok := available[scope]; !ok {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "insufficient_scope",
				})
			}
		}
		return c.Next()
	}
}
