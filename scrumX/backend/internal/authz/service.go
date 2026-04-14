package authz

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) RequireOrgMember(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx, `SELECT role FROM memberships WHERE org_id=$1 AND user_id=$2 LIMIT 1`, orgID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("user is not a member of org")
		}
		return "", err
	}
	return role, nil
}

func (s *Service) ResolveProjectRole(ctx context.Context, orgID, userID, projectID uuid.UUID) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx, `SELECT role FROM project_memberships WHERE org_id=$1 AND project_id=$2 AND user_id=$3 LIMIT 1`,
		orgID, projectID, userID).Scan(&role)
	if err == nil {
		return role, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	return s.RequireOrgMember(ctx, orgID, userID)
}

func CanWrite(role string) bool {
	return role == "Admin" || role == "Member"
}

