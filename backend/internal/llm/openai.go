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
// OpenAI Client
// ---------------------------------------------------------------------------

const (
	defaultOpenAIURL   = "https://api.openai.com/v1/chat/completions"
	defaultOpenAIModel = "gpt-4o-mini"
)

// OpenAIClient implements LLMClient for OpenAI and any OpenAI-compatible API
// (e.g., Ollama, Azure OpenAI, vLLM, etc.).
type OpenAIClient struct {
	config     ProviderConfig
	httpClient *http.Client
}

// NewOpenAIClient creates a new OpenAI client.
func NewOpenAIClient(cfg ProviderConfig) *OpenAIClient {
	if cfg.APIURL == "" {
		cfg.APIURL = defaultOpenAIURL
	}
	if cfg.Model == "" {
		cfg.Model = defaultOpenAIModel
	}
	cfg.Provider = "openai"

	return &OpenAIClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *OpenAIClient) Provider() string { return "openai" }

func (c *OpenAIClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error) {
	model := opts.Model
	if model == "" {
		model = c.config.Model
	}

	// Build request body
	reqBody := openAIRequest{
		Model:    model,
		Messages: make([]openAIMessage, len(messages)),
	}

	// Set temperature (default 0.1 for deterministic SRE analysis)
	reqBody.Temperature = ptrFloat(0.1)
	if opts.Temperature > 0 {
		reqBody.Temperature = ptrFloat(opts.Temperature)
	}

	if opts.MaxTokens > 0 {
		reqBody.MaxTokens = &opts.MaxTokens
	}

	// JSON mode: OpenAI supports response_format
	if opts.JSONMode {
		reqBody.ResponseFormat = &openAIResponseFormat{Type: "json_object"}
	}

	for i, m := range messages {
		reqBody.Messages[i] = openAIMessage{Role: m.Role, Content: m.Content}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &LLMError{Provider: "openai", Message: fmt.Sprintf("marshal request: %v", err)}
	}

	// Apply per-request timeout if specified
	reqCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, "POST", c.config.APIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &LLMError{Provider: "openai", Message: fmt.Sprintf("create request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &LLMError{
			Provider:  "openai",
			Message:   fmt.Sprintf("http call: %v", err),
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &LLMError{Provider: "openai", Message: fmt.Sprintf("read response: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode == 429 || resp.StatusCode >= 500
		return nil, &LLMError{
			Provider:   "openai",
			StatusCode: resp.StatusCode,
			Message:    string(respBytes),
			Retryable:  retryable,
		}
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, &LLMError{Provider: "openai", Message: fmt.Sprintf("unmarshal response: %v", err)}
	}

	if apiResp.Error != nil {
		return nil, &LLMError{
			Provider: "openai",
			Message:  apiResp.Error.Message,
		}
	}

	if len(apiResp.Choices) == 0 {
		return nil, &LLMError{Provider: "openai", Message: "no choices returned"}
	}

	return &ChatResponse{
		Content:      apiResp.Choices[0].Message.Content,
		Provider:     "openai",
		Model:        apiResp.Model,
		FinishReason: apiResp.Choices[0].FinishReason,
		Usage: TokenUsage{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			TotalTokens:      apiResp.Usage.PromptTokens + apiResp.Usage.CompletionTokens,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// OpenAI API types
// ---------------------------------------------------------------------------

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model          string                `json:"model"`
	Messages       []openAIMessage       `json:"messages"`
	Temperature    *float64              `json:"temperature,omitempty"`
	MaxTokens      *int                  `json:"max_tokens,omitempty"`
	ResponseFormat *openAIResponseFormat `json:"response_format,omitempty"`
}

type openAIResponseFormat struct {
	Type string `json:"type"` // "json_object" or "text"
}

type openAIResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func ptrFloat(f float64) *float64 { return &f }
