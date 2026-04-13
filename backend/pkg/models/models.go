package models

import (
	"fmt"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Node Types
// ---------------------------------------------------------------------------

type NodeType string

const (
	NodeTypeScript NodeType = "script"
	NodeTypeLLM    NodeType = "llm"
	NodeTypeMCP    NodeType = "mcp"
	NodeTypeHuman  NodeType = "human"
	NodeTypeRouter NodeType = "router"
)

// ---------------------------------------------------------------------------
// DAG Configuration (persisted / received from frontend)
// ---------------------------------------------------------------------------

// DAGConfig represents a complete workflow definition.
type DAGConfig struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Nodes       []Node    `json:"nodes"`
	Edges       []Edge    `json:"edges"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// Node represents a single node in the DAG.
type Node struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Type   NodeType               `json:"type"`
	Action string                 `json:"action"`           // bash command, LLM prompt template, MCP tool name, etc.
	Config map[string]interface{} `json:"config,omitempty"` // additional per-type configuration
}

// Edge represents a directed connection between two nodes.
type Edge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"` // JSON path expression for conditional routing
}

// ---------------------------------------------------------------------------
// Execution State
// ---------------------------------------------------------------------------

// ExecutionStatus represents the lifecycle of a workflow execution.
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
	StatusTimeout   ExecutionStatus = "timeout"
	StatusSuspended ExecutionStatus = "suspended" // waiting for human approval
)

// Execution represents a single run of a DAG workflow.
type Execution struct {
	ID        string          `json:"id"`
	DAGID     string          `json:"dag_id"`
	DAGName   string          `json:"dag_name"`
	Status    ExecutionStatus `json:"status"`
	State     *GlobalState    `json:"-"` // runtime state, not directly serialized
	Results   []NodeResult    `json:"results,omitempty"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   *time.Time      `json:"ended_at,omitempty"`
	Duration  time.Duration   `json:"duration_ms"`
	Error     string          `json:"error,omitempty"`
}

// NodeResult captures the execution outcome of a single node.
type NodeResult struct {
	NodeID    string        `json:"node_id"`
	NodeName  string        `json:"node_name"`
	NodeType  NodeType      `json:"node_type"`
	Status    string        `json:"status"` // "success", "error", "skipped"
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration_ms"`
	TokensIn  int           `json:"tokens_in,omitempty"`
	TokensOut int           `json:"tokens_out,omitempty"`
}

// ---------------------------------------------------------------------------
// GlobalState: Thread-safe state shared across all nodes in an execution.
// ---------------------------------------------------------------------------

// GlobalState holds key-value pairs accumulated during workflow execution.
type GlobalState struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewGlobalState creates an empty GlobalState.
func NewGlobalState() *GlobalState {
	return &GlobalState{data: make(map[string]interface{})}
}

// Set stores a value by key (thread-safe).
func (s *GlobalState) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Get retrieves a value by key (thread-safe).
func (s *GlobalState) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// GetString retrieves a string value. Returns "" if not found or wrong type.
func (s *GlobalState) GetString(key string) string {
	v, ok := s.Get(key)
	if !ok {
		return ""
	}
	str, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return str
}

// Snapshot returns a copy of the entire state (for serialization / logging).
func (s *GlobalState) Snapshot() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := make(map[string]interface{}, len(s.data))
	for k, v := range s.data {
		snap[k] = v
	}
	return snap
}

// Merge patches multiple key-value pairs into the state.
func (s *GlobalState) Merge(kv map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range kv {
		s.data[k] = v
	}
}
