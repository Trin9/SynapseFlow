package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xunchenzheng/synapse/internal/memory"
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
	retriever *memory.Retriever
}

// NewScheduler creates a Scheduler with the provided executors for each node type.
func NewScheduler(executors map[models.NodeType]NodeExecutor, retriever *memory.Retriever) *Scheduler {
	return &Scheduler{executors: executors, retriever: retriever}
}

// ExecuteResult holds the complete outcome of a workflow execution.
type ExecuteResult struct {
	Results  []models.NodeResult
	Duration time.Duration
	State    *models.GlobalState
	Status   models.ExecutionStatus
	Err      error
}

// Execute runs a DAGConfig through the full pipeline: parse -> schedule -> execute.
// It accepts an optional initialState and a map of already completed nodes for resuming.
func (s *Scheduler) Execute(ctx context.Context, config *models.DAGConfig, initialState *models.GlobalState, completedNodes map[string]models.NodeResult) *ExecuteResult {
	engineStart := time.Now()
	log := logger.L()

	log.Infow("Starting workflow execution",
		"dag_id", config.ID,
		"dag_name", config.Name,
		"node_count", len(config.Nodes),
		"edge_count", len(config.Edges),
		"is_resume", initialState != nil,
	)

	// Step 1: Parse and validate the DAG
	parsed, err := ParseDAG(config)
	if err != nil {
		log.Errorw("DAG parsing failed", "error", err)
		return &ExecuteResult{
			Duration: time.Since(engineStart),
			State:    models.NewGlobalState(),
			Status:   models.StatusFailed,
			Err:      fmt.Errorf("DAG parsing failed: %w", err),
		}
	}

	log.Infow("DAG parsed successfully",
		"levels", len(parsed.Levels),
		"total_nodes", len(config.Nodes),
	)

	// Step 2: Execute level by level
	state := initialState
	if state == nil {
		state = models.NewGlobalState()
	}
	if len(config.Metadata) > 0 {
		metadata := make(map[string]interface{}, len(config.Metadata))
		for key, value := range config.Metadata {
			if _, exists := state.Get(key); exists {
				continue
			}
			metadata[key] = value
		}
		state.Merge(metadata)
	}

	if s.retriever != nil {
		if recalled, recallErr := s.retriever.Inject(ctx, config, state); recallErr != nil {
			log.Warnw("Memory retrieval failed", "dag_id", config.ID, "error", recallErr)
		} else if len(recalled) > 0 {
			log.Infow("Memory retrieval completed", "dag_id", config.ID, "matches", len(recalled))
		}
	}

	allResults := make([]models.NodeResult, 0, len(config.Nodes))
	nodeResults := make(map[string]models.NodeResult)
	if completedNodes != nil {
		for id, res := range completedNodes {
			nodeResults[id] = res
			allResults = append(allResults, res)
		}
	}
	var resultsMu sync.Mutex
	var nodesMu sync.RWMutex

	// Track failed nodes: if a node fails, all its downstream dependents are skipped.
	failedNodes := make(map[string]bool)
	for id, res := range nodeResults {
		if res.Status == "error" {
			failedNodes[id] = true
		}
	}
	var failedMu sync.RWMutex

	// Track suspension
	var suspended bool
	var suspendedMu sync.Mutex

	for levelIdx := 0; levelIdx < len(parsed.Levels); levelIdx++ {
		suspendedMu.Lock()
		if suspended {
			suspendedMu.Unlock()
			break
		}
		suspendedMu.Unlock()

		level := parsed.Levels[levelIdx]
		// Filter out already completed nodes in this level
		remainingNodes := make([]models.Node, 0)
		for _, node := range level {
			nodesMu.RLock()
			_, done := nodeResults[node.ID]
			nodesMu.RUnlock()
			if !done {
				remainingNodes = append(remainingNodes, node)
			}
		}

		if len(remainingNodes) == 0 {
			continue
		}

		log.Infow("Executing level",
			"level", levelIdx,
			"node_count", len(remainingNodes),
		)

		var wg sync.WaitGroup
		for _, node := range remainingNodes {
			wg.Add(1)
			go func(n models.Node) {
				defer wg.Done()
				result := s.executeOrSkip(ctx, n, state, parsed, failedNodes, &failedMu)

				nodesMu.Lock()
				nodeResults[n.ID] = result
				nodesMu.Unlock()

				resultsMu.Lock()
				allResults = append(allResults, result)
				resultsMu.Unlock()

				if result.Status == string(models.StatusSuspended) {
					log.Infow("Node suspended execution",
						"node_id", n.ID,
						"node_name", n.Name,
					)
					suspendedMu.Lock()
					suspended = true
					suspendedMu.Unlock()
				} else if result.Status == "error" {
					log.Errorw("Node execution failed",
						"node_id", n.ID,
						"node_name", n.Name,
						"error", result.Error,
					)
				} else if result.Status == "success" {
					// Check for conditional outgoing edges (loops or branches)
					s.handleOutgoingEdges(ctx, n, state, parsed, &levelIdx, &allResults, &resultsMu, failedNodes, &failedMu, nodeResults, &nodesMu, &suspended, &suspendedMu)
				}
			}(node)
		}
		wg.Wait()
	}

	totalDuration := time.Since(engineStart)

	// Final status determination
	execStatus := models.StatusCompleted
	if suspended {
		execStatus = models.StatusSuspended
	}

	var errorCount, skipCount, successCount int
	for _, r := range allResults {
		switch r.Status {
		case "error":
			errorCount++
			if !suspended {
				execStatus = models.StatusFailed
			}
		case "skipped":
			skipCount++
		case string(models.StatusSuspended):
			// Already handled by the flag
		default:
			successCount++
		}
	}

	log.Infow("Workflow execution completed",
		"dag_id", config.ID,
		"status", execStatus,
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
		Status:   execStatus,
	}
}

// handleOutgoingEdges evaluates conditional edges and triggers downstream nodes if necessary.
func (s *Scheduler) handleOutgoingEdges(
	ctx context.Context,
	node models.Node,
	state *models.GlobalState,
	parsed *ParsedDAG,
	currentLevel *int,
	allResults *[]models.NodeResult,
	resultsMu *sync.Mutex,
	failedNodes map[string]bool,
	failedMu *sync.RWMutex,
	nodeResults map[string]models.NodeResult,
	nodesMu *sync.RWMutex,
	suspended *bool,
	suspendedMu *sync.Mutex,
) {
	log := logger.L()

	for _, edge := range parsed.Edges {
		if edge.From != node.ID || edge.Condition == "" {
			continue
		}

		suspendedMu.Lock()
		if *suspended {
			suspendedMu.Unlock()
			return
		}
		suspendedMu.Unlock()

		met, err := EvaluateCondition(edge.Condition, state)
		if err != nil {
			log.Errorw("Condition evaluation failed", "node_id", node.ID, "target_id", edge.To, "condition", edge.Condition, "error", err)
			continue
		}

		if met {
			targetNode, ok := parsed.NodeMap[edge.To]
			if !ok {
				continue
			}

			// Circuit breaker check
			count := state.IncrementLoopCount(targetNode.ID)
			if count > 3 {
				log.Warnw("Circuit breaker triggered", "node_id", targetNode.ID, "count", count)
				// Break the loop and mark as error
				result := models.NodeResult{
					NodeID:   targetNode.ID,
					NodeName: targetNode.Name,
					NodeType: targetNode.Type,
					Status:   "error",
					Error:    fmt.Sprintf("circuit breaker: max loop count (3) exceeded for node %s", targetNode.ID),
					Duration: 0,
				}
				resultsMu.Lock()
				*allResults = append(*allResults, result)
				resultsMu.Unlock()
				continue
			}

			log.Infow("Condition met, triggering downstream node", "from", node.ID, "to", targetNode.ID, "condition", edge.Condition, "loop_count", count)

			// Execute the target node
			result := s.executeOrSkip(ctx, *targetNode, state, parsed, failedNodes, failedMu)

			nodesMu.Lock()
			nodeResults[targetNode.ID] = result
			nodesMu.Unlock()

			resultsMu.Lock()
			*allResults = append(*allResults, result)
			resultsMu.Unlock()

			if result.Status == string(models.StatusSuspended) {
				suspendedMu.Lock()
				*suspended = true
				suspendedMu.Unlock()
				return
			}

			if result.Status == "success" {
				// Recurse for the target node's outgoing edges
				s.handleOutgoingEdges(ctx, *targetNode, state, parsed, currentLevel, allResults, resultsMu, failedNodes, failedMu, nodeResults, nodesMu, suspended, suspendedMu)
			}
		}
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
	// Check if any essential upstream dependency has failed
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
