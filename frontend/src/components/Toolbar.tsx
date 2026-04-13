import { useCallback, useEffect, useState } from 'react'
import { useGraphStore } from '@/hooks/useGraphStore'
import { createDAG, getDAG, getExecutionNodes, listDAGs, runWorkflow, updateDAG } from '@/api/client'
import type { DAGConfig } from '@/types'

/**
 * Top toolbar with workflow name, Run button, and Clear button.
 */
export function Toolbar() {
  const workflowId = useGraphStore((s) => s.workflowId)
  const workflowName = useGraphStore((s) => s.workflowName)
  const setWorkflowId = useGraphStore((s) => s.setWorkflowId)
  const setWorkflowName = useGraphStore((s) => s.setWorkflowName)
  const isRunning = useGraphStore((s) => s.isRunning)
  const setIsRunning = useGraphStore((s) => s.setIsRunning)
  const setExecutionResult = useGraphStore((s) => s.setExecutionResult)
  const activeExecutionId = useGraphStore((s) => s.activeExecutionId)
  const setActiveExecutionId = useGraphStore((s) => s.setActiveExecutionId)
  const toDAGConfig = useGraphStore((s) => s.toDAGConfig)
  const loadFromDAGConfig = useGraphStore((s) => s.loadFromDAGConfig)
  const clearCanvas = useGraphStore((s) => s.clearCanvas)
  const nodes = useGraphStore((s) => s.nodes)

  const [error, setError] = useState<string | null>(null)
  const [dags, setDags] = useState<DAGConfig[]>([])
  const [selectedLoadId, setSelectedLoadId] = useState<string>('')

  const refreshDAGs = useCallback(async () => {
    try {
      const list = await listDAGs()
      setDags(list)
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    void refreshDAGs()
  }, [refreshDAGs])

  useEffect(() => {
    if (!activeExecutionId || !isRunning) return

    let stopped = false
    const interval = window.setInterval(async () => {
      if (stopped) return
      try {
        const snapshot = await getExecutionNodes(activeExecutionId)
        setExecutionResult(snapshot)
        if (snapshot.status === 'completed' || snapshot.status === 'failed' || snapshot.status === 'timeout') {
          setIsRunning(false)
          setActiveExecutionId(null)
        }
      } catch (e) {
        // Keep polling; surface a lightweight error.
        const msg = e instanceof Error ? e.message : 'Polling failed'
        setError(msg)
      }
    }, 500)

    return () => {
      stopped = true
      window.clearInterval(interval)
    }
  }, [activeExecutionId, isRunning, setExecutionResult, setIsRunning, setActiveExecutionId])

  const handleRun = useCallback(async () => {
    const dag = toDAGConfig()
    if (dag.nodes.length === 0) {
      setError('Add at least one node before running')
      setTimeout(() => setError(null), 3000)
      return
    }

    setIsRunning(true)
    setError(null)
    setExecutionResult(null)
    setActiveExecutionId(null)

    try {
      const start = await runWorkflow(dag)
      setActiveExecutionId(start.execution_id)
      // Populate initial snapshot quickly
      const snapshot = await getExecutionNodes(start.execution_id)
      setExecutionResult(snapshot)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Execution failed'
      setError(msg)
      setTimeout(() => setError(null), 5000)
      setIsRunning(false)
      setActiveExecutionId(null)
    }
  }, [toDAGConfig, setIsRunning, setExecutionResult, setError, setActiveExecutionId])

  const handleSave = useCallback(async () => {
    const dag = toDAGConfig()
    if (dag.nodes.length === 0) {
      setError('Nothing to save (add nodes first)')
      setTimeout(() => setError(null), 3000)
      return
    }

    try {
      if (workflowId) {
        const updated = await updateDAG(workflowId, dag)
        loadFromDAGConfig(updated)
      } else {
        const created = await createDAG(dag)
        loadFromDAGConfig(created)
        setWorkflowId(created.id ?? null)
      }
      await refreshDAGs()
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Save failed'
      setError(msg)
      setTimeout(() => setError(null), 5000)
    }
  }, [toDAGConfig, workflowId, loadFromDAGConfig, refreshDAGs, setWorkflowId])

  const handleLoad = useCallback(async () => {
    if (!selectedLoadId) return
    try {
      const dag = await getDAG(selectedLoadId)
      loadFromDAGConfig(dag)
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Load failed'
      setError(msg)
      setTimeout(() => setError(null), 5000)
    }
  }, [selectedLoadId, loadFromDAGConfig])

  return (
    <div className="h-12 bg-white border-b border-gray-200 flex items-center px-4 gap-4 shrink-0">
      {/* Logo / Title */}
      <div className="flex items-center gap-2">
        <span className="text-lg font-bold text-gray-800">Synapse</span>
        <span className="text-xs text-gray-400">v0.1.0</span>
      </div>

      {/* Workflow name */}
      <input
        type="text"
        value={workflowName}
        onChange={(e) => setWorkflowName(e.target.value)}
        className="ml-4 px-2 py-1 text-sm border border-transparent rounded
                   hover:border-gray-300 focus:border-blue-400 focus:outline-none
                   focus:ring-1 focus:ring-blue-400 transition-colors w-56"
      />

      {/* Spacer */}
      <div className="flex-1" />

      {/* Error message */}
      {error && (
        <span className="text-xs text-red-700 bg-red-50 border border-red-200 px-2 py-1 rounded mr-2">
          {error}
        </span>
      )}

      {/* Node count */}
      <span className="text-xs text-gray-400">
        {nodes.length} node{nodes.length !== 1 ? 's' : ''}
      </span>

      {/* Save / Load */}
      <button
        onClick={handleSave}
        disabled={isRunning || nodes.length === 0}
        className="px-3 py-1 text-sm text-gray-600 border border-gray-300 rounded-md
                   hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed
                   transition-colors"
      >
        Save
      </button>

      <select
        value={selectedLoadId}
        onChange={(e) => setSelectedLoadId(e.target.value)}
        disabled={isRunning || dags.length === 0}
        className="px-2 py-1 text-sm border border-gray-300 rounded-md bg-white
                   disabled:opacity-40 disabled:cursor-not-allowed"
      >
        <option value="">Load DAG...</option>
        {dags.map((d) => (
          <option key={d.id} value={d.id}>
            {d.name} ({d.id})
          </option>
        ))}
      </select>
      <button
        onClick={handleLoad}
        disabled={isRunning || !selectedLoadId}
        className="px-3 py-1 text-sm text-gray-600 border border-gray-300 rounded-md
                   hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed
                   transition-colors"
      >
        Load
      </button>

      {/* Clear button */}
      <button
        onClick={clearCanvas}
        disabled={isRunning || nodes.length === 0}
        className="px-3 py-1 text-sm text-gray-600 border border-gray-300 rounded-md
                   hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed
                   transition-colors"
      >
        Clear
      </button>

      {/* Run button */}
      <button
        onClick={handleRun}
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
    </div>
  )
}
