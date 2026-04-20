package middleware

import (
	"context"
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type OrgMembershipChecker func(ctx context.Context, userID, orgID uuid.UUID) (bool, error)

func OrgContextMiddleware(db *sql.DB) fiber.Handler {
	return orgContextMiddleware(newOrgMembershipChecker(db))
}

func orgContextMiddleware(checkMembership OrgMembershipChecker) fiber.Handler {
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
			userID, ok := MustUserID(c)
			if !ok {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated user context"})
			}
			allowed, err := checkMembership(c.Context(), userID, orgID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to validate org access"})
			}
			if !allowed {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden org access"})
			}
			SetOrgID(c, orgID)
			return c.Next()
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing X-Org-ID header"})
	}
}

func newOrgMembershipChecker(db *sql.DB) OrgMembershipChecker {
	return func(ctx context.Context, userID, orgID uuid.UUID) (bool, error) {
		var exists bool
		err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memberships WHERE user_id=$1 AND org_id=$2)`, userID, orgID).Scan(&exists)
		if err != nil {
			return false, err
		}
		return exists, nil
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
