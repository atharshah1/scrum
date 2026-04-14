package integrations

import (
	"github.com/atharshah1/scrum/scrumX/backend/internal/common"
	"github.com/gofiber/fiber/v2"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(api fiber.Router) {
	r := api.Group("/integrations")
	r.Post("/github", common.NotImplemented)
	r.Post("/jira", common.NotImplemented)
}
