package insights

import (
	"database/sql"
	"slices"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler { return &Handler{db: db} }

func (h *Handler) RegisterRoutes(api fiber.Router) {
	group := api.Group("/insights")
	group.Get("/stuck", h.stuck)
	group.Get("/bottlenecks", h.bottlenecks)
	group.Get("/velocity", h.velocity)
	group.Get("/cycle-time", h.cycleTime)
}

func (h *Handler) stuck(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}

	query := `SELECT id, title, status, updated_at
FROM issues
WHERE org_id=$1 AND deleted_at IS NULL AND status != 'done' AND updated_at < NOW() - INTERVAL '2 days'`
	args := []any{orgID}
	if rawProjectID := c.Query("project_id"); rawProjectID != "" {
		projectID, err := uuid.Parse(rawProjectID)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid project_id")
		}
		query += " AND project_id=$2"
		args = append(args, projectID)
	}
	query += " ORDER BY updated_at ASC"

	rows, err := h.db.QueryContext(c.Context(), query, args...)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	items := make([]fiber.Map, 0)
	for rows.Next() {
		var issueID uuid.UUID
		var title, status string
		var updatedAt time.Time
		if err := rows.Scan(&issueID, &title, &status, &updatedAt); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		items = append(items, fiber.Map{
			"id":         issueID,
			"title":      title,
			"status":     status,
			"updated_at": updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, items)
}

func (h *Handler) bottlenecks(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}

	query := `SELECT status, AVG(EXTRACT(EPOCH FROM (updated_at - created_at))/86400.0) AS avg_days
FROM issues
WHERE org_id=$1 AND deleted_at IS NULL
GROUP BY status
ORDER BY avg_days DESC
LIMIT 1`
	args := []any{orgID}
	if rawProjectID := c.Query("project_id"); rawProjectID != "" {
		projectID, err := uuid.Parse(rawProjectID)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid project_id")
		}
		query = `SELECT status, AVG(EXTRACT(EPOCH FROM (updated_at - created_at))/86400.0) AS avg_days
FROM issues
WHERE org_id=$1 AND project_id=$2 AND deleted_at IS NULL
GROUP BY status
ORDER BY avg_days DESC
LIMIT 1`
		args = append(args, projectID)
	}

	var status string
	var avgDays float64
	if err := h.db.QueryRowContext(c.Context(), query, args...).Scan(&status, &avgDays); err != nil {
		if err == sql.ErrNoRows {
			return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"status": "", "avg_days": 0})
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"status": status, "avg_days": avgDays})
}

func (h *Handler) velocity(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}

	query := `SELECT COUNT(*)::int
FROM issues
WHERE org_id=$1 AND deleted_at IS NULL AND status='done' AND sprint_id IS NOT NULL
GROUP BY sprint_id
ORDER BY MAX(updated_at) DESC
LIMIT 6`
	args := []any{orgID}
	if rawProjectID := c.Query("project_id"); rawProjectID != "" {
		projectID, err := uuid.Parse(rawProjectID)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid project_id")
		}
		query = `SELECT COUNT(*)::int
FROM issues
WHERE org_id=$1 AND project_id=$2 AND deleted_at IS NULL AND status='done' AND sprint_id IS NOT NULL
GROUP BY sprint_id
ORDER BY MAX(updated_at) DESC
LIMIT 6`
		args = append(args, projectID)
	}

	rows, err := h.db.QueryContext(c.Context(), query, args...)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	trend := make([]int, 0, 6)
	for rows.Next() {
		var count int
		if err := rows.Scan(&count); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		trend = append(trend, count)
	}
	if err := rows.Err(); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	slices.Reverse(trend)
	current := 0
	if len(trend) > 0 {
		current = trend[len(trend)-1]
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"current": current, "trend": trend})
}

func (h *Handler) cycleTime(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}

	if rawIssueID := c.Query("issue_id"); rawIssueID != "" {
		issueID, err := uuid.Parse(rawIssueID)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid issue_id")
		}
		var avgDays float64
		err = h.db.QueryRowContext(c.Context(), `SELECT EXTRACT(EPOCH FROM (updated_at - created_at))/86400.0
FROM issues
WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL AND status='done'`, orgID, issueID).Scan(&avgDays)
		if err != nil {
			if err == sql.ErrNoRows {
				return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"issue_id": issueID, "avg_days": 0})
			}
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"issue_id": issueID, "avg_days": avgDays})
	}

	var avgDays float64
	if err := h.db.QueryRowContext(c.Context(), `SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at))/86400.0), 0)
FROM issues
WHERE org_id=$1 AND deleted_at IS NULL AND status='done'`, orgID).Scan(&avgDays); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"avg_days": avgDays})
}
