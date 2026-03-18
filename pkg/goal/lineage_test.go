package goal

import (
	"testing"
)

func TestAppendLineage(t *testing.T) {
	upstream := []LineageRef{
		{GoalID: "g-1", ProjectName: "proj-a", IssueID: "i-1", OPCNode: "node-1", Label: "origin"},
	}

	newRef := LineageRef{GoalID: "g-1", ProjectName: "proj-a", IssueID: "i-2", OPCNode: "node-2", Label: "dispatch"}

	result := AppendLineage(upstream, newRef)

	// Correct length.
	if len(result) != 2 {
		t.Fatalf("expected length 2, got %d", len(result))
	}

	// New ref is at the end.
	if result[1] != newRef {
		t.Errorf("expected last element %+v, got %+v", newRef, result[1])
	}

	// Upstream not mutated.
	if len(upstream) != 1 {
		t.Errorf("upstream was mutated: expected length 1, got %d", len(upstream))
	}
}

func TestAppendLineage_NilUpstream(t *testing.T) {
	ref := LineageRef{GoalID: "g-1", ProjectName: "proj-a", IssueID: "i-1", OPCNode: "node-1", Label: "origin"}
	result := AppendLineage(nil, ref)

	if len(result) != 1 {
		t.Fatalf("expected length 1, got %d", len(result))
	}
	if result[0] != ref {
		t.Errorf("expected %+v, got %+v", ref, result[0])
	}
}

func TestLineageToJSON(t *testing.T) {
	refs := []LineageRef{
		{GoalID: "g-1", ProjectName: "proj-a", IssueID: "i-1", OPCNode: "node-1", Label: "origin"},
		{GoalID: "g-1", ProjectName: "proj-a", IssueID: "i-2", OPCNode: "node-2", Label: "dispatch"},
	}

	jsonStr, err := LineageToJSON(refs)
	if err != nil {
		t.Fatalf("LineageToJSON failed: %v", err)
	}

	// Round-trip: deserialize and verify equality.
	parsed, err := LineageFromJSON(jsonStr)
	if err != nil {
		t.Fatalf("LineageFromJSON failed: %v", err)
	}

	if len(parsed) != len(refs) {
		t.Fatalf("round-trip length mismatch: expected %d, got %d", len(refs), len(parsed))
	}

	for i := range refs {
		if parsed[i] != refs[i] {
			t.Errorf("round-trip mismatch at index %d: expected %+v, got %+v", i, refs[i], parsed[i])
		}
	}
}

func TestLineageToJSON_Empty(t *testing.T) {
	jsonStr, err := LineageToJSON(nil)
	if err != nil {
		t.Fatalf("LineageToJSON(nil) failed: %v", err)
	}
	if jsonStr != "[]" {
		t.Errorf("expected \"[]\", got %q", jsonStr)
	}
}

func TestLineageFromJSON_Empty(t *testing.T) {
	refs, err := LineageFromJSON("")
	if err != nil {
		t.Fatalf("LineageFromJSON empty failed: %v", err)
	}
	if refs != nil {
		t.Errorf("expected nil, got %+v", refs)
	}

	refs, err = LineageFromJSON("[]")
	if err != nil {
		t.Fatalf("LineageFromJSON [] failed: %v", err)
	}
	if refs != nil {
		t.Errorf("expected nil for [], got %+v", refs)
	}
}
