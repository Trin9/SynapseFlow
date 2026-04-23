import { ReactFlowProvider } from '@xyflow/react'
import { Canvas } from './components/Canvas'
import { Sidebar } from './components/Sidebar'
import { ConfigPanel } from './components/ConfigPanel'
import { Toolbar } from './components/Toolbar'
import { ResultsPanel } from './components/ResultsPanel'
import { ExecutionHistory } from './components/ExecutionHistory'
import { useGraphStore } from './hooks/useGraphStore'

export default function App() {
  const showHistory = useGraphStore((s) => s.showHistory)

  return (
    <ReactFlowProvider>
      <div className="h-screen flex flex-col">
        {/* Top toolbar */}
        <Toolbar />

        {/* Main area: sidebar + canvas + config panel + optional history panel */}
        <div className="flex-1 flex overflow-hidden">
          <Sidebar />
          <Canvas />
          <ConfigPanel />
          {showHistory && <ExecutionHistory />}
        </div>

        {/* Bottom results panel (appears after execution) */}
        <ResultsPanel />
      </div>
    </ReactFlowProvider>
  )
}
