package itsm

import (
"github.com/atharshah1/scrum/scrumX/backend/internal/common"
"github.com/gofiber/fiber/v2"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(api fiber.Router) {
incidents := api.Group("/incidents")
incidents.Post("/", common.NotImplemented)
incidents.Get("/", common.NotImplemented)
incidents.Patch("/:id", common.NotImplemented)
api.Post("/alerts", common.NotImplemented)
}
