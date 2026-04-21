package models

import "testing"

func TestGlobalStateGetPath(t *testing.T) {
	state := NewGlobalState()
	state.Set("plan", map[string]interface{}{
		"next_action": "search_code",
		"done":        false,
		"nested": map[string]interface{}{
			"query": "failed to charge card",
		},
	})

	value, ok := state.GetPath("plan.next_action")
	if !ok {
		t.Fatal("expected plan.next_action to exist")
	}
	if value != "search_code" {
		t.Fatalf("expected search_code, got %v", value)
	}

	query, ok := state.GetPath("plan.nested.query")
	if !ok {
		t.Fatal("expected nested query to exist")
	}
	if query != "failed to charge card" {
		t.Fatalf("unexpected nested query: %v", query)
	}

	if _, ok := state.GetPath("plan.missing"); ok {
		t.Fatal("expected missing path lookup to fail")
	}

	if got := state.GetPathString("plan.done"); got != "false" {
		t.Fatalf("expected formatted false, got %q", got)
	}
}
