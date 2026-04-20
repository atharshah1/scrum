package issues

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/pkg/cache"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/validation"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

var (
	allowedIssueSortBy = map[string]struct{}{
		"created_at": {},
		"updated_at": {},
		"priority":   {},
		"status":     {},
		"title":      {},
		"relevance":  {},
	}
	allowedOrder = map[string]struct{}{
		"asc":  {},
		"desc": {},
	}
)

type Handler struct {
	service *Service
	cache   *cache.TTLCache
}

func NewHandler(service *Service, sharedCache *cache.TTLCache) *Handler {
	return &Handler{service: service, cache: sharedCache}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	issues := api.Group("/issues")
	read := issues.Group("", middleware.RequireScopes("read:issues"))
	write := issues.Group("", middleware.RequireScopes("write:issues"))
	write.Post("/", h.create)
	read.Get("/", h.list)
	read.Get("/search/suggestions", h.searchSuggestions)
	read.Get("/search", h.search)
	read.Get("/queries/saved", h.listSavedQueries)
	write.Post("/queries/saved", h.saveQuery)
	write.Delete("/queries/saved/:id", h.deleteSavedQuery)
	read.Get("/queries/recent", h.listRecentQueries)
	read.Get("/:id", h.get)
	write.Patch("/:id", h.update)
	write.Delete("/:id", h.delete)

	write.Post("/:id/comments", h.createComment)
	read.Get("/:id/comments", h.listComments)
	read.Get("/:id/activity", h.listActivity)

	write.Post("/:id/watchers", h.addWatcher)
	write.Delete("/:id/watchers/:userId", h.removeWatcher)
	read.Get("/:id/watchers", h.listWatchers)

	write.Post("/:id/labels", h.addLabel)
	write.Delete("/:id/labels/:label", h.removeLabel)
	read.Get("/:id/labels", h.listLabels)

	write.Post("/:id/link", h.addRelation)
	read.Get("/:id/dependencies", h.listRelations)
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
	if input.ProjectID == uuid.Nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "project_id and title are required")
	}
	title, err := validation.NormalizeRequiredString("title", input.Title, 200)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	description, err := validation.NormalizeOptionalString("description", input.Description, 10000)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	input.Title = title
	input.Description = description

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
	sortBy, err := validation.NormalizeOptionalEnum("sort_by", c.Query("sort_by"), allowedIssueSortBy)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	order, err := validation.NormalizeOptionalEnum("order", c.Query("order"), allowedOrder)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	page, limit, err := validation.NormalizePagination(c.QueryInt("page", 1), c.QueryInt("limit", 20), 20, 100)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	status, err := validation.NormalizeOptionalString("status", c.Query("status"), 64)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	label, err := validation.NormalizeOptionalString("label", c.Query("label"), 64)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	issueType, err := validation.NormalizeOptionalString("issue_type", c.Query("issue_type"), 64)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	searchQuery, err := validation.NormalizeOptionalString("q", c.Query("q"), 200)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	filter := ListIssuesFilter{
		Status:      strings.ToLower(status),
		Label:       strings.ToLower(label),
		IssueType:   strings.ToLower(issueType),
		SearchQuery: searchQuery,
		SortBy:      sortBy,
		Order:       order,
		Page:        page,
		Limit:       limit,
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
	if updatedSince := strings.TrimSpace(c.Query("updated_since")); updatedSince != "" {
		parsed, parseErr := time.Parse(time.RFC3339, updatedSince)
		if parseErr != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid updated_since (expected RFC3339)")
		}
		utc := parsed.UTC()
		filter.UpdatedSince = &utc
	}
	updatedSinceKey := ""
	if filter.UpdatedSince != nil {
		updatedSinceKey = filter.UpdatedSince.UTC().Format(time.RFC3339Nano)
	}
	cacheKey := fmt.Sprintf("issues:%s:p=%s:s=%s:a=%s:sp=%s:l=%s:t=%s:pa=%s:us=%s:q=%s:sb=%s:o=%s:pg=%d:li=%d",
		orgID,
		filter.ProjectID, filter.Status, filter.AssigneeID, filter.SprintID, filter.Label, filter.IssueType, filter.ParentID, updatedSinceKey,
		url.QueryEscape(filter.SearchQuery),
		filter.SortBy, filter.Order, filter.Page, filter.Limit,
	)
	payload, err := h.cache.GetOrLoad(cacheKey, func() (any, error) {
		items, total, listErr := h.service.List(c.Context(), orgID, filter)
		if listErr != nil {
			return nil, listErr
		}
		return fiber.Map{
			"success": true,
			"data":    items,
			"meta":    fiber.Map{"page": filter.Page, "limit": filter.Limit, "total": total},
		}, nil
	})
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(payload)
}

func (h *Handler) search(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	page, limit, err := validation.NormalizePagination(c.QueryInt("page", 1), c.QueryInt("limit", 20), 20, 100)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	query, err := validation.NormalizeOptionalString("q", c.Query("q"), 500)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	if strings.TrimSpace(query) == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "q is required")
	}
	items, total, ast, err := h.service.Search(c.Context(), orgID, actorID, query, page, limit)
	if err != nil {
		var validationErr *IssueSearchValidationError
		if errors.As(err, &validationErr) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"message": validationErr.Message,
					"code":    validationErr.Code,
					"field":   validationErr.Field,
					"token":   validationErr.Token,
				},
			})
		}
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    items,
		"meta": fiber.Map{
			"page":  page,
			"limit": limit,
			"total": total,
			"ast":   ast,
		},
	})
}

func (h *Handler) searchSuggestions(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	suggestions, err := h.service.SearchSuggestions(c.Context(), orgID, actorID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, suggestions)
}

func (h *Handler) saveQuery(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	var payload struct {
		Name  string `json:"name"`
		Query string `json:"query"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	name, err := validation.NormalizeRequiredString("name", payload.Name, 80)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	query, err := validation.NormalizeRequiredString("query", payload.Query, maxIssueSearchLength)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	if _, _, parseErr := parseIssueSearchQuery(query); parseErr != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, parseErr.Error())
	}
	saved, err := h.service.SaveQuery(c.Context(), orgID, actorID, name, query)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, saved)
}

func (h *Handler) listSavedQueries(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	items, err := h.service.ListSavedQueries(c.Context(), orgID, actorID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, items)
}

func (h *Handler) deleteSavedQuery(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	savedID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid id")
	}
	if err := h.service.DeleteSavedQuery(c.Context(), orgID, actorID, savedID); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) listRecentQueries(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	limit := c.QueryInt("limit", 20)
	items, err := h.service.ListRecentQueries(c.Context(), orgID, actorID, limit)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, items)
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
	if input.UpdatedAt == nil || input.UpdatedAt.IsZero() {
		return utils.JSONError(c, fiber.StatusBadRequest, "updated_at is required for optimistic concurrency control; provide the current updated_at value from the issue being updated")
	}
	if input.Title != nil {
		title, err := validation.NormalizeRequiredString("title", *input.Title, 200)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		input.Title = &title
	}
	if input.Description != nil {
		description, err := validation.NormalizeOptionalString("description", *input.Description, 10000)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		input.Description = &description
	}
	if input.Status != nil {
		status, err := validation.NormalizeRequiredString("status", *input.Status, 64)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		status = strings.ToLower(status)
		input.Status = &status
	}
	if input.Priority != nil {
		priority, err := validation.NormalizeRequiredString("priority", *input.Priority, 32)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		priority = strings.ToLower(priority)
		input.Priority = &priority
	}
	if input.IssueType != nil {
		issueType, err := validation.NormalizeRequiredString("issue_type", *input.IssueType, 32)
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		issueType = strings.ToLower(issueType)
		input.IssueType = &issueType
	}
	issue, err := h.service.Update(c.Context(), orgID, actorID, issueID, input)
	if err != nil {
		if errors.Is(err, ErrOptimisticLockConflict) {
			return utils.JSONError(c, fiber.StatusConflict, err.Error())
		}
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
	relationType, err := validation.NormalizeRequiredString("relation_type", payload.RelationType, 64)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	payload.RelationType = strings.ToLower(relationType)
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
	label, err := validation.NormalizeRequiredString("label", payload.Label, 64)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	payload.Label = label
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
	page, limit, err := validation.NormalizePagination(c.QueryInt("page", 1), c.QueryInt("limit", 50), 50, 200)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	paged, total := paginateStrings(items, page, limit)
	return utils.JSONList(c, paged, page, limit, total)
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
	body, err := validation.NormalizeRequiredString("body", payload.Body, 10000)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	payload.Body = body
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
	page, limit, err := validation.NormalizePagination(c.QueryInt("page", 1), c.QueryInt("limit", 50), 50, 200)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	comments, total, err := h.service.ListComments(c.Context(), orgID, issueID, page, limit)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONList(c, comments, page, limit, total)
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
	page, limit, err := validation.NormalizePagination(c.QueryInt("page", 1), c.QueryInt("limit", 50), 50, 200)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	activity, total, err := h.service.ListActivities(c.Context(), orgID, issueID, page, limit)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONList(c, activity, page, limit, total)
}

func paginateStrings(items []string, page, limit int) ([]string, int) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	total := len(items)
	start := (page - 1) * limit
	if start >= total {
		return []string{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return items[start:end], total
}
