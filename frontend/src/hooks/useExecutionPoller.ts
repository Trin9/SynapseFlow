import { useEffect } from 'react'
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
  const setActiveExecutionId = useGraphStore((s) => s.setActiveExecutionId)

  const TERMINAL = new Set(['completed', 'failed', 'timeout'])

  const { data, error } = useQuery({
    queryKey: ['execution-nodes', activeExecutionId],
    queryFn: () => getExecutionNodes(activeExecutionId!),
    enabled: !!activeExecutionId && isRunning,
    refetchInterval: (query) => {
      const status = query.state.data?.status
      return status && TERMINAL.has(status) ? false : 500
    },
  })

  // Sync latest snapshot into Zustand store
  useEffect(() => {
    if (!data) return
    setExecutionResult(data)
    if (TERMINAL.has(data.status)) {
      setIsRunning(false)
      setActiveExecutionId(null)
    }
    // TERMINAL is stable (Set literal), intentionally omitted from deps
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, setExecutionResult, setIsRunning, setActiveExecutionId])

  return {
    pollingError: error instanceof Error ? error.message : error ? String(error) : null,
  }
}
