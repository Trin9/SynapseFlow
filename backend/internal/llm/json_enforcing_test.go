package llm

import (
	"context"
	"testing"
)

type sequenceClient struct {
	name  string
	seq   []*ChatResponse
	calls int
}

func (s *sequenceClient) Provider() string { return s.name }

func (s *sequenceClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error) {
	idx := s.calls
	s.calls++
	if idx >= len(s.seq) {
		return &ChatResponse{Content: "{}", Provider: s.name}, nil
	}
	return s.seq[idx], nil
}

func TestJSONEnforcingClient_RetriesAndAggregatesUsage(t *testing.T) {
	inner := &sequenceClient{name: "openai", seq: []*ChatResponse{
		{Content: "not-json", Provider: "openai", Usage: TokenUsage{PromptTokens: 1, CompletionTokens: 2}},
		{Content: "```json\n{\"ok\":true}\n```", Provider: "openai", Usage: TokenUsage{PromptTokens: 3, CompletionTokens: 4}},
	}}

	client := NewJSONEnforcingClient(inner)
	resp, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, ChatOptions{JSONMode: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inner.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", inner.calls)
	}
	if resp.Content != `{"ok":true}` {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 4 || resp.Usage.CompletionTokens != 6 || resp.Usage.TotalTokens != 10 {
		t.Fatalf("unexpected usage: %#v", resp.Usage)
	}
}
