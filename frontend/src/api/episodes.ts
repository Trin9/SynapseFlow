import axios from 'axios'
import type { Episode, EpisodesResponse, ExecutionSummary } from '@/types/episode'
import type {
  ExecutionSummaryView,
  EpisodeSummaryView,
  TriggerContextView,
  ReviewStateView,
  ReviewActionRequest,
  ReviewActionResponse,
  ReplaySliceView,
  EpisodeDossierView,
  MemoryRecallListView,
  ComparisonSummaryView,
} from '@/types/workspace'

const api = axios.create({
  baseURL: '/api/v1',
  headers: { 'Content-Type': 'application/json' },
})

/** GET /api/v1/executions — paginated list of past executions (raw domain type) */
export async function listExecutions(): Promise<ExecutionSummary[]> {
  const { data } = await api.get<ExecutionSummary[]>('/executions')
  return data ?? []
}

// ─── CR-015: view-model API functions ────────────────────────────────────────

/**
 * GET /api/v1/executions?view=summary
 * Returns the view-model projection for all executions (ExecutionSummaryView[]).
 * Components should prefer this over listExecutions() to stay in workspace.ts types.
 */
export async function listExecutionSummaries(): Promise<ExecutionSummaryView[]> {
  const { data } = await api.get<ExecutionSummaryView[]>('/executions', { params: { view: 'summary' } })
  return data ?? []
}

/**
 * GET /api/v1/executions?dag_id=<id>&view=summary
 * Returns the view-model projection filtered to a single DAG (CR-015).
 */
export async function listExecutionSummariesByDAG(dagId: string): Promise<ExecutionSummaryView[]> {
  const { data } = await api.get<ExecutionSummaryView[]>('/executions', {
    params: { dag_id: dagId, view: 'summary' },
  })
  return data ?? []
}

/**
 * GET /api/v1/executions/:id/episodes?view=summary
 * Returns EpisodeSummaryView[] instead of raw Episode[] (CR-015).
 */
export async function listEpisodeSummariesByExecution(
  execId: string,
): Promise<EpisodeSummaryView[]> {
  const { data } = await api.get<{ episodes: EpisodeSummaryView[] }>(
    `/executions/${execId}/episodes`,
    { params: { view: 'summary' } },
  )
  return data?.episodes ?? []
}

// ─── existing domain API functions ───────────────────────────────────────────

/** GET /api/v1/executions/:id/episodes */
export async function listEpisodesByExecution(execId: string): Promise<Episode[]> {
  const { data } = await api.get<EpisodesResponse>(`/executions/${execId}/episodes`)
  return data?.episodes ?? []
}

/** GET /api/v1/episodes/:id */
export async function getEpisode(id: string): Promise<Episode> {
  const { data } = await api.get<Episode>(`/episodes/${id}`)
  return data
}

// ─── M2.5 — Execution Workspace API calls ─────────────────────────────────

/** GET /api/v1/executions/:id/summary */
export async function getExecutionSummaryView(execId: string): Promise<ExecutionSummaryView> {
  const { data } = await api.get<ExecutionSummaryView>(`/executions/${execId}/summary`)
  return data
}

/** GET /api/v1/executions/:id/trigger-context */
export async function getTriggerContext(execId: string): Promise<TriggerContextView> {
  const { data } = await api.get<TriggerContextView>(`/executions/${execId}/trigger-context`)
  return data
}

/** GET /api/v1/executions/:id/review-state */
export async function getReviewState(execId: string): Promise<ReviewStateView> {
  const { data } = await api.get<ReviewStateView>(`/executions/${execId}/review-state`)
  return data
}

/** POST /api/v1/executions/:id/review-actions */
export async function postReviewAction(
  execId: string,
  req: ReviewActionRequest,
): Promise<ReviewActionResponse> {
  const { data } = await api.post<ReviewActionResponse>(
    `/executions/${execId}/review-actions`,
    req,
  )
  return data
}

/** GET /api/v1/executions/:execId/episodes/:episodeId/replay?percent=N */
export async function getEpisodeReplay(
  execId: string,
  episodeId: string,
  percent?: number,
): Promise<ReplaySliceView> {
  const params = percent !== undefined ? `?percent=${percent}` : ''
  const { data } = await api.get<ReplaySliceView>(
    `/executions/${execId}/episodes/${episodeId}/replay${params}`,
  )
  return data
}

/** GET /api/v1/executions/:execId/episodes/:episodeId/dossier */
export async function getEpisodeDossier(
  execId: string,
  episodeId: string,
): Promise<EpisodeDossierView> {
  const { data } = await api.get<EpisodeDossierView>(
    `/executions/${execId}/episodes/${episodeId}/dossier`,
  )
  return data
}

/** GET /api/v1/executions/:execId/episodes/:episodeId/memory-recalls */
export async function getMemoryRecalls(
  execId: string,
  episodeId: string,
): Promise<MemoryRecallListView> {
  const { data } = await api.get<MemoryRecallListView>(
    `/executions/${execId}/episodes/${episodeId}/memory-recalls`,
  )
  return data
}

/** GET /api/v1/executions/:execId/comparison-targets/:historicalId */
export async function getComparisonTarget(
  execId: string,
  historicalId: string,
): Promise<ComparisonSummaryView> {
  const { data } = await api.get<ComparisonSummaryView>(
    `/executions/${execId}/comparison-targets/${historicalId}`,
  )
  return data
}
