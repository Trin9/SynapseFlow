import { useEffect } from 'react'
import { useExecutionPoller } from '@/hooks/useExecutionPoller'

/**
 * Global execution runtime side-effects container.
 * Keeps polling logic out of toolbar rendering concerns.
 */
export function ExecutionRuntimeOrchestrator() {
  const { pollingError } = useExecutionPoller()

  useEffect(() => {
    if (!pollingError) return
    // Surface as console diagnostics without forcing header-level re-render coupling.
    // eslint-disable-next-line no-console
    console.warn('Execution polling error:', pollingError)
  }, [pollingError])

  return null
}
