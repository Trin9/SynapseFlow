interface ExecutionWorkbenchHeaderProps {
  isReview: boolean
  workflowName: string
  setWorkflowName: (name: string) => void
  activeExecutionId: string | null
  error: string | null
  theme: 'light' | 'dark'
  toggleTheme: () => void
  useWorkbenchLayout: boolean
  setUseWorkbenchLayout: (enabled: boolean) => void
  showLibrary: boolean
  setShowLibrary: (show: boolean) => void
  showHistory: boolean
  setShowHistory: (show: boolean) => void
  nodesCount: number
  isRunning: boolean
  onSave: () => void
  onClear: () => void
  onRun: () => void
  onBackToBuilder: () => void
  onEnterBuilder: () => void
  onEnterReview: () => void
}

export function ExecutionWorkbenchHeader({
  isReview,
  workflowName,
  setWorkflowName,
  activeExecutionId,
  error,
  theme,
  toggleTheme,
  useWorkbenchLayout,
  setUseWorkbenchLayout,
  showLibrary,
  setShowLibrary,
  showHistory,
  setShowHistory,
  nodesCount,
  isRunning,
  onSave,
  onClear,
  onRun,
  onBackToBuilder,
  onEnterBuilder,
  onEnterReview,
}: ExecutionWorkbenchHeaderProps) {
  return (
    <div className="h-14 bg-slate-950 text-slate-100 border-b border-slate-800 flex items-center px-4 gap-3 shrink-0">
      <div className="flex items-center gap-2 shrink-0">
        <span className="text-base font-bold tracking-tight">Synapse Workbench</span>
        <span className="text-[10px] uppercase text-slate-400">v0.1.0</span>
      </div>

      <div className="flex items-center bg-slate-900 border border-slate-700 rounded-md p-0.5">
        <button
          onClick={onEnterBuilder}
          className={`px-3 py-1 text-xs rounded transition-colors ${
            !isReview ? 'bg-slate-200 text-slate-900' : 'text-slate-300 hover:bg-slate-800'
          }`}
        >
          Builder
        </button>
        <button
          onClick={onEnterReview}
          className={`px-3 py-1 text-xs rounded transition-colors ${
            isReview ? 'bg-blue-500 text-white' : 'text-slate-300 hover:bg-slate-800'
          }`}
        >
          Review
        </button>
      </div>

      {!isReview ? (
        <input
          type="text"
          value={workflowName}
          onChange={(e) => setWorkflowName(e.target.value)}
          className="px-2 py-1.5 text-xs rounded bg-slate-900 border border-slate-700 text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-400 w-56"
        />
      ) : (
        <div className="text-xs px-2 py-1 rounded bg-blue-500/15 border border-blue-500/30 text-blue-200 font-mono">
          {activeExecutionId ? `exec:${activeExecutionId.slice(0, 12)}` : 'review mode'}
        </div>
      )}

      <div className="flex items-center gap-2">
        <button
          onClick={() => setShowLibrary(!showLibrary)}
          className={`px-2.5 py-1 text-xs rounded border transition-colors ${
            showLibrary
              ? 'bg-slate-200 text-slate-900 border-slate-200'
              : 'border-slate-700 text-slate-200 hover:bg-slate-800'
          }`}
        >
          Load
        </button>
        <button
          onClick={() => setShowHistory(!showHistory)}
          className={`px-2.5 py-1 text-xs rounded border transition-colors ${
            showHistory
              ? 'bg-slate-200 text-slate-900 border-slate-200'
              : 'border-slate-700 text-slate-200 hover:bg-slate-800'
          }`}
        >
          History
        </button>
      </div>

      <div className="ml-auto flex items-center gap-2">
        {error && (
          <span className="text-xs text-red-200 bg-red-500/20 border border-red-500/40 px-2 py-1 rounded">
            {error}
          </span>
        )}

        {!isReview && (
          <>
            <span className="text-[11px] text-slate-400">{nodesCount} node{nodesCount === 1 ? '' : 's'}</span>
            <button
              onClick={onSave}
              disabled={isRunning || nodesCount === 0}
              className="px-2.5 py-1 text-xs rounded border border-slate-700 text-slate-200 hover:bg-slate-800 disabled:opacity-40"
            >
              Save
            </button>
            <button
              onClick={onClear}
              disabled={isRunning || nodesCount === 0}
              className="px-2.5 py-1 text-xs rounded border border-slate-700 text-slate-200 hover:bg-slate-800 disabled:opacity-40"
            >
              Clear
            </button>
            <button
              onClick={onRun}
              disabled={isRunning || nodesCount === 0}
              className="px-3 py-1 text-xs rounded bg-blue-500 text-white hover:bg-blue-600 disabled:opacity-40"
            >
              {isRunning ? 'Running...' : 'Run'}
            </button>
          </>
        )}

        {isReview && (
          <button
            onClick={onBackToBuilder}
            className="px-2.5 py-1 text-xs rounded border border-slate-700 text-slate-200 hover:bg-slate-800"
          >
            Back to Builder
          </button>
        )}

        <div className="flex items-center rounded border border-slate-700 overflow-hidden">
          <button
            onClick={() => setUseWorkbenchLayout(false)}
            className={`px-2 py-1 text-[11px] ${
              !useWorkbenchLayout ? 'bg-slate-200 text-slate-900' : 'text-slate-300 hover:bg-slate-800'
            }`}
          >
            Classic
          </button>
          <button
            onClick={() => setUseWorkbenchLayout(true)}
            className={`px-2 py-1 text-[11px] ${
              useWorkbenchLayout ? 'bg-blue-500 text-white' : 'text-slate-300 hover:bg-slate-800'
            }`}
          >
            Workbench
          </button>
        </div>

        <button
          onClick={toggleTheme}
          title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
          className="w-8 h-8 flex items-center justify-center rounded border border-slate-700 text-slate-200 hover:bg-slate-800"
        >
          {theme === 'dark' ? '☀' : '🌙'}
        </button>
      </div>
    </div>
  )
}
