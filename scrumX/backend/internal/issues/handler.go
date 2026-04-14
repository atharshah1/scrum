package issues

import (
	"strconv"
	"strings"

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

	issues.Post("/:id/comments", h.addComment)
	issues.Patch("/:id/comments/:commentId", h.updateComment)
	issues.Delete("/:id/comments/:commentId", h.deleteComment)
	issues.Post("/:id/assign", h.assign)
	issues.Post("/:id/labels", h.addLabel)
	issues.Delete("/:id/labels/:label", h.removeLabel)
	issues.Post("/:id/link", h.link)
	issues.Get("/:id/dependencies", h.dependencies)
}

func (h *Handler) create(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}

	var input CreateIssueInput
	if err := c.BodyParser(&input); err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid payload")
	}
	if input.ProjectID == uuid.Nil || strings.TrimSpace(input.Title) == "" {
		return errorResponse(c, fiber.StatusBadRequest, "project_id and title are required")
	}

	issue, err := h.service.Create(c.Context(), orgID, actorID, input)
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": issue})
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	input, err := parseListIssuesInput(c)
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	items, err := h.service.List(c.Context(), orgID, input)
	if err != nil {
		return errorResponse(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"data": items, "meta": fiber.Map{"limit": input.Limit, "offset": input.Offset}})
}

func (h *Handler) get(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	issue, err := h.service.Get(c.Context(), orgID, issueID)
	if err != nil {
		return errorResponse(c, fiber.StatusNotFound, err.Error())
	}
	return c.JSON(fiber.Map{"data": issue})
}

func (h *Handler) update(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	var input UpdateIssueInput
	if err := c.BodyParser(&input); err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid payload")
	}
	issue, err := h.service.Update(c.Context(), orgID, actorID, issueID, input)
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	return c.JSON(fiber.Map{"data": issue})
}

func (h *Handler) delete(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	if err := h.service.Delete(c.Context(), orgID, actorID, issueID); err != nil {
		return errorResponse(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) assign(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	var input AssignIssueInput
	if err := c.BodyParser(&input); err != nil || input.AssigneeID == uuid.Nil {
		return errorResponse(c, fiber.StatusBadRequest, "assignee_id is required")
	}
	issue, err := h.service.Assign(c.Context(), orgID, actorID, issueID, input.AssigneeID)
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	return c.JSON(fiber.Map{"data": issue})
}

func (h *Handler) addComment(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	var input CreateCommentInput
	if err := c.BodyParser(&input); err != nil || strings.TrimSpace(input.Body) == "" {
		return errorResponse(c, fiber.StatusBadRequest, "comment body is required")
	}
	comment, err := h.service.AddComment(c.Context(), orgID, actorID, issueID, input.Body)
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": comment})
}

func (h *Handler) updateComment(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	commentID, err := uuid.Parse(c.Params("commentId"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid comment id")
	}
	var input UpdateCommentInput
	if err := c.BodyParser(&input); err != nil || strings.TrimSpace(input.Body) == "" {
		return errorResponse(c, fiber.StatusBadRequest, "comment body is required")
	}
	comment, err := h.service.UpdateComment(c.Context(), orgID, actorID, issueID, commentID, input.Body)
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	return c.JSON(fiber.Map{"data": comment})
}

func (h *Handler) deleteComment(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	commentID, err := uuid.Parse(c.Params("commentId"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid comment id")
	}
	if err := h.service.DeleteComment(c.Context(), orgID, actorID, issueID, commentID); err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) addLabel(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	var input LabelInput
	if err := c.BodyParser(&input); err != nil || strings.TrimSpace(input.Label) == "" {
		return errorResponse(c, fiber.StatusBadRequest, "label is required")
	}
	if err := h.service.AddLabel(c.Context(), orgID, actorID, issueID, input.Label); err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) removeLabel(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	label := c.Params("label")
	if strings.TrimSpace(label) == "" {
		return errorResponse(c, fiber.StatusBadRequest, "label is required")
	}
	if err := h.service.RemoveLabel(c.Context(), orgID, actorID, issueID, label); err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) link(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	var input CreateIssueLinkInput
	if err := c.BodyParser(&input); err != nil || input.RelatedIssueID == uuid.Nil {
		return errorResponse(c, fiber.StatusBadRequest, "related_issue_id is required")
	}
	link, err := h.service.LinkIssue(c.Context(), orgID, actorID, issueID, input)
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": link})
}

func (h *Handler) dependencies(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return errorResponse(c, fiber.StatusBadRequest, "missing org context")
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	links, err := h.service.ListDependencies(c.Context(), orgID, issueID)
	if err != nil {
		return errorResponse(c, fiber.StatusBadRequest, err.Error())
	}
	return c.JSON(fiber.Map{"data": links})
}

func parseListIssuesInput(c *fiber.Ctx) (ListIssuesInput, error) {
	input := ListIssuesInput{
		Status:    strings.TrimSpace(c.Query("status")),
		SortBy:    strings.TrimSpace(c.Query("sort_by")),
		SortOrder: strings.TrimSpace(c.Query("sort_order")),
		Labels:    parseLabelsFromCSV(c.Query("labels")),
		Limit:     50,
		Offset:    0,
	}
	if project := strings.TrimSpace(c.Query("project_id")); project != "" {
		projectID, err := uuid.Parse(project)
		if err != nil {
			return ListIssuesInput{}, fiber.NewError(fiber.StatusBadRequest, "invalid project_id")
		}
		input.ProjectID = &projectID
	}
	if assignee := strings.TrimSpace(c.Query("assignee")); assignee != "" {
		assigneeID, err := uuid.Parse(assignee)
		if err != nil {
			return ListIssuesInput{}, fiber.NewError(fiber.StatusBadRequest, "invalid assignee")
		}
		input.Assignee = &assigneeID
	}
	if sprint := strings.TrimSpace(c.Query("sprint_id")); sprint != "" {
		sprintID, err := uuid.Parse(sprint)
		if err != nil {
			return ListIssuesInput{}, fiber.NewError(fiber.StatusBadRequest, "invalid sprint_id")
		}
		input.SprintID = &sprintID
	}
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		limit, err := strconv.Atoi(v)
		if err != nil {
			return ListIssuesInput{}, fiber.NewError(fiber.StatusBadRequest, "invalid limit")
		}
		input.Limit = limit
	}
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		offset, err := strconv.Atoi(v)
		if err != nil {
			return ListIssuesInput{}, fiber.NewError(fiber.StatusBadRequest, "invalid offset")
		}
		input.Offset = offset
	}
	return input, nil
}

func parseLabelsFromCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, strings.ToLower(trimmed))
		}
	}
	return out
}

func errorResponse(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}
