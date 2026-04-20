package automation

import (
	"context"

	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	store    *Store
	replayer deadLetterReplayer
}

type deadLetterReplayer interface {
	ReplayDeadLetter(ctx context.Context, dl DeadLetter) error
}

func NewHandler(store *Store, replayer deadLetterReplayer) *Handler {
	return &Handler{store: store, replayer: replayer}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	routes := api.Group("/automation")
	scoped := routes.Group("", middleware.RequireScopes("automation:execute"))
	scoped.Post("/rules", h.createRule)
	scoped.Get("/rules", h.listRules)
	scoped.Delete("/rules/:id", h.deleteRule)
	scoped.Get("/dead-letters", h.listDeadLetters)
	scoped.Get("/dead-letters/:id", h.getDeadLetter)
	scoped.Post("/dead-letters/:id/replay", h.replayDeadLetter)
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

func (h *Handler) listDeadLetters(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	if middleware.Role(c) != "Admin" {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	items, total, err := h.store.ListDeadLetters(c.Context(), orgID, page, limit)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONList(c, items, page, limit, total)
}

func (h *Handler) getDeadLetter(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	if middleware.Role(c) != "Admin" {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	deadLetterID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid dead letter id")
	}
	item, err := h.store.GetDeadLetter(c.Context(), orgID, deadLetterID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusNotFound, "dead letter not found")
	}
	return utils.JSONSuccess(c, fiber.StatusOK, item)
}

func (h *Handler) replayDeadLetter(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	if middleware.Role(c) != "Admin" {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	deadLetterID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid dead letter id")
	}
	item, err := h.store.GetDeadLetter(c.Context(), orgID, deadLetterID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusNotFound, "dead letter not found")
	}
	if h.replayer == nil {
		return utils.JSONError(c, fiber.StatusServiceUnavailable, "dead letter replay service is not configured")
	}
	replayErr := h.replayer.ReplayDeadLetter(c.Context(), item)
	_ = h.store.RecordDeadLetterReplay(c.Context(), orgID, deadLetterID, actorID, replayErr)
	if replayErr != nil {
		return utils.JSONError(c, fiber.StatusBadGateway, replayErr.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"replayed": true, "id": deadLetterID})
}
