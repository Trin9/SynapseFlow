package main

import (
	"context"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	srv := server.NewMCPServer("stdio-test", "0.0.1")

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
		msg := req.GetString("msg", "")
		return mcp.NewToolResultText("echo:" + msg), nil
	})

	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("serve stdio: %v", err)
	}
}
