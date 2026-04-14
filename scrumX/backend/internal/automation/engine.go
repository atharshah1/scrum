package automation

import (
"context"
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
}

func NewEngine(log *slog.Logger, store *Store, webhook webhookCaller, workers int) *Engine {
if workers <= 0 {
workers = 1
}
return &Engine{log: log, store: store, queue: make(chan events.Event, 512), webhook: webhook, workerCount: workers}
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
for _, rule := range e.store.Matching(event.OrgID, event.Type) {
if !e.conditionsMet(rule, event) {
continue
}
for _, action := range rule.Actions {
e.executeAction(ctx, action, event)
}
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

func (e *Engine) executeAction(ctx context.Context, action Action, event events.Event) {
e.log.Info("automation_action", "type", action.Type, "event", event.Type)
if action.Type == "call webhook" && e.webhook != nil {
if err := e.webhook.Send(ctx, event); err != nil {
e.log.Warn("automation_webhook_failed", "error", err)
}
}
}
