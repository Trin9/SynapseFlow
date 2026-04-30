// M4.3 — Horizontal strip showing relevant historical memory recalls for
// the current episode. Sits below the TriggerBanner in the dossier header
// area, outside the three-column workspace.
//
// Data source: GET /executions/:execId/episodes/:episodeId/memory-recalls
// Returns MemoryRecallListView { items, implementation_note }.
//
// Phase D enhancements:
//   • Click a chip to expand its full summary inline below the strip.
//   • "Jump" button on expanded chip sets activeExecutionId + switches to REVIEW.
import { useEffect, useState } from 'react'
import { ArrowRight } from 'lucide-react'
import { getMemoryRecalls } from '@/api/episodes'
import { useGraphStore } from '@/hooks/useGraphStore'
import type { MemoryRecallView } from '@/types/workspace'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

// ─── Confidence badge ──────────────────────────────────────────────────────

function ConfidenceBadge({ confidence }: { confidence?: string }) {
  if (!confidence) return null
  const variant =
    confidence === 'high' ? 'success' as const :
    confidence === 'medium' ? 'warning' as const :
    'destructive' as const
  return <Badge variant={variant} className="text-[9px] uppercase">{confidence}</Badge>
}

// ─── Single recall chip ────────────────────────────────────────────────────

interface RecallChipProps {
  item: MemoryRecallView
  isExpanded: boolean
  onToggle: () => void
}

function RecallChip({ item, isExpanded, onToggle }: RecallChipProps) {
  return (
    <button
      onClick={onToggle}
      className={cn(
        'flex items-center gap-1.5 px-2.5 py-1 border rounded-full text-xs shrink-0 transition-colors text-left',
        isExpanded
          ? 'bg-blue-50 border-blue-200 text-blue-700 dark:bg-blue-900/30 dark:border-blue-700 dark:text-blue-300'
          : 'bg-card border-border text-foreground hover:border-blue-200 hover:bg-blue-50 dark:hover:bg-blue-900/20'
      )}
    >
      {item.caution && <span className="text-amber-500 text-[11px] shrink-0">⚠</span>}
      <span className="font-mono text-muted-foreground max-w-[180px] truncate">
        {item.matched_pattern ?? item.title}
      </span>
      <ConfidenceBadge confidence={item.confidence} />
      {item.source_execution_id && (
        <span className="text-[9px] font-mono text-muted-foreground/50 shrink-0">
          ↩{item.source_execution_id.slice(0, 8)}
        </span>
      )}
    </button>
  )
}

// ─── Inset strip ──────────────────────────────────────────────────────────

interface MemoryRecallInsetProps {
  execId: string
  episodeId: string
}

export function MemoryRecallInset({ execId, episodeId }: MemoryRecallInsetProps) {
  const [items, setItems] = useState<MemoryRecallView[]>([])
  const [implNote, setImplNote] = useState<string>('keyword-matched')
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const setActiveExecutionId = useGraphStore((s) => s.setActiveExecutionId)
  const setAppMode = useGraphStore((s) => s.setAppMode)
  const setSelectedEpisode = useGraphStore((s) => s.setSelectedEpisode)

  useEffect(() => {
    let cancelled = false
    setItems([])
    setExpandedId(null)
    getMemoryRecalls(execId, episodeId)
      .then((list) => {
        if (!cancelled) {
          setItems(list.items)
          setImplNote(list.implementation_note || 'keyword-matched')
        }
      })
      .catch(() => { /* graceful — show nothing on error */ })
    return () => { cancelled = true }
  }, [execId, episodeId])

  if (items.length === 0) return null

  const expanded = expandedId ? items.find((i) => i.id === expandedId) ?? null : null

  function handleJump(sourceExecId: string) {
    setSelectedEpisode(null)
    setActiveExecutionId(sourceExecId)
    setAppMode('REVIEW')
  }

  return (
    <div className="bg-muted/40 border-b shrink-0">
      {/* Chip row */}
      <div className="px-5 py-2 flex items-center gap-3 overflow-hidden">
        <span className="wb-section-header shrink-0 whitespace-nowrap">Related History</span>
        <span className="text-border shrink-0">|</span>
        <div className="flex items-center gap-2 overflow-x-auto flex-1 min-w-0 scrollbar-none">
          {items.map((item) => (
            <RecallChip
              key={item.id}
              item={item}
              isExpanded={expandedId === item.id}
              onToggle={() => setExpandedId((prev) => (prev === item.id ? null : item.id))}
            />
          ))}
        </div>
        <span className="text-[9px] text-muted-foreground/50 italic shrink-0 whitespace-nowrap">
          {implNote}
        </span>
      </div>

      {/* Expanded detail panel */}
      {expanded && (
        <div className="px-5 pb-3 flex items-start gap-4 border-t pt-2">
          <div className="flex-1 min-w-0 space-y-1">
            <p className="text-xs font-semibold text-foreground">
              {expanded.title ?? expanded.matched_pattern}
            </p>
            {expanded.summary && (
              <p className="text-xs text-muted-foreground leading-relaxed">{expanded.summary}</p>
            )}
            {expanded.caution && (
              <p className="text-xs text-amber-600 dark:text-amber-400 flex items-start gap-1">
                <span>⚠</span>
                <span>{expanded.caution}</span>
              </p>
            )}
          </div>
          {expanded.source_execution_id && (
            <Button
              size="xs"
              variant="outline"
              className="shrink-0 text-blue-600 border-blue-200 dark:text-blue-400 dark:border-blue-700"
              onClick={() => handleJump(expanded.source_execution_id!)}
            >
              Jump to execution
              <ArrowRight className="w-3 h-3" />
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
