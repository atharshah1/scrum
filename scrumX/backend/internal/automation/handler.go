package automation

import (
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler { return &Handler{store: store} }

func (h *Handler) RegisterRoutes(api fiber.Router) {
	routes := api.Group("/automation")
	routes.Post("/rules", h.createRule)
	routes.Get("/rules", h.listRules)
	routes.Delete("/rules/:id", h.deleteRule)
}

func (h *Handler) createRule(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	if middleware.Role(c) == "Viewer" {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var rule Rule
	if err := c.BodyParser(&rule); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	rule.OrgID = orgID
	rule, err := h.store.Save(c.Context(), rule)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, rule)
}

func (h *Handler) listRules(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	rules, err := h.store.List(c.Context(), orgID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, rules)
}

func (h *Handler) deleteRule(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	if middleware.Role(c) != "Admin" {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	ruleID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid rule id")
	}
	if err := h.store.Delete(c.Context(), orgID, ruleID); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}
