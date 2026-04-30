import { useState } from 'react'
import { useGraphStore } from '@/hooks/useGraphStore'
import { useExecutionPoller } from '@/hooks/useExecutionPoller'
import { useDAGPersistence } from '@/hooks/useDAGPersistence'
import { useTheme } from '@/contexts/ThemeContext'
import { runWorkflow, getExecutionNodes } from '@/api/client'
import { ExecutionWorkbenchHeader } from '@/components/execution/ExecutionWorkbenchHeader'

/**
 * Top toolbar: workflow name, mode toggle, save/load, clear, history, run.
 *
 * Business logic lives in:
 *  - useExecutionPoller  — 500 ms polling while a run is active
 *  - useDAGPersistence   — DAG CRUD (save / load / list)
 *
 * M2.3: BUILDER controls are hidden in REVIEW mode; REVIEW controls are hidden
 * in BUILDER mode.  The mode toggle and logo are always visible.
 */
export function Toolbar() {
  const appMode = useGraphStore((s) => s.appMode)
  const setAppMode = useGraphStore((s) => s.setAppMode)
  const exitReviewMode = useGraphStore((s) => s.exitReviewMode)
  const workflowName = useGraphStore((s) => s.workflowName)
  const setWorkflowName = useGraphStore((s) => s.setWorkflowName)
  const isRunning = useGraphStore((s) => s.isRunning)
  const setIsRunning = useGraphStore((s) => s.setIsRunning)
  const setExecutionResult = useGraphStore((s) => s.setExecutionResult)
  const setActiveExecutionId = useGraphStore((s) => s.setActiveExecutionId)
  const activeExecutionId = useGraphStore((s) => s.activeExecutionId)
  const toDAGConfig = useGraphStore((s) => s.toDAGConfig)
  const clearCanvas = useGraphStore((s) => s.clearCanvas)
  const nodes = useGraphStore((s) => s.nodes)
  const showHistory = useGraphStore((s) => s.showHistory)
  const setShowHistory = useGraphStore((s) => s.setShowHistory)
  const showLibrary = useGraphStore((s) => s.showLibrary)
  const setShowLibrary = useGraphStore((s) => s.setShowLibrary)
  const useWorkbenchLayout = useGraphStore((s) => s.useWorkbenchLayout)
  const setUseWorkbenchLayout = useGraphStore((s) => s.setUseWorkbenchLayout)

  const { theme, toggleTheme } = useTheme()
  const { pollingError } = useExecutionPoller()
  const { handleSave, saveLoadError } = useDAGPersistence()

  const [runError, setRunError] = useState<string | null>(null)

  const error = runError ?? pollingError ?? saveLoadError

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

  if (useWorkbenchLayout) {
    return (
      <ExecutionWorkbenchHeader
        isReview={isReview}
        workflowName={workflowName}
        setWorkflowName={setWorkflowName}
        activeExecutionId={activeExecutionId}
        error={error}
        theme={theme}
        toggleTheme={toggleTheme}
        useWorkbenchLayout={useWorkbenchLayout}
        setUseWorkbenchLayout={setUseWorkbenchLayout}
        showLibrary={showLibrary}
        setShowLibrary={setShowLibrary}
        showHistory={showHistory}
        setShowHistory={setShowHistory}
        nodesCount={nodes.length}
        isRunning={isRunning}
        onSave={() => void handleSave()}
        onClear={clearCanvas}
        onRun={() => void handleRun()}
        onBackToBuilder={exitReviewMode}
        onEnterBuilder={() => {
          if (isReview) exitReviewMode()
          else setAppMode('BUILDER')
        }}
        onEnterReview={() => setAppMode('REVIEW')}
      />
    )
  }

  return (
    <div className="h-12 bg-white dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700 flex items-center px-4 gap-4 shrink-0">
      {/* Logo / Title */}
      <div className="flex items-center gap-2">
        <span className="text-lg font-bold text-gray-800 dark:text-gray-100">Synapse</span>
        <span className="text-xs text-gray-400 dark:text-gray-500">v0.1.0</span>
      </div>

      {/* Workflow name — BUILDER only */}
      {!isReview && (
        <input
          type="text"
          value={workflowName}
          onChange={(e) => setWorkflowName(e.target.value)}
          className="ml-4 px-2 py-1 text-sm border border-transparent rounded
                     hover:border-gray-300 dark:hover:border-gray-600
                     focus:border-blue-400 focus:outline-none
                     focus:ring-1 focus:ring-blue-400 transition-colors w-56
                     bg-transparent dark:text-gray-200"
        />
      )}

      {/* Review mode: active execution badge */}
      {isReview && activeExecutionId && (
        <span className="ml-4 px-2 py-1 text-xs font-mono bg-blue-50 dark:bg-blue-900/30 border border-blue-200 dark:border-blue-700 text-blue-700 dark:text-blue-300 rounded">
          Reviewing: {activeExecutionId.slice(0, 8)}
        </span>
      )}

      {/* Mode toggle (centred) */}
      <div className="flex-1 flex justify-center">
        <div className="flex bg-gray-100 dark:bg-gray-800 p-1 rounded-lg border border-gray-200 dark:border-gray-700">
          <button
            onClick={() => {
              if (isReview) exitReviewMode()
              else setAppMode('BUILDER')
            }}
            className={`px-4 py-1 text-sm font-medium rounded-md transition-colors ${
              !isReview
                ? 'bg-white dark:bg-gray-700 shadow-sm text-blue-600 dark:text-blue-400'
                : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
            }`}
          >
            SOP Builder
          </button>
          <button
            onClick={() => setAppMode('REVIEW')}
            className={`px-4 py-1 text-sm font-medium rounded-md transition-colors ${
              isReview
                ? 'bg-white dark:bg-gray-700 shadow-sm text-blue-600 dark:text-blue-400'
                : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
            }`}
          >
            Execution Review
          </button>
        </div>
      </div>

      {/* Error message */}
      {error && (
        <span className="text-xs text-red-700 bg-red-50 border border-red-200 px-2 py-1 rounded mr-2">
          {error}
        </span>
      )}

      {/* Layout toggle */}
      <div className="flex items-center rounded-md border border-gray-300 dark:border-gray-600 overflow-hidden">
        <button
          onClick={() => setUseWorkbenchLayout(false)}
          className={`px-2 py-1 text-[11px] transition-colors ${
            !useWorkbenchLayout
              ? 'bg-gray-800 text-white dark:bg-gray-600'
              : 'text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800'
          }`}
          title="Use the classic editor layout"
        >
          Classic
        </button>
        <button
          onClick={() => setUseWorkbenchLayout(true)}
          className={`px-2 py-1 text-[11px] transition-colors ${
            useWorkbenchLayout
              ? 'bg-blue-600 text-white'
              : 'text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800'
          }`}
          title="Use the workbench layout"
        >
          Workbench
        </button>
      </div>

      {/* ---- BUILDER-only controls ---- */}
      {!isReview && (
        <>
          {/* Node count */}
          <span className="text-xs text-gray-400 dark:text-gray-500">
            {nodes.length} node{nodes.length !== 1 ? 's' : ''}
          </span>

          {/* Save */}
          <button
            onClick={() => void handleSave()}
            disabled={isRunning || nodes.length === 0}
            className="px-3 py-1 text-sm text-gray-600 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-md
                       hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-40 disabled:cursor-not-allowed
                       transition-colors"
          >
            Save
          </button>

          {/* Library toggle */}
          <button
            onClick={() => setShowLibrary(!showLibrary)}
            className={`px-3 py-1 text-sm border rounded-md transition-colors
              ${showLibrary
                ? 'bg-gray-800 text-white border-gray-800 dark:bg-gray-600 dark:border-gray-600'
                : 'text-gray-600 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-800'
              }`}
          >
            Library
          </button>

          {/* Clear */}
          <button
            onClick={clearCanvas}
            disabled={isRunning || nodes.length === 0}
            className="px-3 py-1 text-sm text-gray-600 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-md
                       hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-40 disabled:cursor-not-allowed
                       transition-colors"
          >
            Clear
          </button>

          {/* History toggle */}
          <button
            onClick={() => setShowHistory(!showHistory)}
            className={`px-3 py-1 text-sm border rounded-md transition-colors
              ${showHistory
                ? 'bg-gray-800 text-white border-gray-800 dark:bg-gray-600 dark:border-gray-600'
                : 'text-gray-600 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-800'
              }`}
          >
            History
          </button>

          {/* Run */}
          <button
            onClick={() => void handleRun()}
            disabled={isRunning || nodes.length === 0}
            className={`
              px-4 py-1 text-sm font-medium rounded-md transition-colors
              ${isRunning
                ? 'bg-gray-400 text-white cursor-wait'
                : 'bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-40 disabled:cursor-not-allowed'
              }
            `}
          >
            {isRunning ? 'Running...' : 'Run'}
          </button>
        </>
      )}

      {/* ---- REVIEW-only controls ---- */}
      {isReview && (
        <button
          onClick={exitReviewMode}
          className="px-3 py-1 text-sm text-gray-600 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-md
                     hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
        >
          ← Back to Builder
        </button>
      )}

      {/* Theme toggle */}
      <button
        onClick={toggleTheme}
        title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
        className="w-8 h-8 flex items-center justify-center rounded-md text-gray-500 dark:text-gray-400
                   hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors text-base"
      >
        {theme === 'dark' ? '☀' : '🌙'}
      </button>
    </div>
  )
}
