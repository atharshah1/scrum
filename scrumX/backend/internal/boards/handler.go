package boards

import (
"github.com/atharshah1/scrum/scrumX/backend/internal/common"
"github.com/gofiber/fiber/v2"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(api fiber.Router) {
api.Get("/boards/:id", common.NotImplemented)
}
