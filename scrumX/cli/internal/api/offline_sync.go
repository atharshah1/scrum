package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/config"
	"github.com/atharshah1/scrum/scrumX/internal/offline"
)

type SyncStatus struct {
	Mode             string
	LastSyncedAt     time.Time
	PendingOps       int
	PendingConflicts int
	DroppedOps       int
}

const issueProjectSyncKeyPrefix = "issues_by_project:"

func (c *Client) SyncStatus() (SyncStatus, error) {
	cfg, err := c.cfgStore.Load()
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
	return SyncStatus{
		Mode:             state.Mode,
		LastSyncedAt:     state.LastSyncedAt,
		PendingOps:       len(state.PendingOperations),
		PendingConflicts: unresolvedConflicts(state),
		DroppedOps:       state.DroppedOperations,
	}, nil
}

func (c *Client) SyncNow() error {
	cfg, err := c.cfgStore.Load()
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

func (c *Client) CreateIssueSmart(input CreateIssueInput) (Issue, error) {
	created, err := c.CreateIssue(input)
	cfg, cfgErr := c.cfgStore.Load()
	if cfgErr != nil {
		return created, err
	}
	store, storeErr := c.offlineStore(cfg)
	if storeErr != nil {
		return created, err
	}
	if err == nil {
		_, _ = store.Update(func(s *offline.State) error {
			upsertIssueInState(s, fromIssue(created))
			return nil
		})
		_ = c.SyncNow()
		return created, nil
	}
	if !isConnectivityErr(err) {
		return Issue{}, err
	}
	localID := "local-" + fmt.Sprintf("%d", time.Now().UnixNano())
	now := time.Now().UTC()
	local := Issue{
		ID:          localID,
		ProjectID:   strings.TrimSpace(input.ProjectID),
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		IssueType:   strings.TrimSpace(input.IssueType),
		Priority:    strings.TrimSpace(input.Priority),
		Status:      "open",
		Labels:      cleanLabels(input.Labels),
	}
	if local.IssueType == "" {
		local.IssueType = "task"
	}
	if local.Priority == "" {
		local.Priority = "medium"
	}
	if strings.TrimSpace(input.ParentID) != "" {
		v := strings.TrimSpace(input.ParentID)
		local.ParentID = &v
	}
	if strings.TrimSpace(input.SprintID) != "" {
		v := strings.TrimSpace(input.SprintID)
		local.SprintID = &v
	}
	if strings.TrimSpace(input.AssigneeID) != "" {
		v := strings.TrimSpace(input.AssigneeID)
		local.AssigneeID = &v
	}
	body, _ := json.Marshal(input)
	_, _ = store.Update(func(s *offline.State) error {
		s.Mode = offline.ModeOffline
		item := fromIssue(local)
		item.Dirty = true
		item.LocalOnly = true
		item.UpdatedAt = now
		upsertIssueInState(s, item)
		offline.EnqueueOperation(s, offline.Operation{
			ID:        fmt.Sprintf("op-%d", time.Now().UnixNano()),
			Entity:    "issue",
			Action:    "create",
			TargetID:  localID,
			Payload:   body,
			CreatedAt: now,
		})
		return nil
	})
	return local, nil
}

func (c *Client) UpdateIssueSmart(id string, input UpdateIssueInput) (Issue, error) {
	cfg, cfgErr := c.cfgStore.Load()
	if cfgErr != nil {
		return Issue{}, cfgErr
	}
	store, storeErr := c.offlineStore(cfg)
	if storeErr != nil {
		return Issue{}, storeErr
	}
	state, loadErr := store.Load()
	if loadErr == nil && input.UpdatedAt == nil {
		target := offline.ResolveID(state, id)
		if local, ok := state.Issues[target]; ok && !local.UpdatedAt.IsZero() {
			ts := local.UpdatedAt.UTC()
			input.UpdatedAt = &ts
		}
	}
	updated, err := c.UpdateIssue(id, input)
	if err == nil {
		_, _ = store.Update(func(s *offline.State) error {
			upsertIssueInState(s, fromIssue(updated))
			return nil
		})
		_ = c.SyncNow()
		return updated, nil
	}
	if !isConnectivityErr(err) {
		return Issue{}, err
	}
	var optimistic Issue
	state, stateErr := store.Update(func(s *offline.State) error {
		s.Mode = offline.ModeOffline
		target := offline.ResolveID(*s, id)
		item, ok := s.Issues[target]
		if !ok {
			item = offline.Issue{ID: target}
		}
		applyIssuePatch(&item, input)
		item.Dirty = true
		item.UpdatedAt = time.Now().UTC()
		upsertIssueInState(s, item)
		payload, _ := json.Marshal(input)
		offline.EnqueueOperation(s, offline.Operation{
			ID:        fmt.Sprintf("op-%d", time.Now().UnixNano()),
			Entity:    "issue",
			Action:    "update",
			TargetID:  target,
			Payload:   payload,
			CreatedAt: time.Now().UTC(),
		})
		optimistic = toIssue(item)
		return nil
	})
	if stateErr != nil {
		return Issue{}, stateErr
	}
	_ = state
	return optimistic, nil
}

func (c *Client) DeleteIssueSmart(id string) error {
	err := c.DeleteIssue(id)
	cfg, cfgErr := c.cfgStore.Load()
	if cfgErr != nil {
		return err
	}
	store, storeErr := c.offlineStore(cfg)
	if storeErr != nil {
		return err
	}
	if err == nil {
		_, _ = store.Update(func(s *offline.State) error {
			target := offline.ResolveID(*s, id)
			delete(s.Issues, target)
			return nil
		})
		_ = c.SyncNow()
		return nil
	}
	if !isConnectivityErr(err) {
		return err
	}
	_, _ = store.Update(func(s *offline.State) error {
		s.Mode = offline.ModeOffline
		target := offline.ResolveID(*s, id)
		if item, ok := s.Issues[target]; ok {
			item.Deleted = true
			item.Dirty = true
			item.UpdatedAt = time.Now().UTC()
			s.Issues[target] = item
		}
		offline.EnqueueOperation(s, offline.Operation{
			ID:        fmt.Sprintf("op-%d", time.Now().UnixNano()),
			Entity:    "issue",
			Action:    "delete",
			TargetID:  target,
			CreatedAt: time.Now().UTC(),
		})
		return nil
	})
	return nil
}

func (c *Client) ListIssuesSmart(filter IssueListFilter) ([]Issue, error) {
	items, err := c.ListIssues(filter)
	cfg, cfgErr := c.cfgStore.Load()
	if cfgErr != nil {
		return items, err
	}
	store, storeErr := c.offlineStore(cfg)
	if storeErr != nil {
		return items, err
	}
	key := issueQueryKey(filter)
	if err == nil {
		_, _ = store.Update(func(s *offline.State) error {
			ids := make([]string, 0, len(items))
			for _, it := range items {
				upsertIssueInState(s, fromIssue(it))
				if strings.TrimSpace(it.ProjectID) != "" {
					s.Projects[it.ProjectID] = offline.Project{ID: it.ProjectID}
				}
				ids = append(ids, it.ID)
			}
			s.IssueQueryCache[key] = ids
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
	return filterIssues(offline.ListIssues(state, key), filter), nil
}

func (c *Client) SearchIssuesSmart(queryText string, page, limit int) ([]Issue, error) {
	items, err := c.SearchIssues(queryText, page, limit)
	cfg, cfgErr := c.cfgStore.Load()
	if cfgErr != nil {
		return items, err
	}
	store, storeErr := c.offlineStore(cfg)
	if storeErr != nil {
		return items, err
	}
	key := "search:" + strings.TrimSpace(queryText)
	if err == nil {
		_, _ = store.Update(func(s *offline.State) error {
			ids := make([]string, 0, len(items))
			for _, it := range items {
				upsertIssueInState(s, fromIssue(it))
				ids = append(ids, it.ID)
			}
			s.IssueQueryCache[key] = ids
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
	issues := offline.ListIssues(state, key)
	if len(issues) == 0 {
		issues = offline.ListIssues(state, "")
	}
	needle := strings.ToLower(strings.TrimSpace(queryText))
	filtered := make([]Issue, 0, len(issues))
	for _, item := range issues {
		if needle == "" || strings.Contains(strings.ToLower(item.Title), needle) || strings.Contains(strings.ToLower(item.Description), needle) {
			filtered = append(filtered, toIssue(item))
		}
	}
	return filtered, nil
}

func (c *Client) GetIssueSmart(id string) (Issue, error) {
	item, err := c.GetIssue(id)
	cfg, cfgErr := c.cfgStore.Load()
	if cfgErr != nil {
		return item, err
	}
	store, storeErr := c.offlineStore(cfg)
	if storeErr != nil {
		return item, err
	}
	if err == nil {
		_, _ = store.Update(func(s *offline.State) error {
			upsertIssueInState(s, fromIssue(item))
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

func (c *Client) DeleteIssue(id string) error {
	_, err := c.authedRequest(http.MethodDelete, "/issues/"+strings.TrimSpace(id), nil, nil)
	if err == nil {
		c.invalidateIssueCache()
	}
	return err
}

func (c *Client) offlineStore(cfg config.Config) (*offline.Store, error) {
	baseDir := filepath.Dir(c.cfgStore.Path())
	scope := strings.TrimSpace(cfg.APIURL) + "|" + strings.TrimSpace(cfg.CurrentOrgID) + "|" + strings.TrimSpace(cfg.OrgID)
	return offline.NewStore(baseDir, scope)
}

func (c *Client) syncPull(store *offline.Store) error {
	state, err := store.Load()
	if err != nil {
		return err
	}
	since := state.EntitySync["issues"].LastVersion
	users, _ := c.fetchUsersRemote()
	collected := make([]Issue, 0, 200)
	maxSeen := since
	projectMaxSeen := map[string]time.Time{}
	page := 1
	for {
		filter := IssueListFilter{Page: page, Limit: 100, SortBy: "updated_at", Order: "asc"}
		if !since.IsZero() {
			s := since
			filter.UpdatedSince = &s
		}
		chunk, err := c.ListIssues(filter)
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
			projectID := strings.TrimSpace(item.ProjectID)
			if projectID != "" && item.UpdatedAt.After(projectMaxSeen[projectID]) {
				projectMaxSeen[projectID] = item.UpdatedAt
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
			upsertIssueInState(s, item)
			if strings.TrimSpace(it.ProjectID) != "" {
				s.Projects[it.ProjectID] = offline.Project{ID: it.ProjectID}
			}
		}
		for _, user := range users {
			s.Users[user.ID] = offline.User{ID: user.ID, Email: user.Email, Role: user.Role}
		}
		s.EntitySync["issues"] = offline.SyncEntityMeta{LastSyncedAt: now, LastVersion: maxSeen}
		for projectID, lastVersion := range projectMaxSeen {
			s.EntitySync[issueProjectSyncKeyPrefix+projectID] = offline.SyncEntityMeta{LastSyncedAt: now, LastVersion: lastVersion}
		}
		s.EntitySync["users"] = offline.SyncEntityMeta{LastSyncedAt: now}
		s.EntitySync["projects"] = offline.SyncEntityMeta{LastSyncedAt: now}
		return nil
	})
	return err
}

func (c *Client) syncPush(store *offline.Store) error {
	state, err := store.Load()
	if err != nil {
		return err
	}
	cfg, _ := c.cfgStore.Load()
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
		case "create":
			var payload CreateIssueInput
			_ = json.Unmarshal(op.Payload, &payload)
			created, createErr := c.CreateIssue(payload)
			if createErr != nil {
				op.Attempts++
				op.LastError = createErr.Error()
				op.NextAttemptAt = time.Now().Add(offline.Backoff(op.Attempts))
				if isConnectivityErr(createErr) {
					nextQueue = append(nextQueue, op)
					nextQueue = append(nextQueue, state.PendingOperations[idx+1:]...)
					connectivityErr = createErr
					break
				}
				nextQueue = append(nextQueue, op)
				continue
			}
			state.IDAliases[op.TargetID] = created.ID
			delete(state.Issues, op.TargetID)
			upsertIssueInState(&state, fromIssue(created))
		case "update":
			var payload UpdateIssueInput
			_ = json.Unmarshal(op.Payload, &payload)
			if payload.UpdatedAt == nil {
				if local, ok := state.Issues[target]; ok && !local.UpdatedAt.IsZero() {
					ts := local.UpdatedAt.UTC()
					payload.UpdatedAt = &ts
				}
			}
			effectivePayload := payload
			updated, updateErr := c.UpdateIssue(target, payload)
			if updateErr != nil && isConflictErr(updateErr) {
				latest, latestErr := c.GetIssue(target)
				if latestErr == nil {
					local := state.Issues[target]
					localSnapshot, _ := json.Marshal(toIssue(local))
					serverSnapshot, _ := json.Marshal(latest)
					offline.UpsertConflict(&state, offline.ConflictRecord{
						Entity:         "issue",
						TargetID:       target,
						OperationID:    op.ID,
						ActorID:        strings.TrimSpace(cfg.UserID),
						LocalSnapshot:  localSnapshot,
						ServerSnapshot: serverSnapshot,
						Fields:         conflictFields(payload, local, latest),
					})
					upsertIssueInState(&state, fromIssue(latest))
					continue
				}
				updateErr = latestErr
			}
			if updateErr != nil {
				op.Attempts++
				op.LastError = fmt.Sprintf("sync update failed for %s: %v", target, updateErr)
				op.NextAttemptAt = time.Now().Add(offline.Backoff(op.Attempts))
				if isConflictErr(updateErr) {
					if body, marshalErr := json.Marshal(effectivePayload); marshalErr == nil {
						op.Payload = body
					}
				}
				if isConnectivityErr(updateErr) {
					nextQueue = append(nextQueue, op)
					nextQueue = append(nextQueue, state.PendingOperations[idx+1:]...)
					connectivityErr = updateErr
					break
				}
				nextQueue = append(nextQueue, op)
				continue
			}
			upsertIssueInState(&state, fromIssue(updated))
		case "delete":
			if strings.HasPrefix(target, "local-") {
				delete(state.Issues, target)
				continue
			}
			deleteErr := c.DeleteIssue(target)
			if deleteErr != nil {
				if isNotFoundErr(deleteErr) {
					delete(state.Issues, target)
					continue
				}
				op.Attempts++
				op.LastError = deleteErr.Error()
				op.NextAttemptAt = time.Now().Add(offline.Backoff(op.Attempts))
				if isConnectivityErr(deleteErr) {
					nextQueue = append(nextQueue, op)
					nextQueue = append(nextQueue, state.PendingOperations[idx+1:]...)
					connectivityErr = deleteErr
					break
				}
				nextQueue = append(nextQueue, op)
				continue
			}
			delete(state.Issues, target)
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

func unresolvedConflicts(state offline.State) int {
	count := 0
	for _, conflict := range state.Conflicts {
		if !conflict.Resolved {
			count++
		}
	}
	return count
}

func conflictFields(payload UpdateIssueInput, local offline.Issue, latest Issue) []offline.ConflictField {
	fields := make([]offline.ConflictField, 0, 8)
	add := func(field, localValue, serverValue string) {
		if strings.TrimSpace(localValue) == strings.TrimSpace(serverValue) {
			return
		}
		fields = append(fields, offline.ConflictField{
			Field:       field,
			LocalValue:  localValue,
			ServerValue: serverValue,
		})
	}
	if payload.Title != nil {
		add("title", strings.TrimSpace(local.Title), strings.TrimSpace(latest.Title))
	}
	if payload.Description != nil {
		add("description", local.Description, latest.Description)
	}
	if payload.Status != nil {
		add("status", strings.TrimSpace(local.Status), strings.TrimSpace(latest.Status))
	}
	if payload.Priority != nil {
		add("priority", strings.TrimSpace(local.Priority), strings.TrimSpace(latest.Priority))
	}
	if payload.IssueType != nil {
		add("issue_type", strings.TrimSpace(local.IssueType), strings.TrimSpace(latest.IssueType))
	}
	if payload.AssigneeID != nil {
		localID := ""
		if local.AssigneeID != nil {
			localID = strings.TrimSpace(*local.AssigneeID)
		}
		serverID := ""
		if latest.AssigneeID != nil {
			serverID = strings.TrimSpace(*latest.AssigneeID)
		}
		add("assignee_id", localID, serverID)
	}
	if payload.SprintID != nil {
		localID := ""
		if local.SprintID != nil {
			localID = strings.TrimSpace(*local.SprintID)
		}
		serverID := ""
		if latest.SprintID != nil {
			serverID = strings.TrimSpace(*latest.SprintID)
		}
		add("sprint_id", localID, serverID)
	}
	if payload.ParentID != nil {
		localID := ""
		if local.ParentID != nil {
			localID = strings.TrimSpace(*local.ParentID)
		}
		serverID := ""
		if latest.ParentID != nil {
			serverID = strings.TrimSpace(*latest.ParentID)
		}
		add("parent_id", localID, serverID)
	}
	if payload.Labels != nil {
		add("labels", strings.Join(cleanLabels(local.Labels), ", "), strings.Join(cleanLabels(latest.Labels), ", "))
	}
	return fields
}

func (c *Client) fetchUsersRemote() ([]User, error) {
	var out envelope[[]User]
	_, err := c.authedRequest(http.MethodGet, "/users", nil, &out)
	return out.Data, err
}

func upsertIssueInState(state *offline.State, issue offline.Issue) {
	if state.Issues == nil {
		state.Issues = map[string]offline.Issue{}
	}
	if strings.TrimSpace(issue.ID) == "" {
		return
	}
	state.Issues[issue.ID] = issue
}

func fromIssue(issue Issue) offline.Issue {
	return offline.Issue{
		ID:          strings.TrimSpace(issue.ID),
		ProjectID:   strings.TrimSpace(issue.ProjectID),
		ParentID:    issue.ParentID,
		SprintID:    issue.SprintID,
		AssigneeID:  issue.AssigneeID,
		IssueType:   strings.TrimSpace(issue.IssueType),
		Title:       strings.TrimSpace(issue.Title),
		Description: issue.Description,
		Status:      strings.TrimSpace(issue.Status),
		Priority:    strings.TrimSpace(issue.Priority),
		Labels:      cleanLabels(issue.Labels),
		UpdatedAt:   issue.UpdatedAt.UTC(),
	}
}

func toIssue(issue offline.Issue) Issue {
	return Issue{
		ID:          issue.ID,
		ProjectID:   issue.ProjectID,
		ParentID:    issue.ParentID,
		SprintID:    issue.SprintID,
		AssigneeID:  issue.AssigneeID,
		IssueType:   issue.IssueType,
		Title:       issue.Title,
		Description: issue.Description,
		Status:      issue.Status,
		Priority:    issue.Priority,
		Labels:      cleanLabels(issue.Labels),
		UpdatedAt:   issue.UpdatedAt.UTC(),
	}
}

func toUpdateInput(issue offline.Issue) UpdateIssueInput {
	title := issue.Title
	description := issue.Description
	status := issue.Status
	priority := issue.Priority
	issueType := issue.IssueType
	labels := cleanLabels(issue.Labels)
	return UpdateIssueInput{
		ParentID:    issue.ParentID,
		SprintID:    issue.SprintID,
		AssigneeID:  issue.AssigneeID,
		Title:       &title,
		Description: &description,
		Status:      &status,
		Priority:    &priority,
		IssueType:   &issueType,
		Labels:      &labels,
		UpdatedAt:   &issue.UpdatedAt,
	}
}

func applyIssuePatch(issue *offline.Issue, patch UpdateIssueInput) {
	if patch.ParentID != nil {
		issue.ParentID = patch.ParentID
	}
	if patch.SprintID != nil {
		issue.SprintID = patch.SprintID
	}
	if patch.AssigneeID != nil {
		issue.AssigneeID = patch.AssigneeID
	}
	if patch.Title != nil {
		issue.Title = strings.TrimSpace(*patch.Title)
	}
	if patch.Description != nil {
		issue.Description = *patch.Description
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
	if patch.Labels != nil {
		issue.Labels = cleanLabels(*patch.Labels)
	}
}

func issueQueryKey(filter IssueListFilter) string {
	return strings.Join([]string{
		"list",
		strings.TrimSpace(filter.ProjectID),
		strings.TrimSpace(filter.Status),
		strings.TrimSpace(filter.AssigneeID),
		strings.TrimSpace(filter.SprintID),
		strings.TrimSpace(filter.Label),
		strings.TrimSpace(filter.IssueType),
		strings.TrimSpace(filter.Query),
		strings.TrimSpace(filter.SortBy),
		strings.TrimSpace(filter.Order),
		fmt.Sprintf("%d", filter.Page),
		fmt.Sprintf("%d", filter.Limit),
	}, "|")
}

func filterIssues(all []offline.Issue, filter IssueListFilter) []Issue {
	out := make([]Issue, 0, len(all))
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	for _, item := range all {
		if item.Deleted {
			continue
		}
		if v := strings.TrimSpace(filter.ProjectID); v != "" && item.ProjectID != v {
			continue
		}
		if v := strings.ToLower(strings.TrimSpace(filter.Status)); v != "" && strings.ToLower(item.Status) != v {
			continue
		}
		if v := strings.TrimSpace(filter.AssigneeID); v != "" {
			if item.AssigneeID == nil || strings.TrimSpace(*item.AssigneeID) != v {
				continue
			}
		}
		if v := strings.TrimSpace(filter.SprintID); v != "" {
			if item.SprintID == nil || strings.TrimSpace(*item.SprintID) != v {
				continue
			}
		}
		if v := strings.ToLower(strings.TrimSpace(filter.IssueType)); v != "" && strings.ToLower(item.IssueType) != v {
			continue
		}
		if v := strings.ToLower(strings.TrimSpace(filter.Label)); v != "" {
			matched := false
			for _, label := range item.Labels {
				if strings.EqualFold(strings.TrimSpace(label), v) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Title), query) && !strings.Contains(strings.ToLower(item.Description), query) {
			continue
		}
		out = append(out, toIssue(item))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
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

func isConflictErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "409") || strings.Contains(msg, "conflict") || strings.Contains(msg, "optimistic lock")
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "not found")
}

func (c *Client) resolveIssueUpdateConflict(target string, state offline.State, payload UpdateIssueInput) (Issue, UpdateIssueInput, error) {
	latest, err := c.GetIssue(target)
	if err != nil {
		return Issue{}, payload, err
	}
	merged := fromIssue(latest)
	if local, ok := state.Issues[target]; ok && local.UpdatedAt.After(merged.UpdatedAt) {
		merged = local
	}
	applyIssuePatch(&merged, payload)
	retry := toUpdateInput(merged)
	latestTS := latest.UpdatedAt.UTC()
	retry.UpdatedAt = &latestTS
	updated, err := c.UpdateIssue(target, retry)
	return updated, retry, err
}
