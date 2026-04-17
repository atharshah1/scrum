package events

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
)

type subscription struct {
	orgID     uuid.UUID
	projectID *uuid.UUID
}

type WebsocketHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]subscription
	buffer  int
	log     *slog.Logger
}

func NewWebsocketHub(log *slog.Logger, buffer int) *WebsocketHub {
	return &WebsocketHub{clients: map[*websocket.Conn]subscription{}, buffer: buffer, log: log}
}

func (h *WebsocketHub) Add(conn *websocket.Conn, orgID uuid.UUID, projectID *uuid.UUID) {
	h.mu.Lock()
	h.clients[conn] = subscription{orgID: orgID, projectID: projectID}
	h.mu.Unlock()
}

func (h *WebsocketHub) Remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

func (h *WebsocketHub) Broadcast(event Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		h.log.Error("ws_marshal_failed", "error", err)
		return
	}

	h.mu.RLock()
	clients := make(map[*websocket.Conn]subscription, len(h.clients))
	for conn, sub := range h.clients {
		clients[conn] = sub
	}
	h.mu.RUnlock()

	eventProjectID := projectIDFromEvent(event)
	for conn, sub := range clients {
		if sub.orgID != uuid.Nil && sub.orgID != event.OrgID {
			continue
		}
		if sub.projectID != nil && eventProjectID != nil && *sub.projectID != *eventProjectID {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			h.log.Warn("ws_write_failed", "error", err)
			h.Remove(conn)
			_ = conn.Close()
		}
	}
}

func projectIDFromEvent(event Event) *uuid.UUID {
	if id := uuidFromAny(event.Payload["project_id"]); id != nil {
		return id
	}
	if issue, ok := event.Payload["issue"].(map[string]any); ok {
		if id := uuidFromAny(issue["project_id"]); id != nil {
			return id
		}
	}
	if release, ok := event.Payload["release"].(map[string]any); ok {
		if id := uuidFromAny(release["project_id"]); id != nil {
			return id
		}
	}
	return nil
}

func uuidFromAny(v any) *uuid.UUID {
	switch raw := v.(type) {
	case string:
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil
		}
		return &id
	case uuid.UUID:
		id := raw
		return &id
	case *uuid.UUID:
		return raw
	default:
		return nil
	}
}
