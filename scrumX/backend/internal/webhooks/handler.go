package webhooks

import (
	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	dispatcher *Dispatcher
	bus        *events.Bus
}

func NewHandler(dispatcher *Dispatcher, bus *events.Bus) *Handler {
	return &Handler{dispatcher: dispatcher, bus: bus}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	routes := api.Group("/webhooks")
	routes.Post("/", h.create)
	routes.Get("/", h.list)
	routes.Post("/incoming", h.incoming)
}

func (h *Handler) create(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	var payload struct {
		URL string `json:"url"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.URL == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	hook := h.dispatcher.Save(Webhook{OrgID: orgID, URL: payload.URL})
	return utils.JSONSuccess(c, fiber.StatusCreated, hook)
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	return utils.JSONSuccess(c, fiber.StatusOK, h.dispatcher.List(orgID))
}

func (h *Handler) incoming(c *fiber.Ctx) error {
	var payload struct {
		OrgID string         `json:"org_id"`
		Type  string         `json:"type"`
		Data  map[string]any `json:"data"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	orgID, err := uuid.Parse(payload.OrgID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid org_id")
	}
	event := events.New(orgID, payload.Type, uuid.Nil, payload.Data)
	if err := h.bus.Publish(c.Context(), event); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusAccepted, fiber.Map{"message": "accepted"})
}
