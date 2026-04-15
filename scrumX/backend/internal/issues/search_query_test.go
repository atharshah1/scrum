package issues

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseIssueSearchQuery_QuotedValue(t *testing.T) {
	expr, ast, err := parseIssueSearchQuery(`title="payment bug" AND status=done`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if expr == nil {
		t.Fatalf("expected parsed expression")
	}
	if ast.Type != "AND" {
		t.Fatalf("expected AND ast, got %s", ast.Type)
	}
	if len(ast.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(ast.Conditions))
	}
	if ast.Conditions[0].Field != "title" || ast.Conditions[0].Value != "payment bug" {
		t.Fatalf("unexpected first condition: %+v", ast.Conditions[0])
	}
}

func TestParseIssueSearchQuery_ContainsOperator(t *testing.T) {
	expr, _, err := parseIssueSearchQuery(`label~pay AND priority=high`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if expr == nil {
		t.Fatalf("expected parsed expression")
	}
}

func TestParseIssueSearchQuery_InvalidField(t *testing.T) {
	_, _, err := parseIssueSearchQuery(`unknown=1`)
	if err == nil {
		t.Fatalf("expected error")
	}
	validationErr, ok := err.(*IssueSearchValidationError)
	if !ok {
		t.Fatalf("expected IssueSearchValidationError, got %T", err)
	}
	if validationErr.Code != "unsupported_field" {
		t.Fatalf("expected unsupported_field, got %s", validationErr.Code)
	}
}

func TestParseIssueSearchQuery_UnterminatedQuote(t *testing.T) {
	_, _, err := parseIssueSearchQuery(`title="payment bug`)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseIssueSearchQuery_MaxDepthExceeded(t *testing.T) {
	_, _, err := parseIssueSearchQuery(`((((((status=done))))))`)
	if err == nil {
		t.Fatalf("expected error")
	}
	validationErr, ok := err.(*IssueSearchValidationError)
	if !ok {
		t.Fatalf("expected IssueSearchValidationError, got %T", err)
	}
	if validationErr.Code != "max_depth_exceeded" {
		t.Fatalf("expected max_depth_exceeded, got %s", validationErr.Code)
	}
}

func TestBuildIssueConditionSQL_TitleContains(t *testing.T) {
	args := []any{}
	argN := 1
	sql, err := buildIssueConditionSQL(IssueSearchCondition{Field: "title", Op: "~", Value: "pay"}, uuid.New(), &args, &argN)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sql == "" {
		t.Fatalf("expected SQL output")
	}
	if len(args) != 1 {
		t.Fatalf("expected one arg, got %d", len(args))
	}
	if got, ok := args[0].(string); !ok || got != "%p%a%y%" {
		t.Fatalf("expected fuzzy pattern %%p%%a%%y%%, got %#v", args[0])
	}
}

func TestBuildIssueConditionSQL_TitleContainsWithSpaces(t *testing.T) {
	args := []any{}
	argN := 1
	_, err := buildIssueConditionSQL(IssueSearchCondition{Field: "title", Op: "~", Value: "pay bug"}, uuid.New(), &args, &argN)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(args) != 1 {
		t.Fatalf("expected one arg, got %d", len(args))
	}
	if got, ok := args[0].(string); !ok || got != "%p%a%y%b%u%g%" {
		t.Fatalf("expected fuzzy pattern %%p%%a%%y%%b%%u%%g%%, got %#v", args[0])
	}
}
