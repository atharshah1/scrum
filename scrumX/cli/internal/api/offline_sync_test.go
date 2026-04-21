package api

import (
	"strings"
	"testing"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/config"
	"github.com/atharshah1/scrum/scrumX/internal/offline"
)

func TestCreateIssueSmartQueuesOfflineAndUpdatesSyncStatus(t *testing.T) {
	client, cfg, store := newTestClient(t)
	cfg.APIURL = "http://127.0.0.1:1/api/v1"
	cfg.CurrentProjectID = "project-1"
	if err := client.cfgStore.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	issue, err := client.CreateIssueSmart(CreateIssueInput{
		Title:     "Offline-safe issue",
		ProjectID: cfg.CurrentProjectID,
		Labels:    []string{"sync"},
	})
	if err != nil {
		t.Fatalf("create issue smart: %v", err)
	}
	if !strings.HasPrefix(issue.ID, "local-") {
		t.Fatalf("expected local issue id, got %q", issue.ID)
	}
	if issue.Status != "open" {
		t.Fatalf("expected default open status, got %q", issue.Status)
	}

	state, err := store.Load()
	if err != nil {
		t.Fatalf("load offline state: %v", err)
	}
	if state.Mode != offline.ModeOffline {
		t.Fatalf("expected offline mode, got %q", state.Mode)
	}
	if len(state.PendingOperations) != 1 || state.PendingOperations[0].Action != "create" {
		t.Fatalf("expected one queued create op, got %+v", state.PendingOperations)
	}
	local := state.Issues[issue.ID]
	if !local.LocalOnly || !local.Dirty {
		t.Fatalf("expected dirty local-only issue, got %+v", local)
	}

	status, err := client.SyncStatus()
	if err != nil {
		t.Fatalf("sync status: %v", err)
	}
	if status.Mode != offline.ModeOffline || status.PendingOps != 1 || status.PendingConflicts != 0 {
		t.Fatalf("unexpected sync status: %+v", status)
	}
}

func TestSyncStatusCountsOnlyUnresolvedConflicts(t *testing.T) {
	client, _, store := newTestClient(t)
	now := time.Now().UTC()
	if err := store.Save(offline.State{
		Mode: offline.ModeSyncing,
		PendingOperations: []offline.Operation{
			{ID: "op-1", Entity: "issue", Action: "update", TargetID: "i-1", CreatedAt: now},
		},
		Conflicts: []offline.ConflictRecord{
			{ID: "c-open", Entity: "issue", TargetID: "i-1", CreatedAt: now},
			{ID: "c-resolved", Entity: "issue", TargetID: "i-2", Resolved: true, CreatedAt: now, ResolvedAt: now},
		},
		LastSyncedAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	status, err := client.SyncStatus()
	if err != nil {
		t.Fatalf("sync status: %v", err)
	}
	if status.Mode != offline.ModeSyncing {
		t.Fatalf("expected syncing mode, got %q", status.Mode)
	}
	if status.PendingOps != 1 {
		t.Fatalf("expected 1 pending op, got %d", status.PendingOps)
	}
	if status.PendingConflicts != 1 {
		t.Fatalf("expected 1 unresolved conflict, got %d", status.PendingConflicts)
	}
	if status.LastSyncedAt.IsZero() {
		t.Fatalf("expected last sync timestamp to be preserved")
	}
}

func TestConfigStoreSavePersistsCurrentProjectID(t *testing.T) {
	cfgPath := t.TempDir() + "/config.yaml"
	store := config.NewStore(cfgPath)
	input := config.Config{APIURL: "http://127.0.0.1:1/api/v1", CurrentProjectID: "project-1"}
	if err := store.Save(input); err != nil {
		t.Fatalf("save config: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got.CurrentProjectID != input.CurrentProjectID {
		t.Fatalf("expected current project id %q, got %q", input.CurrentProjectID, got.CurrentProjectID)
	}
}
