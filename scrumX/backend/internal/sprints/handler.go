package sprints

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/authz"
	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/cache"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	db    *sql.DB
	bus   events.Publisher
	authz *authz.Service
	cache *cache.TTLCache
}

func NewHandler(db *sql.DB, bus events.Publisher, authzService *authz.Service, sharedCache *cache.TTLCache) *Handler {
	return &Handler{db: db, bus: bus, authz: authzService, cache: sharedCache}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	r := api.Group("/sprints")
	r.Post("/", h.create)
	r.Post("/:id/start", h.start)
	r.Post("/:id/end", h.end)
	r.Post("/:id/issues", h.addIssues)
}

func (h *Handler) create(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	var payload struct {
		BoardID uuid.UUID  `json:"board_id"`
		Name    string     `json:"name"`
		StartAt *time.Time `json:"start_at"`
		EndAt   *time.Time `json:"end_at"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.BoardID == uuid.Nil || payload.Name == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	var projectID uuid.UUID
	if err := h.db.QueryRowContext(c.Context(), `SELECT project_id FROM boards WHERE id=$1 AND org_id=$2`, payload.BoardID, orgID).Scan(&projectID); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "board not found")
	}
	role, err := h.authz.ResolveProjectRole(c.Context(), orgID, actorID, projectID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	id := uuid.New()
	_, err = h.db.ExecContext(c.Context(), `INSERT INTO sprints (id, org_id, board_id, name, status, start_at, end_at) VALUES ($1,$2,$3,$4,'planned',$5,$6)`,
		id, orgID, payload.BoardID, payload.Name, nullableTime(payload.StartAt), nullableTime(payload.EndAt))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	sprint := fiber.Map{"id": id, "org_id": orgID, "board_id": payload.BoardID, "name": payload.Name, "status": "planned", "start_at": payload.StartAt, "end_at": payload.EndAt}
	_ = h.bus.Publish(c.Context(), events.New(orgID, "sprint.created", actorID, map[string]any{"sprint": sprint}))
	h.invalidateProjectCaches(orgID, projectID)
	return utils.JSONSuccess(c, fiber.StatusCreated, sprint)
}

func (h *Handler) start(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	sprintID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid sprint id")
	}
	projectID, err := h.projectIDBySprint(c.Context(), orgID, sprintID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "sprint not found")
	}
	role, err := h.authz.ResolveProjectRole(c.Context(), orgID, actorID, projectID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	result, err := h.db.ExecContext(c.Context(), `UPDATE sprints
SET status='active', start_at=COALESCE(start_at, NOW())
WHERE id=$1 AND org_id=$2 AND status IN ('planned')
AND NOT EXISTS (
	SELECT 1 FROM sprints active
	WHERE active.org_id=$2
	  AND active.board_id = (SELECT board_id FROM sprints target WHERE target.id=$1 AND target.org_id=$2)
	  AND active.status='active'
	  AND active.id <> $1
)`, sprintID, orgID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return utils.JSONError(c, fiber.StatusBadRequest, "sprint not found or cannot be started")
	}
	_ = h.bus.Publish(c.Context(), events.New(orgID, "sprint.started", actorID, map[string]any{"sprint_id": sprintID}))
	h.invalidateProjectCaches(orgID, projectID)
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"id": sprintID, "status": "active"})
}

func (h *Handler) end(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	sprintID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid sprint id")
	}
	projectID, err := h.projectIDBySprint(c.Context(), orgID, sprintID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "active sprint not found")
	}
	role, err := h.authz.ResolveProjectRole(c.Context(), orgID, actorID, projectID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var boardID uuid.UUID
	if err := h.db.QueryRowContext(c.Context(), `SELECT board_id FROM sprints WHERE id=$1 AND org_id=$2 AND status='active'`, sprintID, orgID).Scan(&boardID); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "active sprint not found")
	}
	tx, err := h.db.BeginTx(c.Context(), nil)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(c.Context(), `UPDATE sprints SET status='completed', end_at=COALESCE(end_at, NOW()) WHERE id=$1 AND org_id=$2`, sprintID, orgID); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	nextSprintID := uuid.Nil
	err = tx.QueryRowContext(c.Context(), `SELECT id FROM sprints WHERE org_id=$1 AND board_id=$2 AND status='planned' ORDER BY COALESCE(start_at, end_at, NOW()) ASC, id ASC LIMIT 1`, orgID, boardID).Scan(&nextSprintID)
	if err == nil {
		// Carry forward all incomplete issues to the next planned sprint.
		if _, err := tx.ExecContext(c.Context(), `UPDATE issues SET sprint_id=$1, updated_at=NOW() WHERE org_id=$2 AND sprint_id=$3 AND deleted_at IS NULL AND status NOT IN ('done','closed','resolved')`, nextSprintID, orgID, sprintID); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
	} else if err != sql.ErrNoRows {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	} else {
		// If no next sprint exists, incomplete issues move back to backlog (NULL sprint_id).
		if _, err := tx.ExecContext(c.Context(), `UPDATE issues SET sprint_id=NULL, updated_at=NOW() WHERE org_id=$1 AND sprint_id=$2 AND deleted_at IS NULL AND status NOT IN ('done','closed','resolved')`, orgID, sprintID); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
	}
	if err := tx.Commit(); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	_ = h.bus.Publish(c.Context(), events.New(orgID, "sprint.completed", actorID, map[string]any{"sprint_id": sprintID, "next_sprint_id": nextSprintID}))
	h.invalidateProjectCaches(orgID, projectID)
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"id": sprintID, "status": "completed", "next_sprint_id": nextSprintID})
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

func (h *Handler) addIssues(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	sprintID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid sprint id")
	}
	projectID, err := h.projectIDBySprint(c.Context(), orgID, sprintID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "sprint not found")
	}
	role, err := h.authz.ResolveProjectRole(c.Context(), orgID, actorID, projectID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var payload struct {
		IssueIDs []uuid.UUID `json:"issue_ids"`
	}
	if err := c.BodyParser(&payload); err != nil || len(payload.IssueIDs) == 0 {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	placeholders := make([]string, 0, len(payload.IssueIDs))
	args := []any{sprintID, orgID, projectID}
	for _, issueID := range payload.IssueIDs {
		args = append(args, issueID)
		placeholders = append(placeholders, "$"+itoa(len(args)))
	}
	query := `UPDATE issues SET sprint_id=$1, updated_at=NOW()
WHERE org_id=$2 AND project_id=$3 AND deleted_at IS NULL AND id IN (` + strings.Join(placeholders, ",") + `)`
	if _, err := h.db.ExecContext(c.Context(), query, args...); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	for _, issueID := range payload.IssueIDs {
		_ = h.bus.Publish(c.Context(), events.New(orgID, "issue.moved_to_sprint", actorID, map[string]any{"issue_id": issueID, "sprint_id": sprintID}))
	}
	h.invalidateProjectCaches(orgID, projectID)
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"sprint_id": sprintID, "updated_issues": len(payload.IssueIDs)})
}

func (h *Handler) projectIDBySprint(ctx context.Context, orgID, sprintID uuid.UUID) (uuid.UUID, error) {
	var projectID uuid.UUID
	err := h.db.QueryRowContext(ctx, `SELECT b.project_id FROM sprints s JOIN boards b ON b.id=s.board_id WHERE s.id=$1 AND s.org_id=$2 AND b.org_id=$2`, sprintID, orgID).Scan(&projectID)
	return projectID, err
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func (h *Handler) invalidateProjectCaches(orgID, projectID uuid.UUID) {
	if h.cache == nil {
		return
	}
	h.cache.DeletePrefix("board:" + orgID.String() + ":" + projectID.String() + ":")
	h.cache.DeletePrefix("issues:" + orgID.String() + ":p=" + projectID.String() + ":")
	h.cache.DeletePrefix("issues:" + orgID.String() + ":p=" + uuid.Nil.String() + ":")
}
