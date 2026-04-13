package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xunchenzheng/synapse/pkg/logger"
	"github.com/xunchenzheng/synapse/pkg/models"
)

// ---------------------------------------------------------------------------
// Scheduler: Concurrent DAG execution engine (V1)
// ---------------------------------------------------------------------------

// Scheduler orchestrates the execution of a parsed DAG.
// It runs nodes concurrently within each execution level, respecting dependency ordering.
// When a node fails, all downstream dependents are marked as "skipped".
type Scheduler struct {
	executors map[models.NodeType]NodeExecutor
}

// NewScheduler creates a Scheduler with the provided executors for each node type.
func NewScheduler(executors map[models.NodeType]NodeExecutor) *Scheduler {
	return &Scheduler{executors: executors}
}

// ExecuteResult holds the complete outcome of a workflow execution.
type ExecuteResult struct {
	Results  []models.NodeResult
	Duration time.Duration
	State    *models.GlobalState
	Err      error
}

// Execute runs a DAGConfig through the full pipeline: parse -> schedule -> execute.
func (s *Scheduler) Execute(ctx context.Context, config *models.DAGConfig) *ExecuteResult {
	engineStart := time.Now()
	log := logger.L()

	log.Infow("Starting workflow execution",
		"dag_id", config.ID,
		"dag_name", config.Name,
		"node_count", len(config.Nodes),
		"edge_count", len(config.Edges),
	)

	// Step 1: Parse and validate the DAG
	parsed, err := ParseDAG(config)
	if err != nil {
		log.Errorw("DAG parsing failed", "error", err)
		return &ExecuteResult{
			Duration: time.Since(engineStart),
			State:    models.NewGlobalState(),
			Err:      fmt.Errorf("DAG parsing failed: %w", err),
		}
	}

	log.Infow("DAG parsed successfully",
		"levels", len(parsed.Levels),
		"total_nodes", len(config.Nodes),
	)

	// Step 2: Execute level by level
	state := models.NewGlobalState()
	allResults := make([]models.NodeResult, 0, len(config.Nodes))
	var resultsMu sync.Mutex

	// Track failed nodes: if a node fails, all its downstream dependents are skipped.
	failedNodes := make(map[string]bool)
	var failedMu sync.RWMutex

	for levelIdx, level := range parsed.Levels {
		log.Infow("Executing level",
			"level", levelIdx,
			"node_count", len(level),
		)

		if len(level) == 1 {
			// Single node: execute directly (no goroutine overhead)
			node := level[0]
			result := s.executeOrSkip(ctx, node, state, parsed, failedNodes, &failedMu)
			allResults = append(allResults, result)

			if result.Status == "error" {
				log.Errorw("Node execution failed",
					"node_id", node.ID,
					"node_name", node.Name,
					"error", result.Error,
				)
			}
		} else {
			// Multiple nodes: execute concurrently
			var wg sync.WaitGroup
			for _, node := range level {
				wg.Add(1)
				go func(n models.Node) {
					defer wg.Done()
					result := s.executeOrSkip(ctx, n, state, parsed, failedNodes, &failedMu)

					resultsMu.Lock()
					allResults = append(allResults, result)
					resultsMu.Unlock()

					if result.Status == "error" {
						log.Errorw("Node execution failed",
							"node_id", n.ID,
							"node_name", n.Name,
							"error", result.Error,
						)
					}
				}(node)
			}
			wg.Wait()
		}
	}

	totalDuration := time.Since(engineStart)

	// Aggregate error summary
	var errorCount, skipCount, successCount int
	for _, r := range allResults {
		switch r.Status {
		case "error":
			errorCount++
		case "skipped":
			skipCount++
		default:
			successCount++
		}
	}

	log.Infow("Workflow execution completed",
		"dag_id", config.ID,
		"total_duration", totalDuration,
		"nodes_total", len(allResults),
		"nodes_success", successCount,
		"nodes_error", errorCount,
		"nodes_skipped", skipCount,
	)

	return &ExecuteResult{
		Results:  allResults,
		Duration: totalDuration,
		State:    state,
	}
}

// executeOrSkip checks whether a node should be skipped (due to upstream failure)
// and either skips it or executes it normally.
func (s *Scheduler) executeOrSkip(
	ctx context.Context,
	node models.Node,
	state *models.GlobalState,
	parsed *ParsedDAG,
	failedNodes map[string]bool,
	failedMu *sync.RWMutex,
) models.NodeResult {
	// Check if any upstream dependency has failed
	failedMu.RLock()
	deps := parsed.Deps[node.ID]
	shouldSkip := false
	var failedDep string
	for _, dep := range deps {
		if failedNodes[dep] {
			shouldSkip = true
			failedDep = dep
			break
		}
	}
	failedMu.RUnlock()

	if shouldSkip {
		logger.L().Infow("Skipping node due to upstream failure",
			"node_id", node.ID,
			"node_name", node.Name,
			"failed_dependency", failedDep,
		)
		// Mark this node as failed too so its own dependents get skipped
		failedMu.Lock()
		failedNodes[node.ID] = true
		failedMu.Unlock()

		return models.NodeResult{
			NodeID:   node.ID,
			NodeName: node.Name,
			NodeType: node.Type,
			Status:   "skipped",
			Error:    fmt.Sprintf("skipped: upstream node %q failed", failedDep),
		}
	}

	// Execute normally
	result := s.executeNode(ctx, node, state)

	if result.Status == "error" {
		failedMu.Lock()
		failedNodes[node.ID] = true
		failedMu.Unlock()
	}

	return result
}

// executeNode dispatches a node to the appropriate executor based on its type.
func (s *Scheduler) executeNode(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	log := logger.L()

	executor, ok := s.executors[node.Type]
	if !ok {
		log.Warnw("No executor registered for node type",
			"node_id", node.ID,
			"node_type", node.Type,
		)
		return models.NodeResult{
			NodeID:   node.ID,
			NodeName: node.Name,
			NodeType: node.Type,
			Status:   "error",
			Error:    fmt.Sprintf("no executor registered for node type: %s", node.Type),
		}
	}

	log.Infow("Executing node",
		"node_id", node.ID,
		"node_name", node.Name,
		"node_type", node.Type,
	)

	// Apply per-node timeout if configured (default: 30s for scripts, 60s for LLM)
	timeout := 30 * time.Second
	if node.Type == models.NodeTypeLLM {
		timeout = 60 * time.Second
	}
	if node.Config != nil {
		if t, ok := node.Config["timeout_seconds"].(float64); ok && t > 0 {
			timeout = time.Duration(t) * time.Second
		}
	}

	nodeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := executor.Execute(nodeCtx, node, state)

	log.Infow("Node execution completed",
		"node_id", node.ID,
		"node_name", node.Name,
		"status", result.Status,
		"duration", result.Duration,
	)

	return result
}
