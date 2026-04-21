package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/Trin9/SynapseFlow/backend/internal/mcp"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// MCPExecutor executes an MCP tool call node.
// node.Action is the tool name.
// node.Config["arguments"] (optional) is a JSON object passed as tool arguments.
// Any string values inside arguments are template-rendered via RenderTemplate.
type MCPExecutor struct {
	MCP mcp.ToolCaller
}

func (e *MCPExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()
	result := models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
	}

	if e == nil || e.MCP == nil {
		result.Status = "error"
		result.Error = "MCP manager is not configured"
		result.Duration = time.Since(start)
		return result
	}

	toolName := node.Action
	if toolName == "" {
		result.Status = "error"
		result.Error = "no MCP tool name specified (set node.action)"
		result.Duration = time.Since(start)
		return result
	}

	args, err := renderMCPArguments(node, state)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result
	}

	out, err := e.MCP.CallTool(ctx, toolName, args)
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("mcp call failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Status = "success"
	result.Output = out
	result.Duration = time.Since(start)
	state.Set(node.ID, out)
	return result
}

func renderMCPArguments(node models.Node, state *models.GlobalState) (map[string]interface{}, error) {
	if node.Config == nil {
		return nil, nil
	}
	raw, ok := node.Config["arguments"]
	if !ok || raw == nil {
		return nil, nil
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("config.arguments must be an object")
	}
	out := make(map[string]interface{}, len(obj))
	for k, v := range obj {
		out[k] = renderMCPValue(v, state)
	}
	return out, nil
}

func renderMCPValue(v interface{}, state *models.GlobalState) interface{} {
	switch x := v.(type) {
	case string:
		return RenderTemplate(x, state)
	case map[string]interface{}:
		m := make(map[string]interface{}, len(x))
		for k, vv := range x {
			m[k] = renderMCPValue(vv, state)
		}
		return m
	case []interface{}:
		a := make([]interface{}, 0, len(x))
		for _, vv := range x {
			a = append(a, renderMCPValue(vv, state))
		}
		return a
	default:
		return v
	}
}
