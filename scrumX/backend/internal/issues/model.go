package issues

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Issue struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	SprintID    *uuid.UUID `json:"sprint_id,omitempty"`
	ReporterID  uuid.UUID  `json:"reporter_id"`
	AssigneeID  *uuid.UUID `json:"assignee_id,omitempty"`
	IssueType   string     `json:"issue_type"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	Labels      []string   `json:"labels,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateIssueInput struct {
	ProjectID   uuid.UUID `json:"project_id"`
	ParentID    uuid.UUID `json:"parent_id"`
	SprintID    uuid.UUID `json:"sprint_id"`
	IssueType   string    `json:"issue_type"`
	AssigneeID  uuid.UUID `json:"assignee_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Priority    string    `json:"priority"`
	Labels      []string  `json:"labels"`
}

type UpdateIssueInput struct {
	ParentID    *uuid.UUID `json:"parent_id"`
	SprintID    *uuid.UUID `json:"sprint_id"`
	AssigneeID  *uuid.UUID `json:"assignee_id"`
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	Status      *string    `json:"status"`
	Priority    *string    `json:"priority"`
	IssueType   *string    `json:"issue_type"`
	Labels      []string   `json:"labels"`
}

type ListIssuesFilter struct {
	ProjectID  uuid.UUID
	Status     string
	AssigneeID uuid.UUID
	SprintID   uuid.UUID
	Label      string
	IssueType  string
	ParentID   uuid.UUID
	SortBy     string
	Order      string
	Page       int
	Limit      int
}

type IssueComment struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	IssueID   uuid.UUID `json:"issue_id"`
	AuthorID  uuid.UUID `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type IssueActivity struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	IssueID   uuid.UUID `json:"issue_id"`
	ActorID   uuid.UUID `json:"actor_id"`
	Action    string    `json:"action"`
	Field     string    `json:"field,omitempty"`
	FromValue string    `json:"from_value,omitempty"`
	ToValue   string    `json:"to_value,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type WorkflowTransitionRule struct {
	Allowed       bool
	HasCustomRule bool
	Conditions    map[string]any
	Validators    map[string]any
	PostFunctions map[string]any
}

func defaultRule() WorkflowTransitionRule {
	return WorkflowTransitionRule{
		Conditions:    map[string]any{},
		Validators:    map[string]any{},
		PostFunctions: map[string]any{},
	}
}

func decodeRuleJSON(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

const (
	IssueTypeEpic  = "epic"
	IssueTypeStory = "story"
	IssueTypeTask  = "task"
	IssueTypeBug   = "bug"
)

var DefaultWorkflowTransitions = map[string][]string{
	"todo":        {"in_progress", "done"},
	"in_progress": {"todo", "done"},
	"done":        {"todo"},
}

func ValidIssueType(v string) bool {
	switch v {
	case IssueTypeEpic, IssueTypeStory, IssueTypeTask, IssueTypeBug:
		return true
	default:
		return false
	}
}
