package llm

import (
	"context"
	"errors"
	"testing"
)

type fakeClient struct {
	name  string
	resp  *ChatResponse
	err   error
	calls int
}

func (f *fakeClient) Provider() string { return f.name }

func (f *fakeClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error) {
	f.calls++
	return f.resp, f.err
}

func TestFallbackClient_TriesNextOnFailure(t *testing.T) {
	first := &fakeClient{name: "a", err: errors.New("boom")}
	second := &fakeClient{name: "b", resp: &ChatResponse{Content: "ok", Provider: "b", Model: "m"}}

	client := NewFallbackClient(first, second)
	resp, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, ChatOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if first.calls != 1 || second.calls != 1 {
		t.Fatalf("expected 1 call each, got first=%d second=%d", first.calls, second.calls)
	}
}
