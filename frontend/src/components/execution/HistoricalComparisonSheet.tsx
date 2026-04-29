// M5.3 — Historical Comparison Sheet.
//
// A right-side overlay panel triggered from the ForensicDossierDrawer that
// lets the user compare the current execution against a historical one.
//
// Layout (when comparison is loaded):
//
//   ┌─ EXECUTION COMPARISON ──────────────── [×] ─┐
//   │  [CURRENT]           │  [HISTORICAL]          │
//   │  id / dag / status   │  comparison title /    │
//   │  started / duration  │  outcome / summary     │
//   ├──────────────────────────────────────────────┤
//   │  HIGHLIGHTS                                   │
//   │  • item…                                      │
//   ├──────────────────────────────────────────────┤
//   │  ⚠ CAUTION                                   │
//   │  caution text                                 │
//   └──────────────────────────────────────────────┘
//
// When no historical ID has been submitted yet the panel shows a search form.
import { useState } from 'react'
import { getExecutionSummaryView, getComparisonTarget } from '@/api/episodes'
import type { ExecutionSummaryView, ComparisonSummaryView } from '@/types/workspace'

// ─── Left (current execution) side ───────────────────────────────────────

function CurrentSide({ summary }: { summary: ExecutionSummaryView }) {
  return (
    <div className="flex-1 min-w-0 space-y-2">
      <span className="text-[10px] font-bold text-gray-400 dark:text-gray-500 uppercase tracking-wider block">
        Current
      </span>
      <div className="space-y-1 text-xs text-gray-700 dark:text-gray-300">
        <div className="flex gap-2">
          <span className="text-gray-400 dark:text-gray-500 w-16 shrink-0">DAG</span>
          <span className="font-medium truncate">{summary.dag_name}</span>
        </div>
        <div className="flex gap-2">
          <span className="text-gray-400 dark:text-gray-500 w-16 shrink-0">Status</span>
          <span className="font-mono font-semibold">{summary.status}</span>
        </div>
        <div className="flex gap-2">
          <span className="text-gray-400 dark:text-gray-500 w-16 shrink-0">Kind</span>
          <span className="capitalize">{summary.workflow_kind}</span>
        </div>
        {summary.display.run_label && (
          <div className="flex gap-2">
            <span className="text-gray-400 dark:text-gray-500 w-16 shrink-0">Label</span>
            <span className="italic text-gray-500 dark:text-gray-400 truncate">{summary.display.run_label}</span>
          </div>
        )}
        <div className="flex gap-2">
          <span className="text-gray-400 dark:text-gray-500 w-16 shrink-0">ID</span>
          <span className="font-mono text-gray-300 dark:text-gray-600 truncate">{summary.execution_id.slice(0, 12)}…</span>
        </div>
      </div>
    </div>
  )
}

// ─── Right (historical / comparison) side ─────────────────────────────────

function HistoricalSide({ comparison }: { comparison: ComparisonSummaryView }) {
  const outcomeColor =
    comparison.outcome === 'match'
      ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 border-green-200 dark:border-green-800'
      : comparison.outcome === 'divergent'
      ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 border-red-200 dark:border-red-800'
      : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 border-gray-200 dark:border-gray-600'

  return (
    <div className="flex-1 min-w-0 space-y-2">
      <span className="text-[10px] font-bold text-gray-400 dark:text-gray-500 uppercase tracking-wider block">
        Historical
      </span>
      <div className="space-y-1 text-xs text-gray-700 dark:text-gray-300">
        {comparison.outcome && (
          <span
            className={`inline-block text-[10px] font-bold px-2 py-0.5 rounded border uppercase mb-1 ${outcomeColor}`}
          >
            {comparison.outcome}
          </span>
        )}
        <p className="font-medium leading-snug">{comparison.title}</p>
        {comparison.compared_against && (
          <p className="text-gray-500 dark:text-gray-400 italic text-[11px]">{comparison.compared_against}</p>
        )}
        <div className="flex gap-2">
          <span className="text-gray-400 dark:text-gray-500 w-16 shrink-0">ID</span>
          <span className="font-mono text-gray-300 dark:text-gray-600 truncate">{comparison.execution_id.slice(0, 12)}…</span>
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
    <div className="absolute right-0 top-0 bottom-0 w-[480px] bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-700 shadow-2xl z-20 flex flex-col overflow-hidden">
      {/* Header */}
      <div className="px-5 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between shrink-0">
        <span className="text-xs font-bold text-gray-600 dark:text-gray-300 uppercase tracking-wider">
          Execution Comparison
        </span>
        <button
          onClick={onClose}
          className="text-gray-400 dark:text-gray-500 hover:text-gray-700 dark:hover:text-gray-200 text-xl leading-none w-7 h-7 flex items-center justify-center rounded hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
        >
          ×
        </button>
      </div>

      {/* Search form */}
      <div className="px-5 py-3 border-b border-gray-100 dark:border-gray-800 shrink-0 space-y-2">
        <label className="text-[10px] font-bold text-gray-400 dark:text-gray-500 uppercase tracking-wider block">
          Historical Execution ID
        </label>
        <div className="flex gap-2">
          <input
            type="text"
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') handleFetch() }}
            placeholder="Paste execution UUID…"
            className="flex-1 border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-1.5 text-xs font-mono focus:outline-none focus:ring-2 focus:ring-blue-400 bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100"
          />
          <button
            onClick={handleFetch}
            disabled={loading || !inputValue.trim()}
            className="px-4 py-1.5 bg-blue-600 text-white text-xs font-medium rounded-lg disabled:opacity-50 hover:bg-blue-700 transition-colors"
          >
            {loading ? '…' : 'Compare'}
          </button>
        </div>
        {error && (
          <p className="text-xs text-red-600 dark:text-red-400">{error}</p>
        )}
      </div>

      {/* Comparison body */}
      <div className="flex-1 overflow-y-auto">
        {currentSummary && comparison ? (
          <div className="p-5 space-y-5">
            {/* Split view */}
            <div className="flex gap-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700">
              <CurrentSide summary={currentSummary} />
              <div className="w-px bg-gray-200 dark:bg-gray-700 shrink-0 self-stretch" />
              <HistoricalSide comparison={comparison} />
            </div>

            {/* Summary */}
            {comparison.summary && (
              <div className="space-y-1">
                <span className="text-[10px] font-bold text-gray-400 dark:text-gray-500 uppercase tracking-wider">
                  Summary
                </span>
                <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed bg-gray-50 dark:bg-gray-800 p-3 rounded-lg border border-gray-100 dark:border-gray-700">
                  {comparison.summary}
                </p>
              </div>
            )}

            {/* Highlights */}
            {(comparison.highlights ?? []).length > 0 && (
              <div className="space-y-2">
                <span className="text-[10px] font-bold text-gray-400 dark:text-gray-500 uppercase tracking-wider">
                  Highlights
                </span>
                <ul className="space-y-1">
                  {comparison.highlights!.map((hl, i) => (
                    <li key={i} className="flex gap-2 text-sm text-gray-700 dark:text-gray-300">
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
          <div className="flex flex-col items-center justify-center h-full text-gray-400 dark:text-gray-500 gap-2 px-8 text-center">
            <p className="text-sm">Enter a historical execution ID above to compare it against the current run.</p>
          </div>
        )}
      </div>
    </div>
  )
}
