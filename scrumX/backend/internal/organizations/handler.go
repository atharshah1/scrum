package organizations

import (
	"github.com/atharshah1/scrum/scrumX/backend/internal/common"
	"github.com/gofiber/fiber/v2"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(api fiber.Router) {
	r := api.Group("/orgs")
	r.Post("/", common.NotImplemented)
	r.Get("/", common.NotImplemented)
	r.Post("/:id/invite", common.NotImplemented)
}
