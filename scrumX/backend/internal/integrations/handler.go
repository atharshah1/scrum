package integrations

import (
	"database/sql"
	"encoding/json"
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
	r := api.Group("/integrations")
	r.Get("/", h.list)
	r.Post("/github", h.upsertGitHub)
	r.Post("/jira", h.upsertJira)
	r.Post("/slack", h.upsertSlack)
	r.Post("/cicd", h.upsertCICD)
	r.Post("/deployments", h.upsertDeployments)
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	if _, err := h.authz.RequireOrgMember(c.Context(), orgID, userID); err != nil {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	rows, err := h.db.QueryContext(c.Context(), `SELECT DISTINCT ON (provider) id, provider, credentials, created_at
FROM integrations
WHERE org_id=$1
ORDER BY provider, created_at DESC`, orgID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	out := make([]fiber.Map, 0, 8)
	for rows.Next() {
		var (
			id             uuid.UUID
			provider       string
			credentialsRaw []byte
			createdAt      time.Time
		)
		if err := rows.Scan(&id, &provider, &credentialsRaw, &createdAt); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		keys := credentialKeys(credentialsRaw)
		out = append(out, fiber.Map{
			"id":              id,
			"provider":        provider,
			"configured":      len(keys) > 0,
			"credential_keys": keys,
			"created_at":      createdAt,
		})
	}
	return utils.JSONSuccess(c, fiber.StatusOK, out)
}

func (h *Handler) upsertGitHub(c *fiber.Ctx) error      { return h.upsertProvider(c, "github") }
func (h *Handler) upsertJira(c *fiber.Ctx) error        { return h.upsertProvider(c, "jira") }
func (h *Handler) upsertSlack(c *fiber.Ctx) error       { return h.upsertProvider(c, "slack") }
func (h *Handler) upsertCICD(c *fiber.Ctx) error        { return h.upsertProvider(c, "cicd") }
func (h *Handler) upsertDeployments(c *fiber.Ctx) error { return h.upsertProvider(c, "deployments") }

func (h *Handler) upsertProvider(c *fiber.Ctx, provider string) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	role, err := h.authz.RequireOrgMember(c.Context(), orgID, userID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var payload struct {
		Credentials map[string]any `json:"credentials"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.Credentials == nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "credentials are required")
	}
	credentialsRaw, err := json.Marshal(payload.Credentials)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid credentials")
	}
	var integrationID uuid.UUID
	err = h.db.QueryRowContext(c.Context(), `SELECT id FROM integrations WHERE org_id=$1 AND provider=$2 ORDER BY created_at DESC LIMIT 1`,
		orgID, provider).Scan(&integrationID)
	switch err {
	case nil:
		if _, err := h.db.ExecContext(c.Context(), `UPDATE integrations SET credentials=$3 WHERE id=$1 AND org_id=$2`,
			integrationID, orgID, credentialsRaw); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
	case sql.ErrNoRows:
		integrationID = uuid.New()
		if _, err := h.db.ExecContext(c.Context(), `INSERT INTO integrations (id, org_id, provider, credentials) VALUES ($1,$2,$3,$4)`,
			integrationID, orgID, provider, credentialsRaw); err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
	default:
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{
		"id":              integrationID,
		"provider":        provider,
		"configured":      true,
		"credential_keys": credentialKeys(credentialsRaw),
	})
}

func credentialKeys(raw []byte) []string {
	decoded := map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return []string{}
	}
	keys := make([]string, 0, len(decoded))
	for k := range decoded {
		keys = append(keys, strings.TrimSpace(k))
	}
	return keys
}
