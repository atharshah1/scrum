package projects

import (
	"database/sql"
	"strings"
	"time"

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
	r := api.Group("/projects")
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/:id", h.get)
}

func (h *Handler) create(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	role, err := h.authz.RequireOrgMember(c.Context(), orgID, actorID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var payload struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	name := strings.TrimSpace(payload.Name)
	key := normalizeProjectKey(payload.Key)
	if name == "" || key == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "name and key are required")
	}
	projectID := uuid.New()
	if _, err := h.db.ExecContext(c.Context(), `INSERT INTO projects (id, org_id, key, name) VALUES ($1,$2,$3,$4)`,
		projectID, orgID, key, name); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	_, _ = h.db.ExecContext(c.Context(), `INSERT INTO project_memberships (id, org_id, project_id, user_id, role)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (org_id, project_id, user_id) DO UPDATE SET role=EXCLUDED.role`, uuid.New(), orgID, projectID, actorID, authz.RoleAdmin)
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{
		"id":   projectID,
		"key":  key,
		"name": name,
	})
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	if _, err := h.authz.RequireOrgMember(c.Context(), orgID, actorID); err != nil {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	rows, err := h.db.QueryContext(c.Context(), `SELECT p.id, p.key, p.name, p.created_at, p.updated_at, COALESCE(pm.role, m.role) AS role
FROM projects p
JOIN memberships m ON m.org_id=p.org_id AND m.user_id=$2
LEFT JOIN project_memberships pm ON pm.org_id=p.org_id AND pm.project_id=p.id AND pm.user_id=$2
WHERE p.org_id=$1
ORDER BY p.updated_at DESC, p.created_at DESC`, orgID, actorID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	result := make([]fiber.Map, 0, 16)
	for rows.Next() {
		var (
			id        uuid.UUID
			key       string
			name      string
			createdAt time.Time
			updatedAt time.Time
			role      string
		)
		if err := rows.Scan(&id, &key, &name, &createdAt, &updatedAt, &role); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		result = append(result, fiber.Map{
			"id":         id,
			"key":        key,
			"name":       name,
			"role":       role,
			"created_at": createdAt,
			"updated_at": updatedAt,
		})
	}
	return utils.JSONSuccess(c, fiber.StatusOK, result)
}

func (h *Handler) get(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid project id")
	}
	role, err := h.authz.ResolveProjectRole(c.Context(), orgID, actorID, projectID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var (
		key       string
		name      string
		createdAt time.Time
		updatedAt time.Time
	)
	if err := h.db.QueryRowContext(c.Context(), `SELECT key, name, created_at, updated_at FROM projects WHERE id=$1 AND org_id=$2`,
		projectID, orgID).Scan(&key, &name, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return utils.JSONError(c, fiber.StatusNotFound, "project not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{
		"id":         projectID,
		"key":        key,
		"name":       name,
		"role":       role,
		"created_at": createdAt,
		"updated_at": updatedAt,
	})
}

func normalizeProjectKey(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range v {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
