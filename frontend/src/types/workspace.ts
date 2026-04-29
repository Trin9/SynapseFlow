// Workspace View Model types — mirrors backend pkg/models View structs (M1.1)
// Reference: EXECUTION_WORKSPACE_PROTOCOL_DRAFT_CN.md
//
// IMPORTANT: These are projection/display types only. Domain types (Episode,
// EpisodeEvidence, etc.) live in ./episode.ts.

import type { EpisodeStatus, EpisodeConfidence, EpisodeHandle } from './episode'

// ---------------------------------------------------------------------------
// Execution-level view models
// ---------------------------------------------------------------------------

export interface ExecutionDisplayView {
  run_label?: string
  overview_badge?: string
  trace_title?: string
  trace_summary?: string
}

export interface ExecutionSummaryView {
  execution_id: string
  dag_id: string
  dag_name: string
  status: string
  started_at: string
  ended_at?: string
  duration_ms: number
  mode: string           // "execution"
  workflow_kind: string  // "investigation" | "verification"
  metadata?: Record<string, string>
  display: ExecutionDisplayView
}

// ---------------------------------------------------------------------------
// Episode-level view models
// ---------------------------------------------------------------------------

export interface EpisodeDisplayView {
  verdict?: string
  verdict_label?: string
  summary?: string
  banner: string | null  // null = no banner needed
}

export interface EpisodeSummaryView {
  episode_id: string
  label: string
  status: EpisodeStatus
  display: EpisodeDisplayView
  confidence?: EpisodeConfidence
  evidence_count: number
  handle_count: number
  default_replay_percent: number
}

// ---------------------------------------------------------------------------
// Trigger context view models
// ---------------------------------------------------------------------------

export interface TriggerContextFieldView {
  label: string
  value: string
  range: [number, number]  // replay range [start%, end%] at which field becomes visible
}

export interface TriggerContextSectionView {
  title: string
  fields: TriggerContextFieldView[]
}

export interface TriggerContextView {
  title: string
  summary: string
  sections: TriggerContextSectionView[]
}

// ---------------------------------------------------------------------------
// Replay view models
// ---------------------------------------------------------------------------

export interface ReplayCheckpointView {
  label: string
  headline: string
  detail: string
}

export interface ProcessTraceEntryView {
  id: string
  stage: string    // "Action" | "Round N" | "Verdict" | "Human Review" | "Circuit Breaker"
  title: string
  detail?: string
  status: string   // "success" | "failed" | "running" | "pending"
  chips?: string[]
  range: [number, number]  // [start_percent, end_percent]
}

export interface ReplaySliceView {
  episode_id: string
  percent: number
  checkpoint: ReplayCheckpointView
  visible_process_trace: ProcessTraceEntryView[]
  visible_handles: unknown[]
  visible_state_fields: unknown[]
  visible_runtime_fact_ids: string[]
}

// ---------------------------------------------------------------------------
// Dossier view models
// ---------------------------------------------------------------------------

export interface RuntimeFactView {
  id: string
  title: string
  summary: string
  focus_key?: string
  source_type?: string    // "json" | "log" | "code" | "text"
  collector?: string      // "node_type:node_name"
  handle?: string         // "state:key"
  revision?: string
  time_window?: string
  source_name?: string
  content?: string
  content_ref?: string
  highlight_lines?: number[]
}

export interface ExpectedBehaviorView {
  id: string
  title: string
  body: string
  focus_key?: string
  source_type?: string    // "sop" | "ai"
  source_label?: string   // "Verified SOP" | "AI Hypothesized"
  source_detail?: string  // explanation of source
}

export interface VerdictBridgeItemView {
  id: string
  title: string
  body: string
  focus_key?: string
}

export interface DossierEpisodeRefView {
  episode_id: string
  label: string
}

export interface DossierDisplayView {
  verdict?: string
  verdict_label?: string
  summary?: string
  banner: string | null  // null = no banner
}

// Lightweight HumanIntervention record surfaced inside dossier human_audit_trail.
export interface HumanAuditEntryView {
  node_id: string
  actor: string
  action: string
  detail: string
  timestamp: string
}

export interface EpisodeDossierView {
  episode: DossierEpisodeRefView
  display: DossierDisplayView
  expected_behavior: ExpectedBehaviorView[]
  verdict_bridge: VerdictBridgeItemView[]
  runtime_facts: RuntimeFactView[]
  handles: EpisodeHandle[]
  memory_recalls: MemoryRecallView[]
  human_audit_trail: HumanAuditEntryView[]
}

// ---------------------------------------------------------------------------
// Review state view models
// ---------------------------------------------------------------------------

export interface ReviewStateView {
  status: string        // "pending" | "approved" | "overridden" | "aborted"
  actor?: string
  acted_at?: string
  action_label?: string
  note?: string
  ticket_id?: string
  queue?: string
  resume_cursor?: string
}

// ReviewActionRequest is what the frontend sends to POST /review-actions.
// Maps to backend models.ReviewActionRequest.
export interface ReviewActionRequest {
  episode_id?: string
  status: string        // "approved" | "aborted" | "overridden"
  actor?: string
  note?: string
}

export interface ReviewActionResponse {
  ok: boolean
}

// ---------------------------------------------------------------------------
// Memory recall view models
// ---------------------------------------------------------------------------

export interface MemoryRecallView {
  id: string
  title: string
  summary: string
  matched_pattern?: string
  confidence?: string            // "high" | "medium" | "low"
  source_execution_id?: string
  caution?: string
  recommendation?: string
}

export interface MemoryRecallListView {
  items: MemoryRecallView[]
  implementation_note: string    // "keyword_overlap"
}

// ---------------------------------------------------------------------------
// Comparison view models
// ---------------------------------------------------------------------------

export interface ComparisonSummaryView {
  execution_id: string
  title: string
  summary: string
  outcome?: string
  compared_against?: string
  highlights?: string[]
  caution?: string
}
