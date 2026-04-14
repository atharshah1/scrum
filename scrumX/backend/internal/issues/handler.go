package issues

import (
"github.com/atharshah1/scrum/scrumX/backend/internal/common"
"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
"github.com/gofiber/fiber/v2"
"github.com/google/uuid"
)

type Handler struct {
service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(api fiber.Router) {
issues := api.Group("/issues")
issues.Post("/", h.create)
issues.Get("/", h.list)
issues.Get("/:id", h.get)
issues.Patch("/:id", h.update)
issues.Delete("/:id", h.delete)

issues.Post("/:id/comment", common.NotImplemented)
issues.Post("/:id/assign", common.NotImplemented)
issues.Post("/:id/labels", common.NotImplemented)
issues.Post("/:id/link", common.NotImplemented)
issues.Get("/:id/dependencies", common.NotImplemented)
}

func (h *Handler) create(c *fiber.Ctx) error {
orgID, ok := middleware.MustOrgID(c)
if !ok {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing org context"})
}
actorID, ok := middleware.MustUserID(c)
if !ok {
actorID = uuid.Nil
}

var input CreateIssueInput
if err := c.BodyParser(&input); err != nil {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
}
if input.ProjectID == uuid.Nil || input.Title == "" {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "project_id and title are required"})
}

issue, err := h.service.Create(c.Context(), orgID, actorID, input)
if err != nil {
return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}
return c.Status(fiber.StatusCreated).JSON(issue)
}

func (h *Handler) list(c *fiber.Ctx) error {
orgID, ok := middleware.MustOrgID(c)
if !ok {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing org context"})
}
items, err := h.service.List(c.Context(), orgID)
if err != nil {
return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}
return c.JSON(items)
}

func (h *Handler) get(c *fiber.Ctx) error {
orgID, ok := middleware.MustOrgID(c)
if !ok {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing org context"})
}
issueID, err := uuid.Parse(c.Params("id"))
if err != nil {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
}
issue, err := h.service.Get(c.Context(), orgID, issueID)
if err != nil {
return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
}
return c.JSON(issue)
}

func (h *Handler) update(c *fiber.Ctx) error {
orgID, ok := middleware.MustOrgID(c)
if !ok {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing org context"})
}
actorID, ok := middleware.MustUserID(c)
if !ok {
actorID = uuid.Nil
}
issueID, err := uuid.Parse(c.Params("id"))
if err != nil {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
}
var input UpdateIssueInput
if err := c.BodyParser(&input); err != nil {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
}
issue, err := h.service.Update(c.Context(), orgID, actorID, issueID, input)
if err != nil {
return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}
return c.JSON(issue)
}

func (h *Handler) delete(c *fiber.Ctx) error {
orgID, ok := middleware.MustOrgID(c)
if !ok {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing org context"})
}
actorID, ok := middleware.MustUserID(c)
if !ok {
actorID = uuid.Nil
}
issueID, err := uuid.Parse(c.Params("id"))
if err != nil {
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
}
if err := h.service.Delete(c.Context(), orgID, actorID, issueID); err != nil {
return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}
return c.SendStatus(fiber.StatusNoContent)
}
