package models

import (
	"encoding/json"
	"fmt"
	"strings"
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
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Nodes       []Node            `json:"nodes"`
	Edges       []Edge            `json:"edges"`
	CreatedAt   time.Time         `json:"created_at,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at,omitempty"`
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

// ExecutionCheckpoint stores the persisted runtime state needed for resume.
type ExecutionCheckpoint struct {
	ExecutionID string                 `json:"execution_id"`
	DAGID       string                 `json:"dag_id"`
	State       map[string]interface{} `json:"state"`
	LoopCounts  map[string]int         `json:"loop_counts"`
	UpdatedAt   time.Time              `json:"updated_at"`
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

// Experience stores a reusable troubleshooting memory entry.
type Experience struct {
	ID          string    `json:"id"`
	AlertType   string    `json:"alert_type,omitempty"`
	ServiceName string    `json:"service_name,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Symptom     string    `json:"symptom,omitempty"`
	RootCause   string    `json:"root_cause,omitempty"`
	ActionTaken string    `json:"action_taken,omitempty"`
	Summary     string    `json:"summary,omitempty"`
	Document    string    `json:"document,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Score       float64   `json:"score,omitempty"`

	Embedding []float64 `json:"-"`
}

// ---------------------------------------------------------------------------
// GlobalState: Thread-safe state shared across all nodes in an execution.
// ---------------------------------------------------------------------------

// GlobalState holds key-value pairs accumulated during workflow execution.
type GlobalState struct {
	mu         sync.RWMutex
	data       map[string]interface{}
	loopCounts map[string]int
}

// NewGlobalState creates an empty GlobalState.
func NewGlobalState() *GlobalState {
	return &GlobalState{
		data:       make(map[string]interface{}),
		loopCounts: make(map[string]int),
	}
}

// NewGlobalStateFromSnapshot recreates a state object from persisted snapshots.
func NewGlobalStateFromSnapshot(data map[string]interface{}, loopCounts map[string]int) *GlobalState {
	state := NewGlobalState()
	if data != nil {
		state.Merge(data)
	}
	if loopCounts != nil {
		state.mu.Lock()
		for key, value := range loopCounts {
			state.loopCounts[key] = value
		}
		state.mu.Unlock()
	}
	return state
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
	return formatStateValue(v)
}

// GetPath retrieves a nested value using dotted-path notation such as "plan.next_action".
func (s *GlobalState) GetPath(path string) (interface{}, bool) {
	if path == "" {
		return nil, false
	}

	parts := strings.Split(path, ".")
	s.mu.RLock()
	defer s.mu.RUnlock()

	current, ok := s.data[parts[0]]
	if !ok {
		return nil, false
	}

	for _, part := range parts[1:] {
		next, ok := getNestedValue(current, part)
		if !ok {
			return nil, false
		}
		current = next
	}

	return current, true
}

// GetPathString retrieves a nested value and formats it as a string.
func (s *GlobalState) GetPathString(path string) string {
	v, ok := s.GetPath(path)
	if !ok {
		return ""
	}
	return formatStateValue(v)
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

// LoopCountsSnapshot returns a copy of loop counters for persistence.
func (s *GlobalState) LoopCountsSnapshot() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := make(map[string]int, len(s.loopCounts))
	for k, v := range s.loopCounts {
		snap[k] = v
	}
	return snap
}

// Clone creates a deep copy of the state.
func (s *GlobalState) Clone() *GlobalState {
	if s == nil {
		return nil
	}
	return NewGlobalStateFromSnapshot(s.Snapshot(), s.LoopCountsSnapshot())
}

// Merge patches multiple key-value pairs into the state.
func (s *GlobalState) Merge(kv map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range kv {
		s.data[k] = v
	}
}

// IncrementLoopCount increments the loop count for a node and returns the new count.
func (s *GlobalState) IncrementLoopCount(nodeID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loopCounts[nodeID]++
	return s.loopCounts[nodeID]
}

// GetLoopCount returns the current loop count for a node.
func (s *GlobalState) GetLoopCount(nodeID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loopCounts[nodeID]
}

func getNestedValue(current interface{}, key string) (interface{}, bool) {
	switch typed := current.(type) {
	case map[string]interface{}:
		value, ok := typed[key]
		return value, ok
	case map[string]string:
		value, ok := typed[key]
		return value, ok
	default:
		return nil, false
	}
}

func formatStateValue(v interface{}) string {
	str, ok := v.(string)
	if ok {
		return str
	}
	if data, err := json.Marshal(v); err == nil {
		return string(data)
	}
	return fmt.Sprintf("%v", v)
}
