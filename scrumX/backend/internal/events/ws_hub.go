package events

import (
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
)

type subscription struct {
	orgID     uuid.UUID
	userID    uuid.UUID
	projectID *uuid.UUID
}

type WebsocketHub struct {
	mu         sync.RWMutex
	clients    map[*websocket.Conn]subscription
	buffer     int
	log        *slog.Logger
	maxPerOrg  int
	maxPerUser int
}

func NewWebsocketHub(log *slog.Logger, buffer, maxPerOrg, maxPerUser int) *WebsocketHub {
	return &WebsocketHub{
		clients:    map[*websocket.Conn]subscription{},
		buffer:     buffer,
		log:        log,
		maxPerOrg:  maxPerOrg,
		maxPerUser: maxPerUser,
	}
}

func (h *WebsocketHub) Add(conn *websocket.Conn, orgID, userID uuid.UUID, projectID *uuid.UUID) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.maxPerOrg > 0 {
		orgConnections := 0
		for _, sub := range h.clients {
			if sub.orgID == orgID {
				orgConnections++
			}
		}
		if orgConnections >= h.maxPerOrg {
			return errors.New("organization websocket connection limit reached")
		}
	}
	if h.maxPerUser > 0 {
		userConnections := 0
		for _, sub := range h.clients {
			if sub.orgID == orgID && sub.userID == userID {
				userConnections++
			}
		}
		if userConnections >= h.maxPerUser {
			return errors.New("user websocket connection limit reached")
		}
	}
	h.clients[conn] = subscription{orgID: orgID, userID: userID, projectID: projectID}
	return nil
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

	eventProjectID := event.Scope.ProjectID
	for conn, sub := range clients {
		if sub.orgID != uuid.Nil && sub.orgID != event.OrgID {
			continue
		}
		if sub.projectID != nil {
			if eventProjectID == nil || *sub.projectID != *eventProjectID {
				continue
			}
		}
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			h.log.Warn("ws_write_failed", "error", err)
			h.Remove(conn)
			_ = conn.Close()
		}
	}
}
