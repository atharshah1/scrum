package issues

import (
	"context"
	"database/sql"
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
INSERT INTO issues (id, org_id, project_id, reporter_id, assignee_id, title, description, status, priority, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	_, err := r.db.ExecContext(ctx, query,
		issue.ID, issue.OrgID, issue.ProjectID, issue.ReporterID, issue.AssigneeID,
		issue.Title, issue.Description, issue.Status, issue.Priority, issue.CreatedAt, issue.UpdatedAt,
	)
	return issue, err
}

func (r *Repository) GetByID(ctx context.Context, orgID, issueID uuid.UUID) (Issue, error) {
	var issue Issue
	row := r.db.QueryRowContext(ctx, `SELECT id, org_id, project_id, reporter_id, assignee_id, title, description, status, priority, created_at, updated_at
FROM issues WHERE id=$1 AND org_id=$2`, issueID, orgID)
	if err := row.Scan(&issue.ID, &issue.OrgID, &issue.ProjectID, &issue.ReporterID, &issue.AssigneeID,
		&issue.Title, &issue.Description, &issue.Status, &issue.Priority, &issue.CreatedAt, &issue.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Issue{}, errors.New("issue not found")
		}
		return Issue{}, err
	}
	return issue, nil
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID) ([]Issue, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, org_id, project_id, reporter_id, assignee_id, title, description, status, priority, created_at, updated_at
FROM issues WHERE org_id=$1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	issues := make([]Issue, 0)
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.ID, &issue.OrgID, &issue.ProjectID, &issue.ReporterID, &issue.AssigneeID,
			&issue.Title, &issue.Description, &issue.Status, &issue.Priority, &issue.CreatedAt, &issue.UpdatedAt); err != nil {
			return nil, err
		}
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
		args = append(args, *input.Status)
		argN++
	}
	if input.Priority != nil {
		setParts = append(setParts, "priority = $"+itoa(argN))
		args = append(args, *input.Priority)
		argN++
	}
	args = append(args, issueID, orgID)
	query := `UPDATE issues SET ` + strings.Join(setParts, ", ") + ` WHERE id = $` + itoa(argN) + ` AND org_id = $` + itoa(argN+1)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return Issue{}, err
	}
	return r.GetByID(ctx, orgID, issueID)
}

func (r *Repository) Delete(ctx context.Context, orgID, issueID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM issues WHERE id=$1 AND org_id=$2`, issueID, orgID)
	return err
}

func itoa(v int) string { return strconv.Itoa(v) }

func NewIssue(orgID, projectID, reporterID uuid.UUID, title, description, priority string) Issue {
	now := time.Now().UTC()
	if priority == "" {
		priority = "medium"
	}
	return Issue{
		ID:          uuid.New(),
		OrgID:       orgID,
		ProjectID:   projectID,
		ReporterID:  reporterID,
		Title:       title,
		Description: description,
		Status:      "todo",
		Priority:    priority,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
