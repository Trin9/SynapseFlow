import { useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { listDAGs, getDAG, runSavedDAG } from '@/api/client'
import { listExecutionSummaries } from '@/api/episodes'
import type { DAGConfig } from '@/types'
import type { ExecutionSummaryView } from '@/types/workspace'
import { useGraphStore } from '@/hooks/useGraphStore'

// ─── types ────────────────────────────────────────────────────────────────

type LibraryTab = 'library' | 'history' | 'spec'

// ─── helpers ──────────────────────────────────────────────────────────────

function statusColor(status: ExecutionSummaryView['status']): string {
  switch (status) {
    case 'completed': return 'bg-green-100 text-green-700'
    case 'running':   return 'bg-blue-100 text-blue-700'
    case 'failed':    return 'bg-red-100 text-red-700'
    case 'suspended': return 'bg-amber-100 text-amber-700'
    default:          return 'bg-gray-100 text-gray-500'
  }
}

function formatDate(iso: string): string {
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

function formatDuration(durationMs: number): string {
  if (durationMs <= 0) return '—'
  if (durationMs < 1000) return `${durationMs}ms`
  if (durationMs < 60_000) return `${(durationMs / 1000).toFixed(1)}s`
  return `${Math.round(durationMs / 60_000)}m`
}

// ─── LibraryContent ───────────────────────────────────────────────────────

interface LibraryContentProps {
  dags: DAGConfig[]
  executions: ExecutionSummaryView[]
  onLoad: (id: string) => Promise<void>
  onRun: (id: string) => Promise<void>
  onReview: (execId: string) => void
}

function LibraryContent({ dags, executions, onLoad, onRun, onReview }: LibraryContentProps) {
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  if (dags.length === 0) {
    return (
      <div className="text-xs text-gray-400 dark:text-gray-500 text-center py-10 px-4">
        No saved workflows yet.<br />
        Build one on the canvas, then use Save.
      </div>
    )
  }

  async function withBusy(dagId: string, fn: () => Promise<void>) {
    setBusyId(dagId)
    setActionError(null)
    try {
      await fn()
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Action failed')
      setTimeout(() => setActionError(null), 4000)
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div>
      {actionError && (
        <div className="mx-3 mt-2 text-[11px] text-red-600 bg-red-50 border border-red-200 rounded px-2 py-1">
          {actionError}
        </div>
      )}
      <div className="divide-y divide-gray-100 dark:divide-gray-800">
        {dags.map((dag) => {
          const id = dag.id!
          const recentExecs = executions
            .filter((e) => e.dag_id === id)
            .sort((a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime())
            .slice(0, 5)
          const isBusy = busyId === id
          const isExpanded = expandedId === id

          return (
            <div key={id}>
              <div className="px-3 py-2.5 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
                {/* DAG name + meta */}
                <button
                  className="w-full text-left mb-2"
                  onClick={() => setExpandedId(isExpanded ? null : id)}
                >
                  <div className="text-xs font-semibold text-gray-800 dark:text-gray-100 truncate">{dag.name}</div>
                  <div className="text-[10px] text-gray-400 dark:text-gray-500 mt-0.5">
                    {dag.nodes?.length ?? 0} nodes · {dag.edges?.length ?? 0} edges
                    {recentExecs.length > 0 && (
                      <span className="ml-2 text-blue-400">
                        {recentExecs.length} recent run{recentExecs.length !== 1 ? 's' : ''}
                      </span>
                    )}
                  </div>
                </button>

                {/* Action buttons */}
                <div className="flex gap-1.5">
                  <button
                    disabled={isBusy}
                    onClick={() => withBusy(id, () => onLoad(id))}
                    title="Load DAG onto canvas"
                    className="flex-1 px-2 py-1 text-[10px] font-medium text-gray-600 dark:text-gray-300
                               bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600
                               rounded hover:bg-gray-50 dark:hover:bg-gray-700
                               disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                  >
                    Load
                  </button>
                  <button
                    disabled={isBusy}
                    onClick={() => withBusy(id, () => onRun(id))}
                    title="Run this workflow now"
                    className="flex-1 px-2 py-1 text-[10px] font-medium text-white bg-blue-600
                               rounded hover:bg-blue-700
                               disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                  >
                    {isBusy ? '…' : 'Run'}
                  </button>
                </div>

                {/* Recent executions (expanded) */}
                {isExpanded && (
                  <div className="mt-2 space-y-1 border-t border-gray-100 dark:border-gray-800 pt-2">
                    {recentExecs.length === 0 ? (
                      <div className="text-[10px] text-gray-400 dark:text-gray-500">No executions yet.</div>
                    ) : (
                      recentExecs.map((exec) => (
                        <div key={exec.execution_id} className="flex items-center gap-2">
                          <span className={`text-[9px] font-bold px-1 py-0.5 rounded uppercase shrink-0 ${statusColor(exec.status)}`}>
                            {exec.status}
                          </span>
                          <span className="text-[10px] text-gray-500 dark:text-gray-400 flex-1 truncate min-w-0">
                            {formatDate(exec.started_at)}
                          </span>
                          <span className="text-[10px] text-gray-400 dark:text-gray-500 shrink-0">
                            {formatDuration(exec.duration_ms)}
                          </span>
                          <button
                            onClick={() => onReview(exec.execution_id)}
                            className="text-[10px] font-medium text-blue-600 hover:text-blue-800 shrink-0"
                          >
                            Review
                          </button>
                        </div>
                      ))
                    )}
                  </div>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ─── HistoryContent ───────────────────────────────────────────────────────

interface HistoryContentProps {
  executions: ExecutionSummaryView[]
  onReview: (execId: string) => void
}

function HistoryContent({ executions, onReview }: HistoryContentProps) {
  if (executions.length === 0) {
    return (
      <div className="text-xs text-gray-400 dark:text-gray-500 text-center py-10 px-4">
        No executions recorded yet.
      </div>
    )
  }

  // Group by dag_id, preserving name
  const groups = new Map<string, { dagName: string; execs: ExecutionSummaryView[] }>()
  for (const exec of executions) {
    const key = exec.dag_id || 'unknown'
    if (!groups.has(key)) groups.set(key, { dagName: exec.dag_name || key, execs: [] })
    groups.get(key)!.execs.push(exec)
  }

  // Sort each group newest-first; sort groups by their most recent execution
  const sortedGroups = [...groups.entries()]
    .map(([dagId, { dagName, execs }]) => ({
      dagId,
      dagName,
      execs: [...execs].sort(
        (a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime()
      ),
    }))
    .sort(
      (a, b) =>
        new Date(b.execs[0].started_at).getTime() - new Date(a.execs[0].started_at).getTime()
    )

  return (
    <div className="divide-y divide-gray-100 dark:divide-gray-800">
      {sortedGroups.map(({ dagId, dagName, execs }) => (
        <div key={dagId} className="px-3 py-2.5">
          <div className="text-[10px] font-bold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-1.5">
            {dagName}
            <span className="ml-1 font-normal text-gray-400 dark:text-gray-500 normal-case tracking-normal">
              ({execs.length})
            </span>
          </div>
          <div className="space-y-1">
            {execs.map((exec) => (
              <div key={exec.execution_id} className="flex items-center gap-2">
                <span
                  className={`text-[9px] font-bold px-1 py-0.5 rounded uppercase shrink-0 ${statusColor(exec.status)}`}
                >
                  {exec.status}
                </span>
                <span className="text-[10px] text-gray-500 dark:text-gray-400 flex-1 truncate min-w-0">
                  {formatDate(exec.started_at)}
                </span>
                <span className="text-[10px] text-gray-400 dark:text-gray-500 shrink-0">
                  {formatDuration(exec.duration_ms)}
                </span>
                <button
                  onClick={() => onReview(exec.execution_id)}
                  className="text-[10px] font-medium text-blue-600 hover:text-blue-800 shrink-0"
                >
                  Review
                </button>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

// ─── SpecContent ──────────────────────────────────────────────────────────

interface SpecContentProps {
  onLoadSpec: (dag: DAGConfig) => void
}

function SpecContent({ onLoadSpec }: SpecContentProps) {
  const [text, setText] = useState('')
  const [error, setError] = useState<string | null>(null)

  const handleParse = useCallback(() => {
    setError(null)
    const trimmed = text.trim()
    if (!trimmed) return
    try {
      const parsed = JSON.parse(trimmed) as DAGConfig
      if (!Array.isArray(parsed.nodes)) {
        setError('Invalid spec: "nodes" must be an array')
        return
      }
      if (!parsed.name) {
        setError('Invalid spec: missing "name" field')
        return
      }
      onLoadSpec(parsed)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'JSON parse error')
    }
  }, [text, onLoadSpec])

  return (
    <div className="px-3 py-3 flex flex-col gap-3">
      <div className="text-xs text-gray-500 dark:text-gray-400">
        Paste a DAG spec (JSON) to load it onto the canvas.
      </div>
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={'{\n  "name": "My Workflow",\n  "nodes": [...],\n  "edges": [...]\n}'}
        spellCheck={false}
        rows={16}
        className="font-mono text-[11px] border border-gray-300 dark:border-gray-600 rounded p-2 resize-none
                   focus:outline-none focus:border-blue-400 focus:ring-1 focus:ring-blue-400
                   bg-gray-50 dark:bg-gray-800 dark:text-gray-100 w-full"
      />
      {error && (
        <div className="text-[11px] text-red-600 bg-red-50 border border-red-200 rounded px-2 py-1">
          {error}
        </div>
      )}
      <button
        onClick={handleParse}
        disabled={!text.trim()}
        className="px-3 py-1.5 text-sm font-medium text-white bg-blue-600 rounded
                   hover:bg-blue-700 disabled:opacity-40 disabled:cursor-not-allowed
                   transition-colors"
      >
        Load from Spec
      </button>
    </div>
  )
}

// ─── WorkflowLibrary (main export) ────────────────────────────────────────

export function WorkflowLibrary() {
  const [tab, setTab] = useState<LibraryTab>('library')

  const setShowLibrary = useGraphStore((s) => s.setShowLibrary)
  const loadFromDAGConfig = useGraphStore((s) => s.loadFromDAGConfig)
  const enterReviewMode = useGraphStore((s) => s.enterReviewMode)
  const setActiveExecutionId = useGraphStore((s) => s.setActiveExecutionId)
  const setIsRunning = useGraphStore((s) => s.setIsRunning)

  const { data: dags = [], isLoading: dagsLoading } = useQuery({
    queryKey: ['dags'],
    queryFn: listDAGs,
  })

  const { data: executions = [], isLoading: execsLoading } = useQuery({
    queryKey: ['executions', 'summary'],
    queryFn: listExecutionSummaries,
  })

  const loading = dagsLoading || execsLoading

  // Load DAG onto canvas without running
  const handleLoad = useCallback(
    async (dagId: string) => {
      const dag = await getDAG(dagId)
      loadFromDAGConfig(dag)
      setShowLibrary(false)
    },
    [loadFromDAGConfig, setShowLibrary],
  )

  // Load DAG then immediately start a run
  const handleRun = useCallback(
    async (dagId: string) => {
      const dag = await getDAG(dagId)
      loadFromDAGConfig(dag)
      const result = await runSavedDAG(dagId)
      setActiveExecutionId(result.execution_id)
      setIsRunning(true)
      setShowLibrary(false)
    },
    [loadFromDAGConfig, setActiveExecutionId, setIsRunning, setShowLibrary],
  )

  const handleReview = useCallback(
    (execId: string) => {
      enterReviewMode(execId)
      setShowLibrary(false)
    },
    [enterReviewMode, setShowLibrary],
  )

  const handleLoadSpec = useCallback(
    (dag: DAGConfig) => {
      loadFromDAGConfig(dag)
      setShowLibrary(false)
    },
    [loadFromDAGConfig, setShowLibrary],
  )

  const tabs: { key: LibraryTab; label: string }[] = [
    { key: 'library', label: 'Library' },
    { key: 'history', label: 'History' },
    { key: 'spec',    label: 'Spec' },
  ]

  return (
    <div className="w-80 border-r border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 flex flex-col overflow-hidden shrink-0">
      {/* Header */}
      <div className="px-4 py-2 border-b border-gray-100 dark:border-gray-800 flex items-center justify-between shrink-0">
        <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">Workflow Library</span>
        <button
          onClick={() => setShowLibrary(false)}
          title="Close"
          className="text-xs text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
        >
          ✕
        </button>
      </div>

      {/* Tab bar */}
      <div className="flex border-b border-gray-100 dark:border-gray-800 shrink-0">
        {tabs.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`flex-1 py-1.5 text-xs font-medium transition-colors ${
              tab === key
                ? 'border-b-2 border-blue-500 text-blue-600'
                : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="text-xs text-gray-400 dark:text-gray-500 text-center py-10">Loading…</div>
        ) : tab === 'library' ? (
          <LibraryContent
            dags={dags}
            executions={executions}
            onLoad={handleLoad}
            onRun={handleRun}
            onReview={handleReview}
          />
        ) : tab === 'history' ? (
          <HistoryContent executions={executions} onReview={handleReview} />
        ) : (
          <SpecContent onLoadSpec={handleLoadSpec} />
        )}
      </div>
    </div>
  )
}
