package engine

import (
	"context"
	"time"

	"github.com/xunchenzheng/synapse/pkg/models"
)

// RouterExecutor evaluates a condition or expression and sets the result in GlobalState.
// Downstream edges use this result to decide whether to execute.
type RouterExecutor struct{}

func (e *RouterExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()

	// The action or config.expression is rendered and stored as the node's output.
	// Downstream edges can then use {{nodeID}} in their conditions.
	expression := node.Action
	if expression == "" && node.Config != nil {
		if expr, ok := node.Config["expression"].(string); ok {
			expression = expr
		}
	}

	resultValue := RenderTemplate(expression, state)

	state.Set(node.ID, resultValue)

	return models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
		Status:   "success",
		Output:   resultValue,
		Duration: time.Since(start),
	}
}
