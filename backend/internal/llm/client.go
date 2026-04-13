package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// LLMClient Interface
// ---------------------------------------------------------------------------

// Message represents a single chat message (system, user, assistant).
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatOptions controls LLM request behavior.
type ChatOptions struct {
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	JSONMode    bool    `json:"json_mode,omitempty"` // Request structured JSON output
	Timeout     time.Duration
}

// TokenUsage tracks token consumption for cost/billing awareness.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse holds the result from an LLM API call.
type ChatResponse struct {
	Content      string     `json:"content"`
	Usage        TokenUsage `json:"usage"`
	Model        string     `json:"model"`
	Provider     string     `json:"provider"` // "openai", "anthropic", etc.
	FinishReason string     `json:"finish_reason,omitempty"`
}

// LLMClient is the core abstraction for LLM providers.
// Per AGENTS.md: The LLM is a Node, not a Scheduler. This interface enables
// swapping providers while keeping the engine deterministic.
type LLMClient interface {
	// Chat sends a conversation to the LLM and returns the response.
	Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error)

	// Provider returns the provider name (e.g., "openai", "anthropic").
	Provider() string
}

// ---------------------------------------------------------------------------
// Provider Configuration
// ---------------------------------------------------------------------------

// ProviderConfig holds connection parameters for an LLM provider.
type ProviderConfig struct {
	Provider string `json:"provider"` // "openai" or "anthropic"
	APIURL   string `json:"api_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// LLMError represents a structured error from LLM operations.
type LLMError struct {
	Provider   string `json:"provider"`
	StatusCode int    `json:"status_code,omitempty"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable"`
}

func (e *LLMError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("[%s] HTTP %d: %s", e.Provider, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Provider, e.Message)
}

// ---------------------------------------------------------------------------
// JSON Mode Helpers
// ---------------------------------------------------------------------------

// ValidateJSON checks if the content is valid JSON. Returns the parsed result.
func ValidateJSON(content string) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON output from LLM: %w", err)
	}
	return raw, nil
}
