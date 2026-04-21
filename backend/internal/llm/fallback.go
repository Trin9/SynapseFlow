package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/Trin9/SynapseFlow/backend/pkg/logger"
)

// ---------------------------------------------------------------------------
// FallbackClient: tries providers in order until one succeeds
// ---------------------------------------------------------------------------

// FallbackClient implements LLMClient by trying multiple providers in order.
// Per AGENTS.md: This supports the resilience requirement — if Claude is down,
// fall back to GPT, etc.
type FallbackClient struct {
	clients []LLMClient
}

// NewFallbackClient creates a client that tries providers in the given order.
// The first client is the primary; subsequent ones are fallbacks.
func NewFallbackClient(clients ...LLMClient) *FallbackClient {
	if len(clients) == 0 {
		panic("FallbackClient requires at least one client")
	}
	return &FallbackClient{clients: clients}
}

func (c *FallbackClient) Provider() string {
	names := make([]string, len(c.clients))
	for i, cl := range c.clients {
		names[i] = cl.Provider()
	}
	return "fallback[" + strings.Join(names, ",") + "]"
}

func (c *FallbackClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error) {
	log := logger.L()
	var lastErr error

	for i, client := range c.clients {
		resp, err := client.Chat(ctx, messages, opts)
		if err == nil {
			if i > 0 {
				log.Infow("Fallback succeeded",
					"primary", c.clients[0].Provider(),
					"succeeded_with", client.Provider(),
					"attempt", i+1,
				)
			}
			return resp, nil
		}

		lastErr = err
		log.Warnw("LLM provider failed, trying next",
			"provider", client.Provider(),
			"error", err.Error(),
			"attempt", i+1,
			"remaining", len(c.clients)-i-1,
		)

		// Only retry on retryable errors for non-last clients
		if llmErr, ok := err.(*LLMError); ok && !llmErr.Retryable && i < len(c.clients)-1 {
			// Non-retryable error (e.g., auth failure) — still try next provider
			// since the next provider might have different credentials
			continue
		}
	}

	return nil, fmt.Errorf("all %d LLM providers failed; last error: %w", len(c.clients), lastErr)
}

// ---------------------------------------------------------------------------
// JSONEnforcingClient: wraps any LLMClient to validate/retry JSON output
// ---------------------------------------------------------------------------

// JSONEnforcingClient wraps an LLMClient and ensures the output is valid JSON
// when JSONMode is requested. If the first attempt returns invalid JSON, it
// retries once with an explicit correction prompt.
type JSONEnforcingClient struct {
	inner      LLMClient
	maxRetries int
}

// NewJSONEnforcingClient wraps a client with JSON validation and retry.
func NewJSONEnforcingClient(inner LLMClient) *JSONEnforcingClient {
	return &JSONEnforcingClient{
		inner:      inner,
		maxRetries: 1,
	}
}

func (c *JSONEnforcingClient) Provider() string {
	return c.inner.Provider()
}

func (c *JSONEnforcingClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error) {
	resp, err := c.inner.Chat(ctx, messages, opts)
	if err != nil {
		return nil, err
	}

	// If JSON mode is not requested, return as-is
	if !opts.JSONMode {
		return resp, nil
	}

	// Validate JSON
	if _, jsonErr := ValidateJSON(resp.Content); jsonErr == nil {
		return resp, nil // Valid JSON on first try
	}

	log := logger.L()
	log.Warnw("LLM returned invalid JSON, retrying with correction prompt",
		"provider", c.inner.Provider(),
		"content_length", len(resp.Content),
	)

	// Retry: append the invalid response and a correction message
	retryMessages := make([]Message, len(messages))
	copy(retryMessages, messages)

	retryMessages = append(retryMessages,
		Message{Role: "assistant", Content: resp.Content},
		Message{Role: "user", Content: "Your previous response was not valid JSON. Please output ONLY valid JSON with no markdown formatting, no code fences, and no explanatory text. Output the raw JSON object directly."},
	)

	retryResp, err := c.inner.Chat(ctx, retryMessages, opts)
	if err != nil {
		return nil, fmt.Errorf("JSON retry failed: %w", err)
	}

	// Try to extract JSON from the retry response (handle markdown fences)
	content := extractJSON(retryResp.Content)

	if _, jsonErr := ValidateJSON(content); jsonErr != nil {
		// Still invalid — return the content but log a warning
		log.Errorw("LLM still returned invalid JSON after retry",
			"provider", c.inner.Provider(),
			"content_preview", truncate(content, 200),
		)
		// Return the response anyway — the caller can decide what to do
		retryResp.Content = content
		return retryResp, nil
	}

	// Aggregate token usage from both attempts
	retryResp.Usage.PromptTokens += resp.Usage.PromptTokens
	retryResp.Usage.CompletionTokens += resp.Usage.CompletionTokens
	retryResp.Usage.TotalTokens = retryResp.Usage.PromptTokens + retryResp.Usage.CompletionTokens
	retryResp.Content = content

	return retryResp, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractJSON attempts to pull JSON out of a response that may have
// markdown code fences or surrounding text.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)

	// Strip ```json ... ``` fences
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		return strings.TrimSpace(s)
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		return strings.TrimSpace(s)
	}

	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
