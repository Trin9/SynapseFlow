// Episode types — mirrors backend pkg/models Episode schema (Sprint 7/8)

export type EpisodeType = 'action_verification' | 'investigation_step'

export type EpisodeEvidenceType = 'fact' | 'inference' | 'human_correction'

export interface EpisodeEvidence {
  id: string
  type: EpisodeEvidenceType
  node_id: string
  node_type: string
  label?: string
  content?: string      // small payloads inline
  content_ref?: string  // "artifact://{exec_id}/{ev_id}" for large payloads
  collected_at: string
}

export interface EpisodeVerdict {
  conclusion?: string
  confidence: number   // 0.0–1.0
  causal_chain?: string[]
  gaps?: string[]
  decided_by?: string
  decided_at?: string
}

export interface EpisodeLoopGuard {
  max_iterations: number
  current_iteration: number
  attempted_actions?: string[]
}

export interface EpisodeAuditEntry {
  actor: string
  node_id: string
  field_modified: string
  old_value?: unknown
  new_value?: unknown
  modified_at: string
}

export interface Episode {
  id: string
  exec_id: string
  episode_type: EpisodeType
  handles?: Record<string, string>
  evidence?: EpisodeEvidence[]
  verdict?: EpisodeVerdict
  loop_guard: EpisodeLoopGuard
  audit_trail?: EpisodeAuditEntry[]
  schema_version: number
  created_at: string
  updated_at: string
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
