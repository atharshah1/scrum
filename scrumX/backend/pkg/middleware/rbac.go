package middleware

import "github.com/gofiber/fiber/v2"

func RBACMiddleware(allowedRoles ...string) fiber.Handler {
allowed := map[string]struct{}{}
for _, role := range allowedRoles {
allowed[role] = struct{}{}
}
return func(c *fiber.Ctx) error {
if _, ok := allowed[Role(c)]; !ok {
return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
}
return c.Next()
}
}
