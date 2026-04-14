package timetracking

import (
	"github.com/atharshah1/scrum/scrumX/backend/internal/common"
	"github.com/gofiber/fiber/v2"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(api fiber.Router) {
	r := api.Group("/time")
	r.Post("/start", common.NotImplemented)
	r.Post("/stop", common.NotImplemented)
	r.Get("/report", common.NotImplemented)
}
