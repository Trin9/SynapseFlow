package engine

import (
	"testing"

	"github.com/xunchenzheng/synapse/pkg/models"
)

// ---------------------------------------------------------------------------
// ParseDAG Tests
// ---------------------------------------------------------------------------

func TestParseDAG_LinearGraph(t *testing.T) {
	// A -> B -> C (serial)
	config := &models.DAGConfig{
		Nodes: []models.Node{
			{ID: "a", Name: "Node A", Type: models.NodeTypeScript},
			{ID: "b", Name: "Node B", Type: models.NodeTypeScript},
			{ID: "c", Name: "Node C", Type: models.NodeTypeLLM},
		},
		Edges: []models.Edge{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
		},
	}

	parsed, err := ParseDAG(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.Levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(parsed.Levels))
	}

	for i, level := range parsed.Levels {
		if len(level) != 1 {
			t.Errorf("level %d: expected 1 node, got %d", i, len(level))
		}
	}
}

func TestParseDAG_FanInGraph(t *testing.T) {
	// A, B, C all fan into D (parallel -> serial)
	config := &models.DAGConfig{
		Nodes: []models.Node{
			{ID: "a", Name: "Fetch Logs", Type: models.NodeTypeScript},
			{ID: "b", Name: "Fetch Metrics", Type: models.NodeTypeScript},
			{ID: "c", Name: "Fetch DB", Type: models.NodeTypeScript},
			{ID: "d", Name: "Analyze", Type: models.NodeTypeLLM},
		},
		Edges: []models.Edge{
			{From: "a", To: "d"},
			{From: "b", To: "d"},
			{From: "c", To: "d"},
		},
	}

	parsed, err := ParseDAG(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.Levels) != 2 {
		t.Fatalf("expected 2 levels, got %d", len(parsed.Levels))
	}

	if len(parsed.Levels[0]) != 3 {
		t.Errorf("level 0: expected 3 concurrent nodes, got %d", len(parsed.Levels[0]))
	}
	if len(parsed.Levels[1]) != 1 {
		t.Errorf("level 1: expected 1 node, got %d", len(parsed.Levels[1]))
	}
}

func TestParseDAG_DiamondGraph(t *testing.T) {
	// Diamond: A -> B, A -> C, B -> D, C -> D
	config := &models.DAGConfig{
		Nodes: []models.Node{
			{ID: "a", Name: "Start", Type: models.NodeTypeScript},
			{ID: "b", Name: "Branch 1", Type: models.NodeTypeScript},
			{ID: "c", Name: "Branch 2", Type: models.NodeTypeScript},
			{ID: "d", Name: "Merge", Type: models.NodeTypeLLM},
		},
		Edges: []models.Edge{
			{From: "a", To: "b"},
			{From: "a", To: "c"},
			{From: "b", To: "d"},
			{From: "c", To: "d"},
		},
	}

	parsed, err := ParseDAG(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.Levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(parsed.Levels))
	}

	if len(parsed.Levels[0]) != 1 {
		t.Errorf("level 0: expected 1 node (start), got %d", len(parsed.Levels[0]))
	}
	if len(parsed.Levels[1]) != 2 {
		t.Errorf("level 1: expected 2 concurrent nodes (branches), got %d", len(parsed.Levels[1]))
	}
	if len(parsed.Levels[2]) != 1 {
		t.Errorf("level 2: expected 1 node (merge), got %d", len(parsed.Levels[2]))
	}
}

func TestParseDAG_SingleNode(t *testing.T) {
	config := &models.DAGConfig{
		Nodes: []models.Node{
			{ID: "solo", Name: "Solo Node", Type: models.NodeTypeScript},
		},
		Edges: []models.Edge{},
	}

	parsed, err := ParseDAG(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.Levels) != 1 {
		t.Fatalf("expected 1 level, got %d", len(parsed.Levels))
	}
	if len(parsed.Levels[0]) != 1 {
		t.Errorf("expected 1 node in level 0, got %d", len(parsed.Levels[0]))
	}
}

func TestParseDAG_CycleDetection(t *testing.T) {
	// A -> B -> C -> A (cycle!)
	config := &models.DAGConfig{
		Nodes: []models.Node{
			{ID: "a", Name: "A", Type: models.NodeTypeScript},
			{ID: "b", Name: "B", Type: models.NodeTypeScript},
			{ID: "c", Name: "C", Type: models.NodeTypeScript},
		},
		Edges: []models.Edge{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
			{From: "c", To: "a"},
		},
	}

	_, err := ParseDAG(config)
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}
	if err != ErrIllegalCycle {
		t.Fatalf("expected ErrIllegalCycle, got: %v", err)
	}
}

func TestParseDAG_EmptyDAG(t *testing.T) {
	config := &models.DAGConfig{
		Nodes: []models.Node{},
		Edges: []models.Edge{},
	}

	_, err := ParseDAG(config)
	if err == nil {
		t.Fatal("expected error for empty DAG, got nil")
	}
	if err != ErrEmptyDAG {
		t.Fatalf("expected ErrEmptyDAG, got: %v", err)
	}
}

func TestParseDAG_InvalidEdgeReference(t *testing.T) {
	config := &models.DAGConfig{
		Nodes: []models.Node{
			{ID: "a", Name: "A", Type: models.NodeTypeScript},
		},
		Edges: []models.Edge{
			{From: "a", To: "nonexistent"},
		},
	}

	_, err := ParseDAG(config)
	if err == nil {
		t.Fatal("expected error for invalid edge, got nil")
	}
}

func TestParseDAG_ComplexGraph(t *testing.T) {
	// 5 nodes with multiple parallel paths
	//    A
	//   / \
	//  B   C
	//   \ / \
	//    D   E
	config := &models.DAGConfig{
		Nodes: []models.Node{
			{ID: "a", Name: "A", Type: models.NodeTypeScript},
			{ID: "b", Name: "B", Type: models.NodeTypeScript},
			{ID: "c", Name: "C", Type: models.NodeTypeScript},
			{ID: "d", Name: "D", Type: models.NodeTypeLLM},
			{ID: "e", Name: "E", Type: models.NodeTypeScript},
		},
		Edges: []models.Edge{
			{From: "a", To: "b"},
			{From: "a", To: "c"},
			{From: "b", To: "d"},
			{From: "c", To: "d"},
			{From: "c", To: "e"},
		},
	}

	parsed, err := ParseDAG(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.Levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(parsed.Levels))
	}

	// Level 0: A only
	if len(parsed.Levels[0]) != 1 {
		t.Errorf("level 0: expected 1 node, got %d", len(parsed.Levels[0]))
	}
	// Level 1: B, C concurrent
	if len(parsed.Levels[1]) != 2 {
		t.Errorf("level 1: expected 2 nodes, got %d", len(parsed.Levels[1]))
	}
	// Level 2: D, E concurrent
	if len(parsed.Levels[2]) != 2 {
		t.Errorf("level 2: expected 2 nodes, got %d", len(parsed.Levels[2]))
	}
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func BenchmarkParseDAG_100Nodes(b *testing.B) {
	config := generateChainDAG(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseDAG(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseDAG_1000Nodes(b *testing.B) {
	config := generateChainDAG(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseDAG(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseDAG_FanIn100(b *testing.B) {
	config := generateFanInDAG(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseDAG(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Test Helpers
// ---------------------------------------------------------------------------

func generateChainDAG(n int) *models.DAGConfig {
	nodes := make([]models.Node, n)
	edges := make([]models.Edge, n-1)

	for i := 0; i < n; i++ {
		nodes[i] = models.Node{
			ID:   nodeID(i),
			Name: nodeID(i),
			Type: models.NodeTypeScript,
		}
		if i > 0 {
			edges[i-1] = models.Edge{From: nodeID(i - 1), To: nodeID(i)}
		}
	}

	return &models.DAGConfig{Nodes: nodes, Edges: edges}
}

func generateFanInDAG(n int) *models.DAGConfig {
	nodes := make([]models.Node, n+1)
	edges := make([]models.Edge, n)

	for i := 0; i < n; i++ {
		nodes[i] = models.Node{
			ID:   nodeID(i),
			Name: nodeID(i),
			Type: models.NodeTypeScript,
		}
		edges[i] = models.Edge{From: nodeID(i), To: "sink"}
	}
	nodes[n] = models.Node{ID: "sink", Name: "sink", Type: models.NodeTypeLLM}

	return &models.DAGConfig{Nodes: nodes, Edges: edges}
}

func nodeID(i int) string {
	return "node_" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
