import axios from 'axios'
import type { DAGConfig, ExecutionStartResponse, ExecutionNodesResponse } from '@/types'

const api = axios.create({
  baseURL: '/api/v1',
  headers: { 'Content-Type': 'application/json' },
})

// Run a workflow inline (without saving) - starts async execution
export async function runWorkflow(dag: DAGConfig): Promise<ExecutionStartResponse> {
  const { data } = await api.post<ExecutionStartResponse>('/run', dag)
  return data
}

// DAG CRUD
export async function createDAG(dag: DAGConfig): Promise<DAGConfig> {
  const { data } = await api.post<DAGConfig>('/dags', dag)
  return data
}

export async function listDAGs(): Promise<DAGConfig[]> {
  const { data } = await api.get<DAGConfig[]>('/dags')
  return data
}

export async function getDAG(id: string): Promise<DAGConfig> {
  const { data } = await api.get<DAGConfig>(`/dags/${id}`)
  return data
}

export async function updateDAG(id: string, dag: DAGConfig): Promise<DAGConfig> {
  const { data } = await api.put<DAGConfig>(`/dags/${id}`, dag)
  return data
}

export async function deleteDAG(id: string): Promise<void> {
  await api.delete(`/dags/${id}`)
}

// Run a saved DAG
export async function runSavedDAG(id: string): Promise<ExecutionStartResponse> {
  const { data } = await api.post<ExecutionStartResponse>(`/dags/${id}/run`)
  return data
}

export async function getExecutionNodes(id: string): Promise<ExecutionNodesResponse> {
  const { data } = await api.get<ExecutionNodesResponse>(`/executions/${id}/nodes`)
  return data
}

// Health check
export async function healthCheck(): Promise<{ status: string; version: string }> {
  const { data } = await axios.get('/health')
  return data
}
