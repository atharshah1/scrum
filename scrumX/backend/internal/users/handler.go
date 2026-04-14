package users

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
	r := api.Group("/users")
	r.Get("/", h.list)
	r.Patch("/:id", h.update)
	r.Post("/:id/role", h.upsertRole)
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	rows, err := h.db.QueryContext(c.Context(), `SELECT u.id, u.email, COALESCE(u.full_name, ''), m.role
FROM users u
JOIN memberships m ON m.org_id=u.org_id AND m.user_id=u.id
WHERE u.org_id=$1
ORDER BY u.created_at DESC`, orgID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	result := []fiber.Map{}
	for rows.Next() {
		var id uuid.UUID
		var email, fullName, role string
		if err := rows.Scan(&id, &email, &fullName, &role); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		result = append(result, fiber.Map{"id": id, "email": email, "full_name": fullName, "role": role})
	}
	return utils.JSONSuccess(c, fiber.StatusOK, result)
}

func (h *Handler) update(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid user id")
	}
	var payload struct {
		FullName string `json:"full_name"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if _, err := h.db.ExecContext(c.Context(), `UPDATE users SET full_name=$1, updated_at=NOW() WHERE id=$2 AND org_id=$3`, payload.FullName, userID, orgID); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"id": userID, "full_name": payload.FullName})
}

func (h *Handler) upsertRole(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	actorRole, err := h.authz.RequireOrgMember(c.Context(), orgID, actorID)
	if err != nil || actorRole != "Admin" {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid user id")
	}
	var payload struct {
		Role      string    `json:"role"`
		ProjectID uuid.UUID `json:"project_id"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.Role == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if payload.ProjectID == uuid.Nil {
		_, err = h.db.ExecContext(c.Context(), `INSERT INTO memberships (id, org_id, user_id, role)
VALUES ($1,$2,$3,$4)
ON CONFLICT (org_id, user_id) DO UPDATE SET role=EXCLUDED.role`, uuid.New(), orgID, targetUserID, payload.Role)
	} else {
		_, err = h.db.ExecContext(c.Context(), `INSERT INTO project_memberships (id, org_id, project_id, user_id, role)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (org_id, project_id, user_id) DO UPDATE SET role=EXCLUDED.role`, uuid.New(), orgID, payload.ProjectID, targetUserID, payload.Role)
	}
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{
		"user_id":    targetUserID,
		"org_id":     orgID,
		"project_id": payload.ProjectID,
		"role":       payload.Role,
	})
}

