package workflows

import (
	"database/sql"
	"encoding/json"
	"fmt"

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
	userID, _ := middleware.MustUserID(c)
	if _, err := h.authz.ResolveProjectRole(c.Context(), orgID, userID, projectID); err != nil {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	rows, err := h.db.QueryContext(c.Context(), `SELECT id, from_status, to_status, conditions, validators, post_functions FROM workflow_transitions WHERE org_id=$1 AND project_id=$2 ORDER BY from_status, to_status`, orgID, projectID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	result := []fiber.Map{}
	for rows.Next() {
		var id uuid.UUID
		var fromStatus, toStatus string
		var conditionsRaw, validatorsRaw, postRaw []byte
		if err := rows.Scan(&id, &fromStatus, &toStatus, &conditionsRaw, &validatorsRaw, &postRaw); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		conditions, err := decodeJSON("conditions", conditionsRaw)
		if err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, fmt.Sprintf("decode workflow transition %s: %v", id, err))
		}
		validators, err := decodeJSON("validators", validatorsRaw)
		if err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, fmt.Sprintf("decode workflow transition %s: %v", id, err))
		}
		postFunctions, err := decodeJSON("post_functions", postRaw)
		if err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, fmt.Sprintf("decode workflow transition %s: %v", id, err))
		}
		result = append(result, fiber.Map{
			"id":             id,
			"from_status":    fromStatus,
			"to_status":      toStatus,
			"conditions":     conditions,
			"validators":     validators,
			"post_functions": postFunctions,
		})
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
	userID, _ := middleware.MustUserID(c)
	role, err := h.authz.ResolveProjectRole(c.Context(), orgID, userID, projectID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var payload struct {
		FromStatus   string         `json:"from_status"`
		ToStatus     string         `json:"to_status"`
		Conditions   map[string]any `json:"conditions"`
		Validators   map[string]any `json:"validators"`
		PostFunction map[string]any `json:"post_functions"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.FromStatus == "" || payload.ToStatus == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	conditionsRaw, _ := json.Marshal(payload.Conditions)
	validatorsRaw, _ := json.Marshal(payload.Validators)
	postRaw, _ := json.Marshal(payload.PostFunction)
	transitionID := uuid.New()
	if _, err := h.db.ExecContext(c.Context(), `INSERT INTO workflow_transitions (id, org_id, project_id, from_status, to_status, conditions, validators, post_functions) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		transitionID, orgID, projectID, payload.FromStatus, payload.ToStatus, conditionsRaw, validatorsRaw, postRaw); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{
		"id":             transitionID,
		"project_id":     projectID,
		"from_status":    payload.FromStatus,
		"to_status":      payload.ToStatus,
		"conditions":     payload.Conditions,
		"validators":     payload.Validators,
		"post_functions": payload.PostFunction,
	})
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
	userID, _ := middleware.MustUserID(c)
	role, err := h.authz.ResolveProjectRole(c.Context(), orgID, userID, projectID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
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

func decodeJSON(field string, raw []byte) (map[string]any, error) {
	out := map[string]any{}
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return out, nil
}
