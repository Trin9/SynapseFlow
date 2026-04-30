/**
 * ExecutionNarrativeBanner — Phase C "narrative connector"
 *
 * Shows execution-level display metadata (run_label, overview_badge,
 * trace_title, trace_summary) between the EpisodeOverviewStrip and canvas.
 * Acts as the "trigger → narrative → trace" context band.
 */
import { useQuery } from '@tanstack/react-query'
import { getExecutionSummaryView } from '@/api/episodes'
import type { ExecutionSummaryView } from '@/types/workspace'

interface ExecutionNarrativeBannerProps {
  executionId: string
}

const STATUS_COLORS: Record<string, string> = {
  completed:   'bg-green-50 border-green-200 text-green-700 dark:bg-green-900/20 dark:border-green-700 dark:text-green-300',
  failed:      'bg-red-50 border-red-200 text-red-700 dark:bg-red-900/20 dark:border-red-700 dark:text-red-300',
  running:     'bg-blue-50 border-blue-200 text-blue-700 dark:bg-blue-900/20 dark:border-blue-700 dark:text-blue-300',
  escalated:   'bg-amber-50 border-amber-200 text-amber-700 dark:bg-amber-900/20 dark:border-amber-700 dark:text-amber-300',
}
const DEFAULT_STATUS = 'bg-slate-50 border-slate-200 text-slate-500 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-400'

export function ExecutionNarrativeBanner({ executionId }: ExecutionNarrativeBannerProps) {
  const { data } = useQuery<ExecutionSummaryView>({
    queryKey: ['execution-summary', executionId],
    queryFn: () => getExecutionSummaryView(executionId),
    enabled: !!executionId,
    staleTime: 30_000,
  })

  if (!data) return null

  const { display, status, workflow_kind, dag_name } = data
  const statusColor = STATUS_COLORS[status] ?? DEFAULT_STATUS

  const hasMeta = display.run_label || display.overview_badge || display.trace_title || display.trace_summary

  if (!hasMeta) return null

  return (
    <div className="flex items-start gap-3 px-4 py-1.5 border-b border-slate-100 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 shrink-0 text-xs wb-animate-slide-up">
      {/* Left: dag / kind / status */}
      <div className="flex items-center gap-1.5 shrink-0 pt-0.5">
        <span className="font-medium text-slate-500 dark:text-slate-400 truncate max-w-[100px]">
          {dag_name}
        </span>
        {workflow_kind && (
          <span className="px-1.5 py-0.5 rounded border text-[10px] font-medium border-slate-200 dark:border-slate-700 text-slate-400 dark:text-slate-500 capitalize">
            {workflow_kind}
          </span>
        )}
        <span className={`px-1.5 py-0.5 rounded border text-[10px] font-medium ${statusColor}`}>
          {status}
        </span>
      </div>

      {/* Divider */}
      <span className="text-slate-200 dark:text-slate-700 pt-0.5 shrink-0">|</span>

      {/* Center: narrative text */}
      <div className="flex-1 min-w-0 flex flex-col gap-0.5">
        {display.trace_title && (
          <span className="font-medium text-slate-700 dark:text-slate-200 truncate">
            {display.trace_title}
          </span>
        )}
        {display.trace_summary && (
          <span className="text-slate-500 dark:text-slate-400 line-clamp-1">
            {display.trace_summary}
          </span>
        )}
      </div>

      {/* Right: overview badge + run label */}
      <div className="flex items-center gap-2 shrink-0 pt-0.5">
        {display.overview_badge && (
          <span className="px-2 py-0.5 rounded border border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-700 dark:bg-violet-900/30 dark:text-violet-300 text-[10px] font-semibold">
            {display.overview_badge}
          </span>
        )}
        {display.run_label && (
          <span className="text-[10px] font-mono text-slate-400 dark:text-slate-500">
            {display.run_label}
          </span>
        )}
      </div>
    </div>
  )
}
