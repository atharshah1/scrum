package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/internal/offline"
	"github.com/atharshah1/scrum/scrumX/tui/internal/config"
)

type SyncStatus struct {
	Mode         string
	LastSyncedAt time.Time
	PendingOps   int
}

func (c *Client) SyncStatus() (SyncStatus, error) {
	cfg, err := config.Load()
	if err != nil {
		return SyncStatus{}, err
	}
	store, err := c.offlineStore(cfg)
	if err != nil {
		return SyncStatus{}, err
	}
	state, err := store.Load()
	if err != nil {
		return SyncStatus{}, err
	}
	return SyncStatus{Mode: state.Mode, LastSyncedAt: state.LastSyncedAt, PendingOps: len(state.PendingOperations)}, nil
}

func (c *Client) SyncNow() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := c.offlineStore(cfg)
	if err != nil {
		return err
	}
	if _, err := store.Update(func(s *offline.State) error {
		s.Mode = offline.ModeSyncing
		return nil
	}); err != nil {
		return err
	}
	state, err := store.Load()
	if err != nil {
		return err
	}
	next := make([]offline.Operation, 0, len(state.PendingOperations))
	for _, op := range state.PendingOperations {
		target := offline.ResolveID(state, op.TargetID)
		switch op.Action {
		case "update":
			var payload UpdateIssueInput
			_ = json.Unmarshal(op.Payload, &payload)
			if err := c.UpdateIssue(target, payload); err != nil {
				op.Attempts++
				op.LastError = err.Error()
				op.NextAttemptAt = time.Now().Add(offline.Backoff(op.Attempts))
				next = append(next, op)
			}
		default:
			next = append(next, op)
		}
	}
	state.PendingOperations = next
	if len(state.PendingOperations) == 0 {
		state.Mode = offline.ModeOnline
		state.LastSyncedAt = time.Now().UTC()
	}
	if err := store.Save(state); err != nil {
		return err
	}
	_, err = c.ListIssuesSmart()
	return err
}

func (c *Client) ListIssuesSmart() ([]Issue, error) {
	items, err := c.ListIssues()
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		return items, err
	}
	store, storeErr := c.offlineStore(cfg)
	if storeErr != nil {
		return items, err
	}
	if err == nil {
		_, _ = store.Update(func(s *offline.State) error {
			ids := make([]string, 0, len(items))
			for _, it := range items {
				s.Issues[it.ID] = fromIssue(it)
				ids = append(ids, it.ID)
				if strings.TrimSpace(it.ProjectID) != "" {
					s.Projects[it.ProjectID] = offline.Project{ID: it.ProjectID}
				}
			}
			sort.Strings(ids)
			s.IssueQueryCache["list"] = ids
			s.Mode = offline.ModeOnline
			return nil
		})
		return items, nil
	}
	if !isConnectivityErr(err) {
		return nil, err
	}
	state, stateErr := store.Update(func(s *offline.State) error {
		s.Mode = offline.ModeOffline
		return nil
	})
	if stateErr != nil {
		return nil, stateErr
	}
	return toIssues(state, "list"), nil
}

func (c *Client) SearchIssuesSmart(query string) ([]Issue, error) {
	items, err := c.SearchIssues(query)
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		return items, err
	}
	store, storeErr := c.offlineStore(cfg)
	if storeErr != nil {
		return items, err
	}
	key := "search:" + strings.TrimSpace(query)
	if err == nil {
		_, _ = store.Update(func(s *offline.State) error {
			ids := make([]string, 0, len(items))
			for _, it := range items {
				s.Issues[it.ID] = fromIssue(it)
				ids = append(ids, it.ID)
			}
			s.IssueQueryCache[key] = ids
			s.Mode = offline.ModeOnline
			return nil
		})
		return items, nil
	}
	if !isConnectivityErr(err) {
		return nil, err
	}
	state, stateErr := store.Update(func(s *offline.State) error {
		s.Mode = offline.ModeOffline
		return nil
	})
	if stateErr != nil {
		return nil, stateErr
	}
	issues := toIssues(state, key)
	if len(issues) > 0 {
		return issues, nil
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	out := make([]Issue, 0, len(state.Issues))
	for _, item := range state.Issues {
		if item.Deleted {
			continue
		}
		if needle == "" || strings.Contains(strings.ToLower(item.Title), needle) || strings.Contains(strings.ToLower(item.Description), needle) {
			out = append(out, toIssue(item))
		}
	}
	return out, nil
}

func (c *Client) GetIssueSmart(id string) (Issue, error) {
	item, err := c.GetIssue(id)
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		return item, err
	}
	store, storeErr := c.offlineStore(cfg)
	if storeErr != nil {
		return item, err
	}
	if err == nil {
		_, _ = store.Update(func(s *offline.State) error {
			s.Issues[item.ID] = fromIssue(item)
			s.Mode = offline.ModeOnline
			return nil
		})
		return item, nil
	}
	if !isConnectivityErr(err) {
		return Issue{}, err
	}
	state, stateErr := store.Update(func(s *offline.State) error {
		s.Mode = offline.ModeOffline
		return nil
	})
	if stateErr != nil {
		return Issue{}, stateErr
	}
	target := offline.ResolveID(state, id)
	local, ok := state.Issues[target]
	if !ok || local.Deleted {
		return Issue{}, err
	}
	return toIssue(local), nil
}

func (c *Client) UpdateIssueSmart(issueID string, input UpdateIssueInput) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := c.offlineStore(cfg)
	if err != nil {
		return err
	}
	if state, loadErr := store.Load(); loadErr == nil && input.UpdatedAt == nil {
		target := offline.ResolveID(state, issueID)
		if item, ok := state.Issues[target]; ok && !item.UpdatedAt.IsZero() {
			ts := item.UpdatedAt.UTC()
			input.UpdatedAt = &ts
		}
	}
	if err := c.UpdateIssue(issueID, input); err == nil {
		if status, statusErr := c.SyncStatus(); statusErr == nil && status.PendingOps > 0 {
			_ = c.SyncNow()
		}
		return nil
	} else if !isConnectivityErr(err) {
		return err
	}
	payload, _ := json.Marshal(input)
	_, err = store.Update(func(s *offline.State) error {
		s.Mode = offline.ModeOffline
		target := offline.ResolveID(*s, issueID)
		item, ok := s.Issues[target]
		if !ok {
			item = offline.Issue{ID: target}
		}
		applyPatch(&item, input)
		item.Dirty = true
		item.UpdatedAt = time.Now().UTC()
		s.Issues[target] = item
		s.PendingOperations = append(s.PendingOperations, offline.Operation{
			ID:        fmt.Sprintf("op-%d", time.Now().UnixNano()),
			Entity:    "issue",
			Action:    "update",
			TargetID:  target,
			Payload:   payload,
			CreatedAt: time.Now().UTC(),
		})
		return nil
	})
	return err
}

func (c *Client) UpdateIssueStatusSmart(issueID, status string) error {
	return c.UpdateIssueSmart(issueID, UpdateIssueInput{Status: &status})
}

func (c *Client) AddIssueLabelSmart(issueID, label string) error {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return nil
	}
	if err := c.AddIssueLabel(issueID, trimmed); err == nil {
		return nil
	} else if !isConnectivityErr(err) {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := c.offlineStore(cfg)
	if err != nil {
		return err
	}
	_, err = store.Update(func(s *offline.State) error {
		s.Mode = offline.ModeOffline
		target := offline.ResolveID(*s, issueID)
		item, ok := s.Issues[target]
		if !ok {
			item = offline.Issue{ID: target}
		}
		seen := map[string]struct{}{}
		next := make([]string, 0, len(item.Labels)+1)
		for _, existing := range item.Labels {
			v := strings.TrimSpace(existing)
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			next = append(next, v)
		}
		if _, ok := seen[trimmed]; !ok {
			next = append(next, trimmed)
		}
		item.Labels = next
		item.Dirty = true
		item.UpdatedAt = time.Now().UTC()
		s.Issues[target] = item
		payload, _ := json.Marshal(UpdateIssueInput{Labels: &item.Labels})
		s.PendingOperations = append(s.PendingOperations, offline.Operation{
			ID:        fmt.Sprintf("op-%d", time.Now().UnixNano()),
			Entity:    "issue",
			Action:    "update",
			TargetID:  target,
			Payload:   payload,
			CreatedAt: time.Now().UTC(),
		})
		return nil
	})
	return err
}

func (c *Client) offlineStore(cfg config.Config) (*offline.Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(home, ".scrumx")
	scope := strings.TrimSpace(cfg.APIURL) + "|" + strings.TrimSpace(cfg.CurrentOrgID) + "|" + strings.TrimSpace(cfg.OrgID)
	return offline.NewStore(base, scope)
}

func fromIssue(issue Issue) offline.Issue {
	return offline.Issue{
		ID:          issue.ID,
		ProjectID:   issue.ProjectID,
		AssigneeID:  issue.AssigneeID,
		SprintID:    issue.SprintID,
		IssueType:   issue.IssueType,
		Title:       issue.Title,
		Description: issue.Description,
		Status:      issue.Status,
		Priority:    issue.Priority,
		Labels:      append([]string(nil), issue.Labels...),
		UpdatedAt:   issue.UpdatedAt.UTC(),
	}
}

func toIssue(item offline.Issue) Issue {
	return Issue{
		ID:          item.ID,
		ProjectID:   item.ProjectID,
		AssigneeID:  item.AssigneeID,
		SprintID:    item.SprintID,
		IssueType:   item.IssueType,
		Title:       item.Title,
		Description: item.Description,
		Status:      item.Status,
		Priority:    item.Priority,
		Labels:      append([]string(nil), item.Labels...),
		UpdatedAt:   item.UpdatedAt.UTC(),
	}
}

func toIssues(state offline.State, key string) []Issue {
	items := offline.ListIssues(state, key)
	out := make([]Issue, 0, len(items))
	for _, item := range items {
		out = append(out, toIssue(item))
	}
	return out
}

func applyPatch(issue *offline.Issue, patch UpdateIssueInput) {
	if patch.Title != nil {
		issue.Title = strings.TrimSpace(*patch.Title)
	}
	if patch.Description != nil {
		issue.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.Status != nil {
		issue.Status = strings.ToLower(strings.TrimSpace(*patch.Status))
	}
	if patch.Priority != nil {
		issue.Priority = strings.ToLower(strings.TrimSpace(*patch.Priority))
	}
	if patch.IssueType != nil {
		issue.IssueType = strings.ToLower(strings.TrimSpace(*patch.IssueType))
	}
	if patch.AssigneeID != nil {
		issue.AssigneeID = patch.AssigneeID
	}
	if patch.SprintID != nil {
		issue.SprintID = patch.SprintID
	}
	if patch.Labels != nil {
		issue.Labels = append([]string(nil), (*patch.Labels)...)
	}
}

func isConnectivityErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection refused",
		"no such host",
		"network is unreachable",
		"timeout",
		"dial tcp",
		"eof",
		"temporarily unavailable",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
