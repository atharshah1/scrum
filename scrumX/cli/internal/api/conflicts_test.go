package api

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/config"
	"github.com/atharshah1/scrum/scrumX/internal/offline"
)

func newTestClient(t *testing.T) (*Client, config.Config, *offline.Store) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfgStore := config.NewStore(cfgPath)
	cfg := config.Config{
		APIURL:       "http://example.test/api/v1",
		OrgID:        "org-1",
		CurrentOrgID: "org-1",
		UserID:       "user-1",
	}
	if err := cfgStore.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	client := NewClient(cfgStore)
	store, err := client.offlineStore(cfg)
	if err != nil {
		t.Fatalf("offline store: %v", err)
	}
	return client, cfg, store
}

func TestListAndGetIssueConflicts(t *testing.T) {
	client, _, store := newTestClient(t)
	now := time.Now().UTC()
	if err := store.Save(offline.State{
		Conflicts: []offline.ConflictRecord{
			{ID: "c-1", Entity: "issue", TargetID: "i-1", CreatedAt: now.Add(-time.Minute)},
			{ID: "c-2", Entity: "issue", TargetID: "i-2", Resolved: true, CreatedAt: now},
		},
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	open, err := client.ListIssueConflicts(ConflictFilter{})
	if err != nil {
		t.Fatalf("list open conflicts: %v", err)
	}
	if len(open) != 1 || open[0].ID != "c-1" {
		t.Fatalf("unexpected open conflicts: %+v", open)
	}

	all, err := client.ListIssueConflicts(ConflictFilter{IncludeResolved: true})
	if err != nil {
		t.Fatalf("list all conflicts: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(all))
	}

	got, err := client.GetIssueConflict("i-1", false)
	if err != nil {
		t.Fatalf("get by issue id: %v", err)
	}
	if got.ID != "c-1" {
		t.Fatalf("expected c-1, got %s", got.ID)
	}
}

func TestResolveIssueConflictKeepMineAndRepeat(t *testing.T) {
	client, _, store := newTestClient(t)
	local := Issue{ID: "i-1", ProjectID: "p-1", Title: "local title", Description: "local desc", Status: "open", Priority: "high", IssueType: "task", UpdatedAt: time.Now().UTC().Add(-time.Hour)}
	server := Issue{ID: "i-1", ProjectID: "p-1", Title: "server title", Description: "server desc", Status: "open", Priority: "medium", IssueType: "task", UpdatedAt: time.Now().UTC()}
	localRaw, _ := json.Marshal(local)
	serverRaw, _ := json.Marshal(server)
	if err := store.Save(offline.State{
		Issues: map[string]offline.Issue{
			"i-1": fromIssue(server),
		},
		Conflicts: []offline.ConflictRecord{
			{
				ID:             "c-1",
				Entity:         "issue",
				TargetID:       "i-1",
				LocalSnapshot:  localRaw,
				ServerSnapshot: serverRaw,
				Fields: []offline.ConflictField{
					{Field: "title", LocalValue: "local title", ServerValue: "server title"},
				},
				CreatedAt: time.Now().UTC(),
			},
		},
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	resolved, err := client.ResolveIssueConflict("c-1", ConflictResolutionKeepMine)
	if err != nil {
		t.Fatalf("resolve keep-mine: %v", err)
	}
	if !resolved.Resolved || resolved.Resolution != ConflictResolutionKeepMine {
		t.Fatalf("unexpected resolution state: %+v", resolved)
	}

	state, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.PendingOperations) != 1 || state.PendingOperations[0].Action != "update" {
		t.Fatalf("expected queued update operation, got %+v", state.PendingOperations)
	}
	if !state.Conflicts[0].Resolved {
		t.Fatalf("conflict should be resolved")
	}

	if _, err := client.ResolveIssueConflict("c-1", ConflictResolutionKeepServer); err == nil {
		t.Fatalf("expected repeated resolve to fail")
	}
}

func TestResolveIssueConflictKeepBothAppendsAuditNote(t *testing.T) {
	client, _, store := newTestClient(t)
	local := Issue{ID: "i-2", ProjectID: "p-1", Title: "my title", Description: "existing", Status: "open", Priority: "high", IssueType: "task", UpdatedAt: time.Now().UTC().Add(-time.Hour), Labels: []string{"a"}}
	server := Issue{ID: "i-2", ProjectID: "p-1", Title: "server title", Description: "remote", Status: "open", Priority: "medium", IssueType: "task", UpdatedAt: time.Now().UTC(), Labels: []string{"b"}}
	localRaw, _ := json.Marshal(local)
	serverRaw, _ := json.Marshal(server)
	if err := store.Save(offline.State{
		Issues: map[string]offline.Issue{
			"i-2": fromIssue(server),
		},
		Conflicts: []offline.ConflictRecord{
			{
				ID:             "c-2",
				Entity:         "issue",
				TargetID:       "i-2",
				LocalSnapshot:  localRaw,
				ServerSnapshot: serverRaw,
				Fields: []offline.ConflictField{
					{Field: "title", LocalValue: "my title", ServerValue: "server title"},
					{Field: "labels", LocalValue: "a", ServerValue: "b"},
				},
				CreatedAt: time.Now().UTC(),
			},
		},
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if _, err := client.ResolveIssueConflict("i-2", ConflictResolutionKeepBoth); err != nil {
		t.Fatalf("resolve keep-both: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	got := state.Issues["i-2"].Description
	if !strings.Contains(got, "keep-both resolution") {
		t.Fatalf("expected keep-both note in description: %s", got)
	}
	if !strings.Contains(got, "title server value preserved") {
		t.Fatalf("expected scalar server value note: %s", got)
	}
}

func TestResolveIssueConflictMissingSnapshotFailsForKeepMine(t *testing.T) {
	client, _, store := newTestClient(t)
	if err := store.Save(offline.State{
		Conflicts: []offline.ConflictRecord{
			{ID: "c-3", Entity: "issue", TargetID: "i-3", CreatedAt: time.Now().UTC()},
		},
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if _, err := client.ResolveIssueConflict("c-3", ConflictResolutionKeepMine); err == nil {
		t.Fatalf("expected keep-mine to fail without local snapshot")
	}
}
