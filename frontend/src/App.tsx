import { ReactFlowProvider } from '@xyflow/react'
import { Canvas } from './components/Canvas'
import { Sidebar } from './components/Sidebar'
import { ConfigPanel } from './components/ConfigPanel'
import { Toolbar } from './components/Toolbar'
import { ResultsPanel } from './components/ResultsPanel'

export default function App() {
  return (
    <ReactFlowProvider>
      <div className="h-screen flex flex-col">
        {/* Top toolbar */}
        <Toolbar />

        {/* Main area: sidebar + canvas + config panel */}
        <div className="flex-1 flex overflow-hidden">
          <Sidebar />
          <Canvas />
          <ConfigPanel />
        </div>

        {/* Bottom results panel (appears after execution) */}
        <ResultsPanel />
      </div>
    </ReactFlowProvider>
  )
}
