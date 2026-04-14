package webhooks

import (
	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing org context"})
	}
	var payload struct {
		URL string `json:"url"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}
	hook := h.dispatcher.Save(Webhook{OrgID: orgID, URL: payload.URL})
	return c.Status(fiber.StatusCreated).JSON(hook)
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing org context"})
	}
	return c.JSON(h.dispatcher.List(orgID))
}

func (h *Handler) incoming(c *fiber.Ctx) error {
	var payload struct {
		OrgID string         `json:"org_id"`
		Type  string         `json:"type"`
		Data  map[string]any `json:"data"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}
	orgID, err := uuid.Parse(payload.OrgID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid org_id"})
	}
	event := events.New(orgID, payload.Type, uuid.Nil, payload.Data)
	if err := h.bus.Publish(c.Context(), event); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusAccepted)
}
