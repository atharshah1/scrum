package notifications

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Notification represents a single in-app notification for a user.
type Notification struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	UserID    uuid.UUID `json:"user_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	EntityID  uuid.UUID `json:"entity_id,omitempty"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository handles persistence of notifications.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// Create inserts a new notification record.
func (r *Repository) Create(ctx context.Context, n Notification) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_notifications (id, org_id, user_id, type, title, message, entity_id, is_read, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,FALSE,$8)`,
		n.ID, n.OrgID, n.UserID, n.Type, n.Title, n.Message, nullableUUID(n.EntityID), time.Now().UTC())
	return err
}

// List returns the latest notifications for a user. Unread ones first, then by creation time desc.
func (r *Repository) List(ctx context.Context, orgID, userID uuid.UUID, page, limit int) ([]Notification, int, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_notifications WHERE org_id=$1 AND user_id=$2`,
		orgID, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, org_id, user_id, type, title, message, entity_id, is_read, created_at
		 FROM user_notifications
		 WHERE org_id=$1 AND user_id=$2
		 ORDER BY is_read ASC, created_at DESC
		 LIMIT $3 OFFSET $4`,
		orgID, userID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Notification, 0, limit)
	for rows.Next() {
		var n Notification
		var entityID *uuid.UUID
		if err := rows.Scan(&n.ID, &n.OrgID, &n.UserID, &n.Type, &n.Title, &n.Message, &entityID, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, 0, err
		}
		if entityID != nil {
			n.EntityID = *entityID
		}
		out = append(out, n)
	}
	return out, total, rows.Err()
}

// MarkRead marks a single notification as read for a given user.
func (r *Repository) MarkRead(ctx context.Context, orgID, userID, notifID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE user_notifications SET is_read=TRUE WHERE id=$1 AND org_id=$2 AND user_id=$3`,
		notifID, orgID, userID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return errors.New("notification not found")
	}
	return nil
}

// MarkAllRead marks all unread notifications for a user as read.
func (r *Repository) MarkAllRead(ctx context.Context, orgID, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE user_notifications SET is_read=TRUE WHERE org_id=$1 AND user_id=$2 AND is_read=FALSE`,
		orgID, userID)
	return err
}

// ListIssueWatchers returns all watcher user IDs for an issue.
func (r *Repository) ListIssueWatchers(ctx context.Context, orgID, issueID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT user_id FROM issue_watchers WHERE org_id=$1 AND issue_id=$2`, orgID, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// GetIssueAssignee returns the current assignee ID for an issue.
func (r *Repository) GetIssueAssignee(ctx context.Context, orgID, issueID uuid.UUID) (uuid.UUID, bool, error) {
	var assigneeID *uuid.UUID
	if err := r.db.QueryRowContext(ctx, `SELECT assignee_id FROM issues WHERE org_id=$1 AND id=$2`, orgID, issueID).Scan(&assigneeID); err != nil {
		return uuid.Nil, false, err
	}
	if assigneeID == nil {
		return uuid.Nil, false, nil
	}
	return *assigneeID, true, nil
}

// UserExistsInOrg checks if a user belongs to an organization.
func (r *Repository) UserExistsInOrg(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users WHERE org_id=$1 AND id=$2`, orgID, userID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
