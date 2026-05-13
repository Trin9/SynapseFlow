import { useCallback, useEffect, useState } from 'react'
import { listExecutionSummaries, listEpisodeSummariesByExecution, getEpisode } from '@/api/episodes'
import { getExecutionNodes } from '@/api/client'
import type { ExecutionSummaryView, EpisodeSummaryView } from '@/types/workspace'
import type { ExecutionNodesResponse } from '@/types'
import { NODE_TYPE_INFO } from '@/types'
import { useGraphStore } from '@/hooks/useGraphStore'

// ─── helpers ──────────────────────────────────────────────────────────────

function statusColor(status: string): string {
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

function formatDurationMs(ms: number): string {
  if (ms <= 0) return '—'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.round(ms / 60_000)}m`
}

function formatMaybeJSON(s: string): string {
  const t = s.trim()
  if (!(t.startsWith('{') || t.startsWith('['))) return s
  try { return JSON.stringify(JSON.parse(t), null, 2) } catch { return s }
}

// ─── EpisodeStatsBar ─────────────────────────────────────────────────────
// CR-015: uses EpisodeSummaryView[] (view model) instead of raw Episode[]

function EpisodeStatsBar({ summaries }: { summaries: EpisodeSummaryView[] }) {
  if (!summaries.length) return null
  const pass         = summaries.filter(sv => sv.display.verdict === 'pass').length
  const fail         = summaries.filter(sv => sv.display.verdict === 'fail').length
  const inconclusive = summaries.filter(sv => sv.display.verdict === 'inconclusive').length
  const open         = summaries.filter(sv => !sv.display.verdict).length

  return (
    <div className="flex items-center gap-2 px-2 py-1.5 mb-2 bg-gray-50 dark:bg-gray-800 rounded border border-gray-100 dark:border-gray-700 text-[10px] font-semibold">
      {pass > 0 && (
        <span className="flex items-center gap-1 text-green-700">
          <span className="w-1.5 h-1.5 rounded-full bg-green-500 inline-block" />
          {pass} pass
        </span>
      )}
      {fail > 0 && (
        <span className="flex items-center gap-1 text-red-700">
          <span className="w-1.5 h-1.5 rounded-full bg-red-500 inline-block" />
          {fail} fail
        </span>
      )}
      {inconclusive > 0 && (
        <span className="flex items-center gap-1 text-yellow-700">
          <span className="w-1.5 h-1.5 rounded-full bg-yellow-400 inline-block" />
          {inconclusive} inconclusive
        </span>
      )}
      {open > 0 && (
        <span className="flex items-center gap-1 text-gray-500 dark:text-gray-400">
          <span className="w-1.5 h-1.5 rounded-full bg-gray-300 inline-block" />
          {open} open
        </span>
      )}
      <span className="ml-auto text-gray-400 dark:text-gray-500">{summaries.length} total</span>
    </div>
  )
}

// ─── EpisodeSummaryRow ────────────────────────────────────────────────────
// CR-015: lightweight row for EpisodeSummaryView (replaces EpisodeCard which
// requires a full raw Episode object).

function verdictRowColor(verdict?: string): string {
  switch (verdict) {
    case 'pass':         return 'bg-green-100 text-green-700'
    case 'fail':         return 'bg-red-100 text-red-700'
    case 'inconclusive': return 'bg-yellow-100 text-yellow-700'
    default:             return 'bg-gray-100 text-gray-500'
  }
}

interface EpisodeSummaryRowProps {
  sv: EpisodeSummaryView
  onOpenDetail?: (episodeId: string) => void
}

function EpisodeSummaryRow({ sv, onOpenDetail }: EpisodeSummaryRowProps) {
  const hasVerdict = !!sv.display.verdict
  const label = sv.display.verdict_label ?? sv.display.verdict ?? (hasVerdict ? 'verdict' : 'open')

  return (
    <div className="border border-gray-200 dark:border-gray-700 rounded-md bg-white dark:bg-gray-800 overflow-hidden text-sm">
      <div className="px-3 py-2 flex items-center gap-2 flex-wrap">
        {/* Status */}
        <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase shrink-0 ${statusColor(sv.status)}`}>
          {sv.status}
        </span>

        {/* Verdict */}
        <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded shrink-0 ${verdictRowColor(sv.display.verdict)}`}>
          {label}
        </span>

        {/* Label */}
        <span className="text-xs text-gray-600 dark:text-gray-300 truncate flex-1">{sv.label}</span>

        {/* Stats */}
        <span className="text-xs text-gray-400 dark:text-gray-500 shrink-0">
          {sv.evidence_count}ev
        </span>
        <span className="text-[10px] text-gray-400 dark:text-gray-500 shrink-0" title={sv.episode_id}>
          {sv.episode_id.slice(0, 8)}…
        </span>

        {onOpenDetail && (
          <button
            onClick={() => onOpenDetail(sv.episode_id)}
            className="text-[10px] text-blue-500 hover:text-blue-700 font-medium px-1.5 py-0.5 rounded hover:bg-blue-50 dark:hover:bg-blue-900/20 shrink-0 transition-colors"
          >
            View →
          </button>
        )}
      </div>

      {/* Optional summary text */}
      {sv.display.summary && (
        <div className="px-3 pb-2 text-[10px] text-gray-500 dark:text-gray-400 leading-relaxed">
          {sv.display.summary}
        </div>
      )}

      {/* Banner (human override notice) */}
      {sv.display.banner && (
        <div className="px-3 pb-2 text-[10px] text-amber-700 bg-amber-50 dark:bg-amber-900/20 border-t border-amber-100 dark:border-amber-800">
          {sv.display.banner}
        </div>
      )}
    </div>
  )
}

// ─── ExecutionDetail ──────────────────────────────────────────────────────

interface ExecutionDetailProps {
  execId: string
}

function ExecutionDetail({ execId }: ExecutionDetailProps) {
  const [nodes, setNodes] = useState<ExecutionNodesResponse | null>(null)
  const [episodes, setEpisodes] = useState<EpisodeSummaryView[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<'nodes' | 'episodes'>('nodes')
  const setSelectedEpisode = useGraphStore((s) => s.setSelectedEpisode)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        // CR-015: fetch episode summaries (?view=summary) instead of raw episodes
        const [nodesData, episodeSummaries] = await Promise.all([
          getExecutionNodes(execId),
          listEpisodeSummariesByExecution(execId),
        ])
        if (!cancelled) {
          setNodes(nodesData)
          setEpisodes(episodeSummaries)
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

  // Fetch the full Episode on demand so we can open the ForensicDossierDrawer.
  const handleOpenDetail = useCallback(async (episodeId: string) => {
    try {
      const ep = await getEpisode(episodeId)
      setSelectedEpisode(ep)
    } catch {
      // Best-effort: silently ignore if the fetch fails.
    }
  }, [setSelectedEpisode])

  if (loading) return <div className="text-xs text-gray-400 dark:text-gray-500 px-2 py-3">Loading…</div>
  if (error) return <div className="text-xs text-red-600 px-2 py-2">{error}</div>

  return (
    <div className="mt-2">
      {/* Tab bar */}
      <div className="flex gap-1 mb-2 border-b border-gray-100 dark:border-gray-800 pb-1">
        <button
          onClick={() => setActiveTab('nodes')}
          className={`text-xs px-2 py-1 rounded-t ${activeTab === 'nodes' ? 'bg-gray-100 dark:bg-gray-800 font-semibold text-gray-700 dark:text-gray-200' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}
        >
          Nodes ({nodes?.results?.length ?? 0})
        </button>
        <button
          onClick={() => setActiveTab('episodes')}
          className={`text-xs px-2 py-1 rounded-t ${activeTab === 'episodes' ? 'bg-gray-100 dark:bg-gray-800 font-semibold text-gray-700 dark:text-gray-200' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}
        >
          Episodes ({episodes.length})
        </button>
      </div>

      {/* Node results */}
      {activeTab === 'nodes' && (
        <div className="space-y-1">
          {(nodes?.results ?? []).length === 0 && (
            <p className="text-xs text-gray-400 dark:text-gray-500">No node results yet.</p>
          )}
          {(nodes?.results ?? []).map((r, idx) => {
            const info = NODE_TYPE_INFO[r.node_type as keyof typeof NODE_TYPE_INFO]
            return (
              <div key={`${r.node_id}:${idx}`} className="border border-gray-100 dark:border-gray-800 rounded">
                <details>
                  <summary className="list-none cursor-pointer select-none">
                    <div className="flex items-center gap-2 px-2 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-800">
                      <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${
                        r.status === 'success' ? 'bg-green-500' :
                        r.status === 'skipped' ? 'bg-gray-400' : 'bg-red-500'
                      }`} />
                      <span className={`text-[9px] font-bold uppercase ${info?.color ?? 'text-gray-600'}`}>
                        {r.node_type}
                      </span>
                      <span className="text-xs text-gray-700 dark:text-gray-200 truncate">{r.node_name}</span>
                      <span className="text-[10px] text-gray-400 dark:text-gray-500 ml-auto">{r.duration_ms}ms</span>
                    </div>
                  </summary>
                  <div className="px-2 pb-2">
                    {r.output && (
                      <pre className="text-[10px] text-gray-600 dark:text-gray-300 bg-gray-50 dark:bg-gray-800 rounded p-1.5 overflow-x-auto max-h-28 font-mono whitespace-pre-wrap">
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

      {/* Episodes — CR-015: summary view */}
      {activeTab === 'episodes' && (
        <div className="space-y-2">
          {episodes.length === 0 && (
            <p className="text-xs text-gray-400 dark:text-gray-500">
              No episodes found. DAG needs <code className="font-mono">episode_type</code> in metadata.
            </p>
          )}
          {episodes.length > 0 && <EpisodeStatsBar summaries={episodes} />}
          {episodes.map((sv) => (
            <EpisodeSummaryRow
              key={sv.episode_id}
              sv={sv}
              onOpenDetail={handleOpenDetail}
            />
          ))}
        </div>
      )}
    </div>
  )
}

// ─── ExecutionHistory (main export) ───────────────────────────────────────

export function ExecutionHistory() {
  // CR-015: use ExecutionSummaryView (workspace.ts) instead of ExecutionSummary (episode.ts)
  const [executions, setExecutions] = useState<ExecutionSummaryView[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const enterReviewMode = useGraphStore((s) => s.enterReviewMode)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const list = await listExecutionSummaries()
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
    <div className="flex flex-col h-full bg-background overflow-hidden">
      {/* Header */}
      <div className="px-4 py-2 border-b border-border flex items-center justify-between shrink-0">
        <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">Execution History</span>
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
          <div className="text-xs text-gray-400 dark:text-gray-500 px-4 py-6 text-center">Loading…</div>
        )}
        {!loading && error && (
          <div className="text-xs text-red-600 px-4 py-3">{error}</div>
        )}
        {!loading && !error && executions.length === 0 && (
          <div className="text-xs text-gray-400 dark:text-gray-500 px-4 py-6 text-center">
            No executions yet. Run a workflow to see history here.
          </div>
        )}

        <div className="divide-y divide-gray-100 dark:divide-gray-800">
          {executions.map((exec) => (
            // CR-015: exec.execution_id instead of exec.id (ExecutionSummaryView field)
            <div key={exec.execution_id}>
              <div className="flex items-stretch">
                {/* Main expand/collapse row */}
                <button
                  className="flex-1 text-left px-4 py-2.5 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
                  onClick={() => setExpandedId(expandedId === exec.execution_id ? null : exec.execution_id)}
                >
                  <div className="flex items-center gap-2">
                    <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase ${statusColor(String(exec.status))}`}>
                      {exec.status}
                    </span>
                    <span className="text-xs font-medium text-gray-700 dark:text-gray-200 truncate flex-1">
                      {exec.display.run_label ?? exec.dag_name}
                    </span>
                    <span className="text-[10px] text-gray-400 dark:text-gray-500 shrink-0">
                      {formatDurationMs(exec.duration_ms)}
                    </span>
                  </div>
                  <div className="text-[10px] text-gray-400 dark:text-gray-500 mt-0.5">
                    {formatDate(exec.started_at)}
                  </div>
                </button>

                {/* Review button — enters REVIEW mode for this execution */}
                <button
                  title="Open in Execution Review"
                  onClick={() => enterReviewMode(exec.execution_id)}
                  className="px-2 text-[10px] font-medium text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20
                             border-l border-gray-100 dark:border-gray-800 transition-colors shrink-0"
                >
                  Review
                </button>
              </div>

              {expandedId === exec.execution_id && (
                <div className="px-4 pb-3">
                  <ExecutionDetail execId={exec.execution_id} />
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
