package issues

import "testing"

func TestValidateHierarchy(t *testing.T) {
	tests := []struct {
		name      string
		childType string
		parent    string
		wantErr   bool
	}{
		{name: "story under epic", childType: "story", parent: "epic", wantErr: false},
		{name: "task under story", childType: "task", parent: "story", wantErr: false},
		{name: "bug under story", childType: "bug", parent: "story", wantErr: false},
		{name: "story under story invalid", childType: "story", parent: "story", wantErr: true},
		{name: "task under epic invalid", childType: "task", parent: "epic", wantErr: true},
		{name: "epic cannot have parent", childType: "epic", parent: "story", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHierarchy(tt.childType, tt.parent)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateHierarchy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsAllowedTransition(t *testing.T) {
	if isAllowedTransition("todo", "done") {
		t.Fatal("expected todo -> done to be disallowed")
	}
	if !isAllowedTransition("todo", "in_progress") {
		t.Fatal("expected todo -> in_progress to be allowed")
	}
	if !isAllowedTransition("in_review", "done") {
		t.Fatal("expected in_review -> done to be allowed")
	}
}

func TestValidateRelationType(t *testing.T) {
	if err := validateRelationType("blocks"); err != nil {
		t.Fatalf("expected blocks to be valid: %v", err)
	}
	if err := validateRelationType("relates_to"); err != nil {
		t.Fatalf("expected relates_to to be valid: %v", err)
	}
	if err := validateRelationType("contains"); err == nil {
		t.Fatal("expected contains to be invalid")
	}
}

func TestSplitCSV(t *testing.T) {
	labels := splitCSV("backend, high-priority, , Bug")
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	if labels[0] != "backend" || labels[2] != "bug" {
		t.Fatalf("unexpected labels: %#v", labels)
	}
}
