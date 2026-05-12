// M5.3 — Historical Comparison Sheet.
//
// Phase D enhancements:
//   • ComparisonNarrativeHeader: outcome verdict + dag context at top of body
//   • Scene manifest default comparison target suggestion in search form
//   • Improved current/historical split with run_label and duration
import { useEffect, useState } from 'react'
import { X, GitCompare } from 'lucide-react'
import { getExecutionSummaryView, getComparisonTarget, listExecutionSummariesByDAG } from '@/api/episodes'
import { getSceneManifest } from '@/lib/sceneManifest'
import type { ExecutionSummaryView, ComparisonSummaryView } from '@/types/workspace'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { ScrollArea } from '@/components/ui/scroll-area'

// ─── Narrative header ─────────────────────────────────────────────────────

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

function ComparisonNarrativeHeader({
  current,
  comparison,
}: {
  current: ExecutionSummaryView
  comparison: ComparisonSummaryView
}) {
  const outcome = comparison.outcome ?? 'unknown'
  const label = OUTCOME_LABEL[outcome] ?? outcome

  return (
    <div className="px-4 py-3 rounded-xl border mb-1 bg-muted/40">
      <div className="flex items-center gap-2 mb-1">
        <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">Comparison Result</span>
        <Badge variant={outcomeVariant(outcome)} className="uppercase text-[9px]">{outcome}</Badge>
      </div>
      <p className="text-sm font-semibold leading-snug">{label}</p>
      <p className="text-xs mt-0.5 text-muted-foreground">
        {current.dag_name}
        {current.workflow_kind ? ` · ${current.workflow_kind}` : ''}
        {current.display.run_label ? ` · ${current.display.run_label}` : ''}
      </p>
    </div>
  )
}

// ─── Left (current execution) side ───────────────────────────────────────

function CurrentSide({ summary }: { summary: ExecutionSummaryView }) {
  return (
    <div className="flex-1 min-w-0 space-y-2">
      <span className="wb-section-header block">Current</span>
      <div className="space-y-1 text-xs text-foreground">
        <div className="flex gap-2">
          <span className="text-muted-foreground w-16 shrink-0">DAG</span>
          <span className="font-medium truncate">{summary.dag_name}</span>
        </div>
        <div className="flex gap-2">
          <span className="text-muted-foreground w-16 shrink-0">Status</span>
          <span className="font-mono font-semibold">{summary.status}</span>
        </div>
        <div className="flex gap-2">
          <span className="text-muted-foreground w-16 shrink-0">Kind</span>
          <span className="capitalize">{summary.workflow_kind}</span>
        </div>
        {summary.display.run_label && (
          <div className="flex gap-2">
            <span className="text-muted-foreground w-16 shrink-0">Label</span>
            <span className="italic text-muted-foreground truncate">{summary.display.run_label}</span>
          </div>
        )}
        <div className="flex gap-2">
          <span className="text-muted-foreground w-16 shrink-0">ID</span>
          <span className="font-mono text-muted-foreground/50 truncate">{summary.execution_id.slice(0, 12)}…</span>
        </div>
      </div>
    </div>
  )
}

// ─── Right (historical / comparison) side ─────────────────────────────────

function HistoricalSide({ comparison }: { comparison: ComparisonSummaryView }) {
  return (
    <div className="flex-1 min-w-0 space-y-2">
      <span className="wb-section-header block">Historical</span>
      <div className="space-y-1 text-xs text-foreground">
        {comparison.outcome && (
          <Badge variant={outcomeVariant(comparison.outcome)} className="uppercase text-[9px] mb-1">
            {comparison.outcome}
          </Badge>
        )}
        <p className="font-medium leading-snug">{comparison.title}</p>
        {comparison.compared_against && (
          <p className="text-muted-foreground italic text-[11px]">{comparison.compared_against}</p>
        )}
        <div className="flex gap-2">
          <span className="text-muted-foreground w-16 shrink-0">ID</span>
          <span className="font-mono text-muted-foreground/50 truncate">{comparison.execution_id.slice(0, 12)}…</span>
        </div>
      </div>
    </div>
  )
}

// ─── Sheet ────────────────────────────────────────────────────────────────

export interface HistoricalComparisonSheetProps {
  /** The current execution id (the "left" side of the comparison). */
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
    return () => {
      cancelled = true
    }
  }, [execId])

  // Phase D — suggest scene manifest default comparison target
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

  return (
    <div className="absolute right-0 top-0 bottom-0 w-[480px] bg-card border-l z-20 flex flex-col overflow-hidden wb-animate-slide-right">
      {/* Header */}
      <div className="h-10 px-4 border-b flex items-center justify-between shrink-0">
        <span className="wb-section-header flex items-center gap-1.5">
          <GitCompare className="w-3.5 h-3.5" />
          Execution Comparison
        </span>
        <Button size="xs" variant="ghost" onClick={onClose} className="h-6 w-6 p-0">
          <X className="w-3.5 h-3.5" />
        </Button>
      </div>

      {/* Search form */}
      <div className="px-4 py-3 border-b shrink-0 space-y-2">
        <label className="wb-section-header block">Historical Execution ID</label>
        <div className="flex gap-2">
          <input
            type="text"
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') handleFetch() }}
            placeholder="Paste execution UUID…"
            className="flex-1 border rounded-lg px-3 py-1.5 text-xs font-mono focus:outline-none focus:ring-2 focus:ring-ring bg-background text-foreground"
          />
          <Button size="sm" onClick={handleFetch} disabled={loading || !inputValue.trim()}>
            {loading ? '…' : 'Compare'}
          </Button>
        </div>
        {/* Phase D — scene manifest default comparison suggestion */}
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
          <p className="text-[10px] text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1">
            No comparable historical execution found for this workflow yet.
          </p>
        )}
        {error && <p className="text-xs text-destructive">{error}</p>}
      </div>

      {/* Comparison body */}
      <ScrollArea className="flex-1 min-h-0">
        {currentSummary && comparison ? (
          <div className="p-4 space-y-4">
            {/* Phase D — Narrative outcome header */}
            <ComparisonNarrativeHeader current={currentSummary} comparison={comparison} />

            {/* Split view */}
            <div className="flex gap-4 p-4 bg-muted/40 rounded-xl border">
              <CurrentSide summary={currentSummary} />
              <Separator orientation="vertical" className="self-stretch" />
              <HistoricalSide comparison={comparison} />
            </div>

            {/* Summary */}
            {comparison.summary && (
              <div className="space-y-1">
                <span className="wb-section-header block">Summary</span>
                <p className="text-sm text-foreground leading-relaxed bg-muted/40 p-3 rounded-lg border">
                  {comparison.summary}
                </p>
              </div>
            )}

            {/* Highlights */}
            {(comparison.highlights ?? []).length > 0 && (
              <div className="space-y-2">
                <span className="wb-section-header block">Highlights</span>
                <ul className="space-y-1">
                  {comparison.highlights!.map((hl, i) => (
                    <li key={i} className="flex gap-2 text-sm text-foreground">
                      <span className="text-blue-400 shrink-0">•</span>
                      <span>{hl}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {/* Caution */}
            {comparison.caution && (
              <div className="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3 flex gap-2">
                <span className="text-amber-500 text-sm shrink-0">⚠</span>
                <p className="text-sm text-amber-800 dark:text-amber-300 leading-relaxed">{comparison.caution}</p>
              </div>
            )}
          </div>
        ) : !loading && (
          <div className="flex flex-col items-center justify-center h-full text-muted-foreground gap-2 px-8 text-center py-20">
            <p className="text-sm">Enter a historical execution ID above to compare it against the current run.</p>
          </div>
        )}
      </ScrollArea>
    </div>
  )
}
