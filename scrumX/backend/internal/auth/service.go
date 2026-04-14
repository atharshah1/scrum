package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	OrgID        uuid.UUID `json:"org_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
}

type Service struct {
	db            *sql.DB
	accessSecret  string
	refreshSecret string
}

func NewService(db *sql.DB, accessSecret, refreshSecret string) *Service {
	return &Service{db: db, accessSecret: accessSecret, refreshSecret: refreshSecret}
}

func (s *Service) Register(ctx context.Context, email, password string) (User, TokenPair, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" || !strings.Contains(email, "@") {
		return User{}, TokenPair{}, errors.New("valid email and password are required")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	var existing int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email=$1`, email).Scan(&existing); err != nil {
		return User{}, TokenPair{}, err
	}
	if existing > 0 {
		return User{}, TokenPair{}, errors.New("email already registered")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	defer tx.Rollback()

	orgID := uuid.New()
	userID := uuid.New()
	localPart, _, _ := strings.Cut(email, "@")
	orgSlug := fmt.Sprintf("%s-%s", slugify(localPart), orgID.String()[:8])
	if _, err = tx.ExecContext(ctx, `INSERT INTO organizations (id, name, slug) VALUES ($1,$2,$3)`, orgID, orgSlug, orgSlug); err != nil {
		return User{}, TokenPair{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO users (id, org_id, email, password_hash) VALUES ($1,$2,$3,$4)`, userID, orgID, email, string(passwordHash)); err != nil {
		return User{}, TokenPair{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO memberships (id, org_id, user_id, role) VALUES ($1,$2,$3,'Admin')`, uuid.New(), orgID, userID); err != nil {
		return User{}, TokenPair{}, err
	}

	user := User{ID: userID, OrgID: orgID, Email: email, PasswordHash: string(passwordHash), Role: "Admin"}
	tokens, err := GenerateTokens(user.ID, user.OrgID, user.Role, s.accessSecret, s.refreshSecret)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	if err := s.persistRefreshToken(ctx, tx, user.OrgID, user.ID, tokens); err != nil {
		return User{}, TokenPair{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, TokenPair{}, err
	}
	return user, tokens, err
}

func (s *Service) Login(ctx context.Context, email, password string) (User, TokenPair, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" || !strings.Contains(email, "@") {
		return User{}, TokenPair{}, errors.New("invalid credentials")
	}
	row := s.db.QueryRowContext(ctx, `SELECT u.id, u.org_id, u.email, u.password_hash, m.role
FROM users u
JOIN memberships m ON m.org_id=u.org_id AND m.user_id=u.id
WHERE u.email=$1
ORDER BY u.created_at DESC
LIMIT 1`, email)
	var user User
	if err := row.Scan(&user.ID, &user.OrgID, &user.Email, &user.PasswordHash, &user.Role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, TokenPair{}, errors.New("invalid credentials")
		}
		return User{}, TokenPair{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return User{}, TokenPair{}, errors.New("invalid credentials")
	}
	tokens, err := GenerateTokens(user.ID, user.OrgID, user.Role, s.accessSecret, s.refreshSecret)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	if err := s.persistRefreshToken(ctx, nil, user.OrgID, user.ID, tokens); err != nil {
		return User{}, TokenPair{}, err
	}
	return user, tokens, nil
}

func (s *Service) GetByID(ctx context.Context, userID uuid.UUID) (User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT u.id, u.org_id, u.email, u.password_hash, m.role
FROM users u
JOIN memberships m ON m.org_id=u.org_id AND m.user_id=u.id
WHERE u.id=$1
LIMIT 1`, userID)
	var user User
	if err := row.Scan(&user.ID, &user.OrgID, &user.Email, &user.PasswordHash, &user.Role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, errors.New("user not found")
		}
		return User{}, err
	}
	return user, nil
}

func (s *Service) Refresh(ctx context.Context, userID, orgID uuid.UUID, role, tokenID string) (TokenPair, error) {
	if err := s.validateAndRotateRefreshToken(ctx, userID, orgID, tokenID); err != nil {
		return TokenPair{}, err
	}
	tokens, err := GenerateTokens(userID, orgID, role, s.accessSecret, s.refreshSecret)
	if err != nil {
		return TokenPair{}, err
	}
	if err := s.persistRefreshToken(ctx, nil, orgID, userID, tokens); err != nil {
		return TokenPair{}, err
	}
	return tokens, nil
}

func (s *Service) persistRefreshToken(ctx context.Context, tx *sql.Tx, orgID, userID uuid.UUID, tokens TokenPair) error {
	tokenID, exp, err := ParseRefreshTokenMeta(tokens.RefreshToken, s.refreshSecret)
	if err != nil {
		return err
	}
	execFn := s.db.ExecContext
	if tx != nil {
		execFn = tx.ExecContext
	}
	_, err = execFn(ctx, `INSERT INTO auth_refresh_tokens (id, org_id, user_id, token_id, expires_at) VALUES ($1,$2,$3,$4,$5)`,
		uuid.New(), orgID, userID, tokenID, time.Unix(exp, 0).UTC())
	return err
}

func (s *Service) validateAndRotateRefreshToken(ctx context.Context, userID, orgID uuid.UUID, tokenID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_refresh_tokens
WHERE org_id=$1 AND user_id=$2 AND token_id=$3 AND revoked_at IS NULL AND expires_at > NOW()`,
		orgID, userID, tokenID).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return errors.New("invalid refresh token")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_refresh_tokens SET revoked_at=NOW()
WHERE org_id=$1 AND user_id=$2 AND token_id=$3 AND revoked_at IS NULL`, orgID, userID, tokenID); err != nil {
		return err
	}
	return tx.Commit()
}

// slugify normalizes a string into a lowercase slug, collapsing non-alphanumeric
// runs into single dashes and falling back to "workspace" when nothing usable remains.
func slugify(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return "workspace"
	}
	var b strings.Builder
	lastDash := false
	for _, ch := range v {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "workspace"
	}
	return slug
}
