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
	if err := c.syncPush(store); err != nil {
		_, _ = store.Update(func(s *offline.State) error {
			s.Mode = offline.ModeOffline
			return nil
		})
		return err
	}
	if err := c.syncPull(store); err != nil {
		_, _ = store.Update(func(s *offline.State) error {
			s.Mode = offline.ModeOffline
			return nil
		})
		return err
	}
	_, err = store.Update(func(s *offline.State) error {
		s.Mode = offline.ModeOnline
		s.LastSyncedAt = time.Now().UTC()
		return nil
	})
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
		var expectedUpdatedAt *time.Time
		if !item.UpdatedAt.IsZero() {
			ts := item.UpdatedAt.UTC()
			expectedUpdatedAt = &ts
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
		payload, _ := json.Marshal(UpdateIssueInput{Labels: &item.Labels, UpdatedAt: expectedUpdatedAt})
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

func (c *Client) syncPull(store *offline.Store) error {
	state, err := store.Load()
	if err != nil {
		return err
	}
	since := state.EntitySync["issues"].LastVersion
	collected := make([]Issue, 0, 200)
	maxSeen := since
	page := 1
	for {
		chunk, err := c.ListIssuesSince(since, page, 100)
		if err != nil {
			return err
		}
		if len(chunk) == 0 {
			break
		}
		for _, item := range chunk {
			if item.UpdatedAt.After(maxSeen) {
				maxSeen = item.UpdatedAt
			}
			if since.IsZero() || item.UpdatedAt.After(since) {
				collected = append(collected, item)
			}
		}
		if len(chunk) < 100 {
			break
		}
		page++
		if page > 50 {
			break
		}
	}
	now := time.Now().UTC()
	_, err = store.Update(func(s *offline.State) error {
		for _, it := range collected {
			item := fromIssue(it)
			item.Dirty = false
			item.LocalOnly = false
			item.Deleted = false
			s.Issues[it.ID] = item
			if strings.TrimSpace(it.ProjectID) != "" {
				s.Projects[it.ProjectID] = offline.Project{ID: it.ProjectID}
			}
		}
		s.EntitySync["issues"] = offline.SyncEntityMeta{LastSyncedAt: now, LastVersion: maxSeen}
		return nil
	})
	return err
}

func (c *Client) syncPush(store *offline.Store) error {
	state, err := store.Load()
	if err != nil {
		return err
	}
	if len(state.PendingOperations) == 0 {
		return nil
	}
	nextQueue := make([]offline.Operation, 0, len(state.PendingOperations))
	var connectivityErr error
	for idx, op := range state.PendingOperations {
		if !op.NextAttemptAt.IsZero() && op.NextAttemptAt.After(time.Now()) {
			nextQueue = append(nextQueue, op)
			continue
		}
		target := offline.ResolveID(state, op.TargetID)
		switch op.Action {
		case "update":
			var payload UpdateIssueInput
			_ = json.Unmarshal(op.Payload, &payload)
			if payload.UpdatedAt == nil {
				if local, ok := state.Issues[target]; ok && !local.UpdatedAt.IsZero() {
					ts := local.UpdatedAt.UTC()
					payload.UpdatedAt = &ts
				}
			}
			updateErr := c.UpdateIssue(target, payload)
			if updateErr != nil {
				if strings.Contains(strings.ToLower(updateErr.Error()), "409") || strings.Contains(strings.ToLower(updateErr.Error()), "conflict") {
					latest, latestErr := c.GetIssue(target)
					if latestErr == nil {
						local := fromIssue(latest)
						applyPatch(&local, payload)
						merged := toUpdateInput(local)
						updateErr = c.UpdateIssue(target, merged)
					}
				}
			}
			if updateErr != nil {
				op.Attempts++
				op.LastError = updateErr.Error()
				op.NextAttemptAt = time.Now().Add(offline.Backoff(op.Attempts))
				if isConnectivityErr(updateErr) {
					nextQueue = append(nextQueue, op)
					nextQueue = append(nextQueue, state.PendingOperations[idx+1:]...)
					connectivityErr = updateErr
					break
				}
				nextQueue = append(nextQueue, op)
				continue
			}
			if latest, latestErr := c.GetIssue(target); latestErr == nil {
				item := fromIssue(latest)
				item.Dirty = false
				item.LocalOnly = false
				item.Deleted = false
				state.Issues[target] = item
			} else if local, ok := state.Issues[target]; ok {
				applyPatch(&local, payload)
				local.Dirty = false
				local.LocalOnly = false
				local.Deleted = false
				local.UpdatedAt = time.Now().UTC()
				state.Issues[target] = local
			}
		default:
			nextQueue = append(nextQueue, op)
		}
	}
	state.PendingOperations = nextQueue
	if len(state.PendingOperations) == 0 {
		for id, item := range state.Issues {
			item.Dirty = false
			item.LocalOnly = false
			state.Issues[id] = item
		}
	}
	now := time.Now().UTC()
	state.EntitySync["issues"] = offline.SyncEntityMeta{
		LastSyncedAt: now,
		LastVersion:  state.EntitySync["issues"].LastVersion,
	}
	if connectivityErr != nil {
		state.Mode = offline.ModeOffline
	}
	if err := store.Save(state); err != nil {
		return err
	}
	return connectivityErr
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

func toUpdateInput(issue offline.Issue) UpdateIssueInput {
	title := issue.Title
	description := issue.Description
	status := issue.Status
	priority := issue.Priority
	issueType := issue.IssueType
	labels := append([]string(nil), issue.Labels...)
	return UpdateIssueInput{
		Title:       &title,
		Description: &description,
		Status:      &status,
		Priority:    &priority,
		IssueType:   &issueType,
		AssigneeID:  issue.AssigneeID,
		SprintID:    issue.SprintID,
		Labels:      &labels,
		UpdatedAt:   &issue.UpdatedAt,
	}
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
