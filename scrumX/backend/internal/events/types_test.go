package events

import (
	"testing"

	"github.com/google/uuid"
)

func TestNew_NormalizesScopeFromPayload(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	projectID := uuid.New()
	issueID := uuid.New()

	event := New(orgID, "issue.updated", actorID, map[string]any{
		"project_id": projectID.String(),
		"issue_id":   issueID.String(),
	})

	if event.Scope.ProjectID == nil || *event.Scope.ProjectID != projectID {
		t.Fatalf("expected project scope to be normalized from payload")
	}
	if event.Scope.ResourceType != "issue" {
		t.Fatalf("expected issue resource type, got %q", event.Scope.ResourceType)
	}
	if event.Scope.ResourceID != issueID {
		t.Fatalf("expected issue resource id %s, got %s", issueID, event.Scope.ResourceID)
	}
}

func TestValidate_RejectsMissingResourceScope(t *testing.T) {
	event := Event{
		ID:      uuid.New(),
		OrgID:   uuid.New(),
		ActorID: uuid.New(),
		Type:    "issue.created",
		Payload: map[string]any{},
		Scope:   Scope{},
	}
	if err := Validate(event); err == nil {
		t.Fatal("expected validation error for missing resource scope")
	}
}

func TestValidate_RejectsGenericFallbackResourceScope(t *testing.T) {
	event := Event{
		ID:      uuid.New(),
		OrgID:   uuid.New(),
		ActorID: uuid.New(),
		Type:    "issue.created",
		Payload: map[string]any{},
		Scope: Scope{
			ResourceType: "event",
			ResourceID:   uuid.New(),
		},
	}
	if err := Validate(event); err == nil {
		t.Fatal("expected validation error for generic fallback scope")
	}
}

func TestValidate_RejectsMissingActor(t *testing.T) {
	event := Event{
		ID:    uuid.New(),
		OrgID: uuid.New(),
		Type:  "issue.created",
		Payload: map[string]any{
			"issue_id": uuid.NewString(),
		},
		Scope: Scope{
			ResourceType: "issue",
			ResourceID:   uuid.New(),
		},
	}
	if err := Validate(event); err == nil {
		t.Fatal("expected validation error for missing actor")
	}
}
