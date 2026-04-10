package main

import (
	"testing"
)

func TestRenderTemplate(t *testing.T) {
	state := NewGlobalState()
	state.Set("fetch_logs", "[ERROR] DB timeout at host:3306")
	state.Set("fetch_metrics", "CPU=94%")

	tmpl := "Logs: {{fetch_logs}}\nMetrics: {{fetch_metrics}}\nMissing: {{unknown}}"
	result := renderTemplate(tmpl, state)

	expected := "Logs: [ERROR] DB timeout at host:3306\nMetrics: CPU=94%\nMissing: {{unknown}}"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestBuildDependencyGraph(t *testing.T) {
	cfg := &WorkflowConfig{
		Nodes: []NodeConfig{
			{ID: "a", Name: "A", Type: "script"},
			{ID: "b", Name: "B", Type: "script"},
			{ID: "c", Name: "C", Type: "llm"},
		},
		Edges: []EdgeConfig{
			{From: "a", To: "c"},
			{From: "b", To: "c"},
		},
	}

	nodeMap, deps, _ := buildDependencyGraph(cfg)

	if len(nodeMap) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodeMap))
	}
	if len(deps["a"]) != 0 {
		t.Errorf("Node A should have 0 deps, got %d", len(deps["a"]))
	}
	if len(deps["c"]) != 2 {
		t.Errorf("Node C should have 2 deps, got %d", len(deps["c"]))
	}
}
