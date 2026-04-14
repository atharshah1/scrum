package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/tui/internal/config"
	"github.com/go-resty/resty/v2"
)

type Client struct{}

type Issue struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
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
	r := resty.New().
		SetBaseURL(strings.TrimRight(cfg.APIURL, "/")).
		SetTimeout(20 * time.Second)
	resp, err := r.R().
		SetAuthToken(cfg.AccessToken).
		SetHeader("Accept", "application/json").
		SetHeader("X-Org-ID", cfg.OrgID).
		SetResult(&out).
		Get("/issues")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		if out.Error.Message != "" {
			return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode(), out.Error.Message)
		}
		return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode(), http.StatusText(resp.StatusCode()))
	}
	return out.Data, nil
}
