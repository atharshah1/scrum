package events

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID        uuid.UUID      `json:"id"`
	OrgID     uuid.UUID      `json:"org_id"`
	Type      string         `json:"type"`
	ActorID   uuid.UUID      `json:"actor_id"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

func New(orgID uuid.UUID, eventType string, actorID uuid.UUID, payload map[string]any) Event {
	return Event{
		ID:        uuid.New(),
		OrgID:     orgID,
		Type:      eventType,
		ActorID:   actorID,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
}
