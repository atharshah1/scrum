package webhooks

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	db         *sql.DB
	dispatcher *Dispatcher
	bus        *events.Bus
}

func NewHandler(db *sql.DB, dispatcher *Dispatcher, bus *events.Bus) *Handler {
	return &Handler{db: db, dispatcher: dispatcher, bus: bus}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	routes := api.Group("/webhooks")
	routes.Post("/", h.create)
	routes.Get("/", h.list)
	routes.Post("/incoming", middleware.RateLimitMiddleware(60, time.Minute, nil), h.incoming)
}

func (h *Handler) create(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	var payload struct {
		URL string `json:"url"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.URL == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	hook, err := h.dispatcher.Save(c.Context(), Webhook{OrgID: orgID, URL: payload.URL})
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, hook)
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	hooks, err := h.dispatcher.List(c.Context(), orgID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, hooks)
}

func (h *Handler) incoming(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	actorID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	var payload struct {
		OrgID     *string        `json:"org_id,omitempty"`
		ProjectID *string        `json:"project_id,omitempty"`
		Type      string         `json:"type"`
		Data      map[string]any `json:"data"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if payload.OrgID != nil && strings.TrimSpace(*payload.OrgID) != "" {
		requestOrgID, err := uuid.Parse(strings.TrimSpace(*payload.OrgID))
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid org_id")
		}
		if requestOrgID != orgID {
			return utils.JSONError(c, fiber.StatusForbidden, "org_id does not match tenant context")
		}
	}
	payload.Type = strings.TrimSpace(payload.Type)
	if payload.Type == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "type is required")
	}
	options := make([]events.Option, 0, 1)
	if payload.ProjectID != nil && strings.TrimSpace(*payload.ProjectID) != "" {
		projectID, err := uuid.Parse(strings.TrimSpace(*payload.ProjectID))
		if err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "invalid project_id")
		}
		options = append(options, events.WithProjectID(projectID))
	}
	event := events.New(orgID, payload.Type, actorID, payload.Data, options...)
	if err := h.validateEventReferences(c.Context(), orgID, event); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	if err := h.bus.Publish(c.Context(), event); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusAccepted, fiber.Map{"message": "accepted"})
}

func (h *Handler) validateEventReferences(ctx context.Context, orgID uuid.UUID, event events.Event) error {
	if h.db == nil {
		return errors.New("database connection required for event validation")
	}
	projectID := uuid.Nil
	hasProject := event.Scope.ProjectID != nil && *event.Scope.ProjectID != uuid.Nil
	if hasProject {
		projectID = *event.Scope.ProjectID
	}
	switch strings.ToLower(strings.TrimSpace(event.Scope.ResourceType)) {
	case "issue":
		return h.validateIssueProject(ctx, orgID, event.Scope.ResourceID, projectID, hasProject)
	case "release":
		return h.validateReleaseOrg(ctx, orgID, event.Scope.ResourceID, projectID, hasProject)
	case "deployment":
		payloadReleaseID, hasPayloadReleaseID := parseUUID(event.Payload["release_id"])
		return h.validateDeploymentRelease(ctx, orgID, event.Scope.ResourceID, payloadReleaseID, projectID, hasPayloadReleaseID, hasProject)
	default:
		return nil
	}
}

func (h *Handler) validateIssueProject(ctx context.Context, orgID, issueID, projectID uuid.UUID, hasProject bool) error {
	if issueID == uuid.Nil {
		return errors.New("scope.resource_id is required for issue events")
	}
	var storedProjectID uuid.UUID
	if err := h.db.QueryRowContext(ctx, `SELECT project_id FROM issues WHERE org_id=$1 AND id=$2`, orgID, issueID).Scan(&storedProjectID); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("issue_id does not belong to org")
		}
		return err
	}
	if hasProject && storedProjectID != projectID {
		return errors.New("scope.project_id does not match issue project")
	}
	return nil
}

func (h *Handler) validateReleaseOrg(ctx context.Context, orgID, releaseID, projectID uuid.UUID, hasProject bool) error {
	if releaseID == uuid.Nil {
		return errors.New("scope.resource_id is required for release events")
	}
	var storedProjectID uuid.UUID
	if err := h.db.QueryRowContext(ctx, `SELECT project_id FROM releases WHERE org_id=$1 AND id=$2`, orgID, releaseID).Scan(&storedProjectID); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("release_id does not belong to org")
		}
		return err
	}
	if hasProject && storedProjectID != projectID {
		return errors.New("scope.project_id does not match release project")
	}
	return nil
}

func (h *Handler) validateDeploymentRelease(ctx context.Context, orgID, deploymentID, payloadReleaseID, projectID uuid.UUID, hasPayloadReleaseID, hasProject bool) error {
	if deploymentID == uuid.Nil {
		return errors.New("scope.resource_id is required for deployment events")
	}
	var releaseID sql.NullString
	if err := h.db.QueryRowContext(ctx, `SELECT release_id FROM deployments WHERE org_id=$1 AND id=$2`, orgID, deploymentID).Scan(&releaseID); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("deployment_id does not belong to org")
		}
		return err
	}
	if hasPayloadReleaseID {
		if !releaseID.Valid {
			return errors.New("payload.release_id must match deployment release")
		}
		storedReleaseID, err := uuid.Parse(releaseID.String)
		if err != nil {
			return err
		}
		if storedReleaseID != payloadReleaseID {
			return errors.New("payload.release_id must match deployment release")
		}
	}
	if !releaseID.Valid {
		if hasProject {
			return errors.New("deployment has no release to validate scope.project_id")
		}
		return nil
	}
	storedReleaseID, err := uuid.Parse(releaseID.String)
	if err != nil {
		return err
	}
	return h.validateReleaseOrg(ctx, orgID, storedReleaseID, projectID, hasProject)
}

func parseUUID(value any) (uuid.UUID, bool) {
	raw, ok := value.(string)
	if !ok {
		return uuid.Nil, false
	}
	parsed, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, false
	}
	return parsed, true
}
