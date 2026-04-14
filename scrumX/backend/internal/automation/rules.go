package automation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/google/uuid"
)

type Rule struct {
	ID         uuid.UUID      `json:"id"`
	OrgID      uuid.UUID      `json:"org_id"`
	Name       string         `json:"name"`
	Trigger    string         `json:"trigger"`
	Conditions []Condition    `json:"conditions"`
	Actions    []Action       `json:"actions"`
	DSL        map[string]any `json:"dsl,omitempty"`
}

type Condition struct {
	Type  string `json:"type"`
	Field string `json:"field"`
	Value string `json:"value"`
}

type Action struct {
	Type   string         `json:"type"`
	Params map[string]any `json:"params"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

type DeadLetter struct {
	ID             uuid.UUID      `json:"id"`
	OrgID          uuid.UUID      `json:"org_id"`
	RuleID         *uuid.UUID     `json:"rule_id,omitempty"`
	EventPayload   map[string]any `json:"event_payload"`
	ActionType     string         `json:"action_type"`
	ActionParams   map[string]any `json:"action_params"`
	ErrorMessage   string         `json:"error_message"`
	Attempts       int            `json:"attempts"`
	ReplayCount    int            `json:"replay_count"`
	LastReplayedAt *time.Time     `json:"last_replayed_at,omitempty"`
	LastReplayedBy *uuid.UUID     `json:"last_replayed_by,omitempty"`
	LastReplayErr  string         `json:"last_replay_error,omitempty"`
	Resolved       bool           `json:"resolved"`
	CreatedAt      time.Time      `json:"created_at"`
}

func (s *Store) Save(ctx context.Context, rule Rule) (Rule, error) {
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}
	conditionsRaw, _ := json.Marshal(rule.Conditions)
	actionsRaw, _ := json.Marshal(rule.Actions)
	dslRaw, _ := json.Marshal(rule.DSL)
	_, err := s.db.ExecContext(ctx, `INSERT INTO automation_rules (id, org_id, name, trigger, conditions, actions, dsl, enabled)
VALUES ($1,$2,$3,$4,$5,$6,$7,TRUE)
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, trigger=EXCLUDED.trigger, conditions=EXCLUDED.conditions, actions=EXCLUDED.actions, dsl=EXCLUDED.dsl`,
		rule.ID, rule.OrgID, rule.Name, rule.Trigger, conditionsRaw, actionsRaw, dslRaw)
	return rule, err
}

func (s *Store) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM automation_rules WHERE id=$1 AND org_id=$2`, id, orgID)
	return err
}

func (s *Store) List(ctx context.Context, orgID uuid.UUID) ([]Rule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, org_id, name, trigger, conditions, actions, dsl FROM automation_rules WHERE org_id=$1 AND enabled=TRUE ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var (
			r             Rule
			conditionsRaw []byte
			actionsRaw    []byte
			dslRaw        []byte
		)
		if err := rows.Scan(&r.ID, &r.OrgID, &r.Name, &r.Trigger, &conditionsRaw, &actionsRaw, &dslRaw); err != nil {
			return nil, err
		}
		if err := decodeRuleField("conditions", r.ID, conditionsRaw, &r.Conditions); err != nil {
			return nil, err
		}
		if err := decodeRuleField("actions", r.ID, actionsRaw, &r.Actions); err != nil {
			return nil, err
		}
		if err := decodeRuleField("dsl", r.ID, dslRaw, &r.DSL); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) Matching(ctx context.Context, orgID uuid.UUID, trigger string) ([]Rule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, org_id, name, trigger, conditions, actions, dsl FROM automation_rules
WHERE org_id=$1 AND trigger=$2 AND enabled=TRUE ORDER BY created_at DESC`, orgID, trigger)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var (
			r             Rule
			conditionsRaw []byte
			actionsRaw    []byte
			dslRaw        []byte
		)
		if err := rows.Scan(&r.ID, &r.OrgID, &r.Name, &r.Trigger, &conditionsRaw, &actionsRaw, &dslRaw); err != nil {
			return nil, err
		}
		if err := decodeRuleField("conditions", r.ID, conditionsRaw, &r.Conditions); err != nil {
			return nil, err
		}
		if err := decodeRuleField("actions", r.ID, actionsRaw, &r.Actions); err != nil {
			return nil, err
		}
		if err := decodeRuleField("dsl", r.ID, dslRaw, &r.DSL); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) RecordExecution(ctx context.Context, orgID, ruleID uuid.UUID, eventType, status string, result map[string]any) {
	raw, _ := json.Marshal(result)
	_, _ = s.db.ExecContext(ctx, `INSERT INTO automation_executions (id, org_id, rule_id, event_type, status, result) VALUES ($1,$2,$3,$4,$5,$6)`,
		uuid.New(), orgID, ruleID, eventType, status, raw)
}

// PersistDeadLetter records an automation action that exhausted all retry attempts so
// it can be inspected and replayed later without restarting the process.
func (s *Store) PersistDeadLetter(ctx context.Context, orgID, ruleID uuid.UUID, event events.Event, action Action, attempts int, errMsg string) {
	eventRaw, _ := json.Marshal(event.Payload)
	paramsRaw, _ := json.Marshal(action.Params)
	_, _ = s.db.ExecContext(ctx, `INSERT INTO automation_dead_letters
		(org_id, rule_id, event_payload, action_type, action_params, error_message, attempts)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		orgID, ruleID, eventRaw, action.Type, paramsRaw, errMsg, attempts)
}

func (s *Store) ListDeadLetters(ctx context.Context, orgID uuid.UUID, page, limit int) ([]DeadLetter, int, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM automation_dead_letters WHERE org_id=$1`, orgID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, org_id, rule_id, event_payload, action_type, action_params, error_message, attempts, replay_count, last_replayed_at, last_replayed_by, last_replay_error, resolved, created_at
		FROM automation_dead_letters
		WHERE org_id=$1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, orgID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []DeadLetter{}
	for rows.Next() {
		dl, err := scanDeadLetter(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, dl)
	}
	return out, total, rows.Err()
}

func (s *Store) GetDeadLetter(ctx context.Context, orgID, deadLetterID uuid.UUID) (DeadLetter, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, org_id, rule_id, event_payload, action_type, action_params, error_message, attempts, replay_count, last_replayed_at, last_replayed_by, last_replay_error, resolved, created_at
		FROM automation_dead_letters
		WHERE org_id=$1 AND id=$2`, orgID, deadLetterID)
	return scanDeadLetter(row)
}

func (s *Store) RecordDeadLetterReplay(ctx context.Context, orgID, deadLetterID, actorID uuid.UUID, replayErr error) error {
	var errMsg any
	resolved := true
	if replayErr != nil {
		errMsg = replayErr.Error()
		resolved = false
	}
	_, err := s.db.ExecContext(ctx, `UPDATE automation_dead_letters
		SET replay_count = replay_count + 1,
			last_replayed_at = NOW(),
			last_replayed_by = $3,
			last_replay_error = $4,
			resolved = CASE WHEN $5 THEN TRUE ELSE resolved END
		WHERE org_id = $1 AND id = $2`,
		orgID, deadLetterID, actorID, errMsg, resolved)
	return err
}

func actionFingerprint(action Action) (string, error) {
	raw, err := json.Marshal(map[string]any{
		"type":   action.Type,
		"params": action.Params,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// TryStartActionExecution creates a durable idempotency row for an action execution.
// Returns true only for the first processor that acquires this event/rule/action tuple.
func (s *Store) TryStartActionExecution(ctx context.Context, orgID, ruleID, eventID uuid.UUID, action Action) (bool, error) {
	fingerprint, err := actionFingerprint(action)
	if err != nil {
		return false, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO automation_action_executions
		(org_id, rule_id, event_id, action_fingerprint, status, attempts)
		VALUES ($1,$2,$3,$4,'processing',0)
		ON CONFLICT (org_id, rule_id, event_id, action_fingerprint) DO NOTHING`,
		orgID, ruleID, eventID, fingerprint)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

// RecordActionAttempt increments durable attempt metadata and updates terminal status.
func (s *Store) RecordActionAttempt(ctx context.Context, orgID, ruleID, eventID uuid.UUID, action Action, errMsg string, terminal bool) {
	fingerprint, err := actionFingerprint(action)
	if err != nil {
		return
	}
	status := "processing"
	lastError := any(nil)
	if errMsg != "" {
		lastError = errMsg
		if terminal {
			status = "failed"
		}
	} else {
		status = "success"
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE automation_action_executions
		SET attempts = attempts + 1, status = $5, last_error = $6, updated_at = NOW()
		WHERE org_id=$1 AND rule_id=$2 AND event_id=$3 AND action_fingerprint=$4`,
		orgID, ruleID, eventID, fingerprint, status, lastError)
}

func decodeRuleField(field string, ruleID uuid.UUID, raw []byte, target any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode automation rule %s %s: %w", ruleID, field, err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDeadLetter(s scanner) (DeadLetter, error) {
	var (
		dl              DeadLetter
		ruleID          *uuid.UUID
		eventPayloadRaw []byte
		actionParamsRaw []byte
		lastReplayBy    *uuid.UUID
		lastReplayErr   *string
	)
	if err := s.Scan(&dl.ID, &dl.OrgID, &ruleID, &eventPayloadRaw, &dl.ActionType, &actionParamsRaw, &dl.ErrorMessage, &dl.Attempts, &dl.ReplayCount, &dl.LastReplayedAt, &lastReplayBy, &lastReplayErr, &dl.Resolved, &dl.CreatedAt); err != nil {
		return DeadLetter{}, err
	}
	dl.RuleID = ruleID
	dl.LastReplayedBy = lastReplayBy
	if lastReplayErr != nil {
		dl.LastReplayErr = *lastReplayErr
	}
	if err := decodeJSONMap(eventPayloadRaw, &dl.EventPayload); err != nil {
		return DeadLetter{}, err
	}
	if err := decodeJSONMap(actionParamsRaw, &dl.ActionParams); err != nil {
		return DeadLetter{}, err
	}
	return dl, nil
}

func decodeJSONMap(raw []byte, out *map[string]any) error {
	if len(raw) == 0 {
		*out = map[string]any{}
		return nil
	}
	return json.Unmarshal(raw, out)
}
