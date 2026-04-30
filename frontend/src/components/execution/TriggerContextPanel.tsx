import { useQuery } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { getTriggerContext } from '@/api/episodes'
import type { TriggerContextView } from '@/types/workspace'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ScrollArea } from '@/components/ui/scroll-area'

interface TriggerContextPanelProps {
  executionId: string
  /** If provided, dims fields whose replay range starts after this percent */
  replayPercent?: number
  onClose?: () => void
}

export function TriggerContextPanel({ executionId, replayPercent, onClose }: TriggerContextPanelProps) {
  const { data, isLoading, isError } = useQuery<TriggerContextView>({
    queryKey: ['trigger-context', executionId],
    queryFn: () => getTriggerContext(executionId),
    enabled: !!executionId,
    staleTime: 60_000,
  })

  return (
    <div className="flex flex-col h-full wb-animate-slide-right bg-card">
      {/* Header */}
      <div className="flex items-center justify-between px-3 h-9 border-b shrink-0">
        <div className="flex items-center gap-2">
          <span className="wb-section-header">Trigger Context</span>
          {data && (
            <Badge variant="warning" className="text-[9px] max-w-[140px] truncate">{data.title}</Badge>
          )}
        </div>
        {onClose && (
          <Button size="xs" variant="ghost" onClick={onClose} className="h-6 w-6 p-0">
            <X className="w-3.5 h-3.5" />
          </Button>
        )}
      </div>

      {/* Body */}
      <ScrollArea className="flex-1 min-h-0">
        <div className="px-3 py-2 space-y-3">
          {isLoading && (
            <div className="space-y-2 pt-2">
              {[1,2,3].map(i => <Skeleton key={i} className="h-8 w-full" />)}
            </div>
          )}

          {isError && (
            <p className="text-xs text-destructive px-1 py-2">Failed to load trigger context.</p>
          )}

          {data && (
            <>
              {/* Summary */}
              {data.summary && (
                <p className="text-xs leading-relaxed text-foreground bg-muted/40 rounded px-2 py-1.5 border">
                  {data.summary}
                </p>
              )}

              {/* Sections */}
              {data.sections.map((section, si) => (
                <div key={si}>
                  <div className="wb-section-header mb-1 px-0.5">{section.title}</div>
                  <div className="rounded border overflow-hidden">
                    {section.fields.map((field, fi) => {
                      const dimmed =
                        replayPercent !== undefined &&
                        field.range[0] > replayPercent
                      return (
                        <div
                          key={fi}
                          className={`flex items-start gap-2 px-2 py-1.5 text-xs border-b last:border-b-0 transition-opacity ${
                            dimmed ? 'opacity-30' : ''
                          } ${fi % 2 === 0 ? 'bg-background' : 'bg-muted/30'}`}
                        >
                          <span className="shrink-0 text-muted-foreground w-[90px] font-medium leading-relaxed truncate">
                            {field.label}
                          </span>
                          <span className="text-foreground break-all leading-relaxed font-mono text-[11px]">
                            {field.value}
                          </span>
                        </div>
                      )
                    })}
                  </div>
                </div>
              ))}
            </>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}
