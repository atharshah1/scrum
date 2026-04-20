package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	authorizationCodeTTL = 60 * time.Second
	refreshTokenTTL      = 7 * 24 * time.Hour
	accessTokenTTL       = time.Hour
	authorizeSessionTTL  = 10 * time.Minute
)

type Service struct {
	db            *sql.DB
	accessSecret  string
	refreshSecret string
}

type Client struct {
	ID               uuid.UUID
	ClientID         string
	ClientSecret     string
	ClientSecretHash string
	Name             string
	RedirectURIs     []string
	Scopes           []string
	OwnerOrgID       *uuid.UUID
	IsConfidential   bool
}

type AuthorizeRequest struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type AuthorizationCodeGrant struct {
	GrantType    string `json:"grant_type" form:"grant_type"`
	Code         string `json:"code" form:"code"`
	RedirectURI  string `json:"redirect_uri" form:"redirect_uri"`
	ClientID     string `json:"client_id" form:"client_id"`
	ClientSecret string `json:"client_secret" form:"client_secret"`
	CodeVerifier string `json:"code_verifier" form:"code_verifier"`
}

type RefreshTokenGrant struct {
	GrantType    string `json:"grant_type" form:"grant_type"`
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
	ClientID     string `json:"client_id" form:"client_id"`
	ClientSecret string `json:"client_secret" form:"client_secret"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
}

type Organization struct {
	ID   uuid.UUID
	Name string
	Role string
}

type authorizeSessionClaims struct {
	UserID              string   `json:"user_id"`
	ClientID            string   `json:"client_id"`
	RedirectURI         string   `json:"redirect_uri"`
	State               string   `json:"state"`
	Scopes              []string `json:"scopes"`
	CodeChallenge       string   `json:"code_challenge"`
	CodeChallengeMethod string   `json:"code_challenge_method"`
}

type accessTokenClaims struct {
	Scopes   []string `json:"scopes"`
	ClientID string   `json:"client_id"`
	OrgID    string   `json:"org_id"`
	Role     string   `json:"role"`
	Type     string   `json:"type"`
	jwt.RegisteredClaims
}

func NewService(db *sql.DB, accessSecret, refreshSecret string) *Service {
	return &Service{db: db, accessSecret: accessSecret, refreshSecret: refreshSecret}
}

func (s *Service) ValidateAuthorizeRequest(req AuthorizeRequest) (Client, []string, error) {
	req.ClientID = strings.TrimSpace(req.ClientID)
	req.RedirectURI = strings.TrimSpace(req.RedirectURI)
	req.ResponseType = strings.TrimSpace(req.ResponseType)
	req.State = strings.TrimSpace(req.State)
	req.CodeChallenge = strings.TrimSpace(req.CodeChallenge)
	req.CodeChallengeMethod = strings.TrimSpace(req.CodeChallengeMethod)
	if req.ClientID == "" || req.RedirectURI == "" || req.ResponseType != "code" {
		return Client{}, nil, errors.New("invalid authorize request")
	}
	if req.State == "" {
		return Client{}, nil, errors.New("state is required")
	}
	client, err := s.GetClient(context.Background(), req.ClientID)
	if err != nil {
		return Client{}, nil, err
	}
	if !containsExact(client.RedirectURIs, req.RedirectURI) {
		return Client{}, nil, errors.New("redirect_uri must exactly match a registered redirect")
	}
	if err := validateRedirectURI(req.RedirectURI); err != nil {
		return Client{}, nil, err
	}
	if !client.IsConfidential {
		if req.CodeChallenge == "" {
			return Client{}, nil, errors.New("code_challenge is required for public clients")
		}
		if !strings.EqualFold(req.CodeChallengeMethod, "S256") {
			return Client{}, nil, errors.New("code_challenge_method must be S256")
		}
	}
	scopes, err := normalizeRequestedScopes(req.Scope, client.Scopes)
	if err != nil {
		return Client{}, nil, err
	}
	return client, scopes, nil
}

func (s *Service) GetClient(ctx context.Context, clientID string) (Client, error) {
	var client Client
	var redirectJSON, scopeJSON []byte
	var ownerOrg sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, client_id, COALESCE(client_secret, ''), COALESCE(client_secret_hash, ''), name, redirect_uris, scopes, owner_org_id::text, is_confidential
FROM oauth_clients WHERE client_id=$1 LIMIT 1`, strings.TrimSpace(clientID)).Scan(
		&client.ID,
		&client.ClientID,
		&client.ClientSecret,
		&client.ClientSecretHash,
		&client.Name,
		&redirectJSON,
		&scopeJSON,
		&ownerOrg,
		&client.IsConfidential,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Client{}, errors.New("unknown client_id")
		}
		return Client{}, err
	}
	if err := json.Unmarshal(redirectJSON, &client.RedirectURIs); err != nil {
		return Client{}, err
	}
	if err := json.Unmarshal(scopeJSON, &client.Scopes); err != nil {
		return Client{}, err
	}
	if ownerOrg.Valid {
		if parsed, parseErr := uuid.Parse(ownerOrg.String); parseErr == nil {
			client.OwnerOrgID = &parsed
		}
	}
	return client, nil
}

func (s *Service) AuthenticateUser(ctx context.Context, email, password string) (uuid.UUID, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" || !strings.Contains(email, "@") {
		return uuid.Nil, "", errors.New("invalid credentials")
	}
	var userID uuid.UUID
	var storedEmail, passwordHash string
	if err := s.db.QueryRowContext(ctx, `SELECT id, email, COALESCE(password_hash, '')
FROM users WHERE LOWER(email)=LOWER($1)
ORDER BY created_at DESC LIMIT 1`, email).Scan(&userID, &storedEmail, &passwordHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, "", errors.New("invalid credentials")
		}
		return uuid.Nil, "", err
	}
	if passwordHash == "" || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return uuid.Nil, "", errors.New("invalid credentials")
	}
	return userID, storedEmail, nil
}

func (s *Service) ListUserOrganizations(ctx context.Context, userID uuid.UUID) ([]Organization, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT o.id, o.name, m.role
FROM memberships m
JOIN organizations o ON o.id=m.org_id
WHERE m.user_id=$1
ORDER BY o.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orgs := make([]Organization, 0, 4)
	for rows.Next() {
		var org Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.Role); err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}
	return orgs, rows.Err()
}

func (s *Service) ResolveUserOrg(ctx context.Context, userID, orgID uuid.UUID) (Organization, error) {
	var org Organization
	err := s.db.QueryRowContext(ctx, `SELECT o.id, o.name, m.role
FROM memberships m
JOIN organizations o ON o.id=m.org_id
WHERE m.user_id=$1 AND m.org_id=$2
LIMIT 1`, userID, orgID).Scan(&org.ID, &org.Name, &org.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Organization{}, errors.New("user is not a member of the selected organization")
		}
		return Organization{}, err
	}
	return org, nil
}

func (s *Service) CreateAuthorizeSession(ctx context.Context, userID uuid.UUID, req AuthorizeRequest, scopes []string) (string, error) {
	sessionToken, err := randomToken(32)
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO oauth_sessions (id, session_token_hash, user_id, client_id, redirect_uri, state, scopes, code_challenge, code_challenge_method, expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		uuid.New(),
		hashToken(sessionToken),
		userID,
		strings.TrimSpace(req.ClientID),
		strings.TrimSpace(req.RedirectURI),
		strings.TrimSpace(req.State),
		mustJSON(scopes),
		strings.TrimSpace(req.CodeChallenge),
		strings.TrimSpace(req.CodeChallengeMethod),
		time.Now().UTC().Add(authorizeSessionTTL),
	)
	if err != nil {
		return "", err
	}
	return sessionToken, nil
}

func (s *Service) consumeAuthorizeSession(ctx context.Context, sessionToken string, approved bool) (authorizeSessionClaims, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return authorizeSessionClaims{}, err
	}
	defer tx.Rollback()
	claims := authorizeSessionClaims{}
	var scopeJSON []byte
	err = tx.QueryRowContext(ctx, `SELECT user_id::text, client_id, redirect_uri, state, scopes, code_challenge, code_challenge_method
FROM oauth_sessions
WHERE session_token_hash=$1 AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at > NOW()
LIMIT 1 FOR UPDATE`, hashToken(sessionToken)).Scan(
		&claims.UserID,
		&claims.ClientID,
		&claims.RedirectURI,
		&claims.State,
		&scopeJSON,
		&claims.CodeChallenge,
		&claims.CodeChallengeMethod,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return authorizeSessionClaims{}, errors.New("invalid authorization session")
		}
		return authorizeSessionClaims{}, err
	}
	if err := json.Unmarshal(scopeJSON, &claims.Scopes); err != nil {
		return authorizeSessionClaims{}, err
	}
	if approved {
		_, err = tx.ExecContext(ctx, `UPDATE oauth_sessions SET consumed_at=NOW() WHERE session_token_hash=$1`, hashToken(sessionToken))
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE oauth_sessions SET consumed_at=NOW(), revoked_at=NOW() WHERE session_token_hash=$1`, hashToken(sessionToken))
	}
	if err != nil {
		return authorizeSessionClaims{}, err
	}
	if err := tx.Commit(); err != nil {
		return authorizeSessionClaims{}, err
	}
	return claims, nil
}

func (s *Service) CreateAuthorizationCode(ctx context.Context, userID, orgID uuid.UUID, req authorizeSessionClaims) (string, error) {
	code, err := randomToken(32)
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO oauth_codes (id, code_hash, client_id, user_id, org_id, scopes, redirect_uri, code_challenge, code_challenge_method, expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		uuid.New(),
		hashToken(code),
		req.ClientID,
		userID,
		orgID,
		mustJSON(req.Scopes),
		req.RedirectURI,
		req.CodeChallenge,
		req.CodeChallengeMethod,
		time.Now().UTC().Add(authorizationCodeTTL),
	)
	if err != nil {
		return "", err
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO oauth_authorizations (id, user_id, client_id, org_id, scopes)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (user_id, client_id, org_id)
DO UPDATE SET scopes=EXCLUDED.scopes, updated_at=NOW()`, uuid.New(), userID, req.ClientID, orgID, mustJSON(req.Scopes))
	s.writeAuditLog(ctx, orgID, &userID, "oauth.authorization.approved", "oauth_client", nil, map[string]any{
		"client_id": req.ClientID,
		"scopes":    req.Scopes,
	})
	return code, nil
}

func (s *Service) ExchangeAuthorizationCode(ctx context.Context, grant AuthorizationCodeGrant) (TokenResponse, error) {
	client, err := s.validateClientCredentials(ctx, grant.ClientID, grant.ClientSecret)
	if err != nil {
		return TokenResponse{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TokenResponse{}, err
	}
	defer tx.Rollback()

	var (
		userID              uuid.UUID
		orgID               uuid.UUID
		storedRedirectURI   string
		codeChallenge       string
		codeChallengeMethod string
		scopeJSON           []byte
	)
	err = tx.QueryRowContext(ctx, `SELECT user_id, org_id, redirect_uri, code_challenge, code_challenge_method, scopes
FROM oauth_codes
WHERE code_hash=$1 AND client_id=$2 AND consumed_at IS NULL AND expires_at > NOW()
LIMIT 1 FOR UPDATE`, hashToken(grant.Code), client.ClientID).Scan(&userID, &orgID, &storedRedirectURI, &codeChallenge, &codeChallengeMethod, &scopeJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TokenResponse{}, errors.New("invalid authorization code")
		}
		return TokenResponse{}, err
	}
	if strings.TrimSpace(grant.RedirectURI) != storedRedirectURI {
		return TokenResponse{}, errors.New("redirect_uri mismatch")
	}
	if !client.IsConfidential {
		if err := validatePKCE(grant.CodeVerifier, codeChallenge, codeChallengeMethod); err != nil {
			return TokenResponse{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_codes SET consumed_at=NOW() WHERE code_hash=$1`, hashToken(grant.Code)); err != nil {
		return TokenResponse{}, err
	}
	role, err := s.lookupMembershipRole(ctx, userID, orgID)
	if err != nil {
		return TokenResponse{}, err
	}
	var scopes []string
	if err := json.Unmarshal(scopeJSON, &scopes); err != nil {
		return TokenResponse{}, err
	}
	response, err := s.issueTokenPair(ctx, tx, client.ClientID, userID, orgID, role, scopes)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return TokenResponse{}, err
	}
	return response, nil
}

func (s *Service) RefreshAccessToken(ctx context.Context, grant RefreshTokenGrant) (TokenResponse, error) {
	client, err := s.validateClientCredentials(ctx, grant.ClientID, grant.ClientSecret)
	if err != nil {
		return TokenResponse{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TokenResponse{}, err
	}
	defer tx.Rollback()

	var (
		userID    uuid.UUID
		orgID     uuid.UUID
		scopeJSON []byte
	)
	err = tx.QueryRowContext(ctx, `SELECT user_id, org_id, scopes
FROM oauth_refresh_tokens
WHERE token_hash=$1 AND client_id=$2 AND revoked_at IS NULL AND expires_at > NOW()
LIMIT 1 FOR UPDATE`, hashToken(grant.RefreshToken), client.ClientID).Scan(&userID, &orgID, &scopeJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TokenResponse{}, errors.New("invalid refresh token")
		}
		return TokenResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_refresh_tokens SET revoked_at=NOW() WHERE token_hash=$1`, hashToken(grant.RefreshToken)); err != nil {
		return TokenResponse{}, err
	}
	role, err := s.lookupMembershipRole(ctx, userID, orgID)
	if err != nil {
		return TokenResponse{}, err
	}
	var scopes []string
	if err := json.Unmarshal(scopeJSON, &scopes); err != nil {
		return TokenResponse{}, err
	}
	response, err := s.issueTokenPair(ctx, tx, client.ClientID, userID, orgID, role, scopes)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return TokenResponse{}, err
	}
	return response, nil
}

func (s *Service) RevokeToken(ctx context.Context, clientID, clientSecret, token string) error {
	client, err := s.validateClientCredentials(ctx, clientID, clientSecret)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE oauth_refresh_tokens SET revoked_at=NOW()
WHERE token_hash=$1 AND client_id=$2 AND revoked_at IS NULL`, hashToken(token), client.ClientID)
	return err
}

func (s *Service) IntrospectToken(ctx context.Context, clientID, clientSecret, token string) (map[string]any, error) {
	if strings.TrimSpace(clientID) != "" {
		if _, err := s.validateClientCredentials(ctx, clientID, clientSecret); err != nil {
			return nil, err
		}
	}
	claims, err := s.parseAccessToken(strings.TrimSpace(token))
	if err != nil {
		return map[string]any{"active": false}, nil
	}
	return map[string]any{
		"active":     true,
		"client_id":  claims.ClientID,
		"scope":      strings.Join(claims.Scopes, " "),
		"sub":        claims.Subject,
		"org_id":     claims.OrgID,
		"role":       claims.Role,
		"token_type": claims.Type,
		"exp":        claims.ExpiresAt.Unix(),
		"iat":        claims.IssuedAt.Unix(),
	}, nil
}

func (s *Service) UserInfo(ctx context.Context, accessToken string) (map[string]any, error) {
	claims, err := s.parseAccessToken(accessToken)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, errors.New("invalid subject claim")
	}
	orgID, err := uuid.Parse(claims.OrgID)
	if err != nil {
		return nil, errors.New("invalid org claim")
	}
	var email string
	if err := s.db.QueryRowContext(ctx, `SELECT email FROM users WHERE id=$1 LIMIT 1`, userID).Scan(&email); err != nil {
		return nil, err
	}
	return map[string]any{
		"sub":       userID.String(),
		"email":     email,
		"org_id":    orgID.String(),
		"role":      claims.Role,
		"client_id": claims.ClientID,
		"scopes":    claims.Scopes,
	}, nil
}

func (s *Service) issueTokenPair(ctx context.Context, tx *sql.Tx, clientID string, userID, orgID uuid.UUID, role string, scopes []string) (TokenResponse, error) {
	accessToken, err := s.generateAccessToken(clientID, userID, orgID, role, scopes)
	if err != nil {
		return TokenResponse{}, err
	}
	refreshToken, err := randomToken(48)
	if err != nil {
		return TokenResponse{}, err
	}
	execer := s.db.ExecContext
	if tx != nil {
		execer = tx.ExecContext
	}
	if _, err := execer(ctx, `INSERT INTO oauth_refresh_tokens (id, token_hash, user_id, client_id, org_id, scopes, expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)`, uuid.New(), hashToken(refreshToken), userID, clientID, orgID, mustJSON(scopes), time.Now().UTC().Add(refreshTokenTTL)); err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
		Scope:        strings.Join(scopes, " "),
	}, nil
}

func (s *Service) generateAccessToken(clientID string, userID, orgID uuid.UUID, role string, scopes []string) (string, error) {
	now := time.Now().UTC()
	claims := accessTokenClaims{
		Scopes:   scopes,
		ClientID: clientID,
		OrgID:    orgID.String(),
		Role:     role,
		Type:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.accessSecret))
}

func (s *Service) parseAccessToken(accessToken string) (accessTokenClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	claims := accessTokenClaims{}
	token, err := parser.ParseWithClaims(strings.TrimSpace(accessToken), &claims, func(token *jwt.Token) (any, error) {
		return []byte(s.accessSecret), nil
	})
	if err != nil || !token.Valid {
		return accessTokenClaims{}, errors.New("invalid access token")
	}
	if strings.TrimSpace(claims.Type) != "access" {
		return accessTokenClaims{}, errors.New("invalid token type")
	}
	return claims, nil
}

func (s *Service) validateClientCredentials(ctx context.Context, clientID, clientSecret string) (Client, error) {
	client, err := s.GetClient(ctx, clientID)
	if err != nil {
		return Client{}, err
	}
	if client.IsConfidential {
		if !matchesClientSecret(client, clientSecret) {
			return Client{}, errors.New("invalid client credentials")
		}
	}
	return client, nil
}

func (s *Service) lookupMembershipRole(ctx context.Context, userID, orgID uuid.UUID) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx, `SELECT role FROM memberships WHERE user_id=$1 AND org_id=$2 LIMIT 1`, userID, orgID).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("membership not found")
		}
		return "", err
	}
	return role, nil
}

func (s *Service) writeAuditLog(ctx context.Context, orgID uuid.UUID, actorID *uuid.UUID, action, entityType string, entityID *uuid.UUID, afterValues map[string]any) {
	_, _ = s.db.ExecContext(ctx, `INSERT INTO audit_logs (id, org_id, actor_id, action, entity_type, entity_id, after_values)
VALUES ($1,$2,$3,$4,$5,$6,$7)`, uuid.New(), orgID, actorID, action, entityType, entityID, mustJSON(afterValues))
}

func validateRedirectURI(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("invalid redirect_uri")
	}
	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme != "https" && host != "localhost" && host != "127.0.0.1" {
		return errors.New("redirect_uri must use https unless it is a loopback redirect")
	}
	return nil
}

func normalizeRequestedScopes(scopeText string, allowed []string) ([]string, error) {
	allowedSet := map[string]struct{}{}
	for _, scope := range allowed {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		allowedSet[scope] = struct{}{}
	}
	requested := strings.Fields(strings.TrimSpace(scopeText))
	if len(requested) == 0 {
		requested = append(requested, allowed...)
	}
	out := make([]string, 0, len(requested))
	seen := map[string]struct{}{}
	for _, scope := range requested {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := allowedSet[scope]; !ok {
			return nil, fmt.Errorf("scope %q is not allowed for this client", scope)
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	sort.Strings(out)
	return out, nil
}

func validatePKCE(codeVerifier, codeChallenge, method string) error {
	if strings.TrimSpace(codeVerifier) == "" {
		return errors.New("code_verifier is required")
	}
	if !strings.EqualFold(strings.TrimSpace(method), "S256") {
		return errors.New("unsupported code_challenge_method")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(codeVerifier)))
	encoded := base64.RawURLEncoding.EncodeToString(sum[:])
	if encoded != strings.TrimSpace(codeChallenge) {
		return errors.New("invalid code_verifier")
	}
	return nil
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}

func containsExact(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func subtleConstantTimeCompare(expected, actual string) bool {
	expectedBytes := []byte(expected)
	actualBytes := []byte(actual)
	return subtle.ConstantTimeCompare(expectedBytes, actualBytes) == 1
}

func matchesClientSecret(client Client, providedSecret string) bool {
	providedSecret = strings.TrimSpace(providedSecret)
	if providedSecret == "" {
		return false
	}
	if strings.TrimSpace(client.ClientSecretHash) != "" {
		return bcrypt.CompareHashAndPassword([]byte(client.ClientSecretHash), []byte(normalizeClientSecretForHashing(providedSecret))) == nil
	}
	if strings.TrimSpace(client.ClientSecret) != "" {
		slog.Warn("oauth_client_secret_plaintext_fallback", "client_id", client.ClientID)
		return subtleConstantTimeCompare(client.ClientSecret, providedSecret)
	}
	return false
}

func normalizeClientSecretForHashing(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}
