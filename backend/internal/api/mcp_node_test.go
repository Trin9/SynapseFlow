package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/xunchenzheng/synapse/internal/mcp"
)

type fakeMCP struct{}

func (f *fakeMCP) ListTools(ctx context.Context) ([]mcp.ToolInfo, error) {
	_ = ctx
	return []mcp.ToolInfo{{Name: "echo_tool", Description: "test"}}, nil
}

func (f *fakeMCP) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	_ = ctx
	if toolName != "echo_tool" {
		return "", nil
	}
	msg, _ := args["msg"].(string)
	return "echo:" + msg, nil
}

func TestRunInline_WithMCPNode(t *testing.T) {
	s := NewServer(WithMCPManager(&fakeMCP{}))

	dag := map[string]interface{}{
		"name": "mcp dag",
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo hi"},
			{
				"id":     "b",
				"name":   "B",
				"type":   "mcp",
				"action": "echo_tool",
				"config": map[string]interface{}{"arguments": map[string]interface{}{"msg": "{{a}}"}},
			},
		},
		"edges": []map[string]interface{}{{"from": "a", "to": "b"}},
	}

	runRec := doJSON(t, s, http.MethodPost, "/api/v1/run", dag)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("run expected 202, got %d: %s", runRec.Code, runRec.Body.String())
	}
	var runResp map[string]interface{}
	if err := json.Unmarshal(runRec.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("unmarshal run: %v", err)
	}
	execID, _ := runResp["execution_id"].(string)
	if execID == "" {
		t.Fatalf("expected execution_id")
	}

	w := waitExecutionDone(t, s, execID)
	if w["status"] != "completed" {
		t.Fatalf("expected completed, got %v", w["status"])
	}

	results, _ := w["results"].([]interface{})
	found := false
	for _, r := range results {
		m, _ := r.(map[string]interface{})
		if m["node_id"] == "b" {
			found = true
			if m["status"] != "success" {
				t.Fatalf("expected mcp node success, got %v", m["status"])
			}
			if m["output"] != "echo:hi" {
				t.Fatalf("expected rendered output echo:hi, got %v", m["output"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected node b result in %v", results)
	}
}
