package automation

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"

	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
)

type issueMutator interface {
	UpdateStatus(ctx context.Context, issueID, orgID string, status string) error
}

type webhookCaller interface {
	Send(ctx context.Context, event events.Event) error
}

type Engine struct {
	log         *slog.Logger
	store       *Store
	queue       chan events.Event
	webhook     webhookCaller
	workerCount int
	db          *sql.DB
}

func NewEngine(log *slog.Logger, store *Store, webhook webhookCaller, workers int, db *sql.DB) *Engine {
	if workers <= 0 {
		workers = 1
	}
	return &Engine{log: log, store: store, queue: make(chan events.Event, 512), webhook: webhook, workerCount: workers, db: db}
}

func (e *Engine) Enqueue(event events.Event) {
	select {
	case e.queue <- event:
	default:
		e.log.Warn("automation_queue_full", "event", event.Type)
	}
}

func (e *Engine) Start(ctx context.Context) {
	for i := 0; i < e.workerCount; i++ {
		go e.worker(ctx)
	}
}

func (e *Engine) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-e.queue:
			e.execute(ctx, event)
		}
	}
}

func (e *Engine) execute(ctx context.Context, event events.Event) {
	rules, err := e.store.Matching(ctx, event.OrgID, event.Type)
	if err != nil {
		e.log.Warn("automation_rules_fetch_failed", "error", err)
		return
	}
	for _, rule := range rules {
		if !e.conditionsMet(rule, event) {
			e.store.RecordExecution(ctx, event.OrgID, rule.ID, event.Type, "skipped", map[string]any{"reason": "conditions_not_met"})
			continue
		}
		status := "success"
		for _, action := range rule.Actions {
			if err := e.executeAction(ctx, action, event); err != nil {
				status = "failed"
				e.log.Warn("automation_action_failed", "error", err, "type", action.Type)
			}
		}
		e.store.RecordExecution(ctx, event.OrgID, rule.ID, event.Type, status, map[string]any{"rule_name": rule.Name})
	}
}

func (e *Engine) conditionsMet(rule Rule, event events.Event) bool {
	for _, c := range rule.Conditions {
		switch c.Type {
		case "field equals":
			value, _ := event.Payload[c.Field].(string)
			if !strings.EqualFold(value, c.Value) {
				return false
			}
		case "label exists":
			labels, _ := event.Payload["labels"].([]string)
			found := false
			for _, label := range labels {
				if strings.EqualFold(label, c.Value) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case "user match":
			userID, _ := event.Payload[c.Field].(string)
			if userID != c.Value {
				return false
			}
		}
	}
	return true
}

func (e *Engine) executeAction(ctx context.Context, action Action, event events.Event) error {
	e.log.Info("automation_action", "type", action.Type, "event", event.Type)
	if action.Type == "call webhook" && e.webhook != nil {
		if err := e.webhook.Send(ctx, event); err != nil {
			e.log.Warn("automation_webhook_failed", "error", err)
			return err
		}
		return nil
	}
	if e.db != nil && (action.Type == "update issue" || action.Type == "assign issue") {
		issueID, _ := action.Params["issue_id"].(string)
		if issueID == "" {
			if fromPayload, ok := event.Payload["issue_id"].(string); ok {
				issueID = fromPayload
			}
		}
		if issueID == "" {
			return nil
		}
		status, _ := action.Params["status"].(string)
		assigneeID, _ := action.Params["assignee_id"].(string)
		if status != "" {
			if _, err := e.db.ExecContext(ctx, `UPDATE issues SET status=$1, updated_at=NOW() WHERE id=$2 AND org_id=$3 AND deleted_at IS NULL`,
				strings.ToLower(strings.TrimSpace(status)), issueID, event.OrgID); err != nil {
				return err
			}
		}
		if assigneeID != "" {
			if _, err := e.db.ExecContext(ctx, `UPDATE issues SET assignee_id=$1, updated_at=NOW() WHERE id=$2 AND org_id=$3 AND deleted_at IS NULL`,
				assigneeID, issueID, event.OrgID); err != nil {
				return err
			}
		}
	}
	return nil
}
