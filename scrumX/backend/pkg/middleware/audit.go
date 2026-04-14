package middleware

import (
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func AuditMiddleware(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()
		method := strings.ToUpper(c.Method())
		if method != fiber.MethodPost && method != fiber.MethodPatch && method != fiber.MethodDelete && method != fiber.MethodPut {
			return err
		}
		if c.Response().StatusCode() >= 400 {
			return err
		}
		org, ok := MustOrgID(c)
		if !ok {
			return err
		}
		userID, _ := MustUserID(c)
		_, _ = db.ExecContext(c.Context(), `INSERT INTO audit_logs (org_id, actor_id, action, entity_type, entity_id, before_values, after_values)
VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			org, userID, method+" "+c.Path(), "api", nil, nil, []byte("{}"),
		)
		return err
	}
}
