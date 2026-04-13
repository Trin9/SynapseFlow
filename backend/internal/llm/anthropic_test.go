package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicClient_Chat_SystemExtraction(t *testing.T) {
	var gotKey string
	var gotVersion string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotBody = b

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"x",
			"type":"message",
			"model":"claude-test",
			"role":"assistant",
			"content":[{"type":"text","text":"{\"ok\":true}"}],
			"stop_reason":"end_turn",
			"usage":{"input_tokens":7,"output_tokens":11}
		}`))
	}))
	defer srv.Close()

	client := NewAnthropicClient(ProviderConfig{APIURL: srv.URL, APIKey: "k", Model: "claude-test"})
	resp, err := client.Chat(context.Background(), []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
	}, ChatOptions{MaxTokens: 123})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if gotKey != "k" {
		t.Fatalf("expected x-api-key k, got %q", gotKey)
	}
	if gotVersion == "" {
		t.Fatalf("expected anthropic-version header")
	}

	var req anthropicRequest
	if err := json.Unmarshal(gotBody, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.System != "sys" {
		t.Fatalf("expected system sys, got %q", req.System)
	}
	if req.Model != "claude-test" {
		t.Fatalf("expected model claude-test, got %q", req.Model)
	}
	if req.MaxTokens != 123 {
		t.Fatalf("expected max_tokens 123, got %d", req.MaxTokens)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "hi" {
		t.Fatalf("unexpected messages: %#v", req.Messages)
	}

	if resp.Provider != "anthropic" {
		t.Fatalf("expected provider anthropic, got %q", resp.Provider)
	}
	if resp.Content != `{"ok":true}` {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 7 || resp.Usage.CompletionTokens != 11 || resp.Usage.TotalTokens != 18 {
		t.Fatalf("unexpected usage: %#v", resp.Usage)
	}
}
