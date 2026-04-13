package engine

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/xunchenzheng/synapse/pkg/models"
)

// ScriptExecutor executes bash script commands (Hard Node).
// It is 100% deterministic — no LLM calls, zero token consumption.
type ScriptExecutor struct{}

func (e *ScriptExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()
	result := models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
	}

	// Resolve the command: prefer node.Action, fall back to config["command"]
	command := node.Action
	if command == "" {
		if node.Config != nil {
			if cmd, ok := node.Config["command"].(string); ok {
				command = cmd
			}
		}
	}
	if command == "" {
		result.Status = "error"
		result.Error = "no command specified (set 'action' or config.command)"
		result.Duration = time.Since(start)
		return result
	}

	// Render any {{var}} placeholders in the script command
	command = RenderTemplate(command, state)

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		if stderr.Len() > 0 {
			result.Error += "\nstderr: " + stderr.String()
		}
		result.Duration = time.Since(start)
		return result
	}

	output := strings.TrimSpace(stdout.String())
	result.Status = "success"
	result.Output = output
	result.Duration = time.Since(start)

	// Write output to GlobalState so downstream nodes can reference it via {{node_id}}
	state.Set(node.ID, output)

	return result
}
