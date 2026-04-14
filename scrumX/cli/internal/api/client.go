package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/config"
	"github.com/go-resty/resty/v2"
)

type Client struct {
	cfgStore *config.Store
}

type User struct {
	ID    string `json:"id"`
	OrgID string `json:"org_id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type Issue struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	IssueType   string `json:"issue_type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
}

type envelope[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
	Error   struct {
		Message string `json:"message"`
	} `json:"error"`
}

type authLoginResponse struct {
	User   User      `json:"user"`
	Tokens TokenPair `json:"tokens"`
}

type authRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func NewClient(cfgStore *config.Store) *Client {
	return &Client{cfgStore: cfgStore}
}

func (c *Client) Login(email, password string) (User, TokenPair, error) {
	cfg, err := c.cfgStore.Load()
	if err != nil {
		return User{}, TokenPair{}, err
	}
	body := map[string]string{"email": email, "password": password}
	var out envelope[authLoginResponse]
	if _, err := c.request(cfg.APIURL, http.MethodPost, "/auth/login", "", "", body, &out); err != nil {
		return User{}, TokenPair{}, err
	}
	return out.Data.User, out.Data.Tokens, nil
}

func (c *Client) Refresh(refreshToken string) (TokenPair, error) {
	cfg, err := c.cfgStore.Load()
	if err != nil {
		return TokenPair{}, err
	}
	var out envelope[TokenPair]
	if _, err := c.request(cfg.APIURL, http.MethodPost, "/auth/refresh", "", "", authRefreshRequest{RefreshToken: refreshToken}, &out); err != nil {
		return TokenPair{}, err
	}
	return out.Data, nil
}

func (c *Client) WhoAmI() error {
	_, err := c.authedRequest(http.MethodGet, "/auth/me", nil, &envelope[map[string]any]{})
	return err
}

func (c *Client) CreateIssue(title, projectID, description, issueType, priority string) (Issue, error) {
	body := map[string]any{
		"title":       title,
		"project_id":  strings.TrimSpace(projectID),
		"description": description,
		"issue_type":  issueType,
		"priority":    priority,
	}
	var out envelope[Issue]
	_, err := c.authedRequest(http.MethodPost, "/issues", body, &out)
	return out.Data, err
}

func (c *Client) ListIssues() ([]Issue, error) {
	var out envelope[[]Issue]
	_, err := c.authedRequest(http.MethodGet, "/issues", nil, &out)
	return out.Data, err
}

func (c *Client) GetIssue(id string) (Issue, error) {
	var out envelope[Issue]
	_, err := c.authedRequest(http.MethodGet, "/issues/"+strings.TrimSpace(id), nil, &out)
	return out.Data, err
}

func (c *Client) authedRequest(method, path string, body any, out any) (*resty.Response, error) {
	cfg, err := c.cfgStore.Load()
	if err != nil {
		return nil, err
	}
	if cfg.AccessToken == "" {
		return nil, errors.New("not logged in: run `scrumx auth login`")
	}

	resp, err := c.request(cfg.APIURL, method, path, cfg.AccessToken, cfg.OrgID, body, out)
	if err == nil {
		return resp, nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "401") || cfg.RefreshToken == "" {
		return nil, err
	}

	tokens, refreshErr := c.Refresh(cfg.RefreshToken)
	if refreshErr != nil {
		return nil, fmt.Errorf("request failed (%v) and token refresh failed: %w", err, refreshErr)
	}
	cfg.AccessToken = tokens.AccessToken
	cfg.RefreshToken = tokens.RefreshToken
	if saveErr := c.cfgStore.Save(cfg); saveErr != nil {
		return nil, saveErr
	}
	return c.request(cfg.APIURL, method, path, cfg.AccessToken, cfg.OrgID, body, out)
}

func (c *Client) request(baseURL, method, path, accessToken, orgID string, body any, out any) (*resty.Response, error) {
	r := resty.New().
		SetBaseURL(strings.TrimRight(baseURL, "/")).
		SetTimeout(20 * time.Second)

	req := r.R().SetHeader("Accept", "application/json")
	if out != nil {
		req.SetResult(out)
	}
	if body != nil {
		req.SetBody(body)
	}
	if accessToken != "" {
		req.SetAuthToken(accessToken)
	}
	if orgID != "" {
		req.SetHeader("X-Org-ID", orgID)
	}

	resp, err := req.Execute(method, path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return resp, nil
	}
	msg := parseErrorMessage(resp)
	return resp, fmt.Errorf("request failed (%d): %s", resp.StatusCode(), msg)
}

func parseErrorMessage(resp *resty.Response) string {
	var structured envelope[map[string]any]
	if err := json.Unmarshal(resp.Body(), &structured); err == nil && structured.Error.Message != "" {
		return structured.Error.Message
	}
	var unstructured map[string]any
	if err := json.Unmarshal(resp.Body(), &unstructured); err == nil {
		if v, ok := unstructured["error"].(string); ok && v != "" {
			return v
		}
	}
	if s := strings.TrimSpace(string(resp.Body())); s != "" {
		return s
	}
	return http.StatusText(resp.StatusCode())
}
