package issues

import (
	"fmt"
	"net/url"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/pkg/cache"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
	cache   *cache.TTLCache
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service, cache: cache.NewTTLCache(10 * time.Second)}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	issues := api.Group("/issues")
	issues.Post("/", h.create)
	issues.Get("/", h.list)
	issues.Get("/:id", h.get)
	issues.Patch("/:id", h.update)
	issues.Delete("/:id", h.delete)

	issues.Post("/:id/comments", h.createComment)
	issues.Get("/:id/comments", h.listComments)
	issues.Get("/:id/activity", h.listActivity)

	issues.Post("/:id/watchers", h.addWatcher)
	issues.Delete("/:id/watchers/:userId", h.removeWatcher)
	issues.Get("/:id/watchers", h.listWatchers)

	issues.Post("/:id/labels", h.addLabel)
	issues.Delete("/:id/labels/:label", h.removeLabel)
	issues.Get("/:id/labels", h.listLabels)

	issues.Post("/:id/link", h.addRelation)
	issues.Get("/:id/dependencies", h.listRelations)
}

func (h *Handler) create(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}

	var input CreateIssueInput
	if err := c.BodyParser(&input); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if input.ProjectID == uuid.Nil || input.Title == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "project_id and title are required")
	}

	issue, err := h.service.Create(c.Context(), orgID, actorID, input)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, issue)
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	filter := ListIssuesFilter{
		Status:    c.Query("status"),
		Label:     c.Query("label"),
		IssueType: c.Query("issue_type"),
		SortBy:    c.Query("sort_by"),
		Order:     c.Query("order"),
		Page:      c.QueryInt("page", 1),
		Limit:     c.QueryInt("limit", 20),
	}
	if assignee := c.Query("assignee_id"); assignee != "" {
		id, err := uuid.Parse(assignee)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid assignee_id")
		}
		filter.AssigneeID = id
	}
	if sprint := c.Query("sprint_id"); sprint != "" {
		id, err := uuid.Parse(sprint)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid sprint_id")
		}
		filter.SprintID = id
	}
	if parent := c.Query("parent_id"); parent != "" {
		id, err := uuid.Parse(parent)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid parent_id")
		}
		filter.ParentID = id
	}
	if project := c.Query("project_id"); project != "" {
		id, err := uuid.Parse(project)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid project_id")
		}
		filter.ProjectID = id
	}
	cacheKey := fmt.Sprintf("issues:%s:p=%s:s=%s:a=%s:sp=%s:l=%s:t=%s:pa=%s:sb=%s:o=%s:pg=%d:li=%d",
		orgID,
		filter.ProjectID, filter.Status, filter.AssigneeID, filter.SprintID, filter.Label, filter.IssueType, filter.ParentID,
		filter.SortBy, filter.Order, filter.Page, filter.Limit,
	)
	if cached, ok := h.cache.Get(cacheKey); ok {
		return utils.JSONSuccess(c, fiber.StatusOK, cached)
	}
	items, total, err := h.service.List(c.Context(), orgID, filter)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	payload := fiber.Map{
		"success": true,
		"data":    items,
		"meta":    fiber.Map{"page": filter.Page, "limit": filter.Limit, "total": total},
	}
	h.cache.Set(cacheKey, payload)
	return c.Status(fiber.StatusOK).JSON(payload)
}

func (h *Handler) get(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	issue, err := h.service.Get(c.Context(), orgID, issueID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusNotFound, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, issue)
}

func (h *Handler) update(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	var input UpdateIssueInput
	if err := c.BodyParser(&input); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	issue, err := h.service.Update(c.Context(), orgID, actorID, issueID, input)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, issue)
}

func (h *Handler) delete(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		actorID = uuid.Nil
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	if err := h.service.Delete(c.Context(), orgID, actorID, issueID); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) addRelation(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	var payload struct {
		RelatedIssueID uuid.UUID `json:"related_issue_id"`
		RelationType   string    `json:"relation_type"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.RelatedIssueID == uuid.Nil || payload.RelationType == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if err := h.service.AddRelation(c.Context(), orgID, actorID, issueID, payload.RelatedIssueID, payload.RelationType); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{"message": "relation added"})
}

func (h *Handler) listRelations(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	items, err := h.service.ListRelations(c.Context(), orgID, issueID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, items)
}

func (h *Handler) addWatcher(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	var payload struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.UserID == uuid.Nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if err := h.service.AddWatcher(c.Context(), orgID, actorID, issueID, payload.UserID); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{"message": "watcher added"})
}

func (h *Handler) removeWatcher(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	userID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid user id")
	}
	if err := h.service.RemoveWatcher(c.Context(), orgID, actorID, issueID, userID); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) listWatchers(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	items, err := h.service.ListWatchers(c.Context(), orgID, issueID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, items)
}

func (h *Handler) addLabel(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	var payload struct {
		Label string `json:"label"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if err := h.service.AddLabel(c.Context(), orgID, actorID, issueID, payload.Label); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{"message": "label added"})
}

func (h *Handler) removeLabel(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	label, decodeErr := url.QueryUnescape(c.Params("label"))
	if decodeErr != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid label encoding")
	}
	if err := h.service.RemoveLabel(c.Context(), orgID, actorID, issueID, label); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) listLabels(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	items, err := h.service.ListLabels(c.Context(), orgID, issueID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, items)
}

func (h *Handler) createComment(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, _ := middleware.MustUserID(c)
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	var payload struct {
		Body string `json:"body"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	comment, err := h.service.CreateComment(c.Context(), orgID, actorID, issueID, payload.Body)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, comment)
}

func (h *Handler) listComments(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	comments, err := h.service.ListComments(c.Context(), orgID, issueID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, comments)
}

func (h *Handler) listActivity(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	activity, err := h.service.ListActivities(c.Context(), orgID, issueID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, activity)
}
