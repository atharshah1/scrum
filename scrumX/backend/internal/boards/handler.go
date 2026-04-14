package boards

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/authz"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/cache"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	db    *sql.DB
	authz *authz.Service
	cache *cache.TTLCache
}

var defaultBoardColumns = []boardColumn{
	{Name: "To Do", Statuses: []string{"todo"}, Position: 1},
	{Name: "In Progress", Statuses: []string{"in_progress"}, Position: 2},
	{Name: "Done", Statuses: []string{"done"}, Position: 3},
}

func NewHandler(db *sql.DB, authzService *authz.Service) *Handler {
	return &Handler{db: db, authz: authzService, cache: cache.NewTTLCache(15 * time.Second)}
}

type boardColumn struct {
	ID       uuid.UUID   `json:"id"`
	Name     string      `json:"name"`
	Statuses []string    `json:"statuses"`
	Position int         `json:"position"`
	Issues   []fiber.Map `json:"issues"`
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	api.Get("/boards/:id", h.getBoard)
	api.Post("/boards/:id/columns", h.upsertColumns)
}

func (h *Handler) getBoard(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	boardID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid board id")
	}
	var projectID uuid.UUID
	if err := h.db.QueryRowContext(c.Context(), `SELECT project_id FROM boards WHERE id=$1 AND org_id=$2`, boardID, orgID).Scan(&projectID); err != nil {
		return utils.JSONError(c, fiber.StatusNotFound, "board not found")
	}
	userID, _ := middleware.MustUserID(c)
	role, err := h.authz.ResolveProjectRole(c.Context(), orgID, userID, projectID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	if role == "" {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}

	columns := []boardColumn{}
	rows, err := h.db.QueryContext(c.Context(), `SELECT id, name, statuses, position FROM board_columns WHERE org_id=$1 AND board_id=$2 ORDER BY position ASC`, orgID, boardID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var col boardColumn
		var statusesRaw []byte
		if err := rows.Scan(&col.ID, &col.Name, &statusesRaw, &col.Position); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		_ = json.Unmarshal(statusesRaw, &col.Statuses)
		columns = append(columns, col)
	}
	if len(columns) == 0 {
		columns = append(columns, defaultBoardColumns...)
	}

	page := c.QueryInt("page", 1)
	if page <= 0 {
		page = 1
	}
	limit := c.QueryInt("limit", 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit
	var sprintID *uuid.UUID
	if sprintQ := strings.TrimSpace(c.Query("sprint_id")); sprintQ != "" {
		id, parseErr := uuid.Parse(sprintQ)
		if parseErr != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid sprint_id")
		}
		sprintID = &id
	}
	normalizedSprint := "none"
	if sprintID != nil {
		normalizedSprint = sprintID.String()
	}
	cacheKey := fmt.Sprintf("board:%s:%s:%d:%d:%s", orgID, boardID, page, limit, normalizedSprint)
	if cached, ok := h.cache.Get(cacheKey); ok {
		return utils.JSONSuccess(c, fiber.StatusOK, cached)
	}
	for i := range columns {
		issues, fetchErr := h.loadColumnIssues(c, orgID, projectID, columns[i].Statuses, sprintID, limit, offset)
		if fetchErr != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, fetchErr.Error())
		}
		columns[i].Issues = issues
	}
	resp := fiber.Map{
		"board_id":   boardID,
		"project_id": projectID,
		"columns":    columns,
		"meta":       fiber.Map{"page": page, "limit": limit},
	}
	h.cache.Set(cacheKey, resp)
	return utils.JSONSuccess(c, fiber.StatusOK, resp)
}

func (h *Handler) upsertColumns(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	boardID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid board id")
	}
	var payload struct {
		Columns []struct {
			Name     string   `json:"name"`
			Statuses []string `json:"statuses"`
			Position int      `json:"position"`
		} `json:"columns"`
	}
	if err := c.BodyParser(&payload); err != nil || len(payload.Columns) == 0 {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	tx, err := h.db.BeginTx(c.Context(), nil)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(c.Context(), `DELETE FROM board_columns WHERE org_id=$1 AND board_id=$2`, orgID, boardID); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	for _, col := range payload.Columns {
		normalizedStatuses := make([]string, 0, len(col.Statuses))
		for _, status := range col.Statuses {
			status = strings.ToLower(strings.TrimSpace(status))
			if status != "" {
				normalizedStatuses = append(normalizedStatuses, status)
			}
		}
		raw, _ := json.Marshal(normalizedStatuses)
		if _, err := tx.ExecContext(c.Context(), `INSERT INTO board_columns (id, org_id, board_id, name, statuses, position) VALUES ($1,$2,$3,$4,$5,$6)`,
			uuid.New(), orgID, boardID, col.Name, raw, col.Position); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
	}
	if err := tx.Commit(); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"message": "columns configured"})
}

func (h *Handler) loadColumnIssues(c *fiber.Ctx, orgID, projectID uuid.UUID, statuses []string, sprintID *uuid.UUID, limit, offset int) ([]fiber.Map, error) {
	statuses = normalizeStatuses(statuses)
	if len(statuses) == 0 {
		return []fiber.Map{}, nil
	}
	args := []any{orgID, projectID}
	statusPlaceholders := make([]string, 0, len(statuses))
	for _, status := range statuses {
		args = append(args, status)
		statusPlaceholders = append(statusPlaceholders, "$"+strconv.Itoa(len(args)))
	}
	query := `SELECT id, title, status, priority, assignee_id, sprint_id
FROM issues
WHERE org_id=$1 AND project_id=$2 AND deleted_at IS NULL AND status IN (` + strings.Join(statusPlaceholders, ",") + `)`
	if sprintID != nil {
		args = append(args, *sprintID)
		query += ` AND sprint_id=$` + strconv.Itoa(len(args))
	}
	args = append(args, limit, offset)
	query += ` ORDER BY updated_at DESC, id DESC LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := h.db.QueryContext(c.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]fiber.Map, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		var title, status, priority string
		var assigneeID, sprID *uuid.UUID
		if err := rows.Scan(&id, &title, &status, &priority, &assigneeID, &sprID); err != nil {
			return nil, err
		}
		out = append(out, fiber.Map{
			"id":          id,
			"title":       title,
			"status":      status,
			"priority":    priority,
			"assignee_id": assigneeID,
			"sprint_id":   sprID,
		})
	}
	return out, rows.Err()
}

func normalizeStatuses(statuses []string) []string {
	normalized := make([]string, 0, len(statuses))
	seen := map[string]struct{}{}
	for _, status := range statuses {
		status = strings.ToLower(strings.TrimSpace(status))
		if status == "" {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		normalized = append(normalized, status)
	}
	return normalized
}
