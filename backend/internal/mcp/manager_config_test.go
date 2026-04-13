package mcp

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestManager_Config_OptionalServerFailureDoesNotBreakList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Good SSE server.
	srv := server.NewMCPServer("test", "0.0.1")
	srv.AddTool(mcp.Tool{Name: "echo"}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_ = ctx
		return mcp.NewToolResultText("ok"), nil
	})
	h := server.NewTestServer(srv)
	defer h.Close()

	base, err := url.Parse(h.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	base.Path = "/sse"

	// Config has one optional-broken server and one working server.
	cfg := `servers:
  - name: bad_optional
    transport: sse
    optional: true
    url: ""
  - name: good
    transport: sse
    url: "` + base.String() + `"
`

	configPath := filepath.Join(t.TempDir(), "mcp_servers.yaml")
	if err := os.WriteFile(configPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	mgr := NewManager(configPath)
	tools, err := mgr.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d: %+v", len(tools), tools)
	}
	if tools[0].QualifiedName != "good/echo" {
		t.Fatalf("unexpected tool: %+v", tools[0])
	}
}
