package issues

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Issue{}, err
	}
	defer tx.Rollback()
	query := `
INSERT INTO issues (id, org_id, project_id, parent_id, sprint_id, reporter_id, assignee_id, issue_type, title, description, status, priority, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	_, err = tx.ExecContext(ctx, query,
		issue.ID, issue.OrgID, issue.ProjectID, issue.ParentID, issue.SprintID, issue.ReporterID, issue.AssigneeID,
		issue.IssueType, issue.Title, issue.Description, issue.Status, issue.Priority, issue.CreatedAt, issue.UpdatedAt,
	)
	if err != nil {
		return Issue{}, err
	}
	for _, label := range issue.Labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO issue_labels (org_id, issue_id, label) VALUES ($1,$2,$3)`, issue.OrgID, issue.ID, label); err != nil {
			return Issue{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Issue{}, err
	}
	return issue, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, issueID uuid.UUID) (Issue, error) {
	var issue Issue
	row := r.db.QueryRowContext(ctx, `SELECT id, org_id, project_id, parent_id, sprint_id, reporter_id, assignee_id, issue_type, title, description, status, priority, created_at, updated_at
FROM issues WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`, issueID, orgID)
	if err := row.Scan(&issue.ID, &issue.OrgID, &issue.ProjectID, &issue.ParentID, &issue.SprintID, &issue.ReporterID, &issue.AssigneeID, &issue.IssueType,
		&issue.Title, &issue.Description, &issue.Status, &issue.Priority, &issue.CreatedAt, &issue.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Issue{}, errors.New("issue not found")
		}
		return Issue{}, err
	}
	issue.Labels, _ = r.ListLabels(ctx, orgID, issueID)
	return issue, nil
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, filter ListIssuesFilter) ([]Issue, int, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 20
	}
	if filter.SortBy == "" {
		filter.SortBy = "created_at"
	}
	if filter.Order == "" {
		filter.Order = "desc"
	}
	sortBy := map[string]string{
		"created_at": "created_at",
		"updated_at": "updated_at",
		"priority":   "priority",
		"status":     "status",
		"title":      "title",
		"relevance":  "relevance",
	}[strings.ToLower(filter.SortBy)]
	if sortBy == "" {
		sortBy = "created_at"
	}
	order := strings.ToUpper(filter.Order)
	if order != "ASC" {
		order = "DESC"
	}

	where := []string{"i.org_id = $1", "i.deleted_at IS NULL"}
	args := []any{orgID}
	argN := 2
	searchQueryArgIndex := 0
	if filter.ProjectID != uuid.Nil {
		where = append(where, "i.project_id = $"+itoa(argN))
		args = append(args, filter.ProjectID)
		argN++
	}
	if filter.Status != "" {
		where = append(where, "i.status = $"+itoa(argN))
		args = append(args, filter.Status)
		argN++
	}
	if filter.AssigneeID != uuid.Nil {
		where = append(where, "i.assignee_id = $"+itoa(argN))
		args = append(args, filter.AssigneeID)
		argN++
	}
	if filter.SprintID != uuid.Nil {
		where = append(where, "i.sprint_id = $"+itoa(argN))
		args = append(args, filter.SprintID)
		argN++
	}
	if filter.IssueType != "" {
		where = append(where, "i.issue_type = $"+itoa(argN))
		args = append(args, filter.IssueType)
		argN++
	}
	if filter.ParentID != uuid.Nil {
		where = append(where, "i.parent_id = $"+itoa(argN))
		args = append(args, filter.ParentID)
		argN++
	}
	if filter.Label != "" {
		where = append(where, "EXISTS (SELECT 1 FROM issue_labels l WHERE l.org_id=i.org_id AND l.issue_id=i.id AND l.label=$"+itoa(argN)+")")
		args = append(args, filter.Label)
		argN++
	}
	if filter.SearchQuery != "" {
		searchArgN := argN
		searchQueryArgIndex = searchArgN
		where = append(where, "(i.search_vector @@ websearch_to_tsquery('english',$"+itoa(searchArgN)+") OR i.title ILIKE $"+itoa(searchArgN+1)+" ESCAPE '\\' OR i.description ILIKE $"+itoa(searchArgN+1)+" ESCAPE '\\')")
		args = append(args, filter.SearchQuery, escapeLikePattern(filter.SearchQuery)+"%")
		argN += 2
	}
	whereClause := strings.Join(where, " AND ")

	var total int
	countQ := `SELECT COUNT(*) FROM issues i WHERE ` + whereClause
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (filter.Page - 1) * filter.Limit
	orderByClause := "i." + sortBy + " " + order
	if sortBy == "relevance" {
		if filter.SearchQuery == "" {
			orderByClause = "i.created_at DESC"
		} else {
			orderByClause = "ts_rank_cd(i.search_vector, websearch_to_tsquery('english',$" + itoa(searchQueryArgIndex) + ")) " + order + ", i.updated_at DESC"
		}
	}
	query := `SELECT i.id, i.org_id, i.project_id, i.parent_id, i.sprint_id, i.reporter_id, i.assignee_id, i.issue_type, i.title, i.description, i.status, i.priority, i.created_at, i.updated_at
FROM issues i WHERE ` + whereClause + ` ORDER BY ` + orderByClause + ` LIMIT $` + itoa(argN) + ` OFFSET $` + itoa(argN+1)
	args = append(args, filter.Limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	issues := make([]Issue, 0, filter.Limit)
	issueIDs := make([]uuid.UUID, 0, filter.Limit)
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.ID, &issue.OrgID, &issue.ProjectID, &issue.ParentID, &issue.SprintID, &issue.ReporterID, &issue.AssigneeID, &issue.IssueType,
			&issue.Title, &issue.Description, &issue.Status, &issue.Priority, &issue.CreatedAt, &issue.UpdatedAt); err != nil {
			return nil, 0, err
		}
		issues = append(issues, issue)
		issueIDs = append(issueIDs, issue.ID)
	}
	labelsByIssue, err := r.ListLabelsByIssueIDs(ctx, orgID, issueIDs)
	if err != nil {
		return nil, 0, err
	}
	for i := range issues {
		issues[i].Labels = labelsByIssue[issues[i].ID]
	}
	return issues, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, orgID, issueID uuid.UUID, input UpdateIssueInput) (Issue, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Issue{}, err
	}
	defer tx.Rollback()
	setParts := []string{"updated_at = NOW()"}
	args := []any{}
	argN := 1

	if input.ParentID != nil {
		setParts = append(setParts, "parent_id = $"+itoa(argN))
		args = append(args, *input.ParentID)
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
	if input.IssueType != nil {
		setParts = append(setParts, "issue_type = $"+itoa(argN))
		args = append(args, *input.IssueType)
		argN++
	}
	args = append(args, issueID, orgID)
	query := `UPDATE issues SET ` + strings.Join(setParts, ", ") + ` WHERE id = $` + itoa(argN) + ` AND org_id = $` + itoa(argN+1)
	query += ` AND deleted_at IS NULL`
	if input.UpdatedAt != nil {
		args = append(args, *input.UpdatedAt)
		query += ` AND updated_at = $` + itoa(argN+2)
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return Issue{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		if input.UpdatedAt != nil {
			return Issue{}, ErrOptimisticLockConflict
		}
		return Issue{}, errors.New("issue not found")
	}
	if input.Labels != nil {
		if err := r.replaceLabelsTx(ctx, tx, orgID, issueID, input.Labels); err != nil {
			return Issue{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Issue{}, err
	}
	return r.GetByID(ctx, orgID, issueID)
}

func (r *Repository) Delete(ctx context.Context, orgID, issueID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE issues SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`, issueID, orgID)
	return err
}

func itoa(v int) string { return strconv.Itoa(v) }

func escapeLikePattern(v string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return replacer.Replace(v)
}

func NewIssue(orgID, projectID, reporterID uuid.UUID, input CreateIssueInput) Issue {
	now := time.Now().UTC()
	if input.Priority == "" {
		input.Priority = "medium"
	}
	if input.IssueType == "" {
		input.IssueType = IssueTypeTask
	}
	var parentID *uuid.UUID
	if input.ParentID != uuid.Nil {
		parentID = &input.ParentID
	}
	var sprintID *uuid.UUID
	if input.SprintID != uuid.Nil {
		sprintID = &input.SprintID
	}
	var assigneeID *uuid.UUID
	if input.AssigneeID != uuid.Nil {
		assigneeID = &input.AssigneeID
	}
	return Issue{
		ID:          uuid.New(),
		OrgID:       orgID,
		ProjectID:   projectID,
		ParentID:    parentID,
		SprintID:    sprintID,
		ReporterID:  reporterID,
		AssigneeID:  assigneeID,
		IssueType:   strings.ToLower(input.IssueType),
		Title:       input.Title,
		Description: input.Description,
		Status:      "todo",
		Priority:    input.Priority,
		Labels:      input.Labels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (r *Repository) AddRelation(ctx context.Context, orgID, issueID, relatedIssueID uuid.UUID, relationType string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO issue_relations (org_id, issue_id, related_issue_id, relation_type) VALUES ($1,$2,$3,$4)`,
		orgID, issueID, relatedIssueID, relationType)
	return err
}

// HasCircularRelation reports whether adding issueID → relatedIssueID for relationType
// would create a cycle by checking whether issueID is already reachable from relatedIssueID
// through the existing graph of the same relation type.
func (r *Repository) HasCircularRelation(ctx context.Context, orgID, issueID, relatedIssueID uuid.UUID, relationType string) (bool, error) {
	// A → B creates a cycle when B can already reach A through the same relation.
	const query = `
WITH RECURSIVE chain AS (
  SELECT related_issue_id AS id
  FROM issue_relations
  WHERE org_id=$1 AND issue_id=$2 AND relation_type=$3
  UNION ALL
  SELECT r.related_issue_id
  FROM issue_relations r
  JOIN chain c ON c.id = r.issue_id
  WHERE r.org_id=$1 AND r.relation_type=$3
)
SELECT COUNT(*) FROM chain WHERE id=$4`
	var count int
	if err := r.db.QueryRowContext(ctx, query, orgID, relatedIssueID, relationType, issueID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) ListRelations(ctx context.Context, orgID, issueID uuid.UUID) ([]map[string]any, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT related_issue_id, relation_type FROM issue_relations WHERE org_id=$1 AND issue_id=$2`, orgID, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var relatedID uuid.UUID
		var relationType string
		if err := rows.Scan(&relatedID, &relationType); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"related_issue_id": relatedID, "relation_type": relationType})
	}
	return out, rows.Err()
}

func (r *Repository) AddWatcher(ctx context.Context, orgID, issueID, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO issue_watchers (org_id, issue_id, user_id) VALUES ($1,$2,$3) ON CONFLICT (org_id, issue_id, user_id) DO NOTHING`,
		orgID, issueID, userID)
	return err
}

func (r *Repository) RemoveWatcher(ctx context.Context, orgID, issueID, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM issue_watchers WHERE org_id=$1 AND issue_id=$2 AND user_id=$3`, orgID, issueID, userID)
	return err
}

func (r *Repository) ListWatchers(ctx context.Context, orgID, issueID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT user_id FROM issue_watchers WHERE org_id=$1 AND issue_id=$2`, orgID, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func (r *Repository) AddLabel(ctx context.Context, orgID, issueID uuid.UUID, label string) error {
	label = normalizeLabel(label)
	_, err := r.db.ExecContext(ctx, `INSERT INTO issue_labels (org_id, issue_id, label) VALUES ($1,$2,$3) ON CONFLICT (org_id, issue_id, label) DO NOTHING`, orgID, issueID, label)
	return err
}

func (r *Repository) RemoveLabel(ctx context.Context, orgID, issueID uuid.UUID, label string) error {
	label = normalizeLabel(label)
	_, err := r.db.ExecContext(ctx, `DELETE FROM issue_labels WHERE org_id=$1 AND issue_id=$2 AND label=$3`, orgID, issueID, label)
	return err
}

func (r *Repository) ReplaceLabels(ctx context.Context, orgID, issueID uuid.UUID, labels []string) error {
	return r.replaceLabelsTx(ctx, nil, orgID, issueID, labels)
}

func (r *Repository) replaceLabelsTx(ctx context.Context, tx *sql.Tx, orgID, issueID uuid.UUID, labels []string) error {
	execFn := r.db.ExecContext
	if tx != nil {
		execFn = tx.ExecContext
	}
	if _, err := execFn(ctx, `DELETE FROM issue_labels WHERE org_id=$1 AND issue_id=$2`, orgID, issueID); err != nil {
		return err
	}
	valueParts := make([]string, 0, len(labels))
	args := []any{}
	argN := 1
	for _, label := range labels {
		label = normalizeLabel(label)
		if label == "" {
			continue
		}
		valueParts = append(valueParts, "($"+itoa(argN)+",$"+itoa(argN+1)+",$"+itoa(argN+2)+")")
		args = append(args, orgID, issueID, label)
		argN += 3
	}
	if len(valueParts) == 0 {
		return nil
	}
	if _, err := execFn(ctx, `INSERT INTO issue_labels (org_id, issue_id, label) VALUES `+strings.Join(valueParts, ","), args...); err != nil {
		return err
	}
	return nil
}

func (r *Repository) ListLabels(ctx context.Context, orgID, issueID uuid.UUID) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT label FROM issue_labels WHERE org_id=$1 AND issue_id=$2`, orgID, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, err
		}
		result = append(result, label)
	}
	return result, rows.Err()
}

func (r *Repository) CreateComment(ctx context.Context, orgID, issueID, authorID uuid.UUID, body string) (IssueComment, error) {
	comment := IssueComment{ID: uuid.New(), OrgID: orgID, IssueID: issueID, AuthorID: authorID, Body: body, CreatedAt: time.Now().UTC()}
	_, err := r.db.ExecContext(ctx, `INSERT INTO issue_comments (id, org_id, issue_id, author_id, body, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		comment.ID, comment.OrgID, comment.IssueID, comment.AuthorID, comment.Body, comment.CreatedAt)
	return comment, err
}

func (r *Repository) ListComments(ctx context.Context, orgID, issueID uuid.UUID, page, limit int) ([]IssueComment, int, error) {
	page, limit = normalizePageLimit(page, limit)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issue_comments WHERE org_id=$1 AND issue_id=$2`, orgID, issueID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, org_id, issue_id, author_id, body, created_at FROM issue_comments WHERE org_id=$1 AND issue_id=$2 ORDER BY created_at ASC LIMIT $3 OFFSET $4`, orgID, issueID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []IssueComment{}
	for rows.Next() {
		var c IssueComment
		if err := rows.Scan(&c.ID, &c.OrgID, &c.IssueID, &c.AuthorID, &c.Body, &c.CreatedAt); err != nil {
			return nil, 0, err
		}
		result = append(result, c)
	}
	return result, total, rows.Err()
}

func (r *Repository) AddActivity(ctx context.Context, orgID, issueID, actorID uuid.UUID, action, field, fromValue, toValue string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO issue_activities (id, org_id, issue_id, actor_id, action, field, from_value, to_value, created_at) VALUES (gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,NOW())`,
		orgID, issueID, actorID, action, field, fromValue, toValue)
	return err
}

func (r *Repository) ListActivities(ctx context.Context, orgID, issueID uuid.UUID, page, limit int) ([]IssueActivity, int, error) {
	page, limit = normalizePageLimit(page, limit)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issue_activities WHERE org_id=$1 AND issue_id=$2`, orgID, issueID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, org_id, issue_id, actor_id, action, field, from_value, to_value, created_at FROM issue_activities WHERE org_id=$1 AND issue_id=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`, orgID, issueID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []IssueActivity{}
	for rows.Next() {
		var a IssueActivity
		if err := rows.Scan(&a.ID, &a.OrgID, &a.IssueID, &a.ActorID, &a.Action, &a.Field, &a.FromValue, &a.ToValue, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		result = append(result, a)
	}
	return result, total, rows.Err()
}

func (r *Repository) ProjectExists(ctx context.Context, orgID, projectID uuid.UUID) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id=$1 AND org_id=$2`, projectID, orgID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) IssueBelongsToProject(ctx context.Context, orgID, issueID, projectID uuid.UUID) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE id=$1 AND org_id=$2 AND project_id=$3 AND deleted_at IS NULL`, issueID, orgID, projectID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) SprintBelongsToProject(ctx context.Context, orgID, sprintID, projectID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sprints s JOIN boards b ON b.id=s.board_id WHERE s.id=$1 AND s.org_id=$2 AND b.project_id=$3 AND b.org_id=$2`, sprintID, orgID, projectID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) IsValidTransition(ctx context.Context, orgID, projectID uuid.UUID, fromStatus, toStatus string) (bool, error) {
	rule, err := r.GetTransitionRule(ctx, orgID, projectID, fromStatus, toStatus)
	return rule.Allowed, err
}

func (r *Repository) GetTransitionRule(ctx context.Context, orgID, projectID uuid.UUID, fromStatus, toStatus string) (WorkflowTransitionRule, error) {
	if strings.EqualFold(fromStatus, toStatus) {
		r := defaultRule()
		r.Allowed = true
		return r, nil
	}
	var (
		conditionsRaw []byte
		validatorsRaw []byte
		postRaw       []byte
	)
	rule := defaultRule()
	err := r.db.QueryRowContext(ctx, `SELECT conditions, validators, post_functions FROM workflow_transitions WHERE org_id=$1 AND project_id=$2 AND from_status=$3 AND to_status=$4 LIMIT 1`,
		orgID, projectID, fromStatus, toStatus).Scan(&conditionsRaw, &validatorsRaw, &postRaw)
	if err == nil {
		rule.Allowed = true
		rule.HasCustomRule = true
		if rule.Conditions, err = decodeRuleJSON(conditionsRaw); err != nil {
			return defaultRule(), fmt.Errorf("decode workflow transition conditions: %w", err)
		}
		if rule.Validators, err = decodeRuleJSON(validatorsRaw); err != nil {
			return defaultRule(), fmt.Errorf("decode workflow transition validators: %w", err)
		}
		if rule.PostFunctions, err = decodeRuleJSON(postRaw); err != nil {
			return defaultRule(), fmt.Errorf("decode workflow transition post_functions: %w", err)
		}
		return rule, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return defaultRule(), err
	}
	for _, allowed := range DefaultWorkflowTransitions[fromStatus] {
		if allowed == toStatus {
			rule.Allowed = true
			return rule, nil
		}
	}
	return rule, nil
}

func (r *Repository) GetIssueProjectID(ctx context.Context, orgID, issueID uuid.UUID) (uuid.UUID, error) {
	var projectID uuid.UUID
	err := r.db.QueryRowContext(ctx, `SELECT project_id FROM issues WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`, issueID, orgID).Scan(&projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, errors.New("issue not found")
		}
		return uuid.Nil, err
	}
	return projectID, nil
}

func (r *Repository) ListLabelsByIssueIDs(ctx context.Context, orgID uuid.UUID, issueIDs []uuid.UUID) (map[uuid.UUID][]string, error) {
	result := make(map[uuid.UUID][]string, len(issueIDs))
	if len(issueIDs) == 0 {
		return result, nil
	}
	args := []any{orgID}
	placeholders := make([]string, 0, len(issueIDs))
	for i, issueID := range issueIDs {
		args = append(args, issueID)
		placeholders = append(placeholders, "$"+itoa(i+2))
	}
	query := fmt.Sprintf(`SELECT issue_id, label FROM issue_labels WHERE org_id=$1 AND issue_id IN (%s)`, strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var issueID uuid.UUID
		var label string
		if err := rows.Scan(&issueID, &label); err != nil {
			return nil, err
		}
		result[issueID] = append(result[issueID], label)
	}
	return result, rows.Err()
}

func normalizeLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

func normalizePageLimit(page, limit int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return page, limit
}
