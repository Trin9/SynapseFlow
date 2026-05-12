import { ReactFlowProvider } from '@xyflow/react'
import { Canvas } from './components/Canvas'
import { Sidebar } from './components/Sidebar'
import { ConfigPanel } from './components/ConfigPanel'
import { Toolbar } from './components/Toolbar'
import { ResultsPanel } from './components/ResultsPanel'
import { ExecutionHistory } from './components/ExecutionHistory'
import { EpisodeDetail } from './components/EpisodeDetail'
import { WorkflowLibrary } from './components/library/WorkflowLibrary'
import { useGraphStore } from './hooks/useGraphStore'
import { WorkbenchLayout } from './layouts/WorkbenchLayout'

export default function App() {
  const showHistory = useGraphStore((s) => s.showHistory)
  const showLibrary = useGraphStore((s) => s.showLibrary)
  const appMode = useGraphStore((s) => s.appMode)
  const useWorkbenchLayout = useGraphStore((s) => s.useWorkbenchLayout)
  const isReview = appMode === 'REVIEW'

  return (
    <ReactFlowProvider>
      <div className="h-screen flex flex-col bg-white dark:bg-gray-900">
        {/* Top toolbar */}
        <Toolbar />

        {useWorkbenchLayout ? (
          <WorkbenchLayout />
        ) : (
          <div className="flex-1 min-h-0 flex overflow-hidden">
            {/* Workflow Library — BUILDER only, toggled via Toolbar */}
            {!isReview && showLibrary && (
              <div className="w-[420px] max-w-[85vw] shrink-0 border-r bg-card">
                <WorkflowLibrary />
              </div>
            )}

            {/* Node palette — hidden in REVIEW mode or when Library is open */}
            {!isReview && !showLibrary && <Sidebar />}

            <div className="flex-1 min-w-0 min-h-0">
              <Canvas />
            </div>

            {/* Config panel — hidden in REVIEW mode */}
            {!isReview && <ConfigPanel />}

            {/* ExecutionHistory:
                 - REVIEW mode: always visible (it's the primary review surface)
                 - BUILDER mode: visible only when showHistory is toggled */}
            {(isReview || showHistory) && <ExecutionHistory />}
          </div>
        )}

        {/* Bottom results panel (appears after execution) */}
        <ResultsPanel />

        {/* Episode Detail overlay (Classic layout only; Workbench renders its own drawer) */}
        {!useWorkbenchLayout && <EpisodeDetail />}
      </div>
    </ReactFlowProvider>
  )
}
