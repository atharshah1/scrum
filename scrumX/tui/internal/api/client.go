package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/tui/internal/config"
	"github.com/go-resty/resty/v2"
	"golang.org/x/net/websocket"
)

type Client struct{}

type Issue struct {
	ID         string   `json:"id"`
	ProjectID  string   `json:"project_id"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Priority   string   `json:"priority"`
	IssueType  string   `json:"issue_type"`
	Labels     []string `json:"labels"`
	AssigneeID *string  `json:"assignee_id,omitempty"`
}

type BoardIssue struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Status     string  `json:"status"`
	Priority   string  `json:"priority"`
	AssigneeID *string `json:"assignee_id,omitempty"`
}

type BoardColumn struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Statuses []string     `json:"statuses"`
	Position int          `json:"position"`
	Issues   []BoardIssue `json:"issues"`
}

type Board struct {
	BoardID   string        `json:"board_id"`
	ProjectID string        `json:"project_id"`
	Columns   []BoardColumn `json:"columns"`
}

type WorkflowTransition struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
}

type Notification struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	IsRead   bool   `json:"is_read"`
	Type     string `json:"type"`
	EntityID string `json:"entity_id,omitempty"`
}

type Event struct {
	Type    string         `json:"type"`
	OrgID   string         `json:"org_id"`
	Payload map[string]any `json:"payload"`
}

type envelope[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
	Error   struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewClient() *Client { return &Client{} }

func (c *Client) ListIssues() ([]Issue, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("not logged in: run scrumx auth login first")
	}
	var out envelope[[]Issue]
	resp, err := c.request(cfg, http.MethodGet, "/issues", nil, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode(), parseError(out.Error.Message, resp.StatusCode()))
	}
	return out.Data, nil
}

func (c *Client) GetBoard(boardID, sprintID string) (Board, error) {
	cfg, err := config.Load()
	if err != nil {
		return Board{}, err
	}
	if cfg.AccessToken == "" {
		return Board{}, fmt.Errorf("not logged in: run scrumx auth login first")
	}
	path := "/boards/" + strings.TrimSpace(boardID)
	if sprintID = strings.TrimSpace(sprintID); sprintID != "" {
		path += "?sprint_id=" + url.QueryEscape(sprintID)
	}
	var out envelope[Board]
	resp, err := c.request(cfg, http.MethodGet, path, nil, &out)
	if err != nil {
		return Board{}, err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return Board{}, fmt.Errorf("request failed (%d): %s", resp.StatusCode(), parseError(out.Error.Message, resp.StatusCode()))
	}
	return out.Data, nil
}

func (c *Client) ListAllowedTransitions(projectID, currentStatus string) ([]string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("not logged in")
	}
	var out envelope[[]WorkflowTransition]
	resp, err := c.request(cfg, http.MethodGet, "/workflows/"+strings.TrimSpace(projectID)+"/transitions", nil, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode(), parseError(out.Error.Message, resp.StatusCode()))
	}
	allowed := make([]string, 0)
	seen := map[string]struct{}{}
	from := strings.ToLower(strings.TrimSpace(currentStatus))
	for _, transition := range out.Data {
		if strings.ToLower(strings.TrimSpace(transition.FromStatus)) != from {
			continue
		}
		to := strings.ToLower(strings.TrimSpace(transition.ToStatus))
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

func (c *Client) UpdateIssueStatus(issueID, status string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	body := map[string]string{"status": strings.ToLower(strings.TrimSpace(status))}
	var out envelope[map[string]any]
	resp, err := c.request(cfg, http.MethodPatch, "/issues/"+strings.TrimSpace(issueID), body, &out)
	if err != nil {
		return err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return fmt.Errorf("request failed (%d): %s", resp.StatusCode(), parseError(out.Error.Message, resp.StatusCode()))
	}
	return nil
}

func (c *Client) ListNotifications(limit int) ([]Notification, error) {
	if limit <= 0 {
		limit = 20
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	var out envelope[[]Notification]
	resp, err := c.request(cfg, http.MethodGet, fmt.Sprintf("/notifications?page=1&limit=%d", limit), nil, &out)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode(), parseError(out.Error.Message, resp.StatusCode()))
	}
	return out.Data, nil
}

func (c *Client) StreamEvents(ctx context.Context) (<-chan Event, <-chan error) {
	eventsCh := make(chan Event, 32)
	errCh := make(chan error, 1)
	go func() {
		defer close(eventsCh)
		defer close(errCh)

		cfg, err := config.Load()
		if err != nil {
			errCh <- err
			return
		}
		if cfg.AccessToken == "" {
			errCh <- fmt.Errorf("not logged in")
			return
		}
		wsURL, origin, err := websocketEndpoint(cfg.APIURL)
		if err != nil {
			errCh <- err
			return
		}
		wsCfg, err := websocket.NewConfig(wsURL, origin)
		if err != nil {
			errCh <- err
			return
		}
		wsCfg.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
		orgID := cfg.CurrentOrgID
		if orgID == "" {
			orgID = cfg.OrgID
		}
		if orgID != "" {
			wsCfg.Header.Set("X-Org-ID", orgID)
		}

		conn, err := websocket.DialConfig(wsCfg)
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()

		go func() {
			<-ctx.Done()
			_ = conn.Close()
		}()

		for {
			var msg Event
			if err := websocket.JSON.Receive(conn, &msg); err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					errCh <- err
					return
				}
			}
			select {
			case eventsCh <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()
	return eventsCh, errCh
}

func websocketEndpoint(apiURL string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(apiURL))
	if err != nil {
		return "", "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", "", fmt.Errorf("unsupported scheme for websocket: %s", u.Scheme)
	}
	u.Path = "/ws"
	u.RawQuery = ""
	u.Fragment = ""
	originScheme := "http"
	if u.Scheme == "wss" {
		originScheme = "https"
	}
	origin := originScheme + "://" + u.Host + "/"
	return u.String(), origin, nil
}

func (c *Client) request(cfg config.Config, method, path string, body any, out any) (*resty.Response, error) {
	r := resty.New().
		SetBaseURL(strings.TrimRight(cfg.APIURL, "/")).
		SetTimeout(20 * time.Second)
	orgID := cfg.CurrentOrgID
	if orgID == "" {
		orgID = cfg.OrgID
	}
	req := r.R().
		SetAuthToken(cfg.AccessToken).
		SetHeader("Accept", "application/json").
		SetHeader("X-Org-ID", orgID)
	if body != nil {
		req.SetBody(body)
	}
	if out != nil {
		req.SetResult(out)
	}
	return req.Execute(method, path)
}

func parseError(message string, code int) string {
	if strings.TrimSpace(message) != "" {
		return message
	}
	return http.StatusText(code)
}

func UnmarshalEvent(raw []byte) (Event, error) {
	var e Event
	err := json.Unmarshal(raw, &e)
	return e, err
}
