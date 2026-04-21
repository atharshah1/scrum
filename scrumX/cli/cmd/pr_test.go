package cmd

import (
	"reflect"
	"testing"
)

func TestPickBaseRefPrefersKnownDefaults(t *testing.T) {
	exists := func(candidate string) bool {
		return candidate == "origin/master"
	}
	if got := pickBaseRef("", exists); got != "origin/master" {
		t.Fatalf("unexpected base ref: %q", got)
	}
	if got := pickBaseRef("feature/base", exists); got != "feature/base" {
		t.Fatalf("explicit base ref should win, got %q", got)
	}
}

func TestGitContextLabelsAndTitle(t *testing.T) {
	ctx := gitContext{
		Repo:         "scrum",
		Branch:       "feature/fix-login-bug",
		ChangedFiles: []string{"frontend/app/page.tsx", "backend/internal/issues/service.go", "frontend/app/page.tsx"},
	}

	if got := ctx.Title(); got != "Fix login bug" {
		t.Fatalf("unexpected title: %q", got)
	}
	wantLabels := []string{"area:backend", "area:frontend", "repo:scrum"}
	if got := ctx.Labels(); !reflect.DeepEqual(got, wantLabels) {
		t.Fatalf("unexpected labels: %#v", got)
	}
}
