package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Scope struct {
	ProjectID    *uuid.UUID `json:"project_id,omitempty"`
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
}

type Event struct {
	ID        uuid.UUID      `json:"id"`
	OrgID     uuid.UUID      `json:"org_id"`
	Type      string         `json:"type"`
	ActorID   uuid.UUID      `json:"actor_id"`
	Scope     Scope          `json:"scope"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type Option func(*Event)

func WithProjectID(projectID uuid.UUID) Option {
	return func(event *Event) {
		if projectID == uuid.Nil {
			event.Scope.ProjectID = nil
			return
		}
		id := projectID
		event.Scope.ProjectID = &id
	}
}

func WithResource(resourceType string, resourceID uuid.UUID) Option {
	return func(event *Event) {
		resourceType = strings.TrimSpace(strings.ToLower(resourceType))
		if resourceType == "" || resourceID == uuid.Nil {
			return
		}
		event.Scope.ResourceType = resourceType
		event.Scope.ResourceID = resourceID
	}
}

func New(orgID uuid.UUID, eventType string, actorID uuid.UUID, payload map[string]any, options ...Option) Event {
	if payload == nil {
		payload = map[string]any{}
	}
	eventID := uuid.New()
	event := Event{
		ID:      eventID,
		OrgID:   orgID,
		Type:    eventType,
		ActorID: actorID,
		Payload: payload,
		Scope: Scope{
			ResourceType: "event",
			ResourceID:   eventID,
		},
		CreatedAt: time.Now().UTC(),
	}
	normalizeScopeFromPayload(&event)
	for _, option := range options {
		if option != nil {
			option(&event)
		}
	}
	return event
}

func Validate(event Event) error {
	if event.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if event.Type == "" {
		return errors.New("type is required")
	}
	if strings.TrimSpace(event.Scope.ResourceType) == "" {
		return errors.New("scope.resource_type is required")
	}
	if event.Scope.ResourceID == uuid.Nil {
		return errors.New("scope.resource_id is required")
	}
	return nil
}

func normalizeScopeFromPayload(event *Event) {
	if event == nil {
		return
	}
	if event.Scope.ProjectID == nil {
		if projectID, ok := parseUUID(event.Payload["project_id"]); ok {
			event.Scope.ProjectID = &projectID
		}
	}
	issueMap := asMap(event.Payload["issue"])
	if event.Scope.ProjectID == nil {
		if projectID, ok := parseUUID(issueMap["project_id"]); ok {
			event.Scope.ProjectID = &projectID
		}
	}
	if id, ok := parseUUID(event.Payload["issue_id"]); ok {
		event.Scope.ResourceType = "issue"
		event.Scope.ResourceID = id
		return
	}
	if id, ok := parseUUID(event.Payload["sprint_id"]); ok {
		event.Scope.ResourceType = "sprint"
		event.Scope.ResourceID = id
		return
	}
	if id, ok := parseUUID(event.Payload["release_id"]); ok {
		event.Scope.ResourceType = "release"
		event.Scope.ResourceID = id
		return
	}
	if id, ok := parseUUID(event.Payload["deployment_id"]); ok {
		event.Scope.ResourceType = "deployment"
		event.Scope.ResourceID = id
		return
	}
	if id, ok := parseUUID(event.Payload["incident_id"]); ok {
		event.Scope.ResourceType = "incident"
		event.Scope.ResourceID = id
		return
	}
	if id, ok := parseUUID(event.Payload["alert_id"]); ok {
		event.Scope.ResourceType = "alert"
		event.Scope.ResourceID = id
		return
	}
	if id, ok := parseUUID(issueMap["id"]); ok {
		event.Scope.ResourceType = "issue"
		event.Scope.ResourceID = id
	}
}

func parseUUID(value any) (uuid.UUID, bool) {
	switch v := value.(type) {
	case nil:
		return uuid.Nil, false
	case uuid.UUID:
		if v == uuid.Nil {
			return uuid.Nil, false
		}
		return v, true
	case string:
		parsed, err := uuid.Parse(strings.TrimSpace(v))
		if err != nil {
			return uuid.Nil, false
		}
		return parsed, true
	case fmt.Stringer:
		parsed, err := uuid.Parse(strings.TrimSpace(v.String()))
		if err != nil {
			return uuid.Nil, false
		}
		return parsed, true
	default:
		return uuid.Nil, false
	}
}

func asMap(value any) map[string]any {
	switch v := value.(type) {
	case nil:
		return nil
	case map[string]any:
		return v
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		decoded := map[string]any{}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil
		}
		return decoded
	}
}
