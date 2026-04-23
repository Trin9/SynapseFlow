package engine

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// ScriptExecutor executes bash script commands (Hard Node).
// It is 100% deterministic — no LLM calls, zero token consumption.
//
// If Writer is non-nil and an episode_id is available (from node config or
// GlobalState key "__episode_id__"), each successful execution appends a
// fact-evidence entry to the Episode via EpisodeWriter.AppendFact.
type ScriptExecutor struct {
	Writer *EpisodeWriter // optional; nil = no Episode tracking
}

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

	// Episode tracking: Hard Node appends fact evidence.
	if e.Writer != nil {
		episodeID := configString(node.Config, "episode_id")
		if episodeID == "" {
			episodeID = state.GetString("__episode_id__")
		}
		if episodeID != "" {
			_ = e.Writer.AppendFact(ctx, episodeID, node.ID, node.Type, node.Name, output)
		}
	}

	return result
}
