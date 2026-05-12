// Process Trace Tray: persistent bottom panel in WorkbenchLayout.
// Shows replay slider + trace steps + handles + memory recalls for the focused episode.
import { ChevronDown, ChevronUp, FileText, Database, GitBranch } from 'lucide-react'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getEpisodeReplay, getMemoryRecalls, getEpisode } from '@/api/episodes'
import { useGraphStore } from '@/hooks/useGraphStore'
import { Button } from '@/components/ui/button'
import type { ProcessTraceEntryView, MemoryRecallView } from '@/types/workspace'

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

// ─── Memory recall chip (reuses style from MemoryRecallInset) ─────────────

function RecallChip({ item }: { item: MemoryRecallView }) {
  return (
    <div className="flex items-center gap-1.5 px-2.5 py-1 border rounded-full text-xs shrink-0 bg-card border-border text-foreground">
      <span className="font-mono text-muted-foreground max-w-[140px] truncate">
        {item.matched_pattern ?? item.title}
      </span>
      {item.confidence && (
        <span className={`text-[9px] font-bold px-1 py-0.5 rounded ${
          item.confidence === 'high' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' :
          item.confidence === 'medium' ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' :
          'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
        }`}>
          {item.confidence}
        </span>
      )}
    </div>
  )
}

// ─── Tray ──────────────────────────────────────────────────────────────────

export function ProcessTraceTray() {
  const focusedEpisodeId = useGraphStore((s) => s.focusedEpisodeId)
  const activeExecutionId = useGraphStore((s) => s.activeExecutionId)
  const replayPercent = useGraphStore((s) => s.replayPercent)
  const setReplayPercent = useGraphStore((s) => s.setReplayPercent)
  const setSelectedEpisode = useGraphStore((s) => s.setSelectedEpisode)

  const [collapsed, setCollapsed] = useState(false)
  const [showRecalls, setShowRecalls] = useState(false)

  const { data: replaySlice } = useQuery({
    queryKey: ['process-trace-tray', activeExecutionId, focusedEpisodeId, replayPercent],
    queryFn: () => getEpisodeReplay(activeExecutionId!, focusedEpisodeId!, replayPercent),
    enabled: !!activeExecutionId && !!focusedEpisodeId,
    staleTime: 10_000,
  })

  const { data: memoryRecalls } = useQuery({
    queryKey: ['process-trace-recalls', activeExecutionId, focusedEpisodeId],
    queryFn: () => getMemoryRecalls(activeExecutionId!, focusedEpisodeId!),
    enabled: !!activeExecutionId && !!focusedEpisodeId && showRecalls,
    staleTime: 30_000,
  })

  const visibleTrace = replaySlice?.visible_process_trace ?? []
  const handles = replaySlice?.visible_handles ?? []
  const recalls = memoryRecalls?.items ?? []

  if (!activeExecutionId || !focusedEpisodeId) return null
  if (visibleTrace.length === 0 && replaySlice === undefined) return null

  // Step status breakdown
  const successCount = visibleTrace.filter((s) => s.status === 'success').length
  const failedCount = visibleTrace.filter((s) => s.status === 'failed').length
  const runningCount = visibleTrace.filter((s) => s.status === 'running').length

  async function handleOpenDossier() {
    if (!focusedEpisodeId) return
    try {
      const episode = await getEpisode(focusedEpisodeId)
      setSelectedEpisode(episode)
    } catch { /* silently fail */ }
  }

  return (
    <div className="border-t border-border bg-muted/30 dark:bg-muted/10 shrink-0 wb-animate-slide-up">
      {/* Header bar (always visible) */}
      <div className="flex items-center gap-3 px-4 h-8 border-b border-border shrink-0">
        <span className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider">
          Process Trace
        </span>
        <span className="text-[10px] text-muted-foreground/60 shrink-0">
          {visibleTrace.length} step{visibleTrace.length !== 1 ? 's' : ''}
        </span>

        {/* Step status mini-summary */}
        {successCount > 0 && (
          <span className="text-[10px] text-green-600 dark:text-green-400 shrink-0">{successCount}✓</span>
        )}
        {failedCount > 0 && (
          <span className="text-[10px] text-red-600 dark:text-red-400 shrink-0">{failedCount}✗</span>
        )}
        {runningCount > 0 && (
          <span className="text-[10px] text-blue-600 dark:text-blue-400 shrink-0">{runningCount}…</span>
        )}

        {/* Compact replay slider */}
        <div className="flex-1 flex items-center gap-2 px-2 min-w-0">
          <input
            type="range"
            min={0}
            max={100}
            value={replayPercent}
            onChange={(e) => setReplayPercent(Number(e.target.value))}
            className="flex-1 accent-blue-500 h-1 cursor-pointer min-w-[60px]"
            title={`Replay: ${replayPercent}%`}
          />
          <span className="text-[10px] font-mono text-muted-foreground w-7 text-right shrink-0">
            {replayPercent}%
          </span>
        </div>

        {replaySlice?.checkpoint.headline && (
          <span className="text-[11px] text-muted-foreground truncate max-w-[200px] hidden sm:block">
            {replaySlice.checkpoint.headline}
          </span>
        )}

        {/* Memory recalls toggle */}
        {(recalls.length > 0 || showRecalls) && (
          <button
            onClick={() => setShowRecalls((v) => !v)}
            className={`text-[10px] px-2 py-0.5 rounded border transition-colors shrink-0 ${
              showRecalls
                ? 'bg-purple-100 text-purple-700 border-purple-200 dark:bg-purple-900/30 dark:text-purple-300 dark:border-purple-700'
                : 'text-muted-foreground border-border hover:bg-accent'
            }`}
          >
            {recalls.length > 0 ? `${recalls.length} memory` : 'Memory'}
          </button>
        )}

        <button
          onClick={() => setCollapsed((c) => !c)}
          className="text-muted-foreground hover:text-foreground transition-colors shrink-0"
          title={collapsed ? 'Expand trace' : 'Collapse trace'}
        >
          {collapsed ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
        </button>
      </div>

      {/* Content (collapsible) */}
      {!collapsed && (
        <>
          {/* Trace steps */}
          {visibleTrace.length > 0 && (
            <div className="px-4 py-2 flex items-center gap-2 overflow-x-auto scrollbar-thin border-b border-border/50">
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

          {/* Metadata bar: handles + dossier button */}
          <div className="px-4 py-1.5 flex items-center gap-3 border-b border-border/50">
            {/* Handles chips */}
            {(handles as { type: string; value: string }[]).length > 0 && (
              <div className="flex items-center gap-1.5 flex-1 min-w-0 overflow-x-auto scrollbar-thin">
                <GitBranch className="w-3 h-3 text-muted-foreground shrink-0" />
                {(handles as { type: string; value: string }[]).map((h, i) => (
                  <span key={i} className="text-[10px] font-mono bg-muted rounded px-1.5 py-0.5 border text-muted-foreground shrink-0 truncate max-w-[140px]">
                    {h.type}:{h.value.slice(0, 16)}
                  </span>
                ))}
              </div>
            )}

            {/* Dossier quick-open */}
            <Button
              size="xs"
              variant="ghost"
              onClick={handleOpenDossier}
              className="shrink-0 text-[10px] text-blue-500 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
            >
              <FileText className="w-3 h-3" />
              Open Dossier
            </Button>
          </div>

          {/* Memory recalls panel (expandable) */}
          {showRecalls && recalls.length > 0 && (
            <div className="px-4 py-2 flex items-center gap-2 overflow-x-auto scrollbar-thin">
              <Database className="w-3 h-3 text-muted-foreground shrink-0" />
              {recalls.map((item) => (
                <RecallChip key={item.id} item={item} />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}
