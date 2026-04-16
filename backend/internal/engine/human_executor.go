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

	// In M2, we mark as suspended and wait for API resume
	output := fmt.Sprintf("[Human Review Required] %s", instructions)

	state.Set(node.ID, output)

	return models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
		Status:   string(models.StatusSuspended),
		Output:   output,
		Duration: time.Since(start),
	}
}
