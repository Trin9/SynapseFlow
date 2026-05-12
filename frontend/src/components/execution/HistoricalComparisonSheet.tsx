// M5.3 — Historical Comparison Sheet (Phase D: full-screen split layout).
//
// Full-screen overlay that lets the operator compare the current execution
// against a historical one side-by-side, with explainability sections.
import { useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import { X, GitCompare, Search, Info, AlertTriangle } from 'lucide-react'
import { getExecutionSummaryView, getComparisonTarget, listExecutionSummariesByDAG } from '@/api/episodes'
import { getSceneManifest } from '@/lib/sceneManifest'
import type { ExecutionSummaryView, ComparisonSummaryView } from '@/types/workspace'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'

// ─── Outcome helpers ───────────────────────────────────────────────────────

function outcomeVariant(outcome: string): 'success' | 'destructive' | 'warning' | 'secondary' {
  if (outcome === 'match') return 'success'
  if (outcome === 'divergent') return 'destructive'
  if (outcome === 'partial') return 'warning'
  return 'secondary'
}

const OUTCOME_LABEL: Record<string, string> = {
  match:     'Consistent with historical baseline',
  divergent: 'Diverged from historical baseline',
  partial:   'Partially aligned — notable differences observed',
}

// ─── Execution detail card ─────────────────────────────────────────────────

function ExecutionDetailCard({
  title,
  summary,
  variant = 'default',
}: {
  title: string
  summary: ExecutionSummaryView
  variant?: 'current' | 'historical' | 'default'
}) {
  const isCurrent = variant === 'current'
  const statusVariant = summary.status === 'completed' ? 'success' as const :
    summary.status === 'failed' ? 'destructive' as const :
    summary.status === 'running' ? 'info' as const :
    'secondary' as const

  return (
    <div className={[
      'rounded-xl border p-4 space-y-3',
      isCurrent ? 'border-blue-200 dark:border-blue-700 bg-blue-50/30 dark:bg-blue-900/10' :
      variant === 'historical' ? 'border-violet-200 dark:border-violet-700 bg-violet-50/30 dark:bg-violet-900/10' :
      'bg-card',
    ].join(' ')}>
      <div className="flex items-center gap-2">
        <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
          {title}
        </span>
        <Badge variant={statusVariant} className="text-[9px] uppercase">{summary.status}</Badge>
      </div>

      <div className="space-y-1.5 text-xs">
        <div className="flex gap-2">
          <span className="text-muted-foreground w-14 shrink-0">DAG</span>
          <span className="font-medium truncate">{summary.dag_name}</span>
        </div>
        <div className="flex gap-2">
          <span className="text-muted-foreground w-14 shrink-0">Kind</span>
          <span className="capitalize">{summary.workflow_kind}</span>
        </div>
        {summary.display.run_label && (
          <div className="flex gap-2">
            <span className="text-muted-foreground w-14 shrink-0">Label</span>
            <span className="italic truncate">{summary.display.run_label}</span>
          </div>
        )}
        <div className="flex gap-2">
          <span className="text-muted-foreground w-14 shrink-0">Started</span>
          <span className="font-mono text-[11px]">{new Date(summary.started_at).toLocaleString()}</span>
        </div>
        {summary.duration_ms > 0 && (
          <div className="flex gap-2">
            <span className="text-muted-foreground w-14 shrink-0">Duration</span>
            <span className="font-mono">{(summary.duration_ms / 1000).toFixed(1)}s</span>
          </div>
        )}
        <div className="flex gap-2">
          <span className="text-muted-foreground w-14 shrink-0">ID</span>
          <span className="font-mono text-muted-foreground/50 truncate text-[11px]">{summary.execution_id}</span>
        </div>
      </div>
    </div>
  )
}

// ─── Sheet ────────────────────────────────────────────────────────────────

export interface HistoricalComparisonSheetProps {
  execId: string
  onClose: () => void
}

export function HistoricalComparisonSheet({ execId, onClose }: HistoricalComparisonSheetProps) {
  const [inputValue, setInputValue] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [currentSummary, setCurrentSummary] = useState<ExecutionSummaryView | null>(null)
  const [comparison, setComparison] = useState<ComparisonSummaryView | null>(null)
  const [suggestions, setSuggestions] = useState<ExecutionSummaryView[]>([])
  const [suggestionsLoading, setSuggestionsLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    async function bootstrap() {
      setSuggestionsLoading(true)
      try {
        const current = await getExecutionSummaryView(execId)
        if (cancelled) return
        setCurrentSummary(current)
        const sameDag = await listExecutionSummariesByDAG(current.dag_id)
        if (cancelled) return
        const candidates = sameDag
          .filter((e) => e.execution_id !== execId)
          .sort((a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime())
          .slice(0, 5)
        setSuggestions(candidates)
      } catch {
        if (!cancelled) setSuggestions([])
      } finally {
        if (!cancelled) setSuggestionsLoading(false)
      }
    }
    void bootstrap()
    return () => { cancelled = true }
  }, [execId])

  const manifestDefault = (() => {
    if (!currentSummary) return null
    const m = getSceneManifest(currentSummary.dag_id, currentSummary.dag_name)
    return m?.defaultComparisonTarget ?? null
  })()

  async function handleFetch() {
    const historicalId = inputValue.trim()
    if (!historicalId) return
    setLoading(true)
    setError(null)
    setComparison(null)
    setCurrentSummary(null)
    try {
      const [summary, comp] = await Promise.all([
        getExecutionSummaryView(execId),
        getComparisonTarget(execId, historicalId),
      ])
      setCurrentSummary(summary)
      setComparison(comp)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Comparison failed')
    } finally {
      setLoading(false)
    }
  }

  const outcome = comparison?.outcome

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.2 }}
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 backdrop-blur-sm p-6"
    >
      <motion.div
        initial={{ scale: 0.96, opacity: 0 }}
        animate={{ scale: 1, opacity: 1 }}
        exit={{ scale: 0.96, opacity: 0 }}
        transition={{ type: 'spring', damping: 25, stiffness: 220 }}
        className="w-full max-w-[1400px] h-full max-h-[90vh] bg-card rounded-2xl shadow-2xl flex flex-col overflow-hidden border"
      >
        {/* Header */}
        <div className="h-12 px-5 border-b flex items-center justify-between shrink-0">
          <div className="flex items-center gap-2">
            <GitCompare className="w-4 h-4 text-muted-foreground" />
            <span className="text-sm font-semibold text-foreground">Execution Comparison</span>
            <span className="text-[11px] text-muted-foreground">
              Compare the current workflow execution against a recalled historical baseline
            </span>
          </div>
          <Button size="xs" variant="ghost" onClick={onClose}>
            <X className="w-4 h-4" />
          </Button>
        </div>

        {/* Search form */}
        <div className="px-5 py-2.5 border-b shrink-0 space-y-1.5 bg-muted/20">
          <label className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider">
            Historical Execution ID
          </label>
          <div className="flex gap-2">
            <div className="flex-1 relative">
              <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
              <input
                type="text"
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') handleFetch() }}
                placeholder="Paste execution UUID or pick from suggestions below..."
                className="w-full border rounded-lg pl-9 pr-3 py-2 text-xs font-mono focus:outline-none focus:ring-2 focus:ring-ring bg-background text-foreground"
              />
            </div>
            <Button size="sm" onClick={handleFetch} disabled={loading || !inputValue.trim()}>
              {loading ? '…' : 'Compare'}
            </Button>
          </div>
          {manifestDefault && !comparison && (
            <button
              onClick={() => setInputValue(manifestDefault)}
              className="text-[10px] text-blue-500 dark:text-blue-400 hover:underline flex items-center gap-1"
            >
              <span>↳ Use suggested baseline:</span>
              <span className="font-mono">{manifestDefault.slice(0, 16)}…</span>
            </button>
          )}
          {!loading && suggestions.length > 0 && !comparison && (
            <div className="pt-1">
              <p className="text-[10px] text-muted-foreground mb-1">Recent same-workflow runs:</p>
              <div className="flex flex-wrap gap-1.5">
                {suggestions.map((s) => (
                  <button
                    key={s.execution_id}
                    onClick={() => setInputValue(s.execution_id)}
                    className="text-[10px] px-2 py-1 rounded border hover:bg-accent text-left"
                    title={s.execution_id}
                  >
                    {s.execution_id.slice(0, 8)}… {s.status}
                  </button>
                ))}
              </div>
            </div>
          )}
          {!loading && !suggestionsLoading && suggestions.length === 0 && !comparison && (
            <p className="text-[10px] text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1 dark:bg-amber-900/20 dark:text-amber-400 dark:border-amber-800">
              No comparable historical execution found for this workflow yet.
            </p>
          )}
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>

        {/* Body — side-by-side grid when comparison loaded */}
        {currentSummary && comparison ? (
          <div className="flex-1 min-h-0 grid grid-cols-[1fr_1fr] overflow-hidden">
            {/* Left — Current execution */}
            <ScrollArea className="border-r">
              <div className="p-5 space-y-4">
                {/* Verdict header */}
                <div className="rounded-xl border p-4 bg-muted/30">
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                      Comparison Result
                    </span>
                    {outcome && (
                      <Badge variant={outcomeVariant(outcome)} className="uppercase text-[9px]">{outcome}</Badge>
                    )}
                  </div>
                  <p className="text-sm font-semibold">{OUTCOME_LABEL[outcome ?? 'unknown'] ?? outcome}</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    {currentSummary.dag_name} · {currentSummary.workflow_kind}
                    {currentSummary.display.run_label ? ` · ${currentSummary.display.run_label}` : ''}
                  </p>
                </div>

                <ExecutionDetailCard title="CURRENT EXECUTION" summary={currentSummary} variant="current" />

                {/* Why matched section */}
                <div className="rounded-xl border p-4 bg-muted/30 space-y-2">
                  <div className="flex items-center gap-2">
                    <Info className="w-3.5 h-3.5 text-blue-500" />
                    <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                      Why this comparison was selected
                    </span>
                  </div>
                  <div className="text-xs text-muted-foreground space-y-1">
                    <p>
                      Outcome: <span className="font-semibold text-foreground">{outcome ?? 'unknown'}</span> — {OUTCOME_LABEL[outcome ?? 'unknown'] ?? 'awaiting analysis'}
                    </p>
                    {comparison.title && (
                      <p>
                        Historical run: <span className="font-medium text-foreground">{comparison.title}</span>
                      </p>
                    )}
                    {comparison.compared_against && (
                      <p>
                        Matched against: <span className="font-mono text-[11px]">{comparison.compared_against}</span>
                      </p>
                    )}
                  </div>
                </div>

                {/* Highlights */}
                {(comparison.highlights ?? []).length > 0 && (
                  <div className="space-y-2">
                    <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                      Key Differences
                    </span>
                    <ul className="space-y-1.5">
                      {comparison.highlights!.map((hl, i) => (
                        <li key={i} className="flex gap-2 text-sm rounded-lg border bg-muted/20 px-3 py-2">
                          <span className="text-blue-400 shrink-0 mt-0.5">■</span>
                          <span>{hl}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            </ScrollArea>

            {/* Right — Historical execution */}
            <ScrollArea>
              <div className="p-5 space-y-4">
                <ExecutionDetailCard title="HISTORICAL BASELINE" summary={{
                  execution_id: comparison.execution_id,
                  dag_id: '',
                  dag_name: comparison.title,
                  status: comparison.outcome === 'match' ? 'completed' : 'completed',
                  started_at: '',
                  ended_at: undefined,
                  duration_ms: 0,
                  mode: 'execution',
                  workflow_kind: 'historical',
                  display: { run_label: comparison.compared_against ?? comparison.execution_id.slice(0, 8) },
                }} variant="historical" />

                {/* Summary */}
                {comparison.summary && (
                  <div className="space-y-1">
                    <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                      Summary
                    </span>
                    <p className="text-sm leading-relaxed bg-muted/40 p-3 rounded-lg border">
                      {comparison.summary}
                    </p>
                  </div>
                )}

                {/* Caution */}
                {comparison.caution && (
                  <div className="rounded-xl border border-amber-200 dark:border-amber-700 bg-amber-50 dark:bg-amber-900/20 p-4 space-y-2">
                    <div className="flex items-center gap-2">
                      <AlertTriangle className="w-4 h-4 text-amber-500" />
                      <span className="text-[10px] font-bold uppercase tracking-wider text-amber-700 dark:text-amber-400">
                        Operator Caution
                      </span>
                    </div>
                    <p className="text-sm text-amber-800 dark:text-amber-300 leading-relaxed">
                      {comparison.caution}
                    </p>
                  </div>
                )}
              </div>
            </ScrollArea>
          </div>
        ) : !loading && (
          <div className="flex-1 flex flex-col items-center justify-center text-muted-foreground gap-2 px-8 text-center">
            <GitCompare className="w-8 h-8 opacity-20" />
            <p className="text-sm">Enter a historical execution ID above to compare it against the current run.</p>
            <p className="text-xs opacity-60">Use one of the suggested recent runs, or paste a known execution UUID.</p>
          </div>
        )}
      </motion.div>
    </motion.div>
  )
}
