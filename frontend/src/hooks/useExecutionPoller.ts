import { useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getExecutionNodes } from '@/api/client'
import { useGraphStore } from '@/hooks/useGraphStore'

/**
 * Polls the execution-nodes endpoint while an execution is running.
 * Replaces the bare setInterval + useEffect pattern in the old Toolbar.
 *
 * Side-effects:
 *  - Writes snapshot to `executionResult` in the global store.
 *  - Clears `isRunning` + `activeExecutionId` when execution reaches a terminal state.
 */
export function useExecutionPoller() {
  const activeExecutionId = useGraphStore((s) => s.activeExecutionId)
  const isRunning = useGraphStore((s) => s.isRunning)
  const setExecutionResult = useGraphStore((s) => s.setExecutionResult)
  const setIsRunning = useGraphStore((s) => s.setIsRunning)

  const TERMINAL = new Set(['completed', 'failed', 'timeout', 'suspended'])
  const lastSnapshotSigRef = useRef<string | null>(null)

  const snapshotSig = (payload: {
    execution_id: string
    status: string
    duration_ms?: number
    results?: Array<{ node_id: string; status: string; duration_ms?: number }>
  }) => {
    const results = payload.results ?? []
    const tail = results.length > 0
      ? `${results[results.length - 1].node_id}:${results[results.length - 1].status}:${results[results.length - 1].duration_ms ?? 0}`
      : 'none'
    return `${payload.execution_id}|${payload.status}|${payload.duration_ms ?? 0}|${results.length}|${tail}`
  }

  const { data, error } = useQuery({
    queryKey: ['execution-nodes', activeExecutionId],
    queryFn: () => getExecutionNodes(activeExecutionId!),
    enabled: !!activeExecutionId && isRunning,
    refetchInterval: (query) => {
      const status = query.state.data?.status
      return status && TERMINAL.has(status) ? false : 1200
    },
  })

  // Sync latest snapshot into Zustand store
  useEffect(() => {
    lastSnapshotSigRef.current = null
  }, [activeExecutionId])

  useEffect(() => {
    if (!data) return
    const sig = snapshotSig(data)
    if (lastSnapshotSigRef.current !== sig) {
      setExecutionResult(data)
      lastSnapshotSigRef.current = sig
    }
    if (TERMINAL.has(data.status)) {
      setIsRunning(false)
    }
    // TERMINAL is stable (Set literal), intentionally omitted from deps
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, setExecutionResult, setIsRunning])

  return {
    pollingError: error instanceof Error ? error.message : error ? String(error) : null,
  }
}
