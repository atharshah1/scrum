package issues

import (
	"time"

	"github.com/google/uuid"
)

type Issue struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	ReporterID  uuid.UUID  `json:"reporter_id"`
	AssigneeID  *uuid.UUID `json:"assignee_id,omitempty"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	SprintID    *uuid.UUID `json:"sprint_id,omitempty"`
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
	ProjectID   uuid.UUID  `json:"project_id"`
	AssigneeID  *uuid.UUID `json:"assignee_id"`
	ParentID    *uuid.UUID `json:"parent_id"`
	SprintID    *uuid.UUID `json:"sprint_id"`
	IssueType   string     `json:"issue_type"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    string     `json:"priority"`
	Labels      []string   `json:"labels"`
}

type UpdateIssueInput struct {
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	Status      *string    `json:"status"`
	Priority    *string    `json:"priority"`
	SprintID    *uuid.UUID `json:"sprint_id"`
	AssigneeID  *uuid.UUID `json:"assignee_id"`
}

type ListIssuesInput struct {
	ProjectID *uuid.UUID
	Status    string
	Assignee  *uuid.UUID
	SprintID  *uuid.UUID
	Labels    []string
	Limit     int
	Offset    int
	SortBy    string
	SortOrder string
}

type IssueLink struct {
	ID             uuid.UUID `json:"id"`
	IssueID        uuid.UUID `json:"issue_id"`
	RelatedIssueID uuid.UUID `json:"related_issue_id"`
	RelationType   string    `json:"relation_type"`
}

type CreateIssueLinkInput struct {
	RelatedIssueID uuid.UUID `json:"related_issue_id"`
	RelationType   string    `json:"relation_type"`
}

type IssueComment struct {
	ID        uuid.UUID `json:"id"`
	IssueID   uuid.UUID `json:"issue_id"`
	AuthorID  uuid.UUID `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateCommentInput struct {
	Body string `json:"body"`
}

type UpdateCommentInput struct {
	Body string `json:"body"`
}

type AssignIssueInput struct {
	AssigneeID uuid.UUID `json:"assignee_id"`
}

type LabelInput struct {
	Label string `json:"label"`
}
