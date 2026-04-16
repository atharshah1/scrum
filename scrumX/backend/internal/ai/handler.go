package ai

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/authz"
	"github.com/atharshah1/scrum/scrumX/backend/internal/issues"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	service             *Service
	issueService        *issues.Service
	fromTextEnabled     bool
	fromTextLimitPerMin int
	fromTextDailyQuota  int
	fromTextMaxChars    int
	mu                  sync.Mutex
	perMinute           map[string]int
	perMinuteWindow     time.Time
	perDay              map[string]int
	perDayWindow        string
}

func NewHandler(service *Service, issueService *issues.Service, fromTextEnabled bool, limitPerMin int, dailyQuota int, maxChars int) *Handler {
	return &Handler{
		service:             service,
		issueService:        issueService,
		fromTextEnabled:     fromTextEnabled,
		fromTextLimitPerMin: limitPerMin,
		fromTextDailyQuota:  dailyQuota,
		fromTextMaxChars:    maxChars,
		perMinute:           map[string]int{},
		perMinuteWindow:     time.Now().UTC().Truncate(time.Minute),
		perDay:              map[string]int{},
		perDayWindow:        time.Now().UTC().Format("2006-01-02"),
	}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	group := api.Group("/ai")
	issuesGroup := group.Group("/issues")
	issuesGroup.Post("/from-text", h.generateFromText)
	issuesGroup.Post("/:id/summarize", h.summarizeIssue)
	issuesGroup.Post("/suggest", h.suggestFields)
}

func (h *Handler) generateFromText(c *fiber.Ctx) error {
	if !h.fromTextEnabled {
		return utils.JSONError(c, fiber.StatusServiceUnavailable, "ai issue generation is disabled")
	}
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	role := middleware.Role(c)
	if role != authz.RoleAdmin && role != authz.RoleMember {
		return utils.JSONError(c, fiber.StatusForbidden, "insufficient role for ai issue generation")
	}
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
	if strings.TrimSpace(payload.Text) == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "text is required")
	}
	if h.fromTextMaxChars > 0 && len(payload.Text) > h.fromTextMaxChars {
		return utils.JSONError(c, fiber.StatusBadRequest, fmt.Sprintf("text exceeds maximum length of %d characters", h.fromTextMaxChars))
	}
	if err := h.checkAndConsumeAIBudget(orgID, actorID); err != nil {
		return utils.JSONError(c, fiber.StatusTooManyRequests, err.Error())
	}
	drafts, err := h.service.GenerateIssuesFromText(c.Context(), payload.Text)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, drafts)
}

func (h *Handler) checkAndConsumeAIBudget(orgID, actorID uuid.UUID) error {
	if actorID == uuid.Nil || orgID == uuid.Nil {
		return fmt.Errorf("missing actor or org context")
	}
	now := time.Now().UTC()
	minuteWindow := now.Truncate(time.Minute)
	dayWindow := now.Format("2006-01-02")
	key := orgID.String() + ":" + actorID.String()

	h.mu.Lock()
	defer h.mu.Unlock()

	if minuteWindow.After(h.perMinuteWindow) {
		h.perMinuteWindow = minuteWindow
		h.perMinute = map[string]int{}
	}
	if dayWindow != h.perDayWindow {
		h.perDayWindow = dayWindow
		h.perDay = map[string]int{}
	}

	if h.fromTextLimitPerMin > 0 && h.perMinute[key] >= h.fromTextLimitPerMin {
		return fmt.Errorf("ai request rate limit exceeded")
	}
	if h.fromTextDailyQuota > 0 && h.perDay[key] >= h.fromTextDailyQuota {
		return fmt.Errorf("ai daily quota exceeded")
	}

	h.perMinute[key]++
	h.perDay[key]++
	return nil
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
