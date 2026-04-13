package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/xunchenzheng/synapse/internal/llm"
	"github.com/xunchenzheng/synapse/pkg/models"
)

// ---------------------------------------------------------------------------
// Default system prompt for SRE analysis nodes
// ---------------------------------------------------------------------------

const defaultSRESystemPrompt = "You are a senior SRE root-cause analysis engine. " +
	"Based ONLY on the provided factual data, perform logical reasoning and output a structured JSON conclusion. " +
	"You must NOT suggest calling any external tools, APIs, or commands. " +
	"You must NOT hallucinate information not present in the provided data. " +
	"Output ONLY valid JSON with keys: root_cause, severity (critical/high/medium/low), confidence (0-100), explanation, recommended_action."

// ---------------------------------------------------------------------------
// LLMExecutor: Soft Node executor (uses llm.LLMClient abstraction)
// ---------------------------------------------------------------------------

// LLMExecutor calls an LLM API for reasoning tasks via the llm.LLMClient interface.
// Per AGENTS.md: LLM is a Node, not a Scheduler. It receives collected facts
// from GlobalState and outputs structured JSON conclusions.
type LLMExecutor struct {
	Client llm.LLMClient
}

func (e *LLMExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()
	result := models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
	}

	systemPrompt := defaultSRESystemPrompt

	// Check if node has custom system prompt in config
	if node.Config != nil {
		if sp, ok := node.Config["system_prompt"].(string); ok && sp != "" {
			systemPrompt = sp
		}
	}

	// Resolve prompt: prefer node.Action, fall back to config["prompt"]
	promptTemplate := node.Action
	if promptTemplate == "" {
		if node.Config != nil {
			if p, ok := node.Config["prompt"].(string); ok {
				promptTemplate = p
			}
		}
	}

	// Render user prompt with state values
	userPrompt := RenderTemplate(promptTemplate, state)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := llm.ChatOptions{
		JSONMode: true, // SRE analysis nodes always output JSON
	}

	// Allow node config to override options
	if node.Config != nil {
		if temp, ok := node.Config["temperature"].(float64); ok {
			opts.Temperature = temp
		}
		if model, ok := node.Config["model"].(string); ok {
			opts.Model = model
		}
		if maxTokens, ok := node.Config["max_tokens"].(float64); ok {
			opts.MaxTokens = int(maxTokens)
		}
		// Allow disabling JSON mode for free-form analysis
		if jsonMode, ok := node.Config["json_mode"].(bool); ok {
			opts.JSONMode = jsonMode
		}
	}

	resp, err := e.Client.Chat(ctx, messages, opts)
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("LLM call failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Status = "success"
	result.Output = resp.Content
	result.TokensIn = resp.Usage.PromptTokens
	result.TokensOut = resp.Usage.CompletionTokens
	result.Duration = time.Since(start)

	state.Set(node.ID, resp.Content)

	return result
}

// ---------------------------------------------------------------------------
// MockLLMExecutor: For testing without real LLM API
// ---------------------------------------------------------------------------

// MockLLMExecutor simulates an LLM response for testing/development.
type MockLLMExecutor struct{}

func (e *MockLLMExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()

	// Resolve prompt: prefer node.Action, fall back to config["prompt"]
	promptTemplate := node.Action
	if promptTemplate == "" {
		if node.Config != nil {
			if p, ok := node.Config["prompt"].(string); ok {
				promptTemplate = p
			}
		}
	}

	userPrompt := RenderTemplate(promptTemplate, state)

	mockResponse := fmt.Sprintf(`{
  "root_cause": "Mock analysis based on provided data",
  "severity": "medium",
  "confidence": 75,
  "explanation": "This is a mock LLM response. In production, the LLM would analyze the collected facts and provide real root cause analysis. Input data length: %d chars",
  "recommended_action": "Configure LLM_API_KEY to enable real LLM analysis"
}`, len(userPrompt))

	result := models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
		Status:   "success",
		Output:   mockResponse,
		Duration: time.Since(start),
	}

	state.Set(node.ID, mockResponse)

	return result
}
