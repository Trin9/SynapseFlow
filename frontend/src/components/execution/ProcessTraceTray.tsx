// Batch 2 — Process Trace Tray: persistent bottom panel in WorkbenchLayout.
// Shows replay slider + trace steps for the focused episode, outside the Dossier.
// Fetches data via getEpisodeReplay API, synced with the global replayPercent.
import { ChevronDown, ChevronUp } from 'lucide-react'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getEpisodeReplay } from '@/api/episodes'
import { useGraphStore } from '@/hooks/useGraphStore'
import type { ProcessTraceEntryView } from '@/types/workspace'

// ─── Status dot ───────────────────────────────────────────────────────────

const STATUS_DOT: Record<string, string> = {
  success: 'bg-green-500',
  failed: 'bg-red-500',
  running: 'bg-blue-500 animate-pulse',
  pending: 'bg-gray-300 dark:bg-gray-600',
}

// ─── Stage pill ───────────────────────────────────────────────────────────

const STAGE_PILL: Record<string, string> = {
  'Human Review':    'bg-amber-100 text-amber-700 border-amber-200 dark:bg-amber-900/30 dark:text-amber-300 dark:border-amber-700',
  'Circuit Breaker': 'bg-red-100 text-red-700 border-red-200 dark:bg-red-900/30 dark:text-red-300 dark:border-red-700',
  'Verdict':         'bg-violet-100 text-violet-700 border-violet-200 dark:bg-violet-900/30 dark:text-violet-300 dark:border-violet-700',
  'Action':          'bg-blue-100 text-blue-700 border-blue-200 dark:bg-blue-900/30 dark:text-blue-300 dark:border-blue-700',
}
const DEFAULT_STAGE_PILL = 'bg-gray-100 text-gray-600 border-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600'

// ─── Compact step card ─────────────────────────────────────────────────────

function StepChip({ entry }: { entry: ProcessTraceEntryView }) {
  const dot = STATUS_DOT[entry.status] ?? STATUS_DOT.pending
  const stagePill = STAGE_PILL[entry.stage] ?? DEFAULT_STAGE_PILL

  return (
    <div
      title={entry.detail ?? entry.title}
      className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg border shrink-0
                 bg-card text-card-foreground border-border cursor-default select-none
                 transition-all max-w-[180px]"
    >
      <span className={`w-2 h-2 rounded-full shrink-0 ${dot}`} />
      <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded border uppercase ${stagePill}`}>
        {entry.stage}
      </span>
      <span className="text-[11px] font-medium truncate text-foreground">
        {entry.title}
      </span>
    </div>
  )
}

// ─── Tray ──────────────────────────────────────────────────────────────────

export function ProcessTraceTray() {
  const focusedEpisodeId = useGraphStore((s) => s.focusedEpisodeId)
  const activeExecutionId = useGraphStore((s) => s.activeExecutionId)
  const replayPercent = useGraphStore((s) => s.replayPercent)
  const setReplayPercent = useGraphStore((s) => s.setReplayPercent)

  const [collapsed, setCollapsed] = useState(false)

  const { data: replaySlice } = useQuery({
    queryKey: ['process-trace-tray', activeExecutionId, focusedEpisodeId, replayPercent],
    queryFn: () => getEpisodeReplay(activeExecutionId!, focusedEpisodeId!, replayPercent),
    enabled: !!activeExecutionId && !!focusedEpisodeId,
    staleTime: 10_000,
  })

  const visibleTrace = replaySlice?.visible_process_trace ?? []

  if (!activeExecutionId || !focusedEpisodeId) return null
  if (visibleTrace.length === 0 && replaySlice === undefined) return null

  return (
    <div className="border-t border-border bg-muted/30 dark:bg-muted/10 shrink-0 wb-animate-slide-up">
      {/* Header bar (always visible) */}
      <div className="flex items-center gap-3 px-4 h-8 border-b border-border shrink-0">
        <span className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider">
          Process Trace
        </span>
        <span className="text-[10px] text-muted-foreground/60">
          {visibleTrace.length} step{visibleTrace.length !== 1 ? 's' : ''}
        </span>

        {/* Compact replay slider */}
        <div className="flex-1 flex items-center gap-2 px-2">
          <input
            type="range"
            min={0}
            max={100}
            value={replayPercent}
            onChange={(e) => setReplayPercent(Number(e.target.value))}
            className="flex-1 accent-blue-500 h-1 cursor-pointer"
            title={`Replay: ${replayPercent}%`}
          />
          <span className="text-[10px] font-mono text-muted-foreground w-7 text-right">
            {replayPercent}%
          </span>
        </div>

        {replaySlice?.checkpoint.headline && (
          <span className="text-[11px] text-muted-foreground truncate max-w-[240px] hidden sm:block">
            {replaySlice.checkpoint.headline}
          </span>
        )}

        <button
          onClick={() => setCollapsed((c) => !c)}
          className="text-muted-foreground hover:text-foreground transition-colors"
          title={collapsed ? 'Expand trace' : 'Collapse trace'}
        >
          {collapsed ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
        </button>
      </div>

      {/* Trace steps (collapsible) */}
      {!collapsed && visibleTrace.length > 0 && (
        <div className="px-4 py-2 flex items-center gap-2 overflow-x-auto scrollbar-thin">
          {visibleTrace.map((entry, i) => (
            <div key={entry.id} className="flex items-center gap-2 shrink-0">
              {i > 0 && (
                <span className="text-border text-xs select-none">→</span>
              )}
              <StepChip entry={entry} />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
