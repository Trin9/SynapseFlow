interface ExecutionWorkbenchHeaderProps {
  isReview: boolean
  workflowName: string
  setWorkflowName: (name: string) => void
  error: string | null
  theme: 'light' | 'dark'
  toggleTheme: () => void
  showLibrary: boolean
  setShowLibrary: (show: boolean) => void
  showHistory: boolean
  setShowHistory: (show: boolean) => void
  nodesCount: number
  isRunning: boolean
  onSave: () => void
  onClear: () => void
  onRun: () => void
  onPause: () => void
  onStop: () => void
  onEnterBuilder: () => void
  onEnterReview: () => void
}

export function ExecutionWorkbenchHeader({
  isReview,
  workflowName,
  setWorkflowName,
  error,
  theme,
  toggleTheme,
  showLibrary,
  setShowLibrary,
  showHistory,
  setShowHistory,
  nodesCount,
  isRunning,
  onSave,
  onClear,
  onRun,
  onPause,
  onStop,
  onEnterBuilder,
  onEnterReview,
}: ExecutionWorkbenchHeaderProps) {
  return (
    <div className="h-12 bg-slate-950 text-slate-100 border-b border-slate-800 flex items-center px-4 gap-2 shrink-0">
      {/* Left: Logo + name + status */}
      <div className="flex items-center gap-2 shrink-0 min-w-0">
        <span className="text-sm font-bold tracking-tight">SynapseFlow</span>
        <span className="text-[11px] text-slate-200 truncate max-w-[180px] hidden sm:block">
          {workflowName || 'Untitled Workflow'}
        </span>
      </div>

      {/* Center: Mode toggle */}
      <div className="ml-4 flex items-center gap-1.5">
        <div className="flex items-center bg-slate-900 border border-slate-700 rounded-md p-0.5">
          <button
            onClick={onEnterBuilder}
            className={`px-2.5 py-1 text-[11px] rounded transition-colors ${
              !isReview ? 'bg-slate-200 text-slate-900' : 'text-slate-300 hover:bg-slate-800'
            }`}
          >
            Design
          </button>
          <button
            onClick={onEnterReview}
            className={`px-2.5 py-1 text-[11px] rounded transition-colors ${
              isReview ? 'bg-slate-200 text-slate-900' : 'text-slate-300 hover:bg-slate-800'
            }`}
          >
            Execution
          </button>
        </div>
      </div>

      {/* Right: actions */}
      <div className="ml-auto flex items-center gap-1.5">
        {error && (
          <span className="text-[11px] text-red-200 bg-red-500/20 border border-red-500/40 px-2 py-0.5 rounded">
            {error}
          </span>
        )}

        <button
          onClick={() => setShowLibrary(!showLibrary)}
          disabled={isReview}
          className={`px-2 py-1 text-[11px] rounded border transition-colors ${
            showLibrary && !isReview
              ? 'bg-slate-200 text-slate-900 border-slate-200'
              : 'border-slate-700 text-slate-200 hover:bg-slate-800'
          } disabled:opacity-30 disabled:cursor-not-allowed`}
        >
          Load
        </button>
        <button
          onClick={onClear}
          disabled={isReview || isRunning || nodesCount === 0}
          className="px-2 py-1 text-[11px] rounded border border-slate-700 text-slate-200 hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed"
        >
          Clear
        </button>
        <button
          onClick={onRun}
          disabled={isReview || isRunning || nodesCount === 0}
          className="px-2.5 py-1 text-[11px] rounded bg-emerald-600 text-white hover:bg-emerald-500 disabled:opacity-30 disabled:cursor-not-allowed font-medium"
          title="Run workflow"
        >
          ▶ {isRunning ? 'Running' : 'Run'}
        </button>
        <button
          onClick={onPause}
          disabled={isReview || !isRunning}
          className="px-2 py-1 text-[11px] rounded border border-slate-700 text-slate-200 hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed"
          title="Pause execution"
        >
          ⏸
        </button>
        <button
          onClick={onStop}
          disabled={isReview || !isRunning}
          className="px-2 py-1 text-[11px] rounded border border-slate-700 text-slate-200 hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed"
          title="Stop execution"
        >
          ⏹
        </button>

        <button
          onClick={onSave}
          disabled={isRunning || nodesCount === 0}
          className="px-2 py-1 text-[11px] rounded border border-slate-700 text-slate-200 hover:bg-slate-800 disabled:opacity-40"
        >
          Export
        </button>

        <button
          onClick={() => setShowHistory(!showHistory)}
          className={`px-2 py-1 text-[11px] rounded border transition-colors ${
            showHistory
              ? 'bg-slate-200 text-slate-900 border-slate-200'
              : 'border-slate-700 text-slate-200 hover:bg-slate-800'
          }`}
        >
          History
        </button>

        <input
          type="text"
          value={workflowName}
          disabled={isReview}
          onChange={(e) => setWorkflowName(e.target.value)}
          className="px-2 py-1 text-[11px] rounded bg-slate-900 border border-slate-700 text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-400 w-36 disabled:opacity-55"
        />

        <button
          onClick={toggleTheme}
          title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
          className="w-7 h-7 flex items-center justify-center rounded border border-slate-700 text-slate-200 hover:bg-slate-800"
        >
          {theme === 'dark' ? '☀' : '🌙'}
        </button>
      </div>
    </div>
  )
}
