import axios from 'axios'
import type { Episode, EpisodesResponse, ExecutionSummary } from '@/types/episode'

const api = axios.create({
  baseURL: '/api/v1',
  headers: { 'Content-Type': 'application/json' },
})

/** GET /api/v1/executions — paginated list of past executions */
export async function listExecutions(): Promise<ExecutionSummary[]> {
  const { data } = await api.get<ExecutionSummary[]>('/executions')
  return data ?? []
}

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
