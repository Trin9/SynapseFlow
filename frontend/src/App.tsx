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

export default function App() {
  const showHistory = useGraphStore((s) => s.showHistory)
  const showLibrary = useGraphStore((s) => s.showLibrary)
  const appMode = useGraphStore((s) => s.appMode)
  const isReview = appMode === 'REVIEW'

  return (
    <ReactFlowProvider>
      <div className="h-screen flex flex-col bg-white dark:bg-gray-900">
        {/* Top toolbar */}
        <Toolbar />

        {/* Main area */}
        <div className="flex-1 flex overflow-hidden">
          {/* Workflow Library — BUILDER only, toggled via Toolbar */}
          {!isReview && showLibrary && <WorkflowLibrary />}

          {/* Node palette — hidden in REVIEW mode or when Library is open */}
          {!isReview && !showLibrary && <Sidebar />}

          <Canvas />

          {/* Config panel — hidden in REVIEW mode */}
          {!isReview && <ConfigPanel />}

          {/* ExecutionHistory:
               - REVIEW mode: always visible (it's the primary review surface)
               - BUILDER mode: visible only when showHistory is toggled */}
          {(isReview || showHistory) && <ExecutionHistory />}
        </div>

        {/* Bottom results panel (appears after execution) */}
        <ResultsPanel />

        {/* Episode Detail overlay (renders on top of everything) */}
        <EpisodeDetail />
      </div>
    </ReactFlowProvider>
  )
}
