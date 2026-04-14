package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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

func decodeRuleField(field string, ruleID uuid.UUID, raw []byte, target any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode automation rule %s %s: %w", ruleID, field, err)
	}
	return nil
}
