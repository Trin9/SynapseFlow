import { useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Play, Upload, BookOpen, GitCompare, ChevronRight, Zap, Loader2 } from 'lucide-react'
import { listDAGs, getDAG, runSavedDAG, getExecutionNodes } from '@/api/client'
import { getEpisode, listEpisodeSummariesByExecution, listExecutionSummaries } from '@/api/episodes'
import type { DAGConfig } from '@/types'
import type { ExecutionSummaryView } from '@/types/workspace'
import { useGraphStore } from '@/hooks/useGraphStore'
import { getSceneManifest } from '@/lib/sceneManifest'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

// ─── types ────────────────────────────────────────────────────────────────

type LibraryTab = 'library' | 'history' | 'spec'

// ─── helpers ──────────────────────────────────────────────────────────────

function statusVariant(status: ExecutionSummaryView['status']): "success" | "info" | "destructive" | "warning" | "ghost" {
  switch (status) {
    case 'completed': return 'success'
    case 'running':   return 'info'
    case 'failed':    return 'destructive'
    case 'suspended': return 'warning'
    default:          return 'ghost'
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
  onOpenDossier: (execId: string) => Promise<void>
  onCompare: (execId: string) => Promise<void>
}

function LibraryContent({ dags, executions, onLoad, onRun, onReview, onOpenDossier, onCompare }: LibraryContentProps) {
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

  const quickScenes = [
    {
      key: 'boutique_checkout_consistency_audit',
      title: 'Boutique Checkout Consistency Audit',
    },
    {
      key: 'checkout_payment_unreachable_agent_loop',
      title: 'Checkout Payment Unreachable Agent Loop',
    },
  ]

  return (
    <div>
      {actionError && (
        <div className="mx-3 mt-2 text-[11px] text-destructive bg-destructive/10 border border-destructive/20 rounded px-2 py-1">
          {actionError}
        </div>
      )}

      <div className="px-3 pt-3 pb-2 border-b">
        <div className="wb-section-header mb-2 flex items-center gap-1.5">
          <Zap className="w-3 h-3 text-amber-500" />
          Quick Scene Entry
        </div>
        <div className="grid grid-cols-1 gap-2">
          {quickScenes.map((scene) => {
            const dag = dags.find((d) => {
              const id = (d.id ?? '').toLowerCase()
              const name = (d.name ?? '').toLowerCase().replace(/\s+/g, '_')
              return id === scene.key || name === scene.key
            })

            const manifest = getSceneManifest(scene.key)
            const latestExec = dag
              ? executions
                  .filter((e) => e.dag_id === dag.id)
                  .sort((a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime())[0]
              : undefined

            const disabled = !dag || !latestExec

            return (
              <button
                key={scene.key}
                disabled={disabled}
                onClick={() => latestExec && onReview(latestExec.execution_id)}
                className={cn(
                  "text-left px-3 py-3 rounded-lg border-2 transition-all",
                  disabled
                    ? "bg-muted/40 border-border opacity-50 cursor-not-allowed"
                    : "bg-card border-border hover:border-cyan-500/40 hover:bg-accent/50 hover:shadow-md cursor-pointer"
                )}
                title={disabled ? 'Scene not available yet (missing workflow or execution history)' : 'Open latest execution directly in review workspace'}
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="text-xs font-semibold text-foreground">{scene.title}</div>
                    {manifest?.description && (
                      <p className="mt-1 text-[10px] text-muted-foreground leading-relaxed line-clamp-2">
                        {manifest.description}
                      </p>
                    )}
                  </div>
                  <ChevronRight className={cn("w-4 h-4 shrink-0 mt-0.5", disabled ? "text-muted-foreground/30" : "text-cyan-500")} />
                </div>
                <div className="mt-2 flex items-center gap-2 flex-wrap">
                  {disabled ? (
                    <span className="text-[10px] text-muted-foreground/60">No executions found</span>
                  ) : (
                    <>
                      <Badge variant="secondary" className="text-[9px]">Review</Badge>
                      <span className="text-[10px] text-muted-foreground font-mono">
                        {latestExec!.execution_id.slice(0, 12)}…
                      </span>
                      <Badge variant={statusVariant(latestExec!.status)} className="text-[9px] ml-auto">
                        {latestExec!.status}
                      </Badge>
                    </>
                  )}
                </div>
              </button>
            )
          })}
        </div>
      </div>

      <div className="divide-y">
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
              <div className="px-3 py-3 hover:bg-accent/30 transition-colors">
                {(() => {
                  const scene = getSceneManifest(dag.id, dag.name)
                  const latestExec = recentExecs[0]

                  return (
                    <>
                      {/* DAG name + meta */}
                      <button
                        className="w-full text-left mb-2.5"
                        onClick={() => setExpandedId(isExpanded ? null : id)}
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div className="min-w-0">
                            <div className="text-xs font-semibold text-foreground truncate">
                              {scene?.title ?? dag.name}
                            </div>
                            <div className="text-[10px] text-muted-foreground mt-0.5">
                              {dag.nodes?.length ?? 0} nodes · {dag.edges?.length ?? 0} edges
                              {recentExecs.length > 0 && (
                                <span className="ml-2 text-blue-500 dark:text-blue-400">
                                  {recentExecs.length} run{recentExecs.length !== 1 ? 's' : ''}
                                </span>
                              )}
                            </div>
                          </div>
                          <Badge variant={scene ? "secondary" : "ghost"} className="text-[9px] shrink-0">
                            {scene ? 'Curated' : 'Standard'}
                          </Badge>
                        </div>
                        {scene?.description && (
                          <p className="mt-1.5 text-[11px] text-muted-foreground leading-relaxed">
                            {scene.description}
                          </p>
                        )}
                      </button>

                      {/* Primary actions */}
                      <div className="grid grid-cols-2 gap-1.5">
                        <Button size="xs" variant="outline" disabled={isBusy} onClick={() => withBusy(id, () => onLoad(id))} title="Load DAG onto canvas">
                          <Upload className="w-3 h-3" />
                          Load
                        </Button>
                        <Button size="xs" variant="default" disabled={isBusy} onClick={() => withBusy(id, () => onRun(id))} title="Run this workflow now">
                          {isBusy ? <Loader2 className="w-3 h-3 animate-spin" /> : <Play className="w-3 h-3" />}
                          Run
                        </Button>
                        <Button size="xs" variant="outline" disabled={!latestExec || isBusy} onClick={() => latestExec && withBusy(id, () => onOpenDossier(latestExec.execution_id))} title="Open dossier from the latest execution">
                          <BookOpen className="w-3 h-3" />
                          Open Dossier
                        </Button>
                        <Button size="xs" variant="outline" disabled={!latestExec || isBusy} onClick={() => latestExec && withBusy(id, () => onCompare(latestExec.execution_id))} title="Open dossier and compare against historical runs">
                          <GitCompare className="w-3 h-3" />
                          Compare
                        </Button>
                      </div>

                      {scene?.recommendedReplayPercent != null && (
                        <p className="mt-1.5 text-[10px] text-muted-foreground">
                          Recommended replay: {scene.recommendedReplayPercent}%
                        </p>
                      )}

                      {/* Recent executions (expanded) */}
                      {isExpanded && (
                        <div className="mt-2 space-y-1 border-t pt-2">
                          {recentExecs.length === 0 ? (
                            <div className="text-[10px] text-muted-foreground">No executions yet.</div>
                          ) : (
                            recentExecs.map((exec) => (
                              <div key={exec.execution_id} className="flex items-center gap-2">
                                <Badge variant={statusVariant(exec.status)} className="text-[9px] uppercase">
                                  {exec.status}
                                </Badge>
                                <span className="text-[10px] text-muted-foreground flex-1 truncate min-w-0">
                                  {formatDate(exec.started_at)}
                                </span>
                                <span className="text-[10px] text-muted-foreground shrink-0">
                                  {formatDuration(exec.duration_ms)}
                                </span>
                                <Button size="xs" variant="ghost" className="h-5 px-1.5 text-[10px] text-blue-600" onClick={() => onReview(exec.execution_id)}>
                                  Review
                                </Button>
                              </div>
                            ))
                          )}
                        </div>
                      )}
                    </>
                  )
                })()}
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
      <div className="text-xs text-muted-foreground text-center py-10 px-4">
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
    <div className="divide-y">
      {sortedGroups.map(({ dagId, dagName, execs }) => (
        <div key={dagId} className="px-3 py-2.5">
          <div className="wb-section-header mb-1.5">
            {dagName}
            <span className="ml-1 font-normal text-muted-foreground normal-case tracking-normal">
              ({execs.length})
            </span>
          </div>
          <div className="space-y-1">
            {execs.map((exec) => (
              <div key={exec.execution_id} className="flex items-center gap-2">
                <Badge variant={statusVariant(exec.status)} className="text-[9px] uppercase">
                  {exec.status}
                </Badge>
                <span className="text-[10px] text-muted-foreground flex-1 truncate min-w-0">
                  {formatDate(exec.started_at)}
                </span>
                <span className="text-[10px] text-muted-foreground shrink-0">
                  {formatDuration(exec.duration_ms)}
                </span>
                <Button size="xs" variant="ghost" className="h-5 px-1.5 text-[10px] text-blue-600" onClick={() => onReview(exec.execution_id)}>
                  Review
                </Button>
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
  const setExecutionResult = useGraphStore((s) => s.setExecutionResult)
  const setSelectedEpisode = useGraphStore((s) => s.setSelectedEpisode)
  const setOpenComparisonOnDossier = useGraphStore((s) => s.setOpenComparisonOnDossier)
  const setAppMode = useGraphStore((s) => s.setAppMode)

  const { data: dags = [], isLoading: dagsLoading } = useQuery({
    queryKey: ['dags'],
    queryFn: listDAGs,
  })

  const { data: executions = [], isLoading: execsLoading, error: execsError } = useQuery({
    queryKey: ['executions', 'summary'],
    queryFn: listExecutionSummaries,
  })

  const loading = dagsLoading

  // Load DAG onto canvas without running
  const handleLoad = useCallback(
    async (dagId: string) => {
      const dag = await getDAG(dagId)
      setAppMode('BUILDER')
      loadFromDAGConfig(dag)
      setShowLibrary(false)
    },
    [loadFromDAGConfig, setAppMode, setShowLibrary],
  )

  // Load DAG then immediately start a run
  const handleRun = useCallback(
    async (dagId: string) => {
      const dag = await getDAG(dagId)
      loadFromDAGConfig(dag)
      setExecutionResult(null)
      const result = await runSavedDAG(dagId)
      enterReviewMode(result.execution_id)
      setActiveExecutionId(result.execution_id)
      setIsRunning(true)
      try {
        const snapshot = await getExecutionNodes(result.execution_id)
        setExecutionResult(snapshot)
      } catch {
        // Best-effort snapshot; poller continues even if this call fails.
      }
      setShowLibrary(false)
    },
    [enterReviewMode, loadFromDAGConfig, setActiveExecutionId, setExecutionResult, setIsRunning, setShowLibrary],
  )

  const handleReview = useCallback(
    (execId: string) => {
      enterReviewMode(execId)
      setShowLibrary(false)
    },
    [enterReviewMode, setShowLibrary],
  )

  const handleOpenDossier = useCallback(
    async (execId: string) => {
      enterReviewMode(execId)
      const summaries = await listEpisodeSummariesByExecution(execId)
      if (summaries.length > 0) {
        const episode = await getEpisode(summaries[0].episode_id)
        setSelectedEpisode(episode)
      }
      setShowLibrary(false)
    },
    [enterReviewMode, setSelectedEpisode, setShowLibrary],
  )

  const handleCompare = useCallback(
    async (execId: string) => {
      setOpenComparisonOnDossier(true)
      await handleOpenDossier(execId)
    },
    [handleOpenDossier, setOpenComparisonOnDossier],
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
  void tabs // shadcn Tabs component used instead

  return (
    <div className="w-full h-full flex flex-col overflow-hidden bg-background">
      {/* Tabs */}
      <div className="px-3 pt-2 shrink-0">
        <Tabs value={tab} onValueChange={(v) => setTab(v as LibraryTab)} className="w-full">
          <TabsList className="w-full h-8">
            <TabsTrigger value="library" className="flex-1 text-xs">Library</TabsTrigger>
            <TabsTrigger value="history" className="flex-1 text-xs">History</TabsTrigger>
            <TabsTrigger value="spec" className="flex-1 text-xs">Spec</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      {/* Content */}
      <ScrollArea className="flex-1 min-h-0">
        {execsError && (
          <div className="mx-3 mt-2 text-[11px] text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1">
            Execution history unavailable. Library can still load DAGs.
          </div>
        )}
        {execsLoading && !execsError && (
          <div className="mx-3 mt-2 text-[11px] text-muted-foreground">
            Loading execution history...
          </div>
        )}
        {loading ? (
          <div className="p-3 space-y-2">
            {[1,2,3].map(i => <Skeleton key={i} className="h-24 w-full rounded-lg" />)}
          </div>
        ) : tab === 'library' ? (
          <LibraryContent
            dags={dags}
            executions={executions}
            onLoad={handleLoad}
            onRun={handleRun}
            onReview={handleReview}
            onOpenDossier={handleOpenDossier}
            onCompare={handleCompare}
          />
        ) : tab === 'history' ? (
          <HistoryContent executions={executions} onReview={handleReview} />
        ) : (
          <SpecContent onLoadSpec={handleLoadSpec} />
        )}
      </ScrollArea>
    </div>
  )
}
