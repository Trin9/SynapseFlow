package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestManager_Stdio_ListAndCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	modRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}

	out := filepath.Join(t.TempDir(), "mcp-stdio-test-server")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, "./internal/mcp/testdata/stdio_server")
	cmd.Dir = modRoot
	cmd.Env = os.Environ()
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build stdio test server: %v\n%s", err, string(b))
	}

	conn, tools, err := connectAndDiscover(ctx, serverConfig{Name: "stdio_test", Transport: "stdio", Command: out})
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

	mgr := NewManager("")

	// Seed manager maps the same way init() does.
	mgr.mu.Lock()
	mgr.servers["stdio_test"] = conn
	for _, tt := range tools {
		q := qualify("stdio_test", tt.Name)
		ref := toolRef{
			serverName: "stdio_test",
			toolName:   tt.Name,
			client:     conn.client,
			info: ToolInfo{
				Name:          tt.Name,
				QualifiedName: q,
				Server:        "stdio_test",
				Description:   tt.Description,
			},
		}
		mgr.byQual[q] = ref
		mgr.byName[tt.Name] = append(mgr.byName[tt.Name], ref)
	}
	mgr.mu.Unlock()

	got, err := mgr.CallTool(ctx, "stdio_test/echo", map[string]interface{}{"msg": "hi"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if got != "echo:hi" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func findModuleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", os.ErrNotExist
		}
		wd = parent
	}
}
