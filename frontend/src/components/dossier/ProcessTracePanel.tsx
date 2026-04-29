// M5.1 — Process Trace Panel: compact horizontal execution stepper.
//
// Renders visible_process_trace from the current ReplaySliceView as a
// scrollable horizontal timeline. Each step shows its stage, title, and
// status dot. Steps with stage "Human Review" are specially linked to
// dossier.human_audit_trail entries sharing the same node id.
//
// Human Review steps display:
//   - Amber highlight styling
//   - A badge showing how many audit trail entries belong to this step
//   - A tooltip summarising the matching audit actions (title attribute)
//
// Placement: mounted between the ReplaySlider and the 3-pane layout in
// ForensicDossierDrawer so it stays replay-percent aware.
import type { EpisodeDossierView, ProcessTraceEntryView, HumanAuditEntryView } from '@/types/workspace'

// ─── Status dot ───────────────────────────────────────────────────────────

const STATUS_DOT: Record<string, string> = {
  success: 'bg-green-500',
  failed: 'bg-red-500',
  running: 'bg-blue-500 animate-pulse',
  pending: 'bg-gray-300',
}

// ─── Stage pill ───────────────────────────────────────────────────────────

const STAGE_PILL: Record<string, string> = {
  'Human Review':    'bg-amber-100 text-amber-700 border-amber-200',
  'Circuit Breaker': 'bg-red-100 text-red-700 border-red-200',
  'Verdict':         'bg-violet-100 text-violet-700 border-violet-200',
  'Action':          'bg-blue-100 text-blue-700 border-blue-200',
}
const DEFAULT_STAGE_PILL = 'bg-gray-100 text-gray-600 border-gray-200'

// ─── Single step card ─────────────────────────────────────────────────────

interface StepCardProps {
  entry: ProcessTraceEntryView
  /** Audit entries that match this step's node id (only for Human Review steps). */
  linkedAudit: HumanAuditEntryView[]
}

function StepCard({ entry, linkedAudit }: StepCardProps) {
  const isHumanReview = entry.stage === 'Human Review'
  const dot = STATUS_DOT[entry.status] ?? 'bg-gray-300'
  const stagePill = STAGE_PILL[entry.stage] ?? DEFAULT_STAGE_PILL

  // Tooltip: combine detail + audit summary
  const auditSummary = linkedAudit
    .map((a) => `${a.action}${a.detail ? `: ${a.detail}` : ''}`)
    .join(' | ')
  const tooltip = [entry.detail, auditSummary].filter(Boolean).join('\n')

  return (
    <div
      title={tooltip || undefined}
      className={[
        'flex flex-col gap-1 px-3 py-2 rounded-lg border shrink-0 min-w-[140px] max-w-[200px] cursor-default select-none transition-all',
        isHumanReview
          ? 'bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-800'
          : 'bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700',
      ].join(' ')}
    >
      {/* Stage + status dot */}
      <div className="flex items-center justify-between gap-2">
        <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded border uppercase ${stagePill}`}>
          {entry.stage}
        </span>
        <span className={`w-2 h-2 rounded-full shrink-0 ${dot}`} />
      </div>

      {/* Title */}
      <span className="text-xs font-medium text-gray-800 dark:text-gray-100 leading-snug truncate">
        {entry.title}
      </span>

      {/* Chips (node type tags etc.) */}
      {(entry.chips ?? []).length > 0 && (
        <div className="flex gap-1 flex-wrap mt-0.5">
          {entry.chips!.map((chip, i) => (
            <span
              key={i}
              className="text-[9px] bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 px-1.5 py-0.5 rounded font-mono border border-gray-200 dark:border-gray-600"
            >
              {chip}
            </span>
          ))}
        </div>
      )}

      {/* Human Review audit badge */}
      {isHumanReview && linkedAudit.length > 0 && (
        <div className="flex items-center gap-1 mt-0.5">
          <span className="w-1.5 h-1.5 rounded-full bg-amber-500 shrink-0" />
          <span className="text-[10px] text-amber-700 font-semibold">
            {linkedAudit.length} audit {linkedAudit.length === 1 ? 'entry' : 'entries'}
          </span>
        </div>
      )}
    </div>
  )
}

// ─── Panel ────────────────────────────────────────────────────────────────

interface ProcessTracePanelProps {
  dossier: EpisodeDossierView | null
  /** visible_process_trace from the current replay slice — may be empty. */
  visibleTrace: ProcessTraceEntryView[]
}

export function ProcessTracePanel({ dossier, visibleTrace }: ProcessTracePanelProps) {
  if (visibleTrace.length === 0) return null

  const auditTrail: HumanAuditEntryView[] = dossier?.human_audit_trail ?? []

  return (
    <div className="border-b border-gray-100 dark:border-gray-800 bg-gray-50 dark:bg-gray-800/50 shrink-0">
      {/* Header */}
      <div className="px-5 pt-2 pb-1 flex items-center gap-2">
        <span className="text-[10px] font-bold text-gray-400 dark:text-gray-500 uppercase tracking-wider">
          Process Trace
        </span>
        <span className="text-[10px] text-gray-300 dark:text-gray-600">
          {visibleTrace.length} step{visibleTrace.length !== 1 ? 's' : ''}
        </span>
      </div>

      {/* Horizontal scrollable step bar */}
      <div className="px-5 pb-3 flex items-center gap-2 overflow-x-auto scrollbar-none">
        {visibleTrace.map((entry, i) => {
          const rawNodeId = entry.id.replace(/_human$/, '')
          const linkedAudit = entry.stage === 'Human Review'
            ? auditTrail.filter((a) => a.node_id === rawNodeId)
            : []
          return (
            <div key={entry.id} className="flex items-center gap-2 shrink-0">
              {i > 0 && (
                <span className="text-gray-300 dark:text-gray-600 text-xs select-none">→</span>
              )}
              <StepCard entry={entry} linkedAudit={linkedAudit} />
            </div>
          )
        })}
      </div>
    </div>
  )
}
