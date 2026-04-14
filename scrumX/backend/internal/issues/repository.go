package issues

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, issue Issue) (Issue, error) {
	query := `
INSERT INTO issues (id, org_id, project_id, reporter_id, assignee_id, parent_id, sprint_id, issue_type, title, description, status, priority, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	_, err := r.db.ExecContext(ctx, query,
		issue.ID, issue.OrgID, issue.ProjectID, issue.ReporterID, issue.AssigneeID, issue.ParentID, issue.SprintID, issue.IssueType,
		issue.Title, issue.Description, issue.Status, issue.Priority, issue.CreatedAt, issue.UpdatedAt,
	)
	if err != nil {
		return Issue{}, err
	}
	return r.GetByID(ctx, issue.OrgID, issue.ID)
}

func (r *Repository) GetByID(ctx context.Context, orgID, issueID uuid.UUID) (Issue, error) {
	var issue Issue
	row := r.db.QueryRowContext(ctx, `SELECT id, org_id, project_id, reporter_id, assignee_id, parent_id, sprint_id, issue_type, title, description, status, priority, created_at, updated_at
FROM issues WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`, issueID, orgID)
	if err := row.Scan(&issue.ID, &issue.OrgID, &issue.ProjectID, &issue.ReporterID, &issue.AssigneeID, &issue.ParentID, &issue.SprintID, &issue.IssueType,
		&issue.Title, &issue.Description, &issue.Status, &issue.Priority, &issue.CreatedAt, &issue.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Issue{}, errors.New("issue not found")
		}
		return Issue{}, err
	}
	labels, err := r.ListLabels(ctx, orgID, issueID)
	if err != nil {
		return Issue{}, err
	}
	issue.Labels = labels
	return issue, nil
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, input ListIssuesInput) ([]Issue, error) {
	query := `SELECT i.id, i.org_id, i.project_id, i.reporter_id, i.assignee_id, i.parent_id, i.sprint_id, i.issue_type, i.title, i.description, i.status, i.priority, i.created_at, i.updated_at
FROM issues i
WHERE i.org_id=$1 AND i.deleted_at IS NULL`
	args := []any{orgID}
	argN := 2

	if input.ProjectID != nil {
		query += ` AND i.project_id=$` + itoa(argN)
		args = append(args, *input.ProjectID)
		argN++
	}
	if input.Status != "" {
		query += ` AND i.status=$` + itoa(argN)
		args = append(args, strings.ToLower(input.Status))
		argN++
	}
	if input.Assignee != nil {
		query += ` AND i.assignee_id=$` + itoa(argN)
		args = append(args, *input.Assignee)
		argN++
	}
	if input.SprintID != nil {
		query += ` AND i.sprint_id=$` + itoa(argN)
		args = append(args, *input.SprintID)
		argN++
	}
	if len(input.Labels) > 0 {
		placeholders := make([]string, 0, len(input.Labels))
		for _, label := range input.Labels {
			placeholders = append(placeholders, "$"+itoa(argN))
			args = append(args, strings.TrimSpace(label))
			argN++
		}
		query += ` AND EXISTS (SELECT 1 FROM issue_labels il WHERE il.org_id=i.org_id AND il.issue_id=i.id AND il.label IN (` + strings.Join(placeholders, ",") + `))`
	}

	sortBy := "i.created_at"
	switch input.SortBy {
	case "updated_at":
		sortBy = "i.updated_at"
	case "priority":
		sortBy = "i.priority"
	case "status":
		sortBy = "i.status"
	}
	sortOrder := "DESC"
	if strings.EqualFold(input.SortOrder, "asc") {
		sortOrder = "ASC"
	}
	query += ` ORDER BY ` + sortBy + ` ` + sortOrder

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}
	query += ` LIMIT $` + itoa(argN) + ` OFFSET $` + itoa(argN+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	issues := make([]Issue, 0)
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.ID, &issue.OrgID, &issue.ProjectID, &issue.ReporterID, &issue.AssigneeID, &issue.ParentID, &issue.SprintID, &issue.IssueType,
			&issue.Title, &issue.Description, &issue.Status, &issue.Priority, &issue.CreatedAt, &issue.UpdatedAt); err != nil {
			return nil, err
		}
		labels, err := r.ListLabels(ctx, orgID, issue.ID)
		if err != nil {
			return nil, err
		}
		issue.Labels = labels
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func (r *Repository) Update(ctx context.Context, orgID, issueID uuid.UUID, input UpdateIssueInput) (Issue, error) {
	setParts := []string{"updated_at = NOW()"}
	args := []any{}
	argN := 1

	if input.Title != nil {
		setParts = append(setParts, "title = $"+itoa(argN))
		args = append(args, *input.Title)
		argN++
	}
	if input.Description != nil {
		setParts = append(setParts, "description = $"+itoa(argN))
		args = append(args, *input.Description)
		argN++
	}
	if input.Status != nil {
		setParts = append(setParts, "status = $"+itoa(argN))
		args = append(args, strings.ToLower(*input.Status))
		argN++
	}
	if input.Priority != nil {
		setParts = append(setParts, "priority = $"+itoa(argN))
		args = append(args, strings.ToLower(*input.Priority))
		argN++
	}
	if input.SprintID != nil {
		setParts = append(setParts, "sprint_id = $"+itoa(argN))
		args = append(args, *input.SprintID)
		argN++
	}
	if input.AssigneeID != nil {
		setParts = append(setParts, "assignee_id = $"+itoa(argN))
		args = append(args, *input.AssigneeID)
		argN++
	}
	args = append(args, issueID, orgID)
	query := `UPDATE issues SET ` + strings.Join(setParts, ", ") + ` WHERE id = $` + itoa(argN) + ` AND org_id = $` + itoa(argN+1) + ` AND deleted_at IS NULL`
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return Issue{}, err
	}
	return r.GetByID(ctx, orgID, issueID)
}

func (r *Repository) Delete(ctx context.Context, orgID, issueID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE issues SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND org_id=$2`, issueID, orgID)
	return err
}

func (r *Repository) ProjectBelongsToOrg(ctx context.Context, orgID, projectID uuid.UUID) (bool, error) {
	var found bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND org_id=$2)`, projectID, orgID).Scan(&found)
	return found, err
}

func (r *Repository) SprintBelongsToProject(ctx context.Context, orgID, sprintID, projectID uuid.UUID) (bool, error) {
	var found bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS (
SELECT 1
FROM sprints s
JOIN boards b ON b.id = s.board_id
WHERE s.id=$1 AND s.org_id=$2 AND b.project_id=$3 AND b.org_id=$2
)`, sprintID, orgID, projectID).Scan(&found)
	return found, err
}

func (r *Repository) UserBelongsToOrg(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	var found bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND org_id=$2)`, userID, orgID).Scan(&found)
	return found, err
}

func (r *Repository) AddLink(ctx context.Context, orgID, issueID, relatedIssueID uuid.UUID, relationType string) (IssueLink, error) {
	link := IssueLink{ID: uuid.New(), IssueID: issueID, RelatedIssueID: relatedIssueID, RelationType: relationType}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO issue_links (id, org_id, issue_id, related_issue_id, relation_type)
VALUES ($1,$2,$3,$4,$5)`, link.ID, orgID, issueID, relatedIssueID, relationType)
	return link, err
}

func (r *Repository) ListLinks(ctx context.Context, orgID, issueID uuid.UUID) ([]IssueLink, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, issue_id, related_issue_id, relation_type
FROM issue_links
WHERE org_id=$1 AND (issue_id=$2 OR related_issue_id=$2)
ORDER BY relation_type, id`, orgID, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]IssueLink, 0)
	for rows.Next() {
		var link IssueLink
		if err := rows.Scan(&link.ID, &link.IssueID, &link.RelatedIssueID, &link.RelationType); err != nil {
			return nil, err
		}
		items = append(items, link)
	}
	return items, rows.Err()
}

func (r *Repository) AddLabel(ctx context.Context, orgID, issueID uuid.UUID, label string) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO issue_labels (id, org_id, issue_id, label)
VALUES ($1,$2,$3,$4)
ON CONFLICT (org_id, issue_id, label) DO NOTHING`, uuid.New(), orgID, issueID, strings.TrimSpace(strings.ToLower(label)))
	return err
}

func (r *Repository) RemoveLabel(ctx context.Context, orgID, issueID uuid.UUID, label string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM issue_labels WHERE org_id=$1 AND issue_id=$2 AND label=$3`, orgID, issueID, strings.TrimSpace(strings.ToLower(label)))
	return err
}

func (r *Repository) ListLabels(ctx context.Context, orgID, issueID uuid.UUID) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT label FROM issue_labels WHERE org_id=$1 AND issue_id=$2 ORDER BY label`, orgID, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	labels := make([]string, 0)
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

func (r *Repository) AddComment(ctx context.Context, orgID, issueID, authorID uuid.UUID, body string) (IssueComment, error) {
	now := time.Now().UTC()
	comment := IssueComment{ID: uuid.New(), IssueID: issueID, AuthorID: authorID, Body: body, CreatedAt: now, UpdatedAt: now}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO issue_comments (id, org_id, issue_id, author_id, body, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)`, comment.ID, orgID, issueID, authorID, body, comment.CreatedAt, comment.UpdatedAt)
	return comment, err
}

func (r *Repository) UpdateComment(ctx context.Context, orgID, issueID, commentID uuid.UUID, body string) (IssueComment, error) {
	_, err := r.db.ExecContext(ctx, `
UPDATE issue_comments
SET body=$1, updated_at=NOW()
WHERE id=$2 AND org_id=$3 AND issue_id=$4`, body, commentID, orgID, issueID)
	if err != nil {
		return IssueComment{}, err
	}
	var comment IssueComment
	row := r.db.QueryRowContext(ctx, `
SELECT id, issue_id, author_id, body, created_at, updated_at
FROM issue_comments
WHERE id=$1 AND org_id=$2 AND issue_id=$3`, commentID, orgID, issueID)
	if err := row.Scan(&comment.ID, &comment.IssueID, &comment.AuthorID, &comment.Body, &comment.CreatedAt, &comment.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IssueComment{}, errors.New("comment not found")
		}
		return IssueComment{}, err
	}
	return comment, nil
}

func (r *Repository) DeleteComment(ctx context.Context, orgID, issueID, commentID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM issue_comments WHERE id=$1 AND org_id=$2 AND issue_id=$3`, commentID, orgID, issueID)
	return err
}

func (r *Repository) Assign(ctx context.Context, orgID, issueID, assigneeID uuid.UUID) (Issue, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE issues SET assignee_id=$1, updated_at=NOW() WHERE id=$2 AND org_id=$3 AND deleted_at IS NULL`, assigneeID, issueID, orgID); err != nil {
		return Issue{}, err
	}
	return r.GetByID(ctx, orgID, issueID)
}

func (r *Repository) LogActivity(ctx context.Context, orgID, actorID uuid.UUID, action string, entityID uuid.UUID, metadata map[string]any) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO issue_activities (id, org_id, issue_id, actor_id, action, metadata, created_at)
VALUES ($1,$2,$3,$4,$5,$6::jsonb,NOW())`, uuid.New(), orgID, entityID, actorID, action, toJSON(metadata))
	return err
}

func toJSON(payload map[string]any) string {
	if payload == nil {
		return `{}`
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return `{}`
	}
	return string(out)
}

func itoa(v int) string { return strconv.Itoa(v) }

func NewIssue(orgID, projectID, reporterID uuid.UUID, input CreateIssueInput) Issue {
	now := time.Now().UTC()
	priority := strings.ToLower(input.Priority)
	if priority == "" {
		priority = "medium"
	}
	issueType := strings.ToLower(input.IssueType)
	if issueType == "" {
		issueType = "task"
	}
	return Issue{
		ID:          uuid.New(),
		OrgID:       orgID,
		ProjectID:   projectID,
		ReporterID:  reporterID,
		AssigneeID:  input.AssigneeID,
		ParentID:    input.ParentID,
		SprintID:    input.SprintID,
		IssueType:   issueType,
		Title:       input.Title,
		Description: input.Description,
		Status:      "todo",
		Priority:    priority,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
