package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/xunchenzheng/synapse/pkg/models"
)

// ---------------------------------------------------------------------------
// HumanExecutor: Human-in-the-loop checkpoint (M2 full implementation)
// ---------------------------------------------------------------------------

// HumanExecutor is a placeholder that logs the review request and passes through.
// In M2, this will pause execution and wait for manual approval via API.
type HumanExecutor struct{}

func (e *HumanExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()

	instructions := node.Action
	if instructions == "" && node.Config != nil {
		if p, ok := node.Config["instructions"].(string); ok {
			instructions = p
		}
	}

	output := fmt.Sprintf("[Human Review Checkpoint] %s — auto-approved (placeholder)", instructions)

	state.Set(node.ID, output)

	return models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
		Status:   "success",
		Output:   output,
		Duration: time.Since(start),
	}
}

// ---------------------------------------------------------------------------
// RouterExecutor: Conditional routing node (M2 full implementation)
// ---------------------------------------------------------------------------

// RouterExecutor is a placeholder that evaluates simple conditions.
// In M2, this will support JSON Path expressions and dynamic branching.
type RouterExecutor struct{}

func (e *RouterExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()

	// Render the action template to get the routing expression
	expression := RenderTemplate(node.Action, state)
	if expression == "" && node.Config != nil {
		if expr, ok := node.Config["expression"].(string); ok {
			expression = RenderTemplate(expr, state)
		}
	}

	output := fmt.Sprintf("[Router] Evaluated: %s — all branches enabled (placeholder)", expression)

	state.Set(node.ID, output)

	return models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
		Status:   "success",
		Output:   output,
		Duration: time.Since(start),
	}
}
