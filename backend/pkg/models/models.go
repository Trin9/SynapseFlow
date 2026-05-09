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
	NodeTypeScript         NodeType = "script"
	NodeTypeLLM            NodeType = "llm"
	NodeTypeMCP            NodeType = "mcp"
	NodeTypeHuman          NodeType = "human"
	NodeTypeRouter         NodeType = "router"
	NodeTypeWebInteraction NodeType = "web_interaction"
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
	ExecutionID string    `json:"execution_id,omitempty"` // originating execution (M5.2)
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

// ---------------------------------------------------------------------------
// Episode: product-level execution record (Sprint 7 / Schema v1)
// Reference: SynapseDoc/talk&thoughts/EPISODE_SCHEMA_V1_CN.md
// ---------------------------------------------------------------------------

// EpisodeType distinguishes the two primary verification patterns.
type EpisodeType string

const (
	EpisodeTypeActionVerification EpisodeType = "action_verification"
	EpisodeTypeInvestigationStep  EpisodeType = "investigation_step"
)

// EpisodeStatus represents the lifecycle state of an Episode.
type EpisodeStatus string

const (
	EpisodeStatusPending    EpisodeStatus = "pending"
	EpisodeStatusInProgress EpisodeStatus = "in_progress"
	EpisodeStatusConverged  EpisodeStatus = "converged"
	EpisodeStatusEscalated  EpisodeStatus = "escalated"
	EpisodeStatusFailed     EpisodeStatus = "failed"
)

// EpisodeResult is the business outcome recorded in a Verdict.
type EpisodeResult string

const (
	EpisodeResultPass         EpisodeResult = "pass"
	EpisodeResultFail         EpisodeResult = "fail"
	EpisodeResultInconclusive EpisodeResult = "inconclusive"
)

// EpisodeConfidence is the confidence level recorded in a Verdict.
type EpisodeConfidence string

const (
	EpisodeConfidenceHigh   EpisodeConfidence = "high"
	EpisodeConfidenceMedium EpisodeConfidence = "medium"
	EpisodeConfidenceLow    EpisodeConfidence = "low"
)

// EpisodeTriggerType identifies the source of an Episode trigger.
type EpisodeTriggerType string

const (
	EpisodeTriggerAlert     EpisodeTriggerType = "alert"
	EpisodeTriggerWebhook   EpisodeTriggerType = "webhook"
	EpisodeTriggerManual    EpisodeTriggerType = "manual"
	EpisodeTriggerScheduled EpisodeTriggerType = "scheduled"
)

// EpisodeTrigger describes what initiated the Episode.
type EpisodeTrigger struct {
	Type    EpisodeTriggerType     `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// EpisodeHandleType is the kind of tracking identifier captured.
type EpisodeHandleType string

const (
	HandleTypeRequestID    EpisodeHandleType = "request_id"
	HandleTypeTraceID      EpisodeHandleType = "trace_id"
	HandleTypeOrderID      EpisodeHandleType = "order_id"
	HandleTypeSessionID    EpisodeHandleType = "session_id"
	HandleTypeGitRef       EpisodeHandleType = "git_ref"
	HandleTypeDeployRev    EpisodeHandleType = "deploy_revision"
	HandleTypeFileLocation EpisodeHandleType = "file_location"
	HandleTypePodName      EpisodeHandleType = "pod_name"
	HandleTypeCustom       EpisodeHandleType = "custom"
)

// EpisodeHandle is a structured tracking identifier extracted from action output.
// Only Hard Nodes (handle-extraction type) may append to Handles.
type EpisodeHandle struct {
	Type        EpisodeHandleType `json:"type"`
	Value       string            `json:"value"`
	Source      string            `json:"source"`       // node ID that extracted this handle
	ExtractedAt time.Time         `json:"extracted_at"` // ISO 8601
}

// EpisodeEvidenceType classifies who wrote an evidence entry.
type EpisodeEvidenceType string

const (
	EvidenceTypeFact            EpisodeEvidenceType = "fact"             // written by Hard Node
	EvidenceTypeInference       EpisodeEvidenceType = "inference"        // written by Soft Node
	EvidenceTypeHumanCorrection EpisodeEvidenceType = "human_correction" // written by Human Node
)

// EvidenceCollectorSpec records WHAT query or command was used to collect a
// piece of evidence.  This closes the traceability gap: you can see not only
// WHAT was found, but HOW it was found (which query, which keyword, which SQL).
// Only Hard Nodes populate this field; Soft Nodes leave it nil.
type EvidenceCollectorSpec struct {
	// CollectorType classifies the collection mechanism.
	// Known values: "script" | "log_query" | "db_query" | "api_call" | "code_search"
	CollectorType string `json:"collector_type,omitempty"`

	// Params holds structured, human-readable query parameters.
	// Examples:
	//   log_query  → {"deployment": "checkoutservice", "tail": 120, "keyword": "ORD-001"}
	//   db_query   → {"sql": "SELECT * FROM cart_order WHERE order_id = 'ORD-001'", "db": "orders"}
	//   api_call   → {"url": "https://…", "method": "GET"}
	Params map[string]interface{} `json:"params,omitempty"`

	// RawCommand is the fully-resolved shell command for script-type collectors.
	// All {{template}} placeholders have already been substituted.
	RawCommand string `json:"raw_command,omitempty"`
}

// EpisodeEvidence is one piece of collected evidence inside an Episode.
// Large payloads MUST be stored as artifacts (see EpisodeArtifact) and
// referenced here via ContentRef instead of being inlined.
type EpisodeEvidence struct {
	ID          string              `json:"id"`
	Type        EpisodeEvidenceType `json:"type"`
	NodeID      string              `json:"node_id"`
	NodeType    NodeType            `json:"node_type"`
	Label       string              `json:"label,omitempty"`
	Content     string              `json:"content,omitempty"`     // small payloads only
	ContentRef  string              `json:"content_ref,omitempty"` // "artifact://{exec_id}/{ev_id}"
	CollectedAt time.Time           `json:"collected_at"`

	// CollectorSpec records exactly how this evidence was collected (query params,
	// command, etc.).  Populated by Hard Nodes only; nil for Soft/Human nodes.
	CollectorSpec *EvidenceCollectorSpec `json:"collector_spec,omitempty"`
}

// EpisodeVerdict is the conclusion produced by a Soft Node after analysing
// the collected evidence.  It is the ONLY field a Soft Node may write.
// Per spec: confidence is a label ("high"/"medium"/"low"), not a float.
type EpisodeVerdict struct {
	Result          EpisodeResult     `json:"result,omitempty"`     // pass | fail | inconclusive
	Confidence      EpisodeConfidence `json:"confidence,omitempty"` // high | medium | low
	Conclusion      string            `json:"conclusion,omitempty"`
	CausalChain     []string          `json:"causal_chain,omitempty"`
	Gaps            []string          `json:"gaps,omitempty"`
	Recommendations []string          `json:"recommendations,omitempty"`
	DecidedBy       string            `json:"decided_by,omitempty"` // node ID
	DecidedAt       time.Time         `json:"decided_at,omitempty"`
}

// EpisodeLoopGuard prevents infinite re-investigation loops.
type EpisodeLoopGuard struct {
	MaxIterations    int      `json:"max_iterations"`
	CurrentIteration int      `json:"current_iteration"`
	AttemptedActions []string `json:"attempted_actions,omitempty"`
}

// EpisodeAuditEntry records a Human Node state correction with full trail.
// Kept for backward compatibility; new code should use HumanIntervention.
type EpisodeAuditEntry struct {
	Actor         string      `json:"actor"`
	NodeID        string      `json:"node_id"`
	FieldModified string      `json:"field_modified"`
	OldValue      interface{} `json:"old_value,omitempty"`
	NewValue      interface{} `json:"new_value,omitempty"`
	ModifiedAt    time.Time   `json:"modified_at"`
}

// HumanInterventionAction enumerates the kinds of Human Node corrections.
type HumanInterventionAction string

const (
	HumanActionStateOverride         HumanInterventionAction = "state_override"
	HumanActionEvidenceMarkedInvalid HumanInterventionAction = "evidence_marked_invalid"
	HumanActionHandleInjected        HumanInterventionAction = "handle_injected"
	HumanActionHypothesisCorrected   HumanInterventionAction = "hypothesis_corrected"
	HumanActionSuspended             HumanInterventionAction = "review_requested" // execution paused, awaiting review
	HumanActionResumed               HumanInterventionAction = "resumed"
	HumanActionAborted               HumanInterventionAction = "aborted"
)

// HumanIntervention records a structured Human Node action on an Episode
// per the Episode Schema v1 spec (section 5.3).
type HumanIntervention struct {
	NodeID    string                  `json:"node_id"`
	Actor     string                  `json:"actor"`
	Action    HumanInterventionAction `json:"action"`
	Detail    string                  `json:"detail"`
	Timestamp time.Time               `json:"timestamp"`
}

// ActionContext holds action_verification-specific Episode context.
type ActionContext struct {
	ActionName   string                 `json:"action_name"`
	ActionType   string                 `json:"action_type"` // browser | api | script | mcp_tool
	ActionInput  map[string]interface{} `json:"action_input,omitempty"`
	ActionOutput map[string]interface{} `json:"action_output,omitempty"`
}

// InvestigationContext holds investigation_step-specific Episode context.
type InvestigationContext struct {
	Hypothesis    string   `json:"hypothesis"`
	KnownSignals  []string `json:"known_signals,omitempty"`
	RetrievalPlan string   `json:"retrieval_plan,omitempty"`
}

// EpisodeMemoryExtraction tracks whether this Episode triggered memory extraction.
// Auto-triggered when verdict.confidence == "high" and result != "inconclusive".
// Human-triggered after Human Node resumed/state_override leads to convergence.
type EpisodeMemoryExtraction struct {
	Triggered bool   `json:"triggered"`
	TriggerBy string `json:"trigger_by,omitempty"` // auto_high_confidence | human_confirmed
	Status    string `json:"status,omitempty"`     // pending | completed | failed
}

// Episode is the top-level product object that captures one unit of
// purposeful execution (one action verification or one investigation step).
// Schema version: 1.0 — see EPISODE_SCHEMA_V1_CN.md.
type Episode struct {
	ID              string        `json:"id"`
	ExecID          string        `json:"exec_id"`
	ParentEpisodeID string        `json:"parent_episode_id,omitempty"` // reserved for Sprint 11 nesting
	EpisodeType     EpisodeType   `json:"episode_type"`
	Status          EpisodeStatus `json:"status"` // pending → in_progress → converged | escalated | failed

	// Trigger source (how/why the Episode was created).
	Trigger *EpisodeTrigger `json:"trigger,omitempty"`

	// Type-specific context (only one of these should be non-nil).
	ActionContext        *ActionContext        `json:"action_context,omitempty"`
	InvestigationContext *InvestigationContext `json:"investigation_context,omitempty"`

	// Core evidence fields.
	Handles   []EpisodeHandle   `json:"handles,omitempty"` // structured tracking identifiers
	Evidence  []EpisodeEvidence `json:"evidence,omitempty"`
	Verdict   *EpisodeVerdict   `json:"verdict,omitempty"`
	LoopGuard EpisodeLoopGuard  `json:"loop_guard"`

	// Audit / correction trail.
	AuditTrail         []EpisodeAuditEntry `json:"audit_trail,omitempty"`         // legacy; kept for HumanCorrect compat
	HumanInterventions []HumanIntervention `json:"human_interventions,omitempty"` // v1 spec structured audit

	// Memory extraction trigger state.
	MemoryExtraction *EpisodeMemoryExtraction `json:"memory_extraction,omitempty"`

	// Timestamps.
	SchemaVersion int        `json:"schema_version"` // integer 1 = schema v1.0
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ConcludedAt   *time.Time `json:"concluded_at,omitempty"` // set when status transitions to converged/escalated
}

// EpisodeArtifact stores large evidence payloads that must not be inlined.
type EpisodeArtifact struct {
	ID          string    `json:"id"`
	EpisodeID   string    `json:"episode_id"`
	EvidenceID  string    `json:"evidence_id"`
	ContentType string    `json:"content_type"` // "log_dump" | "trace_export" | "screenshot" | "raw"
	SizeBytes   int64     `json:"size_bytes"`
	StorageURI  string    `json:"storage_uri"` // "artifact://{exec_id}/{ev_id}"
	Content     string    `json:"content,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------------
// View Models: API response types for the Execution Workspace (M1.1)
// These are projection/display types, separate from the internal Episode model.
// Reference: EXECUTION_WORKSPACE_PROTOCOL_DRAFT_CN.md
// ---------------------------------------------------------------------------

// ExecutionDisplayView holds display-layer metadata for an execution.
type ExecutionDisplayView struct {
	RunLabel      string `json:"run_label,omitempty"`
	OverviewBadge string `json:"overview_badge,omitempty"`
	TraceTitle    string `json:"trace_title,omitempty"`
	TraceSummary  string `json:"trace_summary,omitempty"`
}

// ExecutionSummaryView is the high-level execution summary returned by
// GET /api/v1/executions/:execution_id/summary.
type ExecutionSummaryView struct {
	ExecutionID  string               `json:"execution_id"`
	DAGID        string               `json:"dag_id"`
	DAGName      string               `json:"dag_name"`
	Status       ExecutionStatus      `json:"status"`
	StartedAt    time.Time            `json:"started_at"`
	EndedAt      *time.Time           `json:"ended_at,omitempty"`
	DurationMs   int64                `json:"duration_ms"`
	Mode         string               `json:"mode"`          // "execution"
	WorkflowKind string               `json:"workflow_kind"` // "investigation" | "verification"
	Metadata     map[string]string    `json:"metadata,omitempty"`
	Display      ExecutionDisplayView `json:"display"`
}

// EpisodeDisplayView holds display-layer metadata for an episode card.
type EpisodeDisplayView struct {
	Verdict      string  `json:"verdict,omitempty"`       // display verdict (may reflect human review)
	VerdictLabel string  `json:"verdict_label,omitempty"` // human-readable label
	Summary      string  `json:"summary,omitempty"`       // one-line summary
	Banner       *string `json:"banner"`                  // null when no alert banner needed
}

// EpisodeSummaryView is the episode list item returned by
// GET /api/v1/executions/:execution_id/episodes (upgraded response).
type EpisodeSummaryView struct {
	EpisodeID            string             `json:"episode_id"`
	Label                string             `json:"label"`
	Status               EpisodeStatus      `json:"status"`
	Display              EpisodeDisplayView `json:"display"`
	Confidence           EpisodeConfidence  `json:"confidence,omitempty"`
	EvidenceCount        int                `json:"evidence_count"`
	HandleCount          int                `json:"handle_count"`
	DefaultReplayPercent int                `json:"default_replay_percent"`
}

// ProcessTraceEntryView is a single step in the process trace timeline.
// stage values: "Action" | "Round N" | "Verdict" | "Human Review" | "Circuit Breaker"
type ProcessTraceEntryView struct {
	ID     string   `json:"id"`
	Stage  string   `json:"stage"`
	Title  string   `json:"title"`
	Detail string   `json:"detail,omitempty"`
	Status string   `json:"status"` // "success" | "failed" | "running" | "pending"
	Chips  []string `json:"chips,omitempty"`
	Range  [2]int   `json:"range"` // [start_percent, end_percent]
}

// RuntimeFactView is a single evidence fact in the Dossier right column.
// focus_key is the key field for three-column linkage (M4.1).
type RuntimeFactView struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	FocusKey       string `json:"focus_key,omitempty"`
	SourceType     string `json:"source_type,omitempty"` // "json" | "log" | "code" | "text"
	Collector      string `json:"collector,omitempty"`   // "node_type:node_name"
	Handle         string `json:"handle,omitempty"`      // "state:key"
	Revision       string `json:"revision,omitempty"`
	TimeWindow     string `json:"time_window,omitempty"`
	SourceName     string `json:"source_name,omitempty"`
	Content        string `json:"content,omitempty"`
	ContentRef     string `json:"content_ref,omitempty"`
	HighlightLines []int  `json:"highlight_lines,omitempty"`
}

// ExpectedBehaviorView is a single entry in the Dossier left column.
// source_type: "sop" (Verified SOP) | "ai" (AI Hypothesized)
type ExpectedBehaviorView struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	FocusKey     string `json:"focus_key,omitempty"`
	SourceType   string `json:"source_type,omitempty"`   // "sop" | "ai"
	SourceLabel  string `json:"source_label,omitempty"`  // "Verified SOP" | "AI Hypothesized"
	SourceDetail string `json:"source_detail,omitempty"` // explanation of source
}

// VerdictBridgeItemView is a single entry in the Dossier middle column.
type VerdictBridgeItemView struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	FocusKey string `json:"focus_key,omitempty"`
}

// DossierEpisodeRefView is the lightweight episode reference in a dossier.
type DossierEpisodeRefView struct {
	EpisodeID string `json:"episode_id"`
	Label     string `json:"label"`
}

// DossierDisplayView holds the display-layer state of a dossier.
type DossierDisplayView struct {
	Verdict      string  `json:"verdict,omitempty"`
	VerdictLabel string  `json:"verdict_label,omitempty"`
	Summary      string  `json:"summary,omitempty"`
	Banner       *string `json:"banner"` // null when no banner
}

// EpisodeDossierView is the full dossier payload returned by
// GET /api/v1/executions/:execution_id/episodes/:episode_id/dossier.
type EpisodeDossierView struct {
	Episode          DossierEpisodeRefView   `json:"episode"`
	Display          DossierDisplayView      `json:"display"`
	ExpectedBehavior []ExpectedBehaviorView  `json:"expected_behavior"`
	VerdictBridge    []VerdictBridgeItemView `json:"verdict_bridge"`
	RuntimeFacts     []RuntimeFactView       `json:"runtime_facts"`
	Handles          []EpisodeHandle         `json:"handles"`
	MemoryRecalls    []MemoryRecallView      `json:"memory_recalls"`
	HumanAuditTrail  []HumanIntervention     `json:"human_audit_trail"`
}

// MemoryRecallView is a single memory recall item.
// Note: current implementation uses keyword overlap scoring, not semantic vector recall.
// confidence reflects keyword overlap degree, not semantic similarity.
type MemoryRecallView struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Summary           string `json:"summary"`
	MatchedPattern    string `json:"matched_pattern,omitempty"`
	Confidence        string `json:"confidence,omitempty"` // "high" | "medium" | "low"
	SourceExecutionID string `json:"source_execution_id,omitempty"`
	Caution           string `json:"caution,omitempty"`
	Recommendation    string `json:"recommendation,omitempty"`
}

// MemoryRecallListView wraps the memory recall list response.
// implementation_note is fixed to "keyword_overlap" until vector recall is introduced.
type MemoryRecallListView struct {
	Items              []MemoryRecallView `json:"items"`
	ImplementationNote string             `json:"implementation_note"` // "keyword_overlap"
}
