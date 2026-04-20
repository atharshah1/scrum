package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func AuthMiddleware(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing Authorization header"})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid Authorization header"})
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "unexpected signing method")
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid claims"})
		}
		if tokenType := asString(claims["type"]); tokenType != "access" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token type"})
		}

		uid, err := uuid.Parse(asString(claims["sub"]))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid subject claim"})
		}

		SetUserID(c, uid)
		SetRole(c, asString(claims["role"]))
		SetClientID(c, asString(claims["client_id"]))
		SetScopes(c, parseScopes(claims["scopes"]))

		if org := asString(claims["org_id"]); org != "" {
			if oid, parseErr := uuid.Parse(org); parseErr == nil {
				SetOrgID(c, oid)
			}
		}

		return c.Next()
	}
}

func asString(value interface{}) string {
	if value == nil {
		return ""
	}
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}

func parseScopes(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case []string:
		return compactScopes(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, raw := range v {
			if scope := asString(raw); scope != "" {
				out = append(out, scope)
			}
		}
		return compactScopes(out)
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return compactScopes(strings.Fields(v))
	default:
		return nil
	}
}

func compactScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
