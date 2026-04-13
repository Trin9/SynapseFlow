package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIClient_Chat_JSONMode(t *testing.T) {
	var gotAuth string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected content-type application/json, got %q", ct)
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotBody = b

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"x",
			"model":"gpt-test",
			"choices":[{"message":{"role":"assistant","content":"{\"ok\":true}"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":3,"completion_tokens":5}
		}`))
	}))
	defer srv.Close()

	client := NewOpenAIClient(ProviderConfig{APIURL: srv.URL, APIKey: "k", Model: "gpt-test"})
	resp, err := client.Chat(context.Background(), []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
	}, ChatOptions{JSONMode: true})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if gotAuth != "Bearer k" {
		t.Fatalf("expected auth Bearer k, got %q", gotAuth)
	}

	var req openAIRequest
	if err := json.Unmarshal(gotBody, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.Model != "gpt-test" {
		t.Fatalf("expected model gpt-test, got %q", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
		t.Fatalf("expected response_format json_object, got %#v", req.ResponseFormat)
	}

	if resp.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", resp.Provider)
	}
	if resp.Content != `{"ok":true}` {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 3 || resp.Usage.CompletionTokens != 5 || resp.Usage.TotalTokens != 8 {
		t.Fatalf("unexpected usage: %#v", resp.Usage)
	}
}

func TestOpenAIClient_Non200_ReturnsLLMError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limit"))
	}))
	defer srv.Close()

	client := NewOpenAIClient(ProviderConfig{APIURL: srv.URL, APIKey: "k", Model: "gpt-test"})
	_, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, ChatOptions{})
	if err == nil {
		t.Fatalf("expected error")
	}
	llmErr, ok := err.(*LLMError)
	if !ok {
		t.Fatalf("expected *LLMError, got %T", err)
	}
	if llmErr.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", llmErr.Provider)
	}
	if llmErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", llmErr.StatusCode)
	}
	if !llmErr.Retryable {
		t.Fatalf("expected retryable for 429")
	}
	if !strings.Contains(llmErr.Message, "rate limit") {
		t.Fatalf("expected message to contain rate limit, got %q", llmErr.Message)
	}
}
