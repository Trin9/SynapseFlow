package engine

import (
	"errors"
	"fmt"

	"github.com/xunchenzheng/synapse/pkg/models"
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
	// Levels contains nodes grouped by their execution level.
	// Nodes within the same level have no dependencies on each other and can run concurrently.
	Levels [][]models.Node

	// NodeMap provides O(1) lookup of nodes by ID.
	NodeMap map[string]*models.Node

	// Deps maps each node ID to its list of dependency node IDs.
	Deps map[string][]string

	// Dependents maps each node ID to the list of nodes that depend on it.
	Dependents map[string][]string
}

// ParseDAG validates and parses a DAGConfig using Kahn's algorithm for topological sorting.
// It returns execution levels (nodes grouped by concurrency tier) or an error if the graph is invalid.
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

	// Build adjacency structures
	deps := make(map[string][]string, len(config.Nodes))
	dependents := make(map[string][]string, len(config.Nodes))
	inDegree := make(map[string]int, len(config.Nodes))

	for _, n := range config.Nodes {
		deps[n.ID] = nil
		dependents[n.ID] = nil
		inDegree[n.ID] = 0
	}

	for _, e := range config.Edges {
		deps[e.To] = append(deps[e.To], e.From)
		dependents[e.From] = append(dependents[e.From], e.To)
		inDegree[e.To]++
	}

	// Kahn's algorithm: BFS-based topological sort producing execution levels
	var levels [][]models.Node
	processed := 0

	// Seed queue with nodes having in-degree 0
	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		// All nodes in the current queue form one execution level (can run concurrently)
		level := make([]models.Node, 0, len(queue))
		nextQueue := make([]string, 0)

		for _, id := range queue {
			level = append(level, *nodeMap[id])
			processed++

			for _, depID := range dependents[id] {
				inDegree[depID]--
				if inDegree[depID] == 0 {
					nextQueue = append(nextQueue, depID)
				}
			}
		}

		levels = append(levels, level)
		queue = nextQueue
	}

	if processed != len(config.Nodes) {
		return nil, ErrIllegalCycle
	}

	return &ParsedDAG{
		Levels:     levels,
		NodeMap:    nodeMap,
		Deps:       deps,
		Dependents: dependents,
	}, nil
}
