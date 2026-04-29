// M4.3 — Horizontal strip showing relevant historical memory recalls for
// the current episode. Sits below the TriggerBanner in the dossier header
// area, outside the three-column workspace.
//
// Data source: GET /executions/:execId/episodes/:episodeId/memory-recalls
// Returns MemoryRecallListView { items, implementation_note }.
//
// Behaviour:
//   • Renders nothing while loading, on error, or when items is empty.
//   • Labels the strip as "相关历史经验参考" (related historical reference)
//     because current recall is keyword-overlap, not semantic search.
//   • Each chip shows matched_pattern + confidence badge. Caution, if
//     present, is shown as a subtle ⚠ indicator with a tooltip.
import { useEffect, useState } from 'react'
import { getMemoryRecalls } from '@/api/episodes'
import type { MemoryRecallView } from '@/types/workspace'

// ─── Confidence badge ──────────────────────────────────────────────────────

function ConfidenceBadge({ confidence }: { confidence?: string }) {
  if (!confidence) return null
  const style =
    confidence === 'high'
      ? 'bg-green-100 text-green-700'
      : confidence === 'medium'
      ? 'bg-yellow-100 text-yellow-700'
      : 'bg-red-100 text-red-700'
  return (
    <span className={`text-[9px] font-bold px-1 py-0.5 rounded uppercase shrink-0 ${style}`}>
      {confidence}
    </span>
  )
}

// ─── Single recall chip ────────────────────────────────────────────────────

function RecallChip({ item }: { item: MemoryRecallView }) {
  return (
    <div
      title={[item.summary, item.caution ? `⚠ ${item.caution}` : null]
        .filter(Boolean)
        .join('\n\n')}
      className="flex items-center gap-1.5 px-2.5 py-1 bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-full text-xs text-gray-700 dark:text-gray-200 shrink-0 cursor-default hover:border-blue-200 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
    >
      {item.caution && (
        <span className="text-amber-500 text-[11px] shrink-0" title={item.caution}>
          ⚠
        </span>
      )}
      <span className="font-mono text-gray-600 dark:text-gray-300 max-w-[180px] truncate">
        {item.matched_pattern ?? item.title}
      </span>
      <ConfidenceBadge confidence={item.confidence} />
      {/* M5.2 — show originating execution id as a dim monospace tag */}
      {item.source_execution_id && (
        <span
          className="text-[9px] font-mono text-gray-300 dark:text-gray-600 shrink-0"
          title={`Source execution: ${item.source_execution_id}`}
        >
          ↩{item.source_execution_id.slice(0, 8)}
        </span>
      )}
    </div>
  )
}

// ─── Inset strip ──────────────────────────────────────────────────────────

interface MemoryRecallInsetProps {
  execId: string
  episodeId: string
}

export function MemoryRecallInset({ execId, episodeId }: MemoryRecallInsetProps) {
  const [items, setItems] = useState<MemoryRecallView[]>([])
  // CR-012: drive the recall-method label from the backend's implementation_note
  // field rather than hard-coding "keyword-matched".
  const [implNote, setImplNote] = useState<string>('keyword-matched')

  useEffect(() => {
    let cancelled = false
    setItems([])
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

  return (
    <div className="px-5 py-2 bg-gray-50 dark:bg-gray-800 border-b border-gray-100 dark:border-gray-700 shrink-0 flex items-center gap-3 overflow-hidden">
      {/* Label */}
      <span className="text-[10px] font-bold text-gray-400 dark:text-gray-500 uppercase tracking-wider shrink-0 whitespace-nowrap">
        Related History
      </span>
      <span className="text-gray-200 dark:text-gray-600 shrink-0">|</span>

      {/* Horizontally scrollable chip row */}
      <div className="flex items-center gap-2 overflow-x-auto flex-1 min-w-0 scrollbar-none">
        {items.map((item) => (
          <RecallChip key={item.id} item={item} />
        ))}
      </div>

      {/* Right-aligned: note about recall method — driven by backend implementation_note */}
      <span className="text-[9px] text-gray-300 dark:text-gray-600 italic shrink-0 whitespace-nowrap">
        {implNote}
      </span>
    </div>
  )
}
