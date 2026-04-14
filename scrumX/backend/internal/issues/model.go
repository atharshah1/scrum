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
Title       string     `json:"title"`
Description string     `json:"description"`
Status      string     `json:"status"`
Priority    string     `json:"priority"`
CreatedAt   time.Time  `json:"created_at"`
UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateIssueInput struct {
ProjectID   uuid.UUID `json:"project_id"`
Title       string    `json:"title"`
Description string    `json:"description"`
Priority    string    `json:"priority"`
}

type UpdateIssueInput struct {
Title       *string `json:"title"`
Description *string `json:"description"`
Status      *string `json:"status"`
Priority    *string `json:"priority"`
}
