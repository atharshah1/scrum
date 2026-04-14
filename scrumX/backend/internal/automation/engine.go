package automation

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/google/uuid"
)

type storeWriter interface {
	Matching(ctx context.Context, orgID uuid.UUID, trigger string) ([]Rule, error)
	RecordExecution(ctx context.Context, orgID, ruleID uuid.UUID, eventType, status string, result map[string]any)
	PersistDeadLetter(ctx context.Context, orgID, ruleID uuid.UUID, event events.Event, action Action, attempts int, errMsg string)
	TryStartActionExecution(ctx context.Context, orgID, ruleID, eventID uuid.UUID, action Action) (bool, error)
	RecordActionAttempt(ctx context.Context, orgID, ruleID, eventID uuid.UUID, action Action, errMsg string, terminal bool)
}

type issueMutator interface {
	ApplyAutomationUpdate(ctx context.Context, orgID, issueID uuid.UUID, status string, assigneeID *uuid.UUID) error
}

type webhookCaller interface {
	Send(ctx context.Context, event events.Event) error
}

type Engine struct {
	log         *slog.Logger
	store       storeWriter
	queue       chan events.Event
	webhook     webhookCaller
	workerCount int
	issues      issueMutator
	maxRetries  int
	backoff     time.Duration
}

// NewEngine configures automation processing with a fail-open deduplication strategy:
// if durable idempotency persistence is temporarily unavailable, actions are still executed
// (to preserve availability) and may be duplicated until storage recovers.
func NewEngine(log *slog.Logger, store storeWriter, webhook webhookCaller, workers int, issues issueMutator, maxRetries int, backoff time.Duration) *Engine {
	if workers <= 0 {
		workers = 1
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	if backoff <= 0 {
		backoff = 200 * time.Millisecond
	}
	return &Engine{log: log, store: store, queue: make(chan events.Event, 512), webhook: webhook, workerCount: workers, issues: issues, maxRetries: maxRetries, backoff: backoff}
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
			acquired, err := e.store.TryStartActionExecution(ctx, event.OrgID, rule.ID, event.ID, action)
			if err != nil {
				e.log.Warn("automation_dedup_persist_failed", "error", err, "rule_id", rule.ID, "event_id", event.ID)
				// Fail-open: execute action even if durable dedup metadata is unavailable,
				// to avoid dropping automation work on transient store errors.
				if err := e.executeActionWithRetry(ctx, rule.ID, action, event); err != nil {
					status = "failed"
					e.log.Warn("automation_action_failed", "error", err, "type", action.Type)
				}
				continue
			}
			if !acquired {
				e.store.RecordExecution(ctx, event.OrgID, rule.ID, event.Type, "skipped", map[string]any{
					"reason":   "duplicate_action_execution",
					"event_id": event.ID.String(),
				})
				continue
			}
			if err := e.executeActionWithRetry(ctx, rule.ID, action, event); err != nil {
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

func (e *Engine) executeActionWithRetry(ctx context.Context, ruleID uuid.UUID, action Action, event events.Event) error {
	var err error
	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		err = e.executeAction(ctx, action, event)
		if err == nil {
			e.store.RecordActionAttempt(ctx, event.OrgID, ruleID, event.ID, action, "", true)
			return nil
		}
		e.store.RecordActionAttempt(ctx, event.OrgID, ruleID, event.ID, action, err.Error(), attempt == e.maxRetries)
		if attempt == e.maxRetries {
			break
		}
		shift := attempt
		if shift > 10 {
			shift = 10
		}
		timer := time.NewTimer(time.Duration(1<<shift) * e.backoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	// All retries exhausted – persist for later inspection / replay.
	e.store.PersistDeadLetter(ctx, event.OrgID, ruleID, event, action, e.maxRetries+1, err.Error())
	return err
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
	if e.issues != nil && (action.Type == "update issue" || action.Type == "assign issue") {
		issueID, ok := action.Params["issue_id"].(string)
		if !ok && action.Params["issue_id"] != nil {
			return errors.New("automation issue_id must be a string")
		}
		if issueID == "" {
			if fromPayload, ok := event.Payload["issue_id"].(string); ok {
				issueID = fromPayload
			}
		}
		if issueID == "" {
			return nil
		}
		issueUUID, err := uuid.Parse(issueID)
		if err != nil {
			e.log.Warn("automation_invalid_param", "param", "issue_id", "value", issueID)
			return nil
		}
		status, ok := action.Params["status"].(string)
		if !ok && action.Params["status"] != nil {
			return errors.New("automation status must be a string")
		}
		assigneeID, ok := action.Params["assignee_id"].(string)
		if !ok && action.Params["assignee_id"] != nil {
			return errors.New("automation assignee_id must be a string")
		}
		status = strings.ToLower(strings.TrimSpace(status))
		if assigneeID != "" {
			parsed := uuid.Nil
			if parsed, err = uuid.Parse(assigneeID); err != nil {
				e.log.Warn("automation_invalid_param", "param", "assignee_id", "value", assigneeID)
				return nil
			}
			return e.issues.ApplyAutomationUpdate(ctx, event.OrgID, issueUUID, status, &parsed)
		}
		return e.issues.ApplyAutomationUpdate(ctx, event.OrgID, issueUUID, status, nil)
	}
	return nil
}

func (e *Engine) ReplayDeadLetter(ctx context.Context, dl DeadLetter) error {
	action := Action{
		Type:   dl.ActionType,
		Params: dl.ActionParams,
	}
	ruleID := uuid.Nil
	if dl.RuleID != nil {
		ruleID = *dl.RuleID
	}
	event := events.Event{
		ID:        uuid.New(),
		OrgID:     dl.OrgID,
		Type:      "automation.dead_letter.replay",
		ActorID:   uuid.Nil,
		Payload:   dl.EventPayload,
		CreatedAt: time.Now().UTC(),
	}
	return e.executeActionWithRetry(ctx, ruleID, action, event)
}
