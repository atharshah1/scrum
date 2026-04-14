package events

import (
"encoding/json"
"log/slog"
"sync"

"github.com/gofiber/contrib/websocket"
)

type WebsocketHub struct {
mu      sync.RWMutex
clients map[*websocket.Conn]struct{}
buffer  int
log     *slog.Logger
}

func NewWebsocketHub(log *slog.Logger, buffer int) *WebsocketHub {
return &WebsocketHub{clients: map[*websocket.Conn]struct{}{}, buffer: buffer, log: log}
}

func (h *WebsocketHub) Add(conn *websocket.Conn) {
h.mu.Lock()
h.clients[conn] = struct{}{}
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
clients := make([]*websocket.Conn, 0, len(h.clients))
for conn := range h.clients {
clients = append(clients, conn)
}
h.mu.RUnlock()

for _, conn := range clients {
if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
h.log.Warn("ws_write_failed", "error", err)
h.Remove(conn)
_ = conn.Close()
}
}
}
