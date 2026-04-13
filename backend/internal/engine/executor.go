package engine

import (
	"context"
	"regexp"

	"github.com/xunchenzheng/synapse/pkg/models"
)

// ---------------------------------------------------------------------------
// NodeExecutor Interface
// ---------------------------------------------------------------------------

// NodeExecutor defines the contract for executing a single node.
// Different node types (script, llm, mcp, human) implement this interface.
type NodeExecutor interface {
	// Execute runs the node's action using the provided global state.
	// It should read inputs from state and write outputs back to state.
	// Returns a NodeResult capturing the execution outcome.
	Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult
}

// ---------------------------------------------------------------------------
// Template Rendering
// ---------------------------------------------------------------------------

var templateRegex = regexp.MustCompile(`\{\{(\w+)\}\}`)

// RenderTemplate replaces {{key}} placeholders in a template string
// with values from the GlobalState.
func RenderTemplate(tmpl string, state *models.GlobalState) string {
	return templateRegex.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := match[2 : len(match)-2]
		val := state.GetString(key)
		if val != "" {
			return val
		}
		return match // leave placeholder if not found
	})
}
