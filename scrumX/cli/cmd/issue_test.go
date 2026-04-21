package cmd

import (
	"reflect"
	"testing"
)

func TestParseSmartIssueInputExtractsStructuredFields(t *testing.T) {
	got := parseSmartIssueInput("fix login bug p1 assign me #auth label backend desc investigate oauth callback", "user-1")

	if got.Title != "fix login bug" {
		t.Fatalf("unexpected title: %q", got.Title)
	}
	if got.Priority != "critical" {
		t.Fatalf("unexpected priority: %q", got.Priority)
	}
	if got.AssigneeID != "user-1" {
		t.Fatalf("unexpected assignee: %q", got.AssigneeID)
	}
	if got.Description != "investigate oauth callback" {
		t.Fatalf("unexpected description: %q", got.Description)
	}
	wantLabels := []string{"auth", "backend"}
	if !reflect.DeepEqual(got.Labels, wantLabels) {
		t.Fatalf("unexpected labels: %#v", got.Labels)
	}
}

func TestParseSmartIssueInputKeepsUnknownWordsInTitle(t *testing.T) {
	got := parseSmartIssueInput("stabilize sync queue p3 for release", "user-1")

	if got.Title != "stabilize sync queue for release" {
		t.Fatalf("unexpected title: %q", got.Title)
	}
	if got.Priority != "medium" {
		t.Fatalf("unexpected priority: %q", got.Priority)
	}
	if got.AssigneeID != "" {
		t.Fatalf("expected empty assignee, got %q", got.AssigneeID)
	}
}
