package workflows

import (
	"database/sql"

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
	r := api.Group("/workflows")
	r.Get("/:projectId/transitions", h.listTransitions)
	r.Post("/:projectId/transitions", h.createTransition)
	r.Delete("/:projectId/transitions/:transitionId", h.deleteTransition)
}

func (h *Handler) listTransitions(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	projectID, err := uuid.Parse(c.Params("projectId"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid project id")
	}
	rows, err := h.db.QueryContext(c.Context(), `SELECT id, from_status, to_status FROM workflow_transitions WHERE org_id=$1 AND project_id=$2 ORDER BY from_status, to_status`, orgID, projectID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	result := []fiber.Map{}
	for rows.Next() {
		var id uuid.UUID
		var fromStatus, toStatus string
		if err := rows.Scan(&id, &fromStatus, &toStatus); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		result = append(result, fiber.Map{"id": id, "from_status": fromStatus, "to_status": toStatus})
	}
	return utils.JSONSuccess(c, fiber.StatusOK, result)
}

func (h *Handler) createTransition(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	projectID, err := uuid.Parse(c.Params("projectId"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid project id")
	}
	var payload struct {
		FromStatus string `json:"from_status"`
		ToStatus   string `json:"to_status"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.FromStatus == "" || payload.ToStatus == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	transitionID := uuid.New()
	if _, err := h.db.ExecContext(c.Context(), `INSERT INTO workflow_transitions (id, org_id, project_id, from_status, to_status) VALUES ($1,$2,$3,$4,$5)`,
		transitionID, orgID, projectID, payload.FromStatus, payload.ToStatus); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{"id": transitionID, "project_id": projectID, "from_status": payload.FromStatus, "to_status": payload.ToStatus})
}

func (h *Handler) deleteTransition(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	projectID, err := uuid.Parse(c.Params("projectId"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid project id")
	}
	transitionID, err := uuid.Parse(c.Params("transitionId"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid transition id")
	}
	if _, err := h.db.ExecContext(c.Context(), `DELETE FROM workflow_transitions WHERE id=$1 AND org_id=$2 AND project_id=$3`, transitionID, orgID, projectID); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}
