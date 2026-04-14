package notifications

import (
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Handler exposes notification endpoints.
type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	n := api.Group("/notifications")
	n.Get("/", h.list)
	n.Patch("/:id/read", h.markRead)
	n.Post("/read-all", h.markAllRead)
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	items, total, err := h.repo.List(c.Context(), orgID, userID, page, limit)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    items,
		"meta":    fiber.Map{"page": page, "limit": limit, "total": total},
	})
}

func (h *Handler) markRead(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	notifID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid notification id")
	}
	if err := h.repo.MarkRead(c.Context(), orgID, userID, notifID); err != nil {
		return utils.JSONError(c, fiber.StatusNotFound, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"read": true})
}

func (h *Handler) markAllRead(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	if err := h.repo.MarkAllRead(c.Context(), orgID, userID); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"read": true})
}
