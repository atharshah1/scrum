package webhooks

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
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
	db      *sql.DB
	client  *http.Client
	log     *slog.Logger
	bus     *events.Bus
	timeout time.Duration
}

func NewDispatcher(log *slog.Logger, bus *events.Bus, timeout time.Duration, db *sql.DB) *Dispatcher {
	d := &Dispatcher{
		db:      db,
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

func (d *Dispatcher) Save(ctx context.Context, hook Webhook) (Webhook, error) {
	if hook.ID == uuid.Nil {
		hook.ID = uuid.New()
	}
	hook.Enabled = true
	_, err := d.db.ExecContext(ctx, `INSERT INTO webhooks (id, org_id, url, enabled) VALUES ($1,$2,$3,$4)`,
		hook.ID, hook.OrgID, hook.URL, hook.Enabled)
	return hook, err
}

func (d *Dispatcher) List(ctx context.Context, orgID uuid.UUID) ([]Webhook, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT id, org_id, url, enabled FROM webhooks WHERE org_id=$1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Webhook{}
	for rows.Next() {
		var hook Webhook
		if err := rows.Scan(&hook.ID, &hook.OrgID, &hook.URL, &hook.Enabled); err != nil {
			return nil, err
		}
		result = append(result, hook)
	}
	return result, rows.Err()
}

func (d *Dispatcher) Send(ctx context.Context, event events.Event) error {
	hooks, err := d.List(ctx, event.OrgID)
	if err != nil {
		return err
	}
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
