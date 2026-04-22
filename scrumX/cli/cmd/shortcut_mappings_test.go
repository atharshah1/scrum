package cmd

import "testing"

func TestCanonicalCommandMappings(t *testing.T) {
	if issueCmd.Use != "issues" {
		t.Fatalf("expected canonical issues command, got %q", issueCmd.Use)
	}
	assertAlias(t, issueCmd.Aliases, "issue")
	assertAlias(t, issueCmd.Aliases, "i")
	assertAlias(t, issueCreateCmd.Aliases, "c")
	assertAlias(t, issueListCmd.Aliases, "l")
	assertAlias(t, issueUpdateCmd.Aliases, "u")

	assertAlias(t, projectCmd.Aliases, "p")
	assertAlias(t, projectCreateCmd.Aliases, "c")
	assertAlias(t, projectListCmd.Aliases, "l")

	assertAlias(t, sprintCmd.Aliases, "s")
	assertAlias(t, sprintStartCmd.Aliases, "s")
}

func TestRootShortcutFlagsExposeMixedSyntax(t *testing.T) {
	if flag := issueCmd.Flags().Lookup("create"); flag == nil || flag.Shorthand != "c" {
		t.Fatalf("expected issue root create shorthand")
	}
	if flag := issueCmd.Flags().Lookup("list"); flag == nil || flag.Shorthand != "l" {
		t.Fatalf("expected issue root list shorthand")
	}
	if flag := issueCmd.Flags().Lookup("update"); flag == nil || flag.Shorthand != "u" {
		t.Fatalf("expected issue root update shorthand")
	}
	if flag := projectCmd.Flags().Lookup("create"); flag == nil || flag.Shorthand != "c" {
		t.Fatalf("expected project root create shorthand")
	}
	if flag := projectCmd.Flags().Lookup("list"); flag == nil || flag.Shorthand != "l" {
		t.Fatalf("expected project root list shorthand")
	}
	if flag := sprintCmd.Flags().Lookup("start"); flag == nil || flag.Shorthand != "s" {
		t.Fatalf("expected sprint root start shorthand")
	}
}

func assertAlias(t *testing.T, aliases []string, want string) {
	t.Helper()
	for _, alias := range aliases {
		if alias == want {
			return
		}
	}
	t.Fatalf("missing alias %q in %#v", want, aliases)
}
