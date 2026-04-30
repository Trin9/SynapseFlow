import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { FileText, Eye, Database, GitBranch, Loader2 } from 'lucide-react'
import { getEpisode, listEpisodeSummariesByExecution } from '@/api/episodes'
import { useGraphStore } from '@/hooks/useGraphStore'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import type { EpisodeSummaryView } from '@/types/workspace'

function statusVariant(status: string): "success" | "info" | "destructive" | "warning" | "ghost" {
  switch (status) {
    case 'passed': case 'completed': return 'success'
    case 'running': return 'info'
    case 'failed': return 'destructive'
    case 'suspended': return 'warning'
    default: return 'ghost'
  }
}

function verdictVariant(verdict?: string): "success" | "destructive" | "warning" | "outline" {
  switch (verdict) {
    case 'pass': return 'success'
    case 'fail': return 'destructive'
    case 'inconclusive': return 'warning'
    default: return 'outline'
  }
}

interface EpisodeOverviewCardProps {
  summary: EpisodeSummaryView
  onOpenDossier: (episodeId: string) => Promise<void>
  busy: boolean
}

export function EpisodeOverviewCard({ summary, onOpenDossier, busy }: EpisodeOverviewCardProps) {
  const verdictLabel = summary.display.verdict_label ?? summary.display.verdict ?? 'open'

  return (
    <Card className="w-[260px] shrink-0 shadow-sm hover:shadow-md transition-shadow">
      <CardContent className="p-3 space-y-2">
        <div className="flex items-center gap-1.5 flex-wrap">
          <Badge variant={statusVariant(summary.status)} className="text-[10px] uppercase">
            {summary.status}
          </Badge>
          <Badge variant={verdictVariant(summary.display.verdict)} className="text-[10px]">
            {verdictLabel}
          </Badge>
        </div>

        <div className="text-xs font-semibold text-foreground truncate">{summary.label}</div>
        <p className="text-[11px] text-muted-foreground line-clamp-2 min-h-[2.4em]">
          {summary.display.summary ?? 'No summary available yet.'}
        </p>

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
          {summary.confidence && (
            <>
              <span>·</span>
              <span className={cn(
                "font-medium",
                typeof summary.confidence === 'number' && summary.confidence >= 0.8 ? "text-emerald-600 dark:text-emerald-400" :
                typeof summary.confidence === 'number' && summary.confidence < 0.5 ? "text-red-600 dark:text-red-400" : ""
              )}>{typeof summary.confidence === 'number' ? `${Math.round((summary.confidence as number) * 100)}%` : summary.confidence}</span>
            </>
          )}
        </div>

        <div className="flex items-center gap-1.5 pt-1">
          <Button
            size="xs"
            variant="outline"
            className="flex-1 h-7"
            onClick={() => void onOpenDossier(summary.episode_id)}
            disabled={busy}
          >
            {busy ? <Loader2 className="w-3 h-3 animate-spin" /> : <FileText className="w-3 h-3" />}
            Dossier
          </Button>
          <Button
            size="xs"
            variant="ghost"
            className="flex-1 h-7 border border-dashed border-border"
            disabled
            title="View Inside: wired after episode-to-node mapping is finalized"
          >
            <Eye className="w-3 h-3" />
            Inside
          </Button>
        </div>
      </CardContent>
    </Card>
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
        {[1,2,3].map(i => <Skeleton key={i} className="w-[260px] h-[120px] shrink-0 rounded-lg" />)}
      </div>
    )
  }

  if (error) {
    return (
      <div className="px-3 py-2 text-xs text-destructive bg-destructive/10 border-b border-destructive/20">
        Failed to load episode overview.
      </div>
    )
  }

  if (sorted.length === 0) {
    return (
      <div className="px-3 py-2 text-xs text-muted-foreground border-b">
        No episode summary found for this execution.
      </div>
    )
  }

  return (
    <div className="border-b bg-muted/30 px-3 py-2 shrink-0 wb-animate-slide-up">
      <div className="wb-section-header mb-2">Episode Overview</div>
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
