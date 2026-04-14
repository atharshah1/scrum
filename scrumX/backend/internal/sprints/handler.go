package sprints

import (
"github.com/atharshah1/scrum/scrumX/backend/internal/common"
"github.com/gofiber/fiber/v2"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(api fiber.Router) {
r := api.Group("/sprints")
r.Post("/", common.NotImplemented)
r.Post("/:id/start", common.NotImplemented)
r.Post("/:id/end", common.NotImplemented)
}
