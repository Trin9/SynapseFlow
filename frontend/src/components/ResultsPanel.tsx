import { useGraphStore } from '@/hooks/useGraphStore'
import { NODE_TYPE_INFO } from '@/types'

function formatMaybeJSON(s: string): string {
  const t = s.trim()
  if (!t) return s
  if (!(t.startsWith('{') || t.startsWith('['))) return s
  try {
    return JSON.stringify(JSON.parse(t), null, 2)
  } catch {
    return s
  }
}

/**
 * Bottom panel that displays execution results after a workflow run.
 */
export function ResultsPanel() {
  const executionResult = useGraphStore((s) => s.executionResult)
  const setExecutionResult = useGraphStore((s) => s.setExecutionResult)
  const setActiveExecutionId = useGraphStore((s) => s.setActiveExecutionId)
  const setIsRunning = useGraphStore((s) => s.setIsRunning)

  if (!executionResult) return null

  return (
    <div className="border-t border-gray-200 bg-white max-h-72 overflow-y-auto shrink-0">
      {/* Header */}
      <div className="px-4 py-2 border-b border-gray-100 flex items-center justify-between sticky top-0 bg-white">
        <div className="flex items-center gap-3">
          <span className="text-sm font-semibold text-gray-700">
            Execution Results
          </span>
          <span
            className={`text-xs px-2 py-0.5 rounded-full font-medium
              ${executionResult.status === 'completed'
                ? 'bg-green-100 text-green-700'
                : executionResult.status === 'running'
                  ? 'bg-blue-100 text-blue-700'
                  : 'bg-red-100 text-red-700'
              }`}
          >
            {executionResult.status}
          </span>
          <span className="text-xs text-gray-400">
            {typeof executionResult.duration_ms === 'number' ? `${executionResult.duration_ms}ms` : ''}
          </span>
        </div>
        <button
          onClick={() => {
            setExecutionResult(null)
            setActiveExecutionId(null)
            setIsRunning(false)
          }}
          className="text-gray-400 hover:text-gray-600 text-sm"
        >
          Close
        </button>
      </div>

      {/* Node results */}
      <div className="divide-y divide-gray-100">
        {(executionResult.results ?? []).map((result) => {
          const info = NODE_TYPE_INFO[result.node_type]
          return (
            <div key={result.node_id} className="px-4 py-2">
              <details className="group" open={result.status !== 'success'}>
                <summary className="list-none cursor-pointer select-none">
                  <div className="flex items-center gap-2">
                    <span
                      className={`w-2 h-2 rounded-full ${
                        result.status === 'success'
                          ? 'bg-green-500'
                          : result.status === 'skipped'
                            ? 'bg-gray-400'
                            : 'bg-red-500'
                      }`}
                    />
                    <span className={`text-[10px] font-bold uppercase ${info?.color ?? 'text-gray-600'}`}>
                      {result.node_type}
                    </span>
                    <span className="text-sm font-medium text-gray-700">
                      {result.node_name}
                    </span>
                    <span className="text-xs text-gray-400 ml-auto">
                      {result.duration_ms}ms
                    </span>
                    <span className="text-xs text-gray-400 group-open:rotate-90 transition-transform">›</span>
                  </div>
                </summary>

                {result.output && (
                  <pre className="text-xs text-gray-700 bg-gray-50 rounded p-2 mt-2 overflow-x-auto font-mono whitespace-pre">
                    {formatMaybeJSON(result.output)}
                  </pre>
                )}

                {result.error && (
                  <pre className="text-xs text-red-700 bg-red-50 rounded p-2 mt-2 overflow-x-auto font-mono whitespace-pre-wrap">
                    {result.error}
                  </pre>
                )}
              </details>
            </div>
          )
        })}
      </div>
    </div>
  )
}
