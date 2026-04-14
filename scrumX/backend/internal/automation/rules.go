package automation

import (
	"sync"

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
	mu    sync.RWMutex
	rules map[uuid.UUID]Rule
}

func NewStore() *Store {
	return &Store{rules: map[uuid.UUID]Rule{}}
}

func (s *Store) Save(rule Rule) Rule {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}
	s.rules[rule.ID] = rule
	return rule
}

func (s *Store) Delete(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rules, id)
}

func (s *Store) List(orgID uuid.UUID) []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []Rule{}
	for _, rule := range s.rules {
		if rule.OrgID == orgID {
			result = append(result, rule)
		}
	}
	return result
}

func (s *Store) Matching(orgID uuid.UUID, trigger string) []Rule {
	rules := s.List(orgID)
	result := []Rule{}
	for _, rule := range rules {
		if rule.Trigger == trigger {
			result = append(result, rule)
		}
	}
	return result
}
