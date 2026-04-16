package offline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ModeOnline  = "online"
	ModeOffline = "offline"
	ModeSyncing = "syncing"
)

type Issue struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	ParentID    *string   `json:"parent_id,omitempty"`
	SprintID    *string   `json:"sprint_id,omitempty"`
	AssigneeID  *string   `json:"assignee_id,omitempty"`
	IssueType   string    `json:"issue_type"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Labels      []string  `json:"labels"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
	Dirty       bool      `json:"dirty"`
	LocalOnly   bool      `json:"local_only"`
	Deleted     bool      `json:"deleted"`
}

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email,omitempty"`
	FullName string `json:"full_name,omitempty"`
	Role     string `json:"role,omitempty"`
}

type Operation struct {
	ID            string          `json:"id"`
	Entity        string          `json:"entity"`
	Action        string          `json:"action"`
	TargetID      string          `json:"target_id"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	Attempts      int             `json:"attempts"`
	LastError     string          `json:"last_error,omitempty"`
	NextAttemptAt time.Time       `json:"next_attempt_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type SyncEntityMeta struct {
	LastSyncedAt time.Time `json:"last_synced_at,omitempty"`
	LastVersion  time.Time `json:"last_version,omitempty"`
}

type State struct {
	Mode              string                    `json:"mode"`
	LastSyncedAt      time.Time                 `json:"last_synced_at,omitempty"`
	Issues            map[string]Issue          `json:"issues"`
	Projects          map[string]Project        `json:"projects"`
	Users             map[string]User           `json:"users"`
	PendingOperations []Operation               `json:"pending_operations"`
	IDAliases         map[string]string         `json:"id_aliases,omitempty"`
	IssueQueryCache   map[string][]string       `json:"issue_query_cache,omitempty"`
	EntitySync        map[string]SyncEntityMeta `json:"entity_sync,omitempty"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(baseDir, scope string) (*Store, error) {
	if strings.TrimSpace(baseDir) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		baseDir = filepath.Join(home, ".scrumx")
	}
	offlineDir := filepath.Join(baseDir, "offline")
	if err := os.MkdirAll(offlineDir, 0o700); err != nil {
		return nil, fmt.Errorf("create offline dir: %w", err)
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(scope)))
	file := filepath.Join(offlineDir, "state_"+hex.EncodeToString(sum[:])+".json")
	return &Store{path: file}, nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadUnlocked()
}

func (s *Store) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUnlocked(normalizeState(state))
}

func (s *Store) Update(fn func(*State) error) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadUnlocked()
	if err != nil {
		return State{}, err
	}
	if err := fn(&state); err != nil {
		return State{}, err
	}
	state = normalizeState(state)
	if err := s.saveUnlocked(state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Store) loadUnlocked() (State, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return normalizeState(State{Mode: ModeOnline}), nil
		}
		return State{}, fmt.Errorf("read offline state: %w", err)
	}
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return State{}, fmt.Errorf("decode offline state: %w", err)
	}
	return normalizeState(state), nil
}

func (s *Store) saveUnlocked(state State) error {
	payload, err := json.MarshalIndent(normalizeState(state), "", "  ")
	if err != nil {
		return fmt.Errorf("encode offline state: %w", err)
	}
	if err := os.WriteFile(s.path, payload, 0o600); err != nil {
		return fmt.Errorf("write offline state: %w", err)
	}
	return nil
}

func normalizeState(in State) State {
	if strings.TrimSpace(in.Mode) == "" {
		in.Mode = ModeOnline
	}
	if in.Issues == nil {
		in.Issues = map[string]Issue{}
	}
	if in.Projects == nil {
		in.Projects = map[string]Project{}
	}
	if in.Users == nil {
		in.Users = map[string]User{}
	}
	if in.PendingOperations == nil {
		in.PendingOperations = []Operation{}
	}
	if in.IDAliases == nil {
		in.IDAliases = map[string]string{}
	}
	if in.IssueQueryCache == nil {
		in.IssueQueryCache = map[string][]string{}
	}
	if in.EntitySync == nil {
		in.EntitySync = map[string]SyncEntityMeta{}
	}
	return in
}

func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<uint(attempt-1)) * time.Second
}

func ResolveID(state State, id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	if mapped, ok := state.IDAliases[trimmed]; ok && strings.TrimSpace(mapped) != "" {
		return mapped
	}
	return trimmed
}

func ListIssues(state State, key string) []Issue {
	ids := state.IssueQueryCache[key]
	if len(ids) == 0 {
		issues := make([]Issue, 0, len(state.Issues))
		for _, issue := range state.Issues {
			if issue.Deleted {
				continue
			}
			issues = append(issues, issue)
		}
		sort.SliceStable(issues, func(i, j int) bool {
			if !issues[i].UpdatedAt.Equal(issues[j].UpdatedAt) {
				return issues[i].UpdatedAt.After(issues[j].UpdatedAt)
			}
			return issues[i].ID < issues[j].ID
		})
		return issues
	}
	issues := make([]Issue, 0, len(ids))
	for _, id := range ids {
		if item, ok := state.Issues[id]; ok && !item.Deleted {
			issues = append(issues, item)
		}
	}
	return issues
}
