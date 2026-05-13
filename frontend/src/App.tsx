import { ReactFlowProvider } from '@xyflow/react'
import { Toolbar } from './components/Toolbar'
import { ResultsPanel } from './components/ResultsPanel'
import { WorkbenchLayout } from './layouts/WorkbenchLayout'
import { ExecutionRuntimeOrchestrator } from './components/execution/ExecutionRuntimeOrchestrator'

export default function App() {
  return (
    <ReactFlowProvider>
      <div className="h-screen flex flex-col bg-white dark:bg-gray-900">
        <ExecutionRuntimeOrchestrator />
        <Toolbar />
        <WorkbenchLayout />
        <ResultsPanel />
      </div>
    </ReactFlowProvider>
  )
}
