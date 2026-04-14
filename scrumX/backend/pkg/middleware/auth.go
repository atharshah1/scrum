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
return []byte(jwtSecret), nil
})
if err != nil || !token.Valid {
return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
}

claims, ok := token.Claims.(jwt.MapClaims)
if !ok {
return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid claims"})
}

uid, err := uuid.Parse(asString(claims["sub"]))
if err != nil {
return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid subject claim"})
}

SetUserID(c, uid)
SetRole(c, asString(claims["role"]))

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
