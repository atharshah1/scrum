package boards

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	db *sql.DB
}

var defaultBoardColumns = []boardColumn{
	{Name: "To Do", Statuses: []string{"todo"}, Position: 1},
	{Name: "In Progress", Statuses: []string{"in_progress"}, Position: 2},
	{Name: "Done", Statuses: []string{"done"}, Position: 3},
}

func NewHandler(db *sql.DB) *Handler { return &Handler{db: db} }

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

	issueRows, err := h.db.QueryContext(c.Context(), `SELECT id, title, status, priority, assignee_id, sprint_id FROM issues WHERE org_id=$1 AND project_id=$2 ORDER BY updated_at DESC`, orgID, projectID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer issueRows.Close()
	issuesByStatus := map[string][]fiber.Map{}
	for issueRows.Next() {
		var id uuid.UUID
		var title, status, priority string
		var assigneeID, sprintID *uuid.UUID
		if err := issueRows.Scan(&id, &title, &status, &priority, &assigneeID, &sprintID); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		issuesByStatus[status] = append(issuesByStatus[status], fiber.Map{
			"id":          id,
			"title":       title,
			"status":      status,
			"priority":    priority,
			"assignee_id": assigneeID,
			"sprint_id":   sprintID,
		})
	}
	for i := range columns {
		for _, st := range columns[i].Statuses {
			if statusIssues, exists := issuesByStatus[strings.TrimSpace(st)]; exists {
				columns[i].Issues = append(columns[i].Issues, statusIssues...)
			}
		}
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{
		"board_id":   boardID,
		"project_id": projectID,
		"columns":    columns,
	})
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
