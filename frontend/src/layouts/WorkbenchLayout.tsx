import { useEffect, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X, BookOpen, History } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { Canvas } from '@/components/Canvas'
import { Sidebar } from '@/components/Sidebar'
import { ConfigPanel } from '@/components/ConfigPanel'
import { ExecutionHistory } from '@/components/ExecutionHistory'
import { WorkflowLibrary } from '@/components/library/WorkflowLibrary'
import { ForensicDossierDrawer } from '@/components/dossier/ForensicDossierDrawer'
import { listExecutionSummaries, listExecutionSummariesByDAG } from '@/api/episodes'
import { useGraphStore } from '@/hooks/useGraphStore'
import { EpisodeOverviewStrip } from '@/components/execution/EpisodeOverviewCard'
import { TriggerContextPanel } from '@/components/execution/TriggerContextPanel'
import { ExecutionNarrativeBanner } from '@/components/execution/ExecutionNarrativeBanner'
import { ProcessTraceTray } from '@/components/execution/ProcessTraceTray'
import { Button } from '@/components/ui/button'

export function WorkbenchLayout() {
  const appMode = useGraphStore((s) => s.appMode)
  const activeExecutionId = useGraphStore((s) => s.activeExecutionId)
  const selectedEpisode = useGraphStore((s) => s.selectedEpisode)
  const selectedNodeId = useGraphStore((s) => s.selectedNodeId)
  const showHistory = useGraphStore((s) => s.showHistory)
  const setShowHistory = useGraphStore((s) => s.setShowHistory)
  const showLibrary = useGraphStore((s) => s.showLibrary)
  const setShowLibrary = useGraphStore((s) => s.setShowLibrary)
  const setActiveExecutionId = useGraphStore((s) => s.setActiveExecutionId)
  const useWorkbenchLayout = useGraphStore((s) => s.useWorkbenchLayout)
  const showTriggerCtx = useGraphStore((s) => s.showTriggerCtx)
  const setShowTriggerCtx = useGraphStore((s) => s.setShowTriggerCtx)

  const isReview = appMode === 'REVIEW'
  const showRightHistory = showHistory
  const wasReviewRef = useRef(isReview)

  useEffect(() => {
    const justEnteredReview = !wasReviewRef.current && isReview
    if (justEnteredReview && !showHistory) {
      setShowHistory(true)
    }
    wasReviewRef.current = isReview
  }, [isReview, setShowHistory, showHistory])

  const { data: bootstrapExecutions = [] } = useQuery({
    queryKey: ['review-bootstrap-executions'],
    enabled: useWorkbenchLayout && isReview && !activeExecutionId,
    queryFn: async () => {
      const preferred = await listExecutionSummariesByDAG('boutique_checkout_consistency_audit')
      if (preferred.length > 0) return preferred
      return listExecutionSummaries()
    },
  })

  useEffect(() => {
    if (!useWorkbenchLayout || !isReview || activeExecutionId) return
    const first = bootstrapExecutions[0]
    if (!first) return
    setActiveExecutionId(first.execution_id)
  }, [activeExecutionId, bootstrapExecutions, isReview, setActiveExecutionId, useWorkbenchLayout])

  return (
    <div className="flex-1 min-h-0 flex flex-col bg-background">
      <div className="flex-1 min-h-0 flex overflow-hidden relative bg-background">
        {!showLibrary && !isReview && (
          <div className="w-[260px] border-r shrink-0 bg-card">
            <Sidebar />
          </div>
        )}

        {isReview && activeExecutionId && showTriggerCtx && (
          <div className="w-[260px] shrink-0 border-r bg-card">
            <TriggerContextPanel
              executionId={activeExecutionId}
              onClose={() => setShowTriggerCtx(false)}
            />
          </div>
        )}

        <div className="flex-1 min-w-0 flex flex-col">
          {isReview && activeExecutionId && <EpisodeOverviewStrip executionId={activeExecutionId} />}
          {isReview && !activeExecutionId && (
            <div className="h-16 border-b bg-card px-4 py-2 flex items-center text-sm text-muted-foreground">
              Loading focused episode context...
            </div>
          )}
          {isReview && activeExecutionId && <ExecutionNarrativeBanner executionId={activeExecutionId} />}
          {isReview && !activeExecutionId && (
            <div className="h-12 border-b bg-card px-4 py-2 flex items-center text-xs text-muted-foreground">
              Preparing execution narrative and process trace anchors...
            </div>
          )}
          <div className="flex-1 min-h-0">
            <Canvas />
          </div>
          {isReview && activeExecutionId && <ProcessTraceTray />}
        </div>

        {!isReview && selectedNodeId && (
          <div className="w-[320px] border-l shrink-0 bg-card">
            <ConfigPanel />
          </div>
        )}



        <AnimatePresence>
          {showLibrary && (
            <motion.div
              key="library-tray"
              initial={{ x: '-100%' }}
              animate={{ x: 0 }}
              exit={{ x: '-100%' }}
              transition={{ type: 'spring', damping: 28, stiffness: 220 }}
              className="absolute inset-y-0 left-0 z-30 w-[420px] max-w-[85vw] border-r bg-card shadow-xl"
            >
              <div className="h-10 px-3 border-b flex items-center justify-between shrink-0">
                <span className="wb-section-header flex items-center gap-1.5">
                  <BookOpen className="w-3.5 h-3.5" />
                  Load Workflow
                </span>
                <Button size="xs" variant="ghost" onClick={() => setShowLibrary(false)} className="h-6 w-6 p-0">
                  <X className="w-3.5 h-3.5" />
                </Button>
              </div>
              <div className="h-[calc(100%-40px)] overflow-hidden">
                <WorkflowLibrary />
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        <AnimatePresence>
          {showRightHistory && (
            <motion.div
              key="history-tray"
              initial={{ x: '100%' }}
              animate={{ x: 0 }}
              exit={{ x: '100%' }}
              transition={{ type: 'spring', damping: 28, stiffness: 220 }}
              className="absolute inset-y-0 right-0 z-30 w-[460px] max-w-[85vw] border-l bg-card shadow-xl"
            >
              <div className="h-10 px-3 border-b flex items-center justify-between shrink-0">
                <span className="wb-section-header flex items-center gap-1.5">
                  <History className="w-3.5 h-3.5" />
                  Execution History
                </span>
                <Button size="xs" variant="ghost" onClick={() => setShowHistory(false)} className="h-6 w-6 p-0">
                  <X className="w-3.5 h-3.5" />
                </Button>
              </div>
              <div className="h-[calc(100%-40px)] overflow-hidden">
                <ExecutionHistory />
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        <AnimatePresence>
          {selectedEpisode && <ForensicDossierDrawer />}
        </AnimatePresence>
      </div>
    </div>
  )
}
