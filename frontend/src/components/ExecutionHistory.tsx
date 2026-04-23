import { useCallback, useEffect, useState } from 'react'
import { listExecutions, listEpisodesByExecution } from '@/api/episodes'
import { getExecutionNodes } from '@/api/client'
import type { ExecutionSummary, Episode } from '@/types/episode'
import type { ExecutionNodesResponse } from '@/types'
import { NODE_TYPE_INFO } from '@/types'
import { EpisodeCard } from './EpisodeCard'

// ─── helpers ──────────────────────────────────────────────────────────────

function statusColor(status: ExecutionSummary['status']): string {
  switch (status) {
    case 'completed': return 'bg-green-100 text-green-700'
    case 'running': return 'bg-blue-100 text-blue-700'
    case 'failed': return 'bg-red-100 text-red-700'
    case 'suspended': return 'bg-amber-100 text-amber-700'
    default: return 'bg-gray-100 text-gray-500'
  }
}

function formatDate(iso: string): string {
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

function formatDuration(start: string, end?: string): string {
  if (!end) return '—'
  const ms = new Date(end).getTime() - new Date(start).getTime()
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.round(ms / 60_000)}m`
}

function formatMaybeJSON(s: string): string {
  const t = s.trim()
  if (!(t.startsWith('{') || t.startsWith('['))) return s
  try { return JSON.stringify(JSON.parse(t), null, 2) } catch { return s }
}

// ─── ExecutionDetail ──────────────────────────────────────────────────────

interface ExecutionDetailProps {
  execId: string
}

function ExecutionDetail({ execId }: ExecutionDetailProps) {
  const [nodes, setNodes] = useState<ExecutionNodesResponse | null>(null)
  const [episodes, setEpisodes] = useState<Episode[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<'nodes' | 'episodes'>('nodes')

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const [nodesData, episodesData] = await Promise.all([
          getExecutionNodes(execId),
          listEpisodesByExecution(execId),
        ])
        if (!cancelled) {
          setNodes(nodesData)
          setEpisodes(episodesData)
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Load failed')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [execId])

  if (loading) return <div className="text-xs text-gray-400 px-2 py-3">Loading…</div>
  if (error) return <div className="text-xs text-red-600 px-2 py-2">{error}</div>

  return (
    <div className="mt-2">
      {/* Tab bar */}
      <div className="flex gap-1 mb-2 border-b border-gray-100 pb-1">
        <button
          onClick={() => setActiveTab('nodes')}
          className={`text-xs px-2 py-1 rounded-t ${activeTab === 'nodes' ? 'bg-gray-100 font-semibold text-gray-700' : 'text-gray-500 hover:text-gray-700'}`}
        >
          Nodes ({nodes?.results?.length ?? 0})
        </button>
        <button
          onClick={() => setActiveTab('episodes')}
          className={`text-xs px-2 py-1 rounded-t ${activeTab === 'episodes' ? 'bg-gray-100 font-semibold text-gray-700' : 'text-gray-500 hover:text-gray-700'}`}
        >
          Episodes ({episodes.length})
        </button>
      </div>

      {/* Node results */}
      {activeTab === 'nodes' && (
        <div className="space-y-1">
          {(nodes?.results ?? []).length === 0 && (
            <p className="text-xs text-gray-400">No node results yet.</p>
          )}
          {(nodes?.results ?? []).map((r) => {
            const info = NODE_TYPE_INFO[r.node_type as keyof typeof NODE_TYPE_INFO]
            return (
              <div key={r.node_id} className="border border-gray-100 rounded">
                <details>
                  <summary className="list-none cursor-pointer select-none">
                    <div className="flex items-center gap-2 px-2 py-1.5 hover:bg-gray-50">
                      <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${
                        r.status === 'success' ? 'bg-green-500' :
                        r.status === 'skipped' ? 'bg-gray-400' : 'bg-red-500'
                      }`} />
                      <span className={`text-[9px] font-bold uppercase ${info?.color ?? 'text-gray-600'}`}>
                        {r.node_type}
                      </span>
                      <span className="text-xs text-gray-700 truncate">{r.node_name}</span>
                      <span className="text-[10px] text-gray-400 ml-auto">{r.duration_ms}ms</span>
                    </div>
                  </summary>
                  <div className="px-2 pb-2">
                    {r.output && (
                      <pre className="text-[10px] text-gray-600 bg-gray-50 rounded p-1.5 overflow-x-auto max-h-28 font-mono whitespace-pre-wrap">
                        {formatMaybeJSON(r.output)}
                      </pre>
                    )}
                    {r.error && (
                      <pre className="text-[10px] text-red-600 bg-red-50 rounded p-1.5 mt-1 overflow-x-auto max-h-24 font-mono whitespace-pre-wrap">
                        {r.error}
                      </pre>
                    )}
                  </div>
                </details>
              </div>
            )
          })}
        </div>
      )}

      {/* Episodes */}
      {activeTab === 'episodes' && (
        <div className="space-y-2">
          {episodes.length === 0 && (
            <p className="text-xs text-gray-400">
              No episodes found. DAG needs <code className="font-mono">episode_type</code> in metadata.
            </p>
          )}
          {episodes.map((ep) => (
            <EpisodeCard key={ep.id} episode={ep} defaultOpen={episodes.length === 1} />
          ))}
        </div>
      )}
    </div>
  )
}

// ─── ExecutionHistory (main export) ───────────────────────────────────────

export function ExecutionHistory() {
  const [executions, setExecutions] = useState<ExecutionSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const list = await listExecutions()
      // Newest first
      setExecutions([...list].sort((a, b) =>
        new Date(b.started_at).getTime() - new Date(a.started_at).getTime()
      ))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load executions')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void refresh() }, [refresh])

  return (
    <div className="w-96 border-l border-gray-200 bg-white flex flex-col overflow-hidden shrink-0">
      {/* Header */}
      <div className="px-4 py-2 border-b border-gray-100 flex items-center justify-between shrink-0">
        <span className="text-sm font-semibold text-gray-700">Execution History</span>
        <button
          onClick={refresh}
          disabled={loading}
          className="text-xs text-blue-600 hover:text-blue-800 disabled:opacity-40"
        >
          Refresh
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {loading && (
          <div className="text-xs text-gray-400 px-4 py-6 text-center">Loading…</div>
        )}
        {!loading && error && (
          <div className="text-xs text-red-600 px-4 py-3">{error}</div>
        )}
        {!loading && !error && executions.length === 0 && (
          <div className="text-xs text-gray-400 px-4 py-6 text-center">
            No executions yet. Run a workflow to see history here.
          </div>
        )}

        <div className="divide-y divide-gray-100">
          {executions.map((exec) => (
            <div key={exec.id}>
              <button
                className="w-full text-left px-4 py-2.5 hover:bg-gray-50 transition-colors"
                onClick={() => setExpandedId(expandedId === exec.id ? null : exec.id)}
              >
                <div className="flex items-center gap-2">
                  <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase ${statusColor(exec.status)}`}>
                    {exec.status}
                  </span>
                  <span className="text-xs font-medium text-gray-700 truncate flex-1">{exec.dag_name}</span>
                  <span className="text-[10px] text-gray-400 shrink-0">
                    {formatDuration(exec.started_at, exec.ended_at)}
                  </span>
                </div>
                <div className="text-[10px] text-gray-400 mt-0.5">
                  {formatDate(exec.started_at)}
                </div>
                {exec.error && (
                  <div className="text-[10px] text-red-500 mt-0.5 truncate">{exec.error}</div>
                )}
              </button>

              {expandedId === exec.id && (
                <div className="px-4 pb-3">
                  <ExecutionDetail execId={exec.id} />
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
