package api

import (
"encoding/json"
"errors"
"fmt"
"net/http"
"net/url"
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
ID          string   `json:"id"`
ProjectID   string   `json:"project_id"`
ParentID    *string  `json:"parent_id,omitempty"`
SprintID    *string  `json:"sprint_id,omitempty"`
AssigneeID  *string  `json:"assignee_id,omitempty"`
IssueType   string   `json:"issue_type"`
Title       string   `json:"title"`
Description string   `json:"description"`
Status      string   `json:"status"`
Priority    string   `json:"priority"`
Labels      []string `json:"labels"`
}

type CreateIssueInput struct {
Title       string   `json:"title"`
ProjectID   string   `json:"project_id"`
Description string   `json:"description,omitempty"`
IssueType   string   `json:"issue_type,omitempty"`
Priority    string   `json:"priority,omitempty"`
ParentID    string   `json:"parent_id,omitempty"`
SprintID    string   `json:"sprint_id,omitempty"`
AssigneeID  string   `json:"assignee_id,omitempty"`
Labels      []string `json:"labels,omitempty"`
}

type UpdateIssueInput struct {
ParentID    *string   `json:"parent_id,omitempty"`
SprintID    *string   `json:"sprint_id,omitempty"`
AssigneeID  *string   `json:"assignee_id,omitempty"`
Title       *string   `json:"title,omitempty"`
Description *string   `json:"description,omitempty"`
Status      *string   `json:"status,omitempty"`
Priority    *string   `json:"priority,omitempty"`
IssueType   *string   `json:"issue_type,omitempty"`
Labels      *[]string `json:"labels,omitempty"`
}

type WorkflowTransition struct {
ID         string `json:"id"`
FromStatus string `json:"from_status"`
ToStatus   string `json:"to_status"`
}

type BoardIssue struct {
ID         string  `json:"id"`
Title      string  `json:"title"`
Status     string  `json:"status"`
Priority   string  `json:"priority"`
AssigneeID *string `json:"assignee_id,omitempty"`
SprintID   *string `json:"sprint_id,omitempty"`
}

type BoardColumn struct {
ID       string      `json:"id"`
Name     string      `json:"name"`
Statuses []string    `json:"statuses"`
Position int         `json:"position"`
Issues   []BoardIssue `json:"issues"`
}

type Board struct {
BoardID   string        `json:"board_id"`
ProjectID string        `json:"project_id"`
Columns   []BoardColumn `json:"columns"`
}

type Notification struct {
ID        string    `json:"id"`
Type      string    `json:"type"`
Title     string    `json:"title"`
Message   string    `json:"message"`
EntityID  string    `json:"entity_id,omitempty"`
IsRead    bool      `json:"is_read"`
CreatedAt time.Time `json:"created_at"`
}

type envelope[T any] struct {
Success bool `json:"success"`
Data    T    `json:"data"`
Meta    struct {
Page  int `json:"page"`
Limit int `json:"limit"`
Total int `json:"total"`
} `json:"meta"`
Error struct {
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

func (c *Client) CreateIssue(input CreateIssueInput) (Issue, error) {
body := map[string]any{
"title":       strings.TrimSpace(input.Title),
"project_id":  strings.TrimSpace(input.ProjectID),
"description": strings.TrimSpace(input.Description),
"issue_type":  strings.TrimSpace(input.IssueType),
"priority":    strings.TrimSpace(input.Priority),
"labels":      cleanLabels(input.Labels),
}
if v := strings.TrimSpace(input.ParentID); v != "" {
body["parent_id"] = v
}
if v := strings.TrimSpace(input.SprintID); v != "" {
body["sprint_id"] = v
}
if v := strings.TrimSpace(input.AssigneeID); v != "" {
body["assignee_id"] = v
}
var out envelope[Issue]
_, err := c.authedRequest(http.MethodPost, "/issues", body, &out)
return out.Data, err
}

func (c *Client) UpdateIssue(id string, input UpdateIssueInput) (Issue, error) {
body := map[string]any{}
if input.ParentID != nil {
body["parent_id"] = strings.TrimSpace(*input.ParentID)
}
if input.SprintID != nil {
body["sprint_id"] = strings.TrimSpace(*input.SprintID)
}
if input.AssigneeID != nil {
body["assignee_id"] = strings.TrimSpace(*input.AssigneeID)
}
if input.Title != nil {
body["title"] = strings.TrimSpace(*input.Title)
}
if input.Description != nil {
body["description"] = *input.Description
}
if input.Status != nil {
body["status"] = strings.ToLower(strings.TrimSpace(*input.Status))
}
if input.Priority != nil {
body["priority"] = strings.ToLower(strings.TrimSpace(*input.Priority))
}
if input.IssueType != nil {
body["issue_type"] = strings.ToLower(strings.TrimSpace(*input.IssueType))
}
if input.Labels != nil {
body["labels"] = cleanLabels(*input.Labels)
}
var out envelope[Issue]
_, err := c.authedRequest(http.MethodPatch, "/issues/"+strings.TrimSpace(id), body, &out)
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

func (c *Client) AddLabel(issueID, label string) error {
_, err := c.authedRequest(http.MethodPost, "/issues/"+strings.TrimSpace(issueID)+"/labels", map[string]string{"label": strings.TrimSpace(label)}, &envelope[map[string]any]{})
return err
}

func (c *Client) RemoveLabel(issueID, label string) error {
escaped := url.PathEscape(strings.TrimSpace(label))
_, err := c.authedRequest(http.MethodDelete, "/issues/"+strings.TrimSpace(issueID)+"/labels/"+escaped, nil, nil)
return err
}

func (c *Client) ListWorkflowTransitions(projectID string) ([]WorkflowTransition, error) {
var out envelope[[]WorkflowTransition]
_, err := c.authedRequest(http.MethodGet, "/workflows/"+strings.TrimSpace(projectID)+"/transitions", nil, &out)
return out.Data, err
}

func (c *Client) ListAllowedTransitions(issue Issue) ([]string, error) {
projectID := strings.TrimSpace(issue.ProjectID)
if projectID == "" {
return nil, fmt.Errorf("issue missing project_id")
}
transitions, err := c.ListWorkflowTransitions(projectID)
if err != nil {
return nil, err
}
allowed := make([]string, 0)
seen := map[string]struct{}{}
from := strings.ToLower(strings.TrimSpace(issue.Status))
for _, t := range transitions {
if strings.ToLower(strings.TrimSpace(t.FromStatus)) != from {
continue
}
to := strings.ToLower(strings.TrimSpace(t.ToStatus))
if to == "" {
continue
}
if _, ok := seen[to]; ok {
continue
}
seen[to] = struct{}{}
allowed = append(allowed, to)
}
return allowed, nil
}

func (c *Client) StartSprint(sprintID string) error {
_, err := c.authedRequest(http.MethodPost, "/sprints/"+strings.TrimSpace(sprintID)+"/start", nil, &envelope[map[string]any]{})
return err
}

func (c *Client) EndSprint(sprintID string) error {
_, err := c.authedRequest(http.MethodPost, "/sprints/"+strings.TrimSpace(sprintID)+"/end", nil, &envelope[map[string]any]{})
return err
}

func (c *Client) AddIssuesToSprint(sprintID string, issueIDs []string) error {
payload := map[string]any{"issue_ids": compactIDs(issueIDs)}
_, err := c.authedRequest(http.MethodPost, "/sprints/"+strings.TrimSpace(sprintID)+"/issues", payload, &envelope[map[string]any]{})
return err
}

func (c *Client) RemoveIssuesFromSprint(sprintID string, issueIDs []string) error {
payload := map[string]any{"issue_ids": compactIDs(issueIDs)}
_, err := c.authedRequest(http.MethodDelete, "/sprints/"+strings.TrimSpace(sprintID)+"/issues", payload, &envelope[map[string]any]{})
return err
}

func (c *Client) GetBoard(boardID, sprintID string) (Board, error) {
path := "/boards/" + strings.TrimSpace(boardID)
if sprintID = strings.TrimSpace(sprintID); sprintID != "" {
path += "?sprint_id=" + url.QueryEscape(sprintID)
}
var out envelope[Board]
_, err := c.authedRequest(http.MethodGet, path, nil, &out)
return out.Data, err
}

func (c *Client) ListNotifications(page, limit int) ([]Notification, error) {
if page <= 0 {
page = 1
}
if limit <= 0 {
limit = 20
}
path := fmt.Sprintf("/notifications?page=%d&limit=%d", page, limit)
var out envelope[[]Notification]
_, err := c.authedRequest(http.MethodGet, path, nil, &out)
return out.Data, err
}

func (c *Client) MarkNotificationRead(notificationID string) error {
_, err := c.authedRequest(http.MethodPatch, "/notifications/"+strings.TrimSpace(notificationID)+"/read", nil, &envelope[map[string]any]{})
return err
}

func (c *Client) MarkAllNotificationsRead() error {
_, err := c.authedRequest(http.MethodPost, "/notifications/read-all", nil, &envelope[map[string]any]{})
return err
}

func (c *Client) authedRequest(method, path string, body any, out any) (*resty.Response, error) {
cfg, err := c.cfgStore.Load()
if err != nil {
return nil, err
}
if cfg.AccessToken == "" {
return nil, errors.New("not logged in: run `scrumx auth login`")
}
orgID := cfg.CurrentOrgID
if strings.TrimSpace(orgID) == "" {
orgID = cfg.OrgID
}

resp, err := c.request(cfg.APIURL, method, path, cfg.AccessToken, orgID, body, out)
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
return c.request(cfg.APIURL, method, path, cfg.AccessToken, orgID, body, out)
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

func cleanLabels(labels []string) []string {
if len(labels) == 0 {
return nil
}
out := make([]string, 0, len(labels))
seen := map[string]struct{}{}
for _, label := range labels {
v := strings.ToLower(strings.TrimSpace(label))
if v == "" {
continue
}
if _, ok := seen[v]; ok {
continue
}
seen[v] = struct{}{}
out = append(out, v)
}
if len(out) == 0 {
return nil
}
return out
}

func compactIDs(values []string) []string {
out := make([]string, 0, len(values))
seen := map[string]struct{}{}
for _, value := range values {
v := strings.TrimSpace(value)
if v == "" {
continue
}
if _, ok := seen[v]; ok {
continue
}
seen[v] = struct{}{}
out = append(out, v)
}
return out
}
