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
      <div className="min-w-0 flex items-center gap-2 shrink-0">
        <span className="text-base font-bold tracking-tight">SynapseFlow</span>
        <span className="text-[11px] text-slate-200 truncate max-w-[240px]">{workflowName || 'Untitled Workflow'}</span>
        <span className={`text-[10px] px-1.5 py-0.5 rounded border ${isRunning ? 'text-emerald-300 border-emerald-600/50 bg-emerald-500/10' : 'text-slate-300 border-slate-700 bg-slate-900'}`}>
          {isRunning ? 'Running' : isReview ? 'Review' : 'Ready'}
        </span>
        {isReview && activeExecutionId && (
          <span className="text-[10px] text-cyan-200 bg-cyan-500/15 border border-cyan-500/30 px-1.5 py-0.5 rounded font-mono">
            {activeExecutionId.slice(0, 12)}
          </span>
        )}
      </div>

      <div className="ml-auto flex items-center bg-slate-900 border border-slate-700 rounded-md p-0.5">
        <button
          onClick={onEnterBuilder}
          className={`px-3 py-1 text-xs rounded transition-colors ${
            !isReview ? 'bg-slate-200 text-slate-900' : 'text-slate-300 hover:bg-slate-800'
          }`}
        >
          Design
        </button>
        <button
          onClick={onEnterReview}
          className={`px-3 py-1 text-xs rounded transition-colors ${
            isReview ? 'bg-blue-500 text-white' : 'text-slate-300 hover:bg-slate-800'
          }`}
        >
          Execution
        </button>
      </div>

      <div className="flex items-center gap-2 shrink-0">
        <button
          onClick={() => setShowLibrary(!showLibrary)}
          className={`px-2.5 py-1 text-xs rounded border transition-colors ${
            showLibrary
              ? 'bg-slate-200 text-slate-900 border-slate-200'
              : 'border-slate-700 text-slate-200 hover:bg-slate-800'
          }`}
        >
          Load Workflow
        </button>
      </div>

      <div className="flex items-center gap-2">
        {error && (
          <span className="text-xs text-red-200 bg-red-500/20 border border-red-500/40 px-2 py-1 rounded">
            {error}
          </span>
        )}

        {!isReview && (
          <>
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
              className="px-3 py-1 text-xs rounded bg-emerald-600 text-white hover:bg-emerald-500 disabled:opacity-40"
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
            Back to Design
          </button>
        )}

        <button
          onClick={onSave}
          disabled={isRunning || nodesCount === 0}
          className="px-2.5 py-1 text-xs rounded border border-slate-700 text-slate-200 hover:bg-slate-800 disabled:opacity-40"
        >
          Export Workflow Spec
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

        {!isReview && (
          <input
            type="text"
            value={workflowName}
            onChange={(e) => setWorkflowName(e.target.value)}
            className="px-2 py-1.5 text-xs rounded bg-slate-900 border border-slate-700 text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-400 w-48"
          />
        )}

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
