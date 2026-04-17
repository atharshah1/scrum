package events

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Scope struct {
	ProjectID *uuid.UUID `json:"project_id,omitempty"`
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

func New(orgID uuid.UUID, eventType string, actorID uuid.UUID, payload map[string]any, options ...Option) Event {
	if payload == nil {
		payload = map[string]any{}
	}
	event := Event{
		ID:        uuid.New(),
		OrgID:     orgID,
		Type:      eventType,
		ActorID:   actorID,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
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
	return nil
}
