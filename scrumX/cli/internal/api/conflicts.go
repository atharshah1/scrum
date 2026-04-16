package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/internal/offline"
)

const (
	ConflictResolutionKeepMine   = "keep-mine"
	ConflictResolutionKeepServer = "keep-server"
	ConflictResolutionKeepBoth   = "keep-both"
	ConflictResolutionLater      = "later"
)

type ConflictFilter struct {
	IssueID         string
	IncludeResolved bool
}

func (c *Client) ListIssueConflicts(filter ConflictFilter) ([]offline.ConflictRecord, error) {
	cfg, err := c.cfgStore.Load()
	if err != nil {
		return nil, err
	}
	store, err := c.offlineStore(cfg)
	if err != nil {
		return nil, err
	}
	state, err := store.Load()
	if err != nil {
		return nil, err
	}
	targetID := strings.TrimSpace(filter.IssueID)
	out := make([]offline.ConflictRecord, 0, len(state.Conflicts))
	for _, conflict := range state.Conflicts {
		if !filter.IncludeResolved && conflict.Resolved {
			continue
		}
		if targetID != "" && conflict.TargetID != targetID {
			continue
		}
		out = append(out, conflict)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Resolved != out[j].Resolved {
			return !out[i].Resolved
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (c *Client) GetIssueConflict(idOrIssueID string, includeResolved bool) (offline.ConflictRecord, error) {
	needle := strings.TrimSpace(idOrIssueID)
	if needle == "" {
		return offline.ConflictRecord{}, fmt.Errorf("conflict id or issue id is required")
	}
	items, err := c.ListIssueConflicts(ConflictFilter{IncludeResolved: includeResolved})
	if err != nil {
		return offline.ConflictRecord{}, err
	}
	var byIssue *offline.ConflictRecord
	for i := range items {
		if items[i].ID == needle {
			return items[i], nil
		}
		if items[i].TargetID == needle && byIssue == nil {
			byIssue = &items[i]
		}
	}
	if byIssue != nil {
		return *byIssue, nil
	}
	return offline.ConflictRecord{}, fmt.Errorf("conflict not found: %s", needle)
}

func (c *Client) ResolveIssueConflict(idOrIssueID, action string) (offline.ConflictRecord, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if !isValidResolutionAction(action) {
		return offline.ConflictRecord{}, fmt.Errorf("invalid resolution action %q", action)
	}
	cfg, err := c.cfgStore.Load()
	if err != nil {
		return offline.ConflictRecord{}, err
	}
	store, err := c.offlineStore(cfg)
	if err != nil {
		return offline.ConflictRecord{}, err
	}
	target := strings.TrimSpace(idOrIssueID)
	if target == "" {
		return offline.ConflictRecord{}, fmt.Errorf("conflict id or issue id is required")
	}
	now := time.Now().UTC()
	var resolved offline.ConflictRecord
	_, err = store.Update(func(s *offline.State) error {
		idx := -1
		for i := range s.Conflicts {
			if s.Conflicts[i].ID == target {
				idx = i
				break
			}
		}
		if idx < 0 {
			for i := range s.Conflicts {
				if s.Conflicts[i].TargetID == target && !s.Conflicts[i].Resolved {
					idx = i
					break
				}
			}
		}
		if idx < 0 {
			return fmt.Errorf("conflict not found: %s", target)
		}
		conflict := s.Conflicts[idx]
		if conflict.Resolved {
			return fmt.Errorf("conflict already resolved")
		}

		var localIssue Issue
		if len(conflict.LocalSnapshot) > 0 {
			_ = json.Unmarshal(conflict.LocalSnapshot, &localIssue)
		}
		var serverIssue Issue
		if len(conflict.ServerSnapshot) > 0 {
			_ = json.Unmarshal(conflict.ServerSnapshot, &serverIssue)
		}
		targetID := strings.TrimSpace(conflict.TargetID)
		if targetID == "" {
			targetID = strings.TrimSpace(serverIssue.ID)
		}
		if targetID == "" {
			targetID = strings.TrimSpace(localIssue.ID)
		}
		conflict.TargetID = targetID

		switch action {
		case ConflictResolutionKeepServer, ConflictResolutionLater:
			if strings.TrimSpace(serverIssue.ID) != "" {
				item := fromIssue(serverIssue)
				item.Dirty = false
				item.LocalOnly = false
				item.Deleted = false
				upsertIssueInState(s, item)
			}
		case ConflictResolutionKeepMine, ConflictResolutionKeepBoth:
			if strings.TrimSpace(localIssue.ID) == "" {
				return fmt.Errorf("local snapshot missing; cannot apply %s", action)
			}
			item := fromIssue(localIssue)
			item.Dirty = true
			item.LocalOnly = false
			item.Deleted = false
			item.UpdatedAt = now
			if current, ok := s.Issues[targetID]; ok && !current.UpdatedAt.IsZero() {
				item.UpdatedAt = current.UpdatedAt.UTC()
			}
			if action == ConflictResolutionKeepBoth {
				item.Description = appendKeepBothNote(item.Description, conflict, cfg.UserID)
			}
			upsertIssueInState(s, item)
			payload := toUpdateInput(item)
			if body, marshalErr := json.Marshal(payload); marshalErr == nil {
				offline.EnqueueOperation(s, offline.Operation{
					ID:        fmt.Sprintf("op-%d", time.Now().UnixNano()),
					Entity:    "issue",
					Action:    "update",
					TargetID:  targetID,
					Payload:   body,
					CreatedAt: now,
				})
			}
		}

		conflict.Resolved = true
		conflict.Resolution = action
		conflict.ResolvedByID = strings.TrimSpace(cfg.UserID)
		conflict.ResolvedByName = strings.TrimSpace(cfg.UserID)
		conflict.ResolvedAt = now
		s.Conflicts[idx] = conflict
		resolved = conflict
		return nil
	})
	if err != nil {
		return offline.ConflictRecord{}, err
	}
	return resolved, nil
}

func isValidResolutionAction(action string) bool {
	switch action {
	case ConflictResolutionKeepMine, ConflictResolutionKeepServer, ConflictResolutionKeepBoth, ConflictResolutionLater:
		return true
	default:
		return false
	}
}

func appendKeepBothNote(description string, conflict offline.ConflictRecord, resolverID string) string {
	resolver := strings.TrimSpace(resolverID)
	if resolver == "" {
		resolver = "unknown"
	}
	lines := []string{
		"",
		fmt.Sprintf("[conflict:%s] keep-both resolution by %s at %s", conflict.ID, resolver, time.Now().UTC().Format(time.RFC3339)),
	}
	for _, field := range conflict.Fields {
		if isAdditiveConflictField(field.Field) {
			continue
		}
		serverValue := strings.TrimSpace(field.ServerValue)
		if serverValue == "" {
			serverValue = "<empty>"
		}
		lines = append(lines, fmt.Sprintf("- %s server value preserved: %s", field.Field, serverValue))
	}
	note := strings.Join(lines, "\n")
	base := strings.TrimSpace(description)
	if base == "" {
		return strings.TrimSpace(note)
	}
	return base + "\n\n" + strings.TrimSpace(note)
}

func isAdditiveConflictField(field string) bool {
	return strings.EqualFold(strings.TrimSpace(field), "labels")
}
