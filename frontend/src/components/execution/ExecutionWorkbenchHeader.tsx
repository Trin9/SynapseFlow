import { AlignLeft } from 'lucide-react'

interface ExecutionWorkbenchHeaderProps {
  isReview: boolean
  workflowName: string
  setWorkflowName: (name: string) => void
  activeExecutionId: string | null
  error: string | null
  theme: 'light' | 'dark'
  toggleTheme: () => void
  showLibrary: boolean
  setShowLibrary: (show: boolean) => void
  showHistory: boolean
  setShowHistory: (show: boolean) => void
  showTriggerCtx: boolean
  onToggleTriggerCtx: () => void
  nodesCount: number
  isRunning: boolean
  onSave: () => void
  onClear: () => void
  onRun: () => void
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
  showLibrary,
  setShowLibrary,
  showHistory,
  setShowHistory,
  showTriggerCtx,
  onToggleTriggerCtx,
  nodesCount,
  isRunning,
  onSave,
  onClear,
  onRun,
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
        {isReview && activeExecutionId && (
          <span className="text-[10px] text-cyan-200 bg-cyan-500/15 border border-cyan-500/30 px-1.5 py-0.5 rounded font-mono">
            {activeExecutionId.slice(0, 12)}
          </span>
        )}
      </div>

      {/* Center: Mode toggle + Trigger Context */}
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
              isReview ? 'bg-blue-500 text-white' : 'text-slate-300 hover:bg-slate-800'
            }`}
          >
            Execution
          </button>
        </div>

        {isReview && (
          <button
            onClick={onToggleTriggerCtx}
            className={`px-2 py-1 text-[11px] rounded border transition-colors flex items-center gap-1 ${
              showTriggerCtx
                ? 'text-amber-300 border-amber-700 bg-amber-500/10'
                : 'border-slate-700 text-slate-300 hover:bg-slate-800'
            }`}
            title={showTriggerCtx ? 'Hide trigger context' : 'Show trigger context'}
          >
            <AlignLeft className="w-3 h-3" />
            <span className="hidden sm:inline">Trigger</span>
          </button>
        )}
      </div>

      {/* Right: actions */}
      <div className="ml-auto flex items-center gap-1.5">
        {error && (
          <span className="text-[11px] text-red-200 bg-red-500/20 border border-red-500/40 px-2 py-0.5 rounded">
            {error}
          </span>
        )}

        {!isReview && (
          <>
            <button
              onClick={() => setShowLibrary(!showLibrary)}
              className={`px-2 py-1 text-[11px] rounded border transition-colors ${
                showLibrary
                  ? 'bg-slate-200 text-slate-900 border-slate-200'
                  : 'border-slate-700 text-slate-200 hover:bg-slate-800'
              }`}
            >
              Load
            </button>
            <button
              onClick={onClear}
              disabled={isRunning || nodesCount === 0}
              className="px-2 py-1 text-[11px] rounded border border-slate-700 text-slate-200 hover:bg-slate-800 disabled:opacity-40"
            >
              Clear
            </button>
            <button
              onClick={onRun}
              disabled={isRunning || nodesCount === 0}
              className="px-2.5 py-1 text-[11px] rounded bg-emerald-600 text-white hover:bg-emerald-500 disabled:opacity-40 font-medium"
            >
              {isRunning ? 'Running...' : 'Run'}
            </button>
          </>
        )}

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

        {!isReview && (
          <input
            type="text"
            value={workflowName}
            onChange={(e) => setWorkflowName(e.target.value)}
            className="px-2 py-1 text-[11px] rounded bg-slate-900 border border-slate-700 text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-400 w-36"
          />
        )}

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
