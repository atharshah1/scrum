package itsm

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/authz"
	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	db    *sql.DB
	authz *authz.Service
	bus   events.Publisher
}

func NewHandler(db *sql.DB, authzService *authz.Service, bus events.Publisher) *Handler {
	return &Handler{db: db, authz: authzService, bus: bus}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	incidents := api.Group("/incidents")
	incidents.Post("/", h.createIncident)
	incidents.Get("/", h.listIncidents)
	incidents.Patch("/:id", h.patchIncident)
	api.Post("/alerts", middleware.RateLimitMiddleware(30, time.Minute, nil), h.createAlert)
}

func (h *Handler) createIncident(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	role, err := h.authz.RequireOrgMember(c.Context(), orgID, userID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var payload struct {
		Title    string `json:"title"`
		Severity string `json:"severity"`
		Status   string `json:"status"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	payload.Title = strings.TrimSpace(payload.Title)
	payload.Severity = strings.ToLower(strings.TrimSpace(payload.Severity))
	payload.Status = strings.ToLower(strings.TrimSpace(payload.Status))
	if payload.Title == "" || payload.Severity == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "title and severity are required")
	}
	if payload.Status == "" {
		payload.Status = "open"
	}
	incidentID := uuid.New()
	if _, err := h.db.ExecContext(c.Context(), `INSERT INTO incidents (id, org_id, title, severity, status) VALUES ($1,$2,$3,$4,$5)`,
		incidentID, orgID, payload.Title, payload.Severity, payload.Status); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	_ = h.bus.Publish(c.Context(), events.New(orgID, "incident.created", userID, map[string]any{
		"incident_id": incidentID,
		"title":       payload.Title,
		"severity":    payload.Severity,
		"status":      payload.Status,
	}))
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{
		"id":       incidentID,
		"title":    payload.Title,
		"severity": payload.Severity,
		"status":   payload.Status,
	})
}

func (h *Handler) listIncidents(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	if _, err := h.authz.RequireOrgMember(c.Context(), orgID, userID); err != nil {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	statusFilter := strings.ToLower(strings.TrimSpace(c.Query("status")))
	query := `SELECT id, title, severity, status, created_at, updated_at FROM incidents WHERE org_id=$1`
	args := []any{orgID}
	if statusFilter != "" {
		query += ` AND LOWER(status)=$2`
		args = append(args, statusFilter)
	}
	query += ` ORDER BY updated_at DESC, created_at DESC`
	rows, err := h.db.QueryContext(c.Context(), query, args...)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	result := make([]fiber.Map, 0, 16)
	for rows.Next() {
		var (
			id        uuid.UUID
			title     string
			severity  string
			status    string
			createdAt time.Time
			updatedAt time.Time
		)
		if err := rows.Scan(&id, &title, &severity, &status, &createdAt, &updatedAt); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		result = append(result, fiber.Map{
			"id":         id,
			"title":      title,
			"severity":   severity,
			"status":     status,
			"created_at": createdAt,
			"updated_at": updatedAt,
		})
	}
	return utils.JSONSuccess(c, fiber.StatusOK, result)
}

func (h *Handler) patchIncident(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	role, err := h.authz.RequireOrgMember(c.Context(), orgID, userID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	incidentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid incident id")
	}
	var payload struct {
		Title    *string `json:"title"`
		Severity *string `json:"severity"`
		Status   *string `json:"status"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	if payload.Title == nil && payload.Severity == nil && payload.Status == nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "at least one field is required")
	}
	if payload.Title != nil {
		v := strings.TrimSpace(*payload.Title)
		payload.Title = &v
	}
	if payload.Severity != nil {
		v := strings.ToLower(strings.TrimSpace(*payload.Severity))
		payload.Severity = &v
	}
	if payload.Status != nil {
		v := strings.ToLower(strings.TrimSpace(*payload.Status))
		payload.Status = &v
	}
	_, err = h.db.ExecContext(c.Context(), `UPDATE incidents
SET title=COALESCE($3, title),
    severity=COALESCE($4, severity),
    status=COALESCE($5, status),
    updated_at=NOW()
WHERE org_id=$1 AND id=$2`, orgID, incidentID, payload.Title, payload.Severity, payload.Status)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"id": incidentID, "updated": true})
}

func (h *Handler) createAlert(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	role, err := h.authz.RequireOrgMember(c.Context(), orgID, userID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var payload struct {
		Source     string         `json:"source"`
		Payload    map[string]any `json:"payload"`
		IncidentID *uuid.UUID     `json:"incident_id"`
		Title      string         `json:"title"`
		Severity   string         `json:"severity"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	payload.Source = strings.TrimSpace(payload.Source)
	if payload.Source == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "source is required")
	}
	if payload.Payload == nil {
		payload.Payload = map[string]any{}
	}
	tx, err := h.db.BeginTx(c.Context(), nil)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer tx.Rollback()
	incidentID := payload.IncidentID
	if incidentID == nil {
		title := strings.TrimSpace(payload.Title)
		if title == "" {
			title = "Alert from " + payload.Source
		}
		severity := strings.ToLower(strings.TrimSpace(payload.Severity))
		if severity == "" {
			severity = "high"
		}
		newIncidentID := uuid.New()
		if _, err := tx.ExecContext(c.Context(), `INSERT INTO incidents (id, org_id, title, severity, status) VALUES ($1,$2,$3,$4,'open')`,
			newIncidentID, orgID, title, severity); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		incidentID = &newIncidentID
		_ = h.bus.Publish(c.Context(), events.New(orgID, "incident.created", userID, map[string]any{
			"incident_id": newIncidentID,
			"title":       title,
			"severity":    severity,
			"status":      "open",
		}))
	}
	payloadRaw, err := json.Marshal(payload.Payload)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid alert payload")
	}
	alertID := uuid.New()
	if _, err := tx.ExecContext(c.Context(), `INSERT INTO alerts (id, org_id, incident_id, source, payload) VALUES ($1,$2,$3,$4,$5)`,
		alertID, orgID, incidentID, payload.Source, payloadRaw); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	if err := tx.Commit(); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	_ = h.bus.Publish(c.Context(), events.New(orgID, "alert.created", userID, map[string]any{
		"alert_id":     alertID,
		"incident_id":  incidentID,
		"source":       payload.Source,
		"autocreated":  payload.IncidentID == nil,
		"payload_size": len(payloadRaw),
	}))
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{
		"id":          alertID,
		"incident_id": incidentID,
		"source":      payload.Source,
	})
}
