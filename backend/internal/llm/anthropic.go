package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ---------------------------------------------------------------------------
// Anthropic Client (Claude Messages API)
// ---------------------------------------------------------------------------

const (
	defaultAnthropicURL   = "https://api.anthropic.com/v1/messages"
	defaultAnthropicModel = "claude-sonnet-4-20250514"
	anthropicAPIVersion   = "2023-06-01"
)

// AnthropicClient implements LLMClient for the native Claude Messages API.
type AnthropicClient struct {
	config     ProviderConfig
	httpClient *http.Client
}

// NewAnthropicClient creates a new Anthropic client.
func NewAnthropicClient(cfg ProviderConfig) *AnthropicClient {
	if cfg.APIURL == "" {
		cfg.APIURL = defaultAnthropicURL
	}
	if cfg.Model == "" {
		cfg.Model = defaultAnthropicModel
	}
	cfg.Provider = "anthropic"

	return &AnthropicClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *AnthropicClient) Provider() string { return "anthropic" }

func (c *AnthropicClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error) {
	model := opts.Model
	if model == "" {
		model = c.config.Model
	}

	// Anthropic API: system prompt is a top-level field, not a message.
	// Separate system messages from user/assistant messages.
	var systemPrompt string
	var apiMessages []anthropicMessage

	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt = m.Content
		} else {
			apiMessages = append(apiMessages, anthropicMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	// Ensure at least one user message
	if len(apiMessages) == 0 {
		return nil, &LLMError{Provider: "anthropic", Message: "at least one user message is required"}
	}

	maxTokens := 4096
	if opts.MaxTokens > 0 {
		maxTokens = opts.MaxTokens
	}

	reqBody := anthropicRequest{
		Model:     model,
		Messages:  apiMessages,
		MaxTokens: maxTokens,
	}

	if systemPrompt != "" {
		reqBody.System = systemPrompt
	}

	// Temperature
	if opts.Temperature > 0 {
		reqBody.Temperature = ptrFloat(opts.Temperature)
	} else {
		reqBody.Temperature = ptrFloat(0.1)
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &LLMError{Provider: "anthropic", Message: fmt.Sprintf("marshal request: %v", err)}
	}

	// Apply per-request timeout
	reqCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, "POST", c.config.APIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &LLMError{Provider: "anthropic", Message: fmt.Sprintf("create request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &LLMError{
			Provider:  "anthropic",
			Message:   fmt.Sprintf("http call: %v", err),
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &LLMError{Provider: "anthropic", Message: fmt.Sprintf("read response: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode == 429 || resp.StatusCode >= 500
		return nil, &LLMError{
			Provider:   "anthropic",
			StatusCode: resp.StatusCode,
			Message:    string(respBytes),
			Retryable:  retryable,
		}
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, &LLMError{Provider: "anthropic", Message: fmt.Sprintf("unmarshal response: %v", err)}
	}

	if apiResp.Error != nil {
		return nil, &LLMError{
			Provider: "anthropic",
			Message:  fmt.Sprintf("%s: %s", apiResp.Error.Type, apiResp.Error.Message),
		}
	}

	// Extract text content from content blocks
	var content string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	if content == "" {
		return nil, &LLMError{Provider: "anthropic", Message: "no text content in response"}
	}

	return &ChatResponse{
		Content:      content,
		Provider:     "anthropic",
		Model:        apiResp.Model,
		FinishReason: apiResp.StopReason,
		Usage: TokenUsage{
			PromptTokens:     apiResp.Usage.InputTokens,
			CompletionTokens: apiResp.Usage.OutputTokens,
			TotalTokens:      apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Anthropic API types
// ---------------------------------------------------------------------------

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Model      string                  `json:"model"`
	Role       string                  `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
