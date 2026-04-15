package ai

import (
	"github.com/atharshah1/scrum/scrumX/backend/internal/issues"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	service      *Service
	issueService *issues.Service
}

func NewHandler(service *Service, issueService *issues.Service) *Handler {
	return &Handler{service: service, issueService: issueService}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	group := api.Group("/ai")
	issuesGroup := group.Group("/issues")
	issuesGroup.Post("/from-text", h.generateFromText)
	issuesGroup.Post("/:id/summarize", h.summarizeIssue)
	issuesGroup.Post("/suggest", h.suggestFields)
}

func (h *Handler) generateFromText(c *fiber.Ctx) error {
	var payload struct {
		Text      string    `json:"text"`
		ProjectID uuid.UUID `json:"project_id"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if payload.ProjectID == uuid.Nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "project_id is required")
	}
	drafts, err := h.service.GenerateIssuesFromText(c.Context(), payload.Text)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, drafts)
}

func (h *Handler) summarizeIssue(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	issue, err := h.issueService.Get(c.Context(), orgID, issueID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusNotFound, err.Error())
	}
	comments, _, err := h.issueService.ListComments(c.Context(), orgID, issueID, 1, 100)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	summary, err := h.service.SummarizeIssue(c.Context(), issue, comments)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"summary": summary})
}

func (h *Handler) suggestFields(c *fiber.Ctx) error {
	var payload struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	suggestion, err := h.service.SuggestFields(c.Context(), payload.Title, payload.Description)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, suggestion)
}
