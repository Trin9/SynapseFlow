package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Trin9/SynapseFlow/backend/pkg/logger"
	"github.com/goccy/go-yaml"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// ToolInfo is the API-facing description of an available MCP tool.
// `qualified_name` is always present and is stable across tool name collisions.
type ToolInfo struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name"`
	Server        string `json:"server"`
	Description   string `json:"description,omitempty"`
	// InputSchema is the MCP tool's JSON schema (if provided by the server).
	// It is surfaced to the frontend so it can build argument forms.
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// ToolCaller is the minimal surface the API and engine need.
type ToolCaller interface {
	ListTools(ctx context.Context) ([]ToolInfo, error)
	CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error)
}

type serversConfig struct {
	Servers []serverConfig `yaml:"servers"`
}

type serverConfig struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Transport   string            `yaml:"transport"`
	Optional    bool              `yaml:"optional"`
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Env         map[string]string `yaml:"env"`
	URL         string            `yaml:"url"`
}

type serverConn struct {
	name        string
	description string
	transport   transport.Interface
	client      *client.Client
}

type toolRef struct {
	serverName string
	toolName   string
	client     *client.Client
	info       ToolInfo
}

const qualifiedSep = "/"

// Manager owns MCP tool discovery and invocation.
// It reads backend/configs/mcp_servers.yaml and connects to servers using both stdio and sse transports.
// Tools are cached at startup (first use) for fast list/call.
type Manager struct {
	configPath string

	initOnce sync.Once
	initErr  error

	mu      sync.RWMutex
	servers map[string]*serverConn
	byName  map[string][]toolRef
	byQual  map[string]toolRef
}

func DefaultServersConfigPath() string {
	// backend/configs/mcp_servers.yaml
	// Relative path is resolved from the backend module working directory.
	return filepath.Join("configs", "mcp_servers.yaml")
}

func NewManager(configPath string) *Manager {
	if configPath == "" {
		configPath = DefaultServersConfigPath()
	}
	return &Manager{
		configPath: configPath,
		servers:    make(map[string]*serverConn),
		byName:     make(map[string][]toolRef),
		byQual:     make(map[string]toolRef),
	}
}

func (m *Manager) ListTools(ctx context.Context) ([]ToolInfo, error) {
	if err := m.ensureInit(ctx); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]ToolInfo, 0, len(m.byQual))
	for _, ref := range m.byQual {
		list = append(list, ref.info)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Server != list[j].Server {
			return list[i].Server < list[j].Server
		}
		return list[i].Name < list[j].Name
	})
	return list, nil
}

func (m *Manager) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	if err := m.ensureInit(ctx); err != nil {
		return "", err
	}
	ref, err := m.resolveToolRef(toolName)
	if err != nil {
		return "", err
	}

	res, err := ref.client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      ref.toolName,
			Arguments: args,
		},
	})
	if err != nil {
		return "", err
	}

	text := extractText(res)
	if res.IsError {
		if text == "" {
			text = "tool returned error"
		}
		return "", fmt.Errorf("%s", text)
	}
	if text != "" {
		return text, nil
	}
	if res.StructuredContent != nil {
		b, err := json.Marshal(res.StructuredContent)
		if err != nil {
			return "", fmt.Errorf("marshal structured content: %w", err)
		}
		return string(b), nil
	}
	return "", nil
}

// Close releases resources held by connected MCP servers.
//
// Note: Manager initialization is one-shot (sync.Once). After Close, the manager
// should be considered unusable for further ListTools/CallTool.
func (m *Manager) Close(ctx context.Context) error {
	_ = ctx

	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for name, s := range m.servers {
		if s == nil {
			continue
		}
		if s.client != nil {
			if err := s.client.Close(); err != nil {
				errs = append(errs, fmt.Errorf("mcp server %q: client close: %w", name, err))
			}
		}
		if s.transport != nil {
			if err := s.transport.Close(); err != nil {
				errs = append(errs, fmt.Errorf("mcp server %q: transport close: %w", name, err))
			}
		}
	}

	// Drop references to allow GC.
	m.servers = make(map[string]*serverConn)
	m.byName = make(map[string][]toolRef)
	m.byQual = make(map[string]toolRef)

	return errors.Join(errs...)
}

func (m *Manager) ensureInit(ctx context.Context) error {
	m.initOnce.Do(func() {
		m.initErr = m.init(ctx)
	})
	return m.initErr
}

func (m *Manager) init(ctx context.Context) error {
	cfg, err := m.loadConfig()
	if err != nil {
		return err
	}
	if len(cfg.Servers) == 0 {
		return nil
	}

	log := logger.L()

	for _, s := range cfg.Servers {
		if s.Name == "" {
			continue
		}
		perServerCtx := ctx
		cancel := func() {}
		if _, ok := ctx.Deadline(); !ok {
			// Avoid hanging /api/v1/tools forever on misconfigured or unhealthy MCP servers.
			perServerCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		}
		conn, tools, err := connectAndDiscover(perServerCtx, s)
		cancel()
		if err != nil {
			if s.Optional {
				log.Warnw("MCP server connect failed (optional)", "name", s.Name, "transport", s.Transport, "error", err)
				continue
			}
			return fmt.Errorf("mcp server %q: %w", s.Name, err)
		}

		m.mu.Lock()
		m.servers[s.Name] = conn
		for _, t := range tools {
			q := qualify(s.Name, t.Name)
			var schema json.RawMessage
			// InputSchema is optional in MCP; marshaling an empty schema typically yields "{}".
			if b, err := json.Marshal(t.InputSchema); err == nil {
				bb := strings.TrimSpace(string(b))
				if bb != "" && bb != "null" && bb != "{}" {
					schema = b
				}
			}
			info := ToolInfo{
				Name:          t.Name,
				QualifiedName: q,
				Server:        s.Name,
				Description:   t.Description,
				InputSchema:   schema,
			}
			ref := toolRef{
				serverName: s.Name,
				toolName:   t.Name,
				client:     conn.client,
				info:       info,
			}
			m.byQual[q] = ref
			m.byName[t.Name] = append(m.byName[t.Name], ref)
		}
		m.mu.Unlock()

		log.Infow("MCP server connected", "name", s.Name, "transport", s.Transport, "tool_count", len(tools))
	}

	return nil
}

func (m *Manager) loadConfig() (*serversConfig, error) {
	b, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &serversConfig{}, nil
		}
		return nil, fmt.Errorf("read mcp config: %w", err)
	}
	var cfg serversConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse mcp config: %w", err)
	}
	return &cfg, nil
}

func connectAndDiscover(ctx context.Context, cfg serverConfig) (*serverConn, []mcp.Tool, error) {
	tr, err := buildTransport(cfg)
	if err != nil {
		return nil, nil, err
	}
	c := client.NewClient(tr)
	if err := c.Start(ctx); err != nil {
		_ = tr.Close()
		return nil, nil, fmt.Errorf("start: %w", err)
	}
	_, err = c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "synapse",
				Version: "0.1.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	})
	if err != nil {
		_ = c.Close()
		return nil, nil, fmt.Errorf("initialize: %w", err)
	}
	toolsRes, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		_ = c.Close()
		return nil, nil, fmt.Errorf("list tools: %w", err)
	}

	conn := &serverConn{
		name:        cfg.Name,
		description: cfg.Description,
		transport:   tr,
		client:      c,
	}
	return conn, toolsRes.Tools, nil
}

func buildTransport(cfg serverConfig) (transport.Interface, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Transport)) {
	case "stdio":
		if cfg.Command == "" {
			return nil, fmt.Errorf("missing command for stdio transport")
		}
		// Inherit the current process environment by default.
		// Then overlay any explicitly configured env values.
		envMap := make(map[string]string)
		for _, kv := range os.Environ() {
			k, v, ok := strings.Cut(kv, "=")
			if ok && k != "" {
				envMap[k] = v
			}
		}
		for k, v := range cfg.Env {
			if k != "" {
				envMap[k] = v
			}
		}
		env := make([]string, 0, len(envMap))
		for k, v := range envMap {
			env = append(env, k+"="+v)
		}
		return transport.NewStdio(cfg.Command, env, cfg.Args...), nil
	case "sse":
		if cfg.URL == "" {
			return nil, fmt.Errorf("missing url for sse transport")
		}
		return transport.NewSSE(cfg.URL)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", cfg.Transport)
	}
}

func qualify(serverName, toolName string) string {
	return serverName + qualifiedSep + toolName
}

func splitQualified(s string) (serverName, toolName string, ok bool) {
	for _, sep := range []string{qualifiedSep, "::", "."} {
		if strings.Contains(s, sep) {
			parts := strings.SplitN(s, sep, 2)
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				return parts[0], parts[1], true
			}
		}
	}
	return "", "", false
}

func (m *Manager) resolveToolRef(name string) (toolRef, error) {
	// Fast path: qualified name.
	m.mu.RLock()
	ref, ok := m.byQual[name]
	m.mu.RUnlock()
	if ok {
		return ref, nil
	}

	if server, tool, ok := splitQualified(name); ok {
		q := qualify(server, tool)
		m.mu.RLock()
		ref, ok := m.byQual[q]
		m.mu.RUnlock()
		if ok {
			return ref, nil
		}
		return toolRef{}, fmt.Errorf("tool not found: %s", name)
	}

	// Unqualified name resolution.
	m.mu.RLock()
	candidates := m.byName[name]
	m.mu.RUnlock()
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) == 0 {
		return toolRef{}, fmt.Errorf("tool not found: %s", name)
	}
	quals := make([]string, 0, len(candidates))
	for _, c := range candidates {
		quals = append(quals, c.info.QualifiedName)
	}
	sort.Strings(quals)
	return toolRef{}, fmt.Errorf("tool name %q is ambiguous; use one of: %s", name, strings.Join(quals, ", "))
}

func extractText(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	var parts []string
	for _, c := range res.Content {
		switch v := c.(type) {
		case mcp.TextContent:
			if v.Text != "" {
				parts = append(parts, v.Text)
			}
		case *mcp.TextContent:
			if v != nil && v.Text != "" {
				parts = append(parts, v.Text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
