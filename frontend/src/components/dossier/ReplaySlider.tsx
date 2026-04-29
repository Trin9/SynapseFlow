// M3.4 — Replay slider bar shown above the three-pane dossier layout.
// Appears only when a dossier is loaded; checkpoint label shows on the right.
import type { EpisodeDossierView, ReplaySliceView } from '@/types/workspace'

interface ReplaySliderProps {
  dossier: EpisodeDossierView | null
  replayPercent: number
  onPercentChange: (n: number) => void
  replaySlice: ReplaySliceView | null
}

export function ReplaySlider({
  dossier,
  replayPercent,
  onPercentChange,
  replaySlice,
}: ReplaySliderProps) {
  if (!dossier) return null

  return (
    <div className="px-5 py-2 border-b border-gray-100 dark:border-gray-700 shrink-0 flex items-center gap-3 bg-white dark:bg-gray-900">
      <span className="text-[10px] font-bold text-gray-400 dark:text-gray-500 uppercase tracking-wider shrink-0">
        Replay
      </span>
      <input
        type="range"
        min={0}
        max={100}
        value={replayPercent}
        onChange={(e) => onPercentChange(Number(e.target.value))}
        className="flex-1 accent-blue-500 h-1.5 cursor-pointer"
      />
      <span className="text-[11px] font-mono text-gray-400 dark:text-gray-500 shrink-0 w-8 text-right">
        {replayPercent}%
      </span>
      {replaySlice && (
        <span className="text-[11px] text-gray-500 dark:text-gray-400 shrink-0 max-w-xs truncate font-medium">
          {replaySlice.checkpoint.label}: {replaySlice.checkpoint.headline}
        </span>
      )}
    </div>
  )
}
