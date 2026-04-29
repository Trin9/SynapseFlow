// Episode types — mirrors backend pkg/models Episode schema (Sprint 7/8 + CollectorSpec)

export type EpisodeType = 'action_verification' | 'investigation_step'
export type EpisodeStatus = 'pending' | 'in_progress' | 'converged' | 'escalated' | 'failed'
export type EpisodeEvidenceType = 'fact' | 'inference' | 'human_correction'
export type EpisodeResult = 'pass' | 'fail' | 'inconclusive'
export type EpisodeConfidence = 'high' | 'medium' | 'low'

// EvidenceCollectorSpec — what query/command was used to collect the evidence.
// Only present on evidence written by Hard Nodes (script, mcp, etc.).
export interface EvidenceCollectorSpec {
  collector_type?: string   // "script" | "log_query" | "db_query" | "api_call" | "code_search"
  params?: Record<string, unknown>  // structured query params (keyword, sql, url…)
  raw_command?: string              // fully-resolved shell command for script collectors
}

export interface EpisodeEvidence {
  id: string
  type: EpisodeEvidenceType
  node_id: string
  node_type: string
  label?: string
  content?: string      // small payloads inline
  content_ref?: string  // "artifact://{exec_id}/{ev_id}" for large payloads
  collected_at: string
  collector_spec?: EvidenceCollectorSpec   // NEW: traceability — HOW this was collected
}

export interface EpisodeVerdict {
  result?: EpisodeResult        // "pass" | "fail" | "inconclusive"
  confidence?: EpisodeConfidence // "high" | "medium" | "low"  (string, NOT a number)
  conclusion?: string
  causal_chain?: string[]
  gaps?: string[]
  recommendations?: string[]
  decided_by?: string
  decided_at?: string
}

export interface EpisodeLoopGuard {
  max_iterations: number
  current_iteration: number
  attempted_actions?: string[]
}

// EpisodeHandle — a structured tracking identifier extracted from action output.
// Maps to backend models.EpisodeHandle.
export interface EpisodeHandle {
  type: string    // "order_id" | "session_id" | "trace_id" | "request_id" | "git_ref" | …
  value: string
  source: string  // node_id that extracted this handle
  extracted_at: string
}

export interface EpisodeAuditEntry {
  actor: string
  node_id: string
  field_modified: string
  old_value?: unknown
  new_value?: unknown
  modified_at: string
}

export interface ActionContext {
  action_name: string
  action_type?: string   // "browser" | "api" | "script" | "mcp_tool"
  action_input?: Record<string, unknown>
  action_output?: Record<string, unknown>
}

export interface InvestigationContext {
  hypothesis: string
  known_signals?: string[]
  retrieval_plan?: string
}

// EpisodeTrigger — mirrors backend models.EpisodeTrigger
export interface EpisodeTrigger {
  type: string
  payload?: Record<string, unknown>
}

export interface Episode {
  id: string
  exec_id: string
  parent_episode_id?: string
  episode_type: EpisodeType
  status: EpisodeStatus
  trigger?: EpisodeTrigger                 // NEW (M3.1)
  handles?: EpisodeHandle[]             // structured tracking identifiers (order_id, session_id…)
  action_context?: ActionContext
  investigation_context?: InvestigationContext
  evidence?: EpisodeEvidence[]
  verdict?: EpisodeVerdict
  loop_guard: EpisodeLoopGuard
  audit_trail?: EpisodeAuditEntry[]
  schema_version: number
  created_at: string
  updated_at: string
  concluded_at?: string
}

// GET /api/v1/executions/:id/episodes
export interface EpisodesResponse {
  episodes: Episode[]
}

// Lightweight Execution summary for history list
export interface ExecutionSummary {
  id: string
  dag_id: string
  dag_name: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'timeout' | 'suspended'
  started_at: string
  ended_at?: string
  error?: string
}

// View Model types (projection/display types for the Execution Workspace) live
// in ./workspace.ts — do not add them here.
