package users

import (
	"github.com/atharshah1/scrum/scrumX/backend/internal/common"
	"github.com/gofiber/fiber/v2"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(api fiber.Router) {
	r := api.Group("/users")
	r.Get("/", common.NotImplemented)
	r.Patch("/:id", common.NotImplemented)
	r.Post("/:id/role", common.NotImplemented)
}
