package mcp

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestManager_SSE_ListAndCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// In-process MCP server exposed over SSE via httptest.
	srv := server.NewMCPServer("test", "0.0.1")
	srv.AddTool(mcp.Tool{
		Name:        "echo",
		Description: "echo tool",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"msg": map[string]any{"type": "string"},
			},
		},
	}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_ = ctx
		args := req.GetArguments()
		msg, _ := args["msg"].(string)
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("echo:" + msg)}}, nil
	})

	h := server.NewTestServer(srv)
	defer h.Close()

	base, err := url.Parse(h.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	// Default MCP SSE base path used by mcp-go test server.
	base.Path = "/sse"

	mgr := NewManager("")
	// Bypass yaml config and connect directly.
	conn, tools, err := connectAndDiscover(ctx, serverConfig{Name: "sse_test", Transport: "sse", URL: base.String()})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = conn.client.Close()
		_ = conn.transport.Close()
	}()
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("unexpected tools: %+v", tools)
	}

	// Seed manager maps the same way init() does.
	mgr.mu.Lock()
	mgr.servers["sse_test"] = conn
	for _, tt := range tools {
		q := qualify("sse_test", tt.Name)
		ref := toolRef{
			serverName: "sse_test",
			toolName:   tt.Name,
			client:     conn.client,
			info: ToolInfo{
				Name:          tt.Name,
				QualifiedName: q,
				Server:        "sse_test",
				Description:   tt.Description,
			},
		}
		mgr.byQual[q] = ref
		mgr.byName[tt.Name] = append(mgr.byName[tt.Name], ref)
	}
	mgr.mu.Unlock()

	got, err := mgr.CallTool(ctx, "sse_test/echo", map[string]interface{}{"msg": "hi"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if got != "echo:hi" {
		t.Fatalf("unexpected output: %q", got)
	}
}
