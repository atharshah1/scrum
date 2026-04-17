package webhooks

import (
	"strings"
	"time"

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
	routes.Post("/incoming", middleware.RateLimitMiddleware(60, time.Minute, nil), h.incoming)
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
	hook, err := h.dispatcher.Save(c.Context(), Webhook{OrgID: orgID, URL: payload.URL})
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, hook)
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	hooks, err := h.dispatcher.List(c.Context(), orgID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, hooks)
}

func (h *Handler) incoming(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	var payload struct {
		OrgID     *string        `json:"org_id,omitempty"`
		ProjectID *string        `json:"project_id,omitempty"`
		Type      string         `json:"type"`
		Data      map[string]any `json:"data"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if payload.OrgID != nil && strings.TrimSpace(*payload.OrgID) != "" {
		requestOrgID, err := uuid.Parse(strings.TrimSpace(*payload.OrgID))
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid org_id")
		}
		if requestOrgID != orgID {
			return utils.JSONError(c, fiber.StatusForbidden, "org_id does not match tenant context")
		}
	}
	payload.Type = strings.TrimSpace(payload.Type)
	if payload.Type == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "type is required")
	}
	options := make([]events.Option, 0, 1)
	if payload.ProjectID != nil && strings.TrimSpace(*payload.ProjectID) != "" {
		projectID, err := uuid.Parse(strings.TrimSpace(*payload.ProjectID))
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid project_id")
		}
		options = append(options, events.WithProjectID(projectID))
	}
	event := events.New(orgID, payload.Type, actorID, payload.Data, options...)
	if err := h.bus.Publish(c.Context(), event); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusAccepted, fiber.Map{"message": "accepted"})
}
