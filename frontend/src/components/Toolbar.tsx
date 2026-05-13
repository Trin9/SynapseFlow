import { useState } from 'react'
import { useGraphStore } from '@/hooks/useGraphStore'
import { useDAGPersistence } from '@/hooks/useDAGPersistence'
import { useTheme } from '@/contexts/ThemeContext'
import { runWorkflow, getExecutionNodes } from '@/api/client'
import { ExecutionWorkbenchHeader } from '@/components/execution/ExecutionWorkbenchHeader'

/**
 * Top toolbar: workflow name, mode toggle, save/load, clear, history, run.
 * Always delegates to ExecutionWorkbenchHeader (Classic layout removed).
 */
export function Toolbar() {
  const appMode = useGraphStore((s) => s.appMode)
  const setAppMode = useGraphStore((s) => s.setAppMode)
  const enterReviewMode = useGraphStore((s) => s.enterReviewMode)
  const exitReviewMode = useGraphStore((s) => s.exitReviewMode)
  const workflowName = useGraphStore((s) => s.workflowName)
  const setWorkflowName = useGraphStore((s) => s.setWorkflowName)
  const isRunning = useGraphStore((s) => s.isRunning)
  const setIsRunning = useGraphStore((s) => s.setIsRunning)
  const setExecutionResult = useGraphStore((s) => s.setExecutionResult)
  const setActiveExecutionId = useGraphStore((s) => s.setActiveExecutionId)
  const toDAGConfig = useGraphStore((s) => s.toDAGConfig)
  const clearCanvas = useGraphStore((s) => s.clearCanvas)
  const nodes = useGraphStore((s) => s.nodes)
  const showHistory = useGraphStore((s) => s.showHistory)
  const setShowHistory = useGraphStore((s) => s.setShowHistory)
  const setShowTriggerCtx = useGraphStore((s) => s.setShowTriggerCtx)
  const showLibrary = useGraphStore((s) => s.showLibrary)
  const setShowLibrary = useGraphStore((s) => s.setShowLibrary)

  const { theme, toggleTheme } = useTheme()
  const { handleSave, saveLoadError } = useDAGPersistence()

  const [runError, setRunError] = useState<string | null>(null)

  const error = runError ?? saveLoadError

  const handleRun = async () => {
    const dag = toDAGConfig()
    if (dag.nodes.length === 0) {
      setRunError('Add at least one node before running')
      setTimeout(() => setRunError(null), 3000)
      return
    }

    setIsRunning(true)
    setRunError(null)
    setExecutionResult(null)
    setActiveExecutionId(null)

    try {
      const start = await runWorkflow(dag)
      enterReviewMode(start.execution_id)
      setIsRunning(true)
      setActiveExecutionId(start.execution_id)
      const snapshot = await getExecutionNodes(start.execution_id)
      setExecutionResult(snapshot)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Execution failed'
      setRunError(msg)
      setTimeout(() => setRunError(null), 5000)
      setIsRunning(false)
      setActiveExecutionId(null)
    }
  }

  const isReview = appMode === 'REVIEW'

  return (
    <ExecutionWorkbenchHeader
      isReview={isReview}
      workflowName={workflowName}
      setWorkflowName={setWorkflowName}
      error={error}
      theme={theme}
      toggleTheme={toggleTheme}
      showLibrary={showLibrary}
      setShowLibrary={setShowLibrary}
      showHistory={showHistory}
      setShowHistory={setShowHistory}
      nodesCount={nodes.length}
      isRunning={isRunning}
      onSave={() => void handleSave()}
      onClear={clearCanvas}
      onRun={() => void handleRun()}
      onPause={() => {/* TODO: pause execution */}}
      onStop={() => { setIsRunning(false); setActiveExecutionId(null) }}
      onEnterBuilder={() => {
        if (isReview) exitReviewMode()
        else setAppMode('BUILDER')
      }}
      onEnterReview={() => {
        setShowHistory(false)
        setShowTriggerCtx(false)
        enterReviewMode()
      }}
    />
  )
}
