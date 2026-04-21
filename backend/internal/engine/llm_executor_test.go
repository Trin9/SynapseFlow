package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Trin9/SynapseFlow/backend/internal/llm"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

type fakeLLMClient struct {
	response string
	err      error
}

func (f *fakeLLMClient) Chat(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.ChatResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &llm.ChatResponse{Content: f.response}, nil
}

func (f *fakeLLMClient) Provider() string {
	return "fake"
}

func TestLLMExecutor_WritesStructuredState(t *testing.T) {
	executor := &LLMExecutor{Client: &fakeLLMClient{response: `{"next_action":"search_code","query":"failed to charge card","done":false}`}}
	state := models.NewGlobalState()
	node := models.Node{
		ID:     "planner",
		Name:   "Planner",
		Type:   models.NodeTypeLLM,
		Action: "plan next step",
		Config: map[string]interface{}{
			"json_mode":         true,
			"state_key":         "plan",
			"parse_json_output": true,
		},
	}

	result := executor.Execute(context.Background(), node, state)
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}

	if got := state.GetString("planner"); got == "" {
		t.Fatal("expected raw planner output to be persisted")
	}

	if got := state.GetPathString("plan.next_action"); got != "search_code" {
		t.Fatalf("expected structured next_action, got %q", got)
	}
	if got := state.GetPathString("plan.query"); got != "failed to charge card" {
		t.Fatalf("expected structured query, got %q", got)
	}
}

func TestLLMExecutor_InvalidStructuredOutputFails(t *testing.T) {
	executor := &LLMExecutor{Client: &fakeLLMClient{response: `not-json`}}
	state := models.NewGlobalState()
	node := models.Node{
		ID:     "planner",
		Name:   "Planner",
		Type:   models.NodeTypeLLM,
		Action: "plan next step",
		Config: map[string]interface{}{
			"json_mode":         true,
			"state_key":         "plan",
			"parse_json_output": true,
		},
	}

	result := executor.Execute(context.Background(), node, state)
	if result.Status != "error" {
		t.Fatalf("expected error, got %s", result.Status)
	}
	if state.GetPathString("plan.next_action") != "" {
		t.Fatal("expected invalid structured output not to populate plan state")
	}
}

func TestLLMExecutor_AppliesStructuredDefaults(t *testing.T) {
	executor := &LLMExecutor{Client: &fakeLLMClient{response: `{"next_action":"search_logs"}`}}
	state := models.NewGlobalState()
	node := models.Node{
		ID:     "planner",
		Name:   "Planner",
		Type:   models.NodeTypeLLM,
		Action: "plan next step",
		Config: map[string]interface{}{
			"json_mode":         true,
			"state_key":         "plan",
			"parse_json_output": true,
			"json_defaults": map[string]interface{}{
				"done":   false,
				"report": "",
			},
			"required_json_keys": []interface{}{"next_action", "done"},
		},
	}

	result := executor.Execute(context.Background(), node, state)
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}

	if got := state.GetPathString("plan.done"); got != "false" {
		t.Fatalf("expected default done=false, got %q", got)
	}
	if got := state.GetPathString("plan.report"); got != "" {
		t.Fatalf("expected default empty report, got %q", got)
	}
}

func TestLLMExecutor_DefaultsReplaceNullValues(t *testing.T) {
	executor := &LLMExecutor{Client: &fakeLLMClient{response: `{"next_action":"fetch_snippet","line":null,"symbol":null}`}}
	state := models.NewGlobalState()
	node := models.Node{
		ID:     "planner",
		Name:   "Planner",
		Type:   models.NodeTypeLLM,
		Action: "plan next step",
		Config: map[string]interface{}{
			"json_mode":         true,
			"state_key":         "plan",
			"parse_json_output": true,
			"json_defaults": map[string]interface{}{
				"line":   0,
				"symbol": "",
				"done":   false,
			},
			"required_json_keys": []interface{}{"next_action", "done"},
		},
	}

	result := executor.Execute(context.Background(), node, state)
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}

	if got := state.GetPathString("plan.line"); got != "0" {
		t.Fatalf("expected null line to default to 0, got %q", got)
	}
	if got := state.GetPathString("plan.symbol"); got != "" {
		t.Fatalf("expected null symbol to default to empty string, got %q", got)
	}
}

func TestLLMExecutor_RequiredStructuredKeyFails(t *testing.T) {
	executor := &LLMExecutor{Client: &fakeLLMClient{response: `{"done":false}`}}
	state := models.NewGlobalState()
	node := models.Node{
		ID:     "planner",
		Name:   "Planner",
		Type:   models.NodeTypeLLM,
		Action: "plan next step",
		Config: map[string]interface{}{
			"json_mode":          true,
			"state_key":          "plan",
			"parse_json_output":  true,
			"required_json_keys": []interface{}{"next_action", "done"},
		},
	}

	result := executor.Execute(context.Background(), node, state)
	if result.Status != "error" {
		t.Fatalf("expected error, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "next_action") {
		t.Fatalf("expected missing next_action error, got %q", result.Error)
	}
}

func TestLLMExecutor_PropagatesClientError(t *testing.T) {
	executor := &LLMExecutor{Client: &fakeLLMClient{err: errors.New("boom")}}
	state := models.NewGlobalState()
	node := models.Node{ID: "planner", Name: "Planner", Type: models.NodeTypeLLM, Action: "plan next step"}

	result := executor.Execute(context.Background(), node, state)
	if result.Status != "error" {
		t.Fatalf("expected error, got %s", result.Status)
	}
}
