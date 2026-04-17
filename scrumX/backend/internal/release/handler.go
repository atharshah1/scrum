package releasemodule

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/authz"
	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	db    *sql.DB
	authz *authz.Service
	bus   events.Publisher
}

func NewHandler(db *sql.DB, authzService *authz.Service, bus events.Publisher) *Handler {
	return &Handler{db: db, authz: authzService, bus: bus}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	r := api.Group("/releases")
	r.Post("/", h.createRelease)
	r.Get("/", h.listReleases)
	r.Post("/:id/deploy", h.deployRelease)
	api.Post("/deployments", h.createDeployment)
}

func (h *Handler) createRelease(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	var payload struct {
		ProjectID uuid.UUID `json:"project_id"`
		Version   string    `json:"version"`
		Status    string    `json:"status"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	payload.Version = strings.TrimSpace(payload.Version)
	payload.Status = strings.TrimSpace(payload.Status)
	if payload.ProjectID == uuid.Nil || payload.Version == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "project_id and version are required")
	}
	role, err := h.authz.ResolveProjectRole(c.Context(), orgID, userID, payload.ProjectID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	if payload.Status == "" {
		payload.Status = "planned"
	}
	releaseID := uuid.New()
	if _, err := h.db.ExecContext(c.Context(), `INSERT INTO releases (id, org_id, project_id, version, status) VALUES ($1,$2,$3,$4,$5)`,
		releaseID, orgID, payload.ProjectID, payload.Version, strings.ToLower(payload.Status)); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	_ = h.bus.Publish(c.Context(), events.New(orgID, "release.created", userID, map[string]any{
		"release_id": releaseID,
		"project_id": payload.ProjectID,
		"version":    payload.Version,
		"status":     strings.ToLower(payload.Status),
	}, events.WithProjectID(payload.ProjectID)))
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{
		"id":         releaseID,
		"project_id": payload.ProjectID,
		"version":    payload.Version,
		"status":     strings.ToLower(payload.Status),
	})
}

func (h *Handler) listReleases(c *fiber.Ctx) error {
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
	projectFilter := strings.TrimSpace(c.Query("project_id"))
	statusFilter := strings.ToLower(strings.TrimSpace(c.Query("status")))
	args := []any{orgID}
	where := []string{"r.org_id=$1"}
	if projectFilter != "" {
		projectID, err := uuid.Parse(projectFilter)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid project_id")
		}
		role, err := h.authz.ResolveProjectRole(c.Context(), orgID, userID, projectID)
		if err != nil || role == "" {
			return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
		}
		args = append(args, projectID)
		where = append(where, "r.project_id=$2")
	}
	if statusFilter != "" {
		args = append(args, statusFilter)
		statusArg := strconv.Itoa(len(args))
		where = append(where, "LOWER(r.status)=$"+statusArg)
	}
	query := `SELECT r.id, r.project_id, r.version, r.status, r.created_at,
COALESCE((SELECT COUNT(1) FROM deployments d WHERE d.org_id=r.org_id AND d.release_id=r.id),0) AS deployment_count,
COALESCE((SELECT d.status FROM deployments d WHERE d.org_id=r.org_id AND d.release_id=r.id ORDER BY d.created_at DESC LIMIT 1),'') AS latest_deployment_status
FROM releases r
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY r.created_at DESC`
	rows, err := h.db.QueryContext(c.Context(), query, args...)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	result := make([]fiber.Map, 0, 16)
	for rows.Next() {
		var (
			releaseID            uuid.UUID
			projectID            uuid.UUID
			version              string
			status               string
			createdAt            time.Time
			deploymentCount      int
			latestDeploymentStat string
		)
		if err := rows.Scan(&releaseID, &projectID, &version, &status, &createdAt, &deploymentCount, &latestDeploymentStat); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		result = append(result, fiber.Map{
			"id":                       releaseID,
			"project_id":               projectID,
			"version":                  version,
			"status":                   status,
			"created_at":               createdAt,
			"deployment_count":         deploymentCount,
			"latest_deployment_status": latestDeploymentStat,
		})
	}
	return utils.JSONSuccess(c, fiber.StatusOK, result)
}

func (h *Handler) deployRelease(c *fiber.Ctx) error {
	releaseID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid release id")
	}
	var payload struct {
		EnvironmentID *uuid.UUID `json:"environment_id"`
		Status        string     `json:"status"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	return h.createDeploymentFromInput(c, releaseID, payload.EnvironmentID, payload.Status)
}

func (h *Handler) createDeployment(c *fiber.Ctx) error {
	var payload struct {
		ReleaseID     uuid.UUID  `json:"release_id"`
		EnvironmentID *uuid.UUID `json:"environment_id"`
		Status        string     `json:"status"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if payload.ReleaseID == uuid.Nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "release_id is required")
	}
	return h.createDeploymentFromInput(c, payload.ReleaseID, payload.EnvironmentID, payload.Status)
}

func (h *Handler) createDeploymentFromInput(c *fiber.Ctx, releaseID uuid.UUID, environmentID *uuid.UUID, status string) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "started"
	}

	var projectID uuid.UUID
	if err := h.db.QueryRowContext(c.Context(), `SELECT project_id FROM releases WHERE id=$1 AND org_id=$2`, releaseID, orgID).Scan(&projectID); err != nil {
		if err == sql.ErrNoRows {
			return utils.JSONError(c, fiber.StatusNotFound, "release not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	role, err := h.authz.ResolveProjectRole(c.Context(), orgID, userID, projectID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	if environmentID != nil {
		var envProjectID uuid.UUID
		if err := h.db.QueryRowContext(c.Context(), `SELECT project_id FROM environments WHERE id=$1 AND org_id=$2`,
			*environmentID, orgID).Scan(&envProjectID); err != nil {
			if err == sql.ErrNoRows {
				return utils.JSONError(c, fiber.StatusBadRequest, "environment not found")
			}
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		if envProjectID != projectID {
			return utils.JSONError(c, fiber.StatusBadRequest, "environment is not mapped to release project")
		}
	}

	deploymentID := uuid.New()
	if _, err := h.db.ExecContext(c.Context(), `INSERT INTO deployments (id, org_id, release_id, environment_id, status) VALUES ($1,$2,$3,$4,$5)`,
		deploymentID, orgID, releaseID, environmentID, status); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	_ = h.bus.Publish(c.Context(), events.New(orgID, "deployment.created", userID, map[string]any{
		"deployment_id":  deploymentID,
		"release_id":     releaseID,
		"environment_id": environmentID,
		"status":         status,
	}, events.WithProjectID(projectID)))
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{
		"id":             deploymentID,
		"release_id":     releaseID,
		"environment_id": environmentID,
		"status":         status,
	})
}
