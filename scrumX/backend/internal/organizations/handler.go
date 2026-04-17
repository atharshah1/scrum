package organizations

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
	r := api.Group("/orgs")
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Post("/:id/invite", h.invite)
}

func (h *Handler) list(c *fiber.Ctx) error {
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	rows, err := h.db.QueryContext(c.Context(), `SELECT o.id, o.name, o.slug, m.role, o.created_at
FROM memberships m
JOIN organizations o ON o.id=m.org_id
WHERE m.user_id=$1
ORDER BY o.created_at DESC`, actorID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	result := make([]fiber.Map, 0, 8)
	for rows.Next() {
		var (
			id        uuid.UUID
			name      string
			slug      string
			role      string
			createdAt time.Time
		)
		if err := rows.Scan(&id, &name, &slug, &role, &createdAt); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		result = append(result, fiber.Map{
			"id":         id,
			"name":       name,
			"slug":       slug,
			"role":       role,
			"created_at": createdAt,
		})
	}
	return utils.JSONSuccess(c, fiber.StatusOK, result)
}

func (h *Handler) create(c *fiber.Ctx) error {
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	var payload struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "name is required")
	}
	slug := normalizeSlug(payload.Slug)
	if slug == "" {
		slug = normalizeSlug(name)
	}
	if slug == "" {
		slug = "workspace"
	}
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	slug = slug + "-" + suffix

	tx, err := h.db.BeginTx(c.Context(), nil)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer tx.Rollback()

	orgID := uuid.New()
	if _, err := tx.ExecContext(c.Context(), `INSERT INTO organizations (id, name, slug) VALUES ($1,$2,$3)`, orgID, name, slug); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	if _, err := tx.ExecContext(c.Context(), `INSERT INTO memberships (id, org_id, user_id, role) VALUES ($1,$2,$3,$4)`,
		uuid.New(), orgID, actorID, authz.RoleAdmin); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	if err := tx.Commit(); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{
		"id":                orgID,
		"name":              name,
		"slug":              slug,
		"bootstrap_user_id": actorID,
	})
}

func (h *Handler) invite(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	pathOrgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid organization id")
	}
	if pathOrgID != orgID {
		return utils.JSONError(c, fiber.StatusForbidden, "cross-org invite is not allowed")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	role, err := h.authz.RequireOrgMember(c.Context(), orgID, actorID)
	if err != nil || role != authz.RoleAdmin {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var payload struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	targetRole := strings.TrimSpace(payload.Role)
	if email == "" || !strings.Contains(email, "@") {
		return utils.JSONError(c, fiber.StatusBadRequest, "valid email is required")
	}
	switch targetRole {
	case authz.RoleAdmin, authz.RoleMember, authz.RoleViewer:
	default:
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid role")
	}

	tx, err := h.db.BeginTx(c.Context(), nil)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer tx.Rollback()

	var invitedUserID uuid.UUID
	// Global identity lookup: users are reusable across organizations and membership
	// rows establish tenancy boundaries.
	err = tx.QueryRowContext(c.Context(), `SELECT id FROM users WHERE email=$1 ORDER BY created_at DESC LIMIT 1`, email).Scan(&invitedUserID)
	if err != nil {
		if err != sql.ErrNoRows {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		invitedUserID = uuid.New()
		if _, err := tx.ExecContext(c.Context(), `INSERT INTO users (id, org_id, email) VALUES ($1,$2,$3)`, invitedUserID, orgID, email); err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
	}
	if _, err := tx.ExecContext(c.Context(), `INSERT INTO memberships (id, org_id, user_id, role)
VALUES ($1,$2,$3,$4)
ON CONFLICT (org_id, user_id) DO UPDATE SET role=EXCLUDED.role`, uuid.New(), orgID, invitedUserID, targetRole); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	if err := tx.Commit(); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{
		"org_id":  orgID,
		"user_id": invitedUserID,
		"email":   email,
		"role":    targetRole,
	})
}

func normalizeSlug(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, ch := range v {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
