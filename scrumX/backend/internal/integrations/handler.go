package integrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/authz"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	db     *sql.DB
	authz  *authz.Service
	crypto *credentialCipher
}

func NewHandler(db *sql.DB, authzService *authz.Service, encryptionKey string) (*Handler, error) {
	crypto, err := newCredentialCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	return &Handler{db: db, authz: authzService, crypto: crypto}, nil
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	r := api.Group("/integrations")
	r.Get("/", h.list)
	r.Post("/github", h.upsertGitHub)
	r.Post("/jira", h.upsertJira)
	r.Post("/slack", h.upsertSlack)
	r.Post("/cicd", h.upsertCICD)
	r.Post("/deployments", h.upsertDeployments)
	r.Post("/:provider/test", h.testProvider)
}

func (h *Handler) list(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	if _, err := h.authz.RequireOrgMember(c.Context(), orgID, userID); err != nil {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	rows, err := h.db.QueryContext(c.Context(), `SELECT DISTINCT ON (provider) id, provider, credentials, created_at
FROM integrations
WHERE org_id=$1
ORDER BY provider, created_at DESC`, orgID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	out := make([]fiber.Map, 0, 8)
	for rows.Next() {
		var (
			id             uuid.UUID
			provider       string
			credentialsRaw []byte
			createdAt      time.Time
		)
		if err := rows.Scan(&id, &provider, &credentialsRaw, &createdAt); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
		_, keys, err := h.crypto.Decrypt(credentialsRaw)
		if err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, "unable to decrypt stored credentials")
		}
		out = append(out, fiber.Map{
			"id":              id,
			"provider":        provider,
			"configured":      len(keys) > 0,
			"credential_keys": keys,
			"created_at":      createdAt,
		})
	}
	return utils.JSONSuccess(c, fiber.StatusOK, out)
}

func (h *Handler) upsertGitHub(c *fiber.Ctx) error      { return h.upsertProvider(c, "github") }
func (h *Handler) upsertJira(c *fiber.Ctx) error        { return h.upsertProvider(c, "jira") }
func (h *Handler) upsertSlack(c *fiber.Ctx) error       { return h.upsertProvider(c, "slack") }
func (h *Handler) upsertCICD(c *fiber.Ctx) error        { return h.upsertProvider(c, "cicd") }
func (h *Handler) upsertDeployments(c *fiber.Ctx) error { return h.upsertProvider(c, "deployments") }

func (h *Handler) upsertProvider(c *fiber.Ctx, provider string) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	role, err := h.authz.RequireOrgMember(c.Context(), orgID, userID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	var payload struct {
		Credentials map[string]any `json:"credentials"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.Credentials == nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "credentials are required")
	}
	if err := validateProviderCredentials(provider, payload.Credentials); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	keys := mapKeys(payload.Credentials)
	credentialsRaw, err := json.Marshal(payload.Credentials)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid credentials")
	}
	encryptedCredentials, err := h.crypto.Encrypt(credentialsRaw, keys)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "failed to encrypt credentials")
	}
	tx, err := h.db.BeginTx(c.Context(), nil)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	defer tx.Rollback()
	var integrationID uuid.UUID
	err = tx.QueryRowContext(c.Context(), `SELECT id FROM integrations WHERE org_id=$1 AND provider=$2 ORDER BY created_at DESC LIMIT 1`,
		orgID, provider).Scan(&integrationID)
	switch err {
	case nil:
		if _, err := tx.ExecContext(c.Context(), `UPDATE integrations SET credentials=$3 WHERE id=$1 AND org_id=$2`,
			integrationID, orgID, encryptedCredentials); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
		}
	case sql.ErrNoRows:
		integrationID = uuid.New()
		if _, err := tx.ExecContext(c.Context(), `INSERT INTO integrations (id, org_id, provider, credentials) VALUES ($1,$2,$3,$4)`,
			integrationID, orgID, provider, encryptedCredentials); err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
	default:
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	if err := tx.Commit(); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{
		"id":              integrationID,
		"provider":        provider,
		"configured":      true,
		"credential_keys": keys,
	})
}

func (h *Handler) testProvider(c *fiber.Ctx) error {
	orgID, ok := middleware.MustOrgID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusBadRequest, "missing org context")
	}
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "missing user context")
	}
	role, err := h.authz.RequireOrgMember(c.Context(), orgID, userID)
	if err != nil || !authz.CanWrite(role) {
		return utils.JSONError(c, fiber.StatusForbidden, "forbidden")
	}
	provider := strings.ToLower(strings.TrimSpace(c.Params("provider")))
	if provider == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "provider is required")
	}
	var credentialsRaw []byte
	if err := h.db.QueryRowContext(c.Context(), `SELECT credentials FROM integrations WHERE org_id=$1 AND provider=$2 ORDER BY created_at DESC LIMIT 1`,
		orgID, provider).Scan(&credentialsRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.JSONError(c, fiber.StatusNotFound, "integration is not configured")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	decrypted, keys, err := h.crypto.Decrypt(credentialsRaw)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "unable to decrypt credentials")
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(decrypted, &decoded); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "stored credentials are invalid")
	}
	if err := validateProviderCredentials(provider, decoded); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	if err := testProviderConnectivity(c.Context(), provider, decoded); err != nil {
		return utils.JSONError(c, fiber.StatusBadGateway, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{
		"provider":        provider,
		"configured":      true,
		"credential_keys": keys,
		"tested_at":       time.Now().UTC(),
	})
}

func mapKeys(decoded map[string]any) []string {
	keys := make([]string, 0, len(decoded))
	for k := range decoded {
		keys = append(keys, strings.TrimSpace(k))
	}
	return keys
}

func validateProviderCredentials(provider string, credentials map[string]any) error {
	requiredByProvider := map[string][]string{
		"github":      {"token"},
		"jira":        {"base_url", "email", "api_token"},
		"slack":       {"bot_token"},
		"cicd":        {"base_url", "token"},
		"deployments": {"base_url", "token"},
	}
	required, ok := requiredByProvider[provider]
	if !ok {
		return errors.New("unsupported integration provider")
	}
	for _, field := range required {
		raw, exists := credentials[field]
		if !exists || strings.TrimSpace(asString(raw)) == "" {
			return errors.New("missing required credential: " + field)
		}
	}
	return nil
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}

func testProviderConnectivity(ctx context.Context, provider string, credentials map[string]any) error {
	client := newProviderHTTPClient()
	switch provider {
	case "github":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(asString(credentials["token"])))
		req.Header.Set("Accept", "application/vnd.github+json")
		return doProviderRequest(client, req)
	case "jira":
		baseURL := strings.TrimSpace(asString(credentials["base_url"]))
		email := strings.TrimSpace(asString(credentials["email"]))
		apiToken := strings.TrimSpace(asString(credentials["api_token"]))
		endpoint, err := buildProviderURL(ctx, baseURL, "/rest/api/3/myself")
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth(email, apiToken)
		return doProviderRequest(client, req)
	case "slack":
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/auth.test", strings.NewReader(""))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(asString(credentials["bot_token"])))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		respBody, err := doProviderRequestWithBody(client, req)
		if err != nil {
			return err
		}
		var decoded map[string]any
		if err := json.Unmarshal(respBody, &decoded); err != nil {
			return fmt.Errorf("slack auth response is invalid")
		}
		ok, _ := decoded["ok"].(bool)
		if !ok {
			return errors.New("slack auth.test rejected credentials")
		}
		return nil
	case "cicd", "deployments":
		baseURL := strings.TrimSpace(asString(credentials["base_url"]))
		token := strings.TrimSpace(asString(credentials["token"]))
		endpoint, err := buildProviderURL(ctx, baseURL, "")
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return doProviderRequest(client, req)
	default:
		return errors.New("unsupported integration provider")
	}
}

func buildProviderURL(ctx context.Context, baseURL, suffix string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid base_url")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", errors.New("base_url scheme is not allowed")
	}
	if err := validateProviderHost(ctx, parsed.Hostname()); err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + suffix
	return parsed.String(), nil
}

func validateProviderHost(ctx context.Context, hostname string) error {
	host := strings.ToLower(strings.TrimSpace(hostname))
	if host == "" {
		return errors.New("base_url host is required")
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" || host == "::" || strings.HasSuffix(host, ".localhost") {
		return errors.New("base_url host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return errors.New("base_url host must be publicly routable")
		}
		return nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil || len(addrs) == 0 {
		return errors.New("unable to resolve base_url host")
	}
	for _, addr := range addrs {
		if isPrivateOrLocalIP(addr.IP) {
			return errors.New("base_url host must not resolve to private or local addresses")
		}
	}
	return nil
}

func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 10:
			return true
		case v4[0] == 127:
			return true
		case v4[0] == 169 && v4[1] == 254:
			return true
		case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
			return true
		case v4[0] == 192 && v4[1] == 168:
			return true
		default:
			return false
		}
	}
	return len(ip) == net.IPv6len && (ip[0]&0xfe) == 0xfc
}

func newProviderHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			host, _, splitErr := net.SplitHostPort(conn.RemoteAddr().String())
			if splitErr != nil {
				_ = conn.Close()
				return nil, splitErr
			}
			if ip := net.ParseIP(host); isPrivateOrLocalIP(ip) {
				_ = conn.Close()
				return nil, errors.New("provider host resolved to private or local address")
			}
			return conn, nil
		},
	}
	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}
}

func doProviderRequest(client *http.Client, req *http.Request) error {
	_, err := doProviderRequestWithBody(client, req)
	return err
}

func doProviderRequestWithBody(client *http.Client, req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(body) == 0 {
			return nil, fmt.Errorf("provider returned status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}
