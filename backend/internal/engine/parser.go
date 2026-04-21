package engine

import (
	"errors"
	"fmt"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

var (
	// ErrIllegalCycle is returned when the DAG contains a cycle.
	ErrIllegalCycle = errors.New("illegal cycle detected in DAG")
	// ErrEmptyDAG is returned when the DAG has no nodes.
	ErrEmptyDAG = errors.New("DAG has no nodes")
	// ErrNodeNotFound is returned when an edge references a non-existent node.
	ErrNodeNotFound = errors.New("edge references non-existent node")
)

// ParsedDAG holds the result of parsing a DAGConfig.
type ParsedDAG struct {
	// Levels contains nodes grouped by their execution level (based on essential dependencies).
	Levels [][]models.Node

	// NodeMap provides O(1) lookup of nodes by ID.
	NodeMap map[string]*models.Node

	// Deps maps each node ID to its list of dependency node IDs.
	Deps map[string][]string

	// Dependents maps each node ID to the list of nodes that depend on it.
	Dependents map[string][]string

	// ConditionalDeps maps each node ID to the list of upstream nodes connected via conditional edges.
	ConditionalDeps map[string][]string

	// Edges holds all edges from the original config.
	Edges []models.Edge
}

// ParseDAG validates and parses a DAGConfig using Kahn's algorithm for topological sorting.
// It considers only "essential" edges (those without conditions) for topological sorting.
// Conditional edges are stored but do not block the initial execution level calculation.
func ParseDAG(config *models.DAGConfig) (*ParsedDAG, error) {
	if len(config.Nodes) == 0 {
		return nil, ErrEmptyDAG
	}

	// Build node map
	nodeMap := make(map[string]*models.Node, len(config.Nodes))
	for i := range config.Nodes {
		nodeMap[config.Nodes[i].ID] = &config.Nodes[i]
	}

	// Validate edges reference existing nodes
	for _, e := range config.Edges {
		if _, ok := nodeMap[e.From]; !ok {
			return nil, fmt.Errorf("%w: from=%q", ErrNodeNotFound, e.From)
		}
		if _, ok := nodeMap[e.To]; !ok {
			return nil, fmt.Errorf("%w: to=%q", ErrNodeNotFound, e.To)
		}
	}

	// Build adjacency structures for essential edges
	deps := make(map[string][]string, len(config.Nodes))
	dependents := make(map[string][]string, len(config.Nodes))
	conditionalDeps := make(map[string][]string, len(config.Nodes))
	inDegree := make(map[string]int, len(config.Nodes))

	for _, n := range config.Nodes {
		deps[n.ID] = nil
		dependents[n.ID] = nil
		conditionalDeps[n.ID] = nil
		inDegree[n.ID] = 0
	}

	for _, e := range config.Edges {
		// Only essential edges (no condition) contribute to the topological sort
		if e.Condition == "" {
			deps[e.To] = append(deps[e.To], e.From)
			dependents[e.From] = append(dependents[e.From], e.To)
			inDegree[e.To]++
			continue
		}
		conditionalDeps[e.To] = append(conditionalDeps[e.To], e.From)
	}

	// Kahn's algorithm: BFS-based topological sort producing execution levels
	var levels [][]models.Node
	processed := 0
	visited := make(map[string]bool)

	// Seed queue with nodes having in-degree 0
	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		level := make([]models.Node, 0, len(queue))
		nextQueue := make([]string, 0)

		for _, id := range queue {
			if visited[id] {
				continue
			}
			visited[id] = true
			level = append(level, *nodeMap[id])
			processed++

			for _, depID := range dependents[id] {
				inDegree[depID]--
				if inDegree[depID] == 0 {
					nextQueue = append(nextQueue, depID)
				}
			}
		}

		if len(level) > 0 {
			levels = append(levels, level)
		}
		queue = nextQueue
	}

	// If some nodes have essential cycles, they will not be processed.
	if processed != len(config.Nodes) {
		return nil, ErrIllegalCycle
	}

	return &ParsedDAG{
		Levels:          levels,
		NodeMap:         nodeMap,
		Deps:            deps,
		Dependents:      dependents,
		ConditionalDeps: conditionalDeps,
		Edges:           config.Edges,
	}, nil
}
