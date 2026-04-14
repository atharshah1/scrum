package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/google/uuid"
)

type Webhook struct {
	ID      uuid.UUID `json:"id"`
	OrgID   uuid.UUID `json:"org_id"`
	URL     string    `json:"url"`
	Enabled bool      `json:"enabled"`
}

type Dispatcher struct {
	mu      sync.RWMutex
	hooks   map[uuid.UUID]Webhook
	client  *http.Client
	log     *slog.Logger
	bus     *events.Bus
	timeout time.Duration
}

func NewDispatcher(log *slog.Logger, bus *events.Bus, timeout time.Duration) *Dispatcher {
	d := &Dispatcher{
		hooks:   map[uuid.UUID]Webhook{},
		client:  &http.Client{Timeout: timeout},
		log:     log,
		bus:     bus,
		timeout: timeout,
	}
	bus.Subscribe("*", func(event events.Event) {
		ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
		defer cancel()
		if err := d.Send(ctx, event); err != nil {
			log.Warn("webhook_dispatch_failed", "error", err, "event", event.Type)
		}
	})
	return d
}

func (d *Dispatcher) Save(hook Webhook) Webhook {
	d.mu.Lock()
	defer d.mu.Unlock()
	if hook.ID == uuid.Nil {
		hook.ID = uuid.New()
	}
	hook.Enabled = true
	d.hooks[hook.ID] = hook
	return hook
}

func (d *Dispatcher) List(orgID uuid.UUID) []Webhook {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := []Webhook{}
	for _, hook := range d.hooks {
		if hook.OrgID == orgID {
			result = append(result, hook)
		}
	}
	return result
}

func (d *Dispatcher) Send(ctx context.Context, event events.Event) error {
	hooks := d.List(event.OrgID)
	for _, hook := range hooks {
		if !hook.Enabled {
			continue
		}
		payload, _ := json.Marshal(event)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := d.client.Do(req)
		if err != nil {
			d.log.Warn("webhook_call_failed", "url", hook.URL, "error", err)
			continue
		}
		_ = resp.Body.Close()
	}
	return nil
}
