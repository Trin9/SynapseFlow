import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { FileText, Eye, Database, GitBranch, Loader2, CheckCircle, XCircle, AlertTriangle, Circle } from 'lucide-react'
import { getEpisode, listEpisodeSummariesByExecution } from '@/api/episodes'
import { useGraphStore } from '@/hooks/useGraphStore'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import type { EpisodeSummaryView } from '@/types/workspace'

// ─── Verdict visual map (matches Demo's SuperNode badge colours) ─────────────

type VerdictTone = 'pass' | 'fail' | 'inconclusive' | 'open'

const VERDICT_COLORS: Record<VerdictTone, { left: string; badge: string; icon: React.ReactNode; label: string }> = {
  pass: {
    left: 'bg-emerald-500 dark:bg-emerald-600',
    badge: 'bg-emerald-100 text-emerald-700 border-emerald-200 dark:bg-emerald-900/30 dark:text-emerald-300 dark:border-emerald-700',
    icon: <CheckCircle className="w-3 h-3" />,
    label: 'PASS',
  },
  fail: {
    left: 'bg-red-500 dark:bg-red-600',
    badge: 'bg-red-100 text-red-700 border-red-200 dark:bg-red-900/30 dark:text-red-300 dark:border-red-700',
    icon: <XCircle className="w-3 h-3" />,
    label: 'FAIL',
  },
  inconclusive: {
    left: 'bg-amber-500 dark:bg-amber-600',
    badge: 'bg-amber-100 text-amber-700 border-amber-200 dark:bg-amber-900/30 dark:text-amber-300 dark:border-amber-700',
    icon: <AlertTriangle className="w-3 h-3" />,
    label: 'INCONCLUSIVE',
  },
  open: {
    left: 'bg-gray-300 dark:bg-gray-600',
    badge: 'bg-gray-100 text-gray-500 border-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:border-gray-700',
    icon: <Circle className="w-3 h-3" />,
    label: 'OPEN',
  },
}

function resolveVerdictTone(summary: EpisodeSummaryView): VerdictTone {
  const v = summary.display.verdict
  if (v === 'pass') return 'pass'
  if (v === 'fail') return 'fail'
  if (v === 'inconclusive') return 'inconclusive'
  return 'open'
}

interface EpisodeOverviewCardProps {
  summary: EpisodeSummaryView
  onOpenDossier: (episodeId: string) => Promise<void>
  busy: boolean
}

export function EpisodeOverviewCard({ summary, onOpenDossier, busy }: EpisodeOverviewCardProps) {
  const tone = resolveVerdictTone(summary)
  const v = VERDICT_COLORS[tone]
  const verdictDisplay = v.icon
  const verdictLabel = summary.display.verdict_label ?? v.label
  const confidenceDisplay =
    summary.confidence != null
      ? typeof summary.confidence === 'number'
        ? `${Math.round(summary.confidence * 100)}%`
        : summary.confidence
      : null

  return (
    <div className={cn(
      'w-[270px] shrink-0 rounded-lg border shadow-sm hover:shadow-md transition-shadow',
      'bg-card text-card-foreground overflow-hidden flex',
    )}>
      {/* Left verdict colour stripe */}
      <div className={cn('w-[5px] shrink-0', v.left)} />

      <div className="flex-1 min-w-0 p-3 space-y-2">
        {/* Top row: episode type label + verdict badge */}
        <div className="flex items-center gap-1.5 flex-wrap">
          <span className={cn(
            'text-[11px] font-semibold text-muted-foreground truncate max-w-[100px]',
          )}>
            {summary.label}
          </span>
          <span className={cn(
            'ml-auto inline-flex items-center gap-1 text-[10px] font-bold px-2 py-0.5 rounded border uppercase shrink-0',
            v.badge,
          )}>
            {verdictDisplay}
            {verdictLabel}
          </span>
        </div>

        {/* Summary line */}
        <p className="text-[11px] text-muted-foreground line-clamp-2 min-h-[2.4em] leading-relaxed">
          {summary.display.summary ?? 'No summary available yet.'}
        </p>

        {/* Stat row: evidence + handles + confidence */}
        <div className="text-[10px] text-muted-foreground flex items-center gap-2">
          <span className="flex items-center gap-0.5">
            <Database className="w-2.5 h-2.5" />
            {summary.evidence_count}
          </span>
          <span>·</span>
          <span className="flex items-center gap-0.5">
            <GitBranch className="w-2.5 h-2.5" />
            {summary.handle_count}
          </span>
          {confidenceDisplay && (
            <>
              <span>·</span>
              <span className={cn(
                'font-semibold',
                tone === 'pass' && 'text-emerald-600 dark:text-emerald-400',
                tone === 'fail' && 'text-red-600 dark:text-red-400',
                tone === 'inconclusive' && 'text-amber-600 dark:text-amber-400',
              )}>
                {confidenceDisplay}
              </span>
            </>
          )}
        </div>

        {/* Action buttons */}
        <div className="flex items-center gap-1.5 pt-1">
          <Button
            size="xs"
            variant={tone === 'pass' ? 'default' : 'outline'}
            className={cn(
              'flex-1 h-7 text-[11px]',
              tone === 'pass' && 'bg-emerald-600 hover:bg-emerald-500',
            )}
            onClick={() => void onOpenDossier(summary.episode_id)}
            disabled={busy}
          >
            {busy ? <Loader2 className="w-3 h-3 animate-spin" /> : <FileText className="w-3 h-3" />}
            Open Dossier
          </Button>
          <Button
            size="xs"
            variant="ghost"
            className="flex-1 h-7 text-[11px] border border-dashed border-border"
            disabled
            title="View Inside: wired after episode-to-node mapping is finalized"
          >
            <Eye className="w-3 h-3" />
            Inside
          </Button>
        </div>
      </div>
    </div>
  )
}

export function EpisodeOverviewStrip({ executionId }: { executionId: string }) {
  const setSelectedEpisode = useGraphStore((s) => s.setSelectedEpisode)
  const [openingEpisodeId, setOpeningEpisodeId] = useState<string | null>(null)

  const { data: summaries = [], isLoading, error } = useQuery({
    queryKey: ['episode-overview', executionId],
    queryFn: () => listEpisodeSummariesByExecution(executionId),
    enabled: !!executionId,
  })

  const sorted = useMemo(
    () => [...summaries].sort((a, b) => a.label.localeCompare(b.label)),
    [summaries],
  )

  async function handleOpenDossier(episodeId: string) {
    setOpeningEpisodeId(episodeId)
    try {
      const episode = await getEpisode(episodeId)
      setSelectedEpisode(episode)
    } finally {
      setOpeningEpisodeId(null)
    }
  }

  if (isLoading) {
    return (
      <div className="border-b px-3 py-2 flex gap-2 items-start">
        <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider shrink-0 self-center">Episodes</span>
        {[1,2,3].map(i => <Skeleton key={i} className="w-[270px] h-[130px] shrink-0 rounded-lg" />)}
      </div>
    )
  }

  if (error) {
    return (
      <div className="px-3 py-2 text-xs text-destructive bg-destructive/10 border-b border-destructive/20 flex items-center gap-2">
        <AlertTriangle className="w-3 h-3" />
        Failed to load episode overview.
      </div>
    )
  }

  if (sorted.length === 0) {
    return (
      <div className="px-3 py-2 text-xs text-muted-foreground border-b flex items-center gap-2">
        <Circle className="w-3 h-3" />
        No episode summary found for this execution.
      </div>
    )
  }

  return (
    <div className="border-b bg-muted/30 dark:bg-muted/10 px-3 py-2 shrink-0 wb-animate-slide-up">
      <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
        Episode Overview
      </div>
      <div className="flex gap-2 overflow-x-auto pb-2 scrollbar-thin">
        {sorted.map((summary) => (
          <EpisodeOverviewCard
            key={summary.episode_id}
            summary={summary}
            busy={openingEpisodeId === summary.episode_id}
            onOpenDossier={handleOpenDossier}
          />
        ))}
      </div>
    </div>
  )
}
