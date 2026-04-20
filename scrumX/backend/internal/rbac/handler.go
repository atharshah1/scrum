package rbac

import (
	"database/sql"

	"github.com/atharshah1/scrum/scrumX/backend/internal/authz"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	db    *sql.DB
	authz *authz.Service
}

func NewHandler(db *sql.DB, authzService *authz.Service) *Handler {
	return &Handler{db: db, authz: authzService}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	api.Get("/rbac", middleware.RequireScopes("admin:org"), h.getEffectiveAccess)
}

func (h *Handler) getEffectiveAccess(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	orgRole, err := h.authz.RequireOrgMember(c.Context(), orgID, userID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}

	projectRoles := make([]fiber.Map, 0, 8)
	rows, err := h.db.QueryContext(c.Context(), `SELECT project_id, role FROM project_memberships WHERE org_id=$1 AND user_id=$2 ORDER BY created_at DESC`,
		orgID, userID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var (
			projectID uuid.UUID
			role      string
		)
		if err := rows.Scan(&projectID, &role); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		projectRoles = append(projectRoles, fiber.Map{
			"project_id": projectID,
			"role":       role,
			"can_write":  authz.CanWrite(role),
		})
	}

	response := fiber.Map{
		"user_id":        userID,
		"org_id":         orgID,
		"org_role":       orgRole,
		"org_can_write":  authz.CanWrite(orgRole),
		"project_scopes": projectRoles,
	}
	if projectIDRaw := c.Query("project_id"); projectIDRaw != "" {
		projectID, parseErr := uuid.Parse(projectIDRaw)
		if parseErr != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid project_id")
		}
		projectRole, resolveErr := h.authz.ResolveProjectRole(c.Context(), orgID, userID, projectID)
		if resolveErr != nil {
			return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
		}
		response["effective"] = fiber.Map{
			"project_id": projectID,
			"role":       projectRole,
			"can_write":  authz.CanWrite(projectRole),
		}
	}
	return utils.JSONSuccess(c, fiber.StatusOK, response)
}
