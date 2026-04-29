// M5.1 — Human Review panel: audit trail timeline + action buttons.
//
// Rendered as a full-width strip at the bottom of ForensicDossierDrawer when
// either (a) dossier.human_audit_trail has at least one entry, or (b) the
// episode currently awaits human review (isHumanInLoop = status "escalated"
// or verdict confidence "low").
//
// Layout:
//   ┌─ HUMAN REVIEW  [status badge] ──────────────────────────────────┐
//   │  [audit entry] → [audit entry] → …  (horizontal, scrollable)    │
//   ├──────────────────────────────────────────────────────────────────┤
//   │  ⚠ AI needs human intervention  [Approve] [Override] [Abort]    │
//   └──────────────────────────────────────────────────────────────────┘
//
// The action row is only rendered when isHumanInLoop. The timeline row is
// only rendered when there are audit entries. Either or both may be present.
import { formatDate } from './_shared'
import type { Episode } from '@/types/episode'
import type { EpisodeDossierView, HumanAuditEntryView } from '@/types/workspace'

// ─── Action metadata ──────────────────────────────────────────────────────

type ActionMeta = { label: string; color: string; dot: string }

const ACTION_META: Record<string, ActionMeta> = {
  resumed: {
    label: 'Approved & Resumed',
    color: 'text-green-700 bg-green-50 border-green-200',
    dot: 'bg-green-500',
  },
  state_override: {
    label: 'State Overridden',
    color: 'text-blue-700 bg-blue-50 border-blue-200',
    dot: 'bg-blue-500',
  },
  aborted: {
    label: 'Aborted',
    color: 'text-red-700 bg-red-50 border-red-200',
    dot: 'bg-red-500',
  },
  review_requested: {
    label: 'Escalated for Review',
    color: 'text-amber-700 bg-amber-50 border-amber-200',
    dot: 'bg-amber-500',
  },
  evidence_marked_invalid: {
    label: 'Evidence Invalidated',
    color: 'text-orange-700 bg-orange-50 border-orange-200',
    dot: 'bg-orange-500',
  },
  handle_injected: {
    label: 'Handle Injected',
    color: 'text-violet-700 bg-violet-50 border-violet-200',
    dot: 'bg-violet-400',
  },
  hypothesis_corrected: {
    label: 'Hypothesis Corrected',
    color: 'text-teal-700 bg-teal-50 border-teal-200',
    dot: 'bg-teal-500',
  },
}

const FALLBACK_META: ActionMeta = {
  label: '',
  color: 'text-gray-600 bg-gray-50 border-gray-200',
  dot: 'bg-gray-400',
}

// ─── Single audit entry chip ──────────────────────────────────────────────

function AuditEntry({ entry }: { entry: HumanAuditEntryView }) {
  const meta = ACTION_META[entry.action] ?? { ...FALLBACK_META, label: entry.action }
  const tooltip = [
    entry.detail ? entry.detail : null,
    entry.actor ? `by ${entry.actor}` : null,
    formatDate(entry.timestamp),
  ]
    .filter(Boolean)
    .join(' · ')

  return (
    <div
      title={tooltip}
      className={`flex items-center gap-2 px-3 py-1.5 rounded-lg border text-xs shrink-0 cursor-default select-none ${meta.color}`}
    >
      <span className={`w-2 h-2 rounded-full shrink-0 ${meta.dot}`} />
      <span className="font-semibold whitespace-nowrap">{meta.label}</span>
      {entry.actor && (
        <span className="opacity-60 whitespace-nowrap">by {entry.actor}</span>
      )}
      <span className="opacity-40 font-mono whitespace-nowrap tabular-nums">
        {formatDate(entry.timestamp)}
      </span>
      {entry.detail && (
        <span className="italic opacity-60 max-w-[160px] truncate">{entry.detail}</span>
      )}
    </div>
  )
}

// ─── Panel ────────────────────────────────────────────────────────────────

export interface HumanReviewPanelProps {
  ep: Episode
  dossier: EpisodeDossierView | null
  reviewLoading: boolean
  reviewError: string | null
  abortPending: boolean
  onApprove: () => void
  /** Called when the user clicks "Override State" — toggles the override note
   *  popover managed by ForensicDossierDrawer. */
  onOverrideToggle: () => void
  onAbort: () => void
  onAbortCancel: () => void
}

export function HumanReviewPanel({
  ep,
  dossier,
  reviewLoading,
  reviewError,
  abortPending,
  onApprove,
  onOverrideToggle,
  onAbort,
  onAbortCancel,
}: HumanReviewPanelProps) {
  const auditTrail: HumanAuditEntryView[] = dossier?.human_audit_trail ?? []
  const lastAction = auditTrail.length > 0 ? auditTrail[auditTrail.length - 1].action : null
  const reviewResolved =
    lastAction === 'resumed' ||
    lastAction === 'aborted' ||
    lastAction === 'state_override' ||
    lastAction === 'hypothesis_corrected'
  const reviewRequested = lastAction === 'review_requested'

  // CR-013: also treat dossier.display.verdict_label as a resolved signal so that
  // the action row disappears after approve/override/abort even when the episode
  // status has not yet propagated to the frontend.
  const dossierIndicatesResolved =
    dossier?.display?.verdict_label != null && (
      ['Approved', 'Aborted'].some((v) => dossier!.display.verdict_label!.startsWith(v)) ||
      dossier!.display.verdict_label.startsWith('Overridden')
    )

  const isHumanInLoop =
    reviewRequested ||
    (!reviewResolved && !dossierIndicatesResolved &&
      (ep.status === 'escalated' || ep.verdict?.confidence === 'low'))

  // Nothing to show: no history and not waiting for review.
  if (auditTrail.length === 0 && !isHumanInLoop) return null

  return (
    <div className="border-t border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 shrink-0">

      {/* ── Panel header ──────────────────────────────────────────────── */}
      <div className="px-5 py-2 flex items-center gap-2 border-b border-gray-100 dark:border-gray-800">
        <span className="text-[10px] font-bold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
          Human Review
        </span>
        {isHumanInLoop ? (
          <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 uppercase">
            Awaiting Review
          </span>
        ) : (
          auditTrail.length > 0 && (
            <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 uppercase">
              Reviewed
            </span>
          )
        )}
        {auditTrail.length > 0 && (
          <span className="text-[10px] text-gray-400 dark:text-gray-500 ml-1">
            {auditTrail.length} intervention{auditTrail.length !== 1 ? 's' : ''}
          </span>
        )}
      </div>

      {/* ── Audit trail timeline (horizontal, scrollable) ─────────────── */}
      {auditTrail.length > 0 && (
        <div className="px-5 py-2.5 flex items-center gap-2 overflow-x-auto scrollbar-none border-b border-gray-100 dark:border-gray-800">
          {auditTrail.map((entry, i) => (
            <div key={i} className="flex items-center gap-2 shrink-0">
              {i > 0 && <span className="text-gray-300 dark:text-gray-600 text-xs select-none">→</span>}
              <AuditEntry entry={entry} />
            </div>
          ))}
        </div>
      )}

      {/* ── Action buttons (only when awaiting review) ───────────────── */}
      {isHumanInLoop && (
        <div className="px-5 py-3 flex items-center gap-3 flex-wrap">
          <span className="text-sm font-semibold text-amber-600 dark:text-amber-400 flex items-center gap-1.5">
            <span aria-hidden>⚠</span>
            AI needs human intervention
          </span>

          {reviewError && (
            <span className="text-xs text-red-600 dark:text-red-400 font-medium">{reviewError}</span>
          )}

          <div className="flex gap-2 ml-auto flex-wrap">
            <button
              onClick={onApprove}
              disabled={reviewLoading}
              className="px-4 py-1.5 bg-green-500 text-white text-sm font-medium rounded-lg disabled:opacity-50 hover:bg-green-600 transition-colors"
            >
              {reviewLoading ? '…' : 'Approve & Continue'}
            </button>

            <button
              onClick={onOverrideToggle}
              disabled={reviewLoading}
              className="px-4 py-1.5 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 text-sm font-medium rounded-lg disabled:opacity-50 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
            >
              Override State
            </button>

            <button
              onClick={onAbort}
              disabled={reviewLoading}
              className={`px-4 py-1.5 text-sm font-medium rounded-lg transition-colors disabled:opacity-50 ${
                abortPending
                  ? 'bg-red-600 text-white hover:bg-red-700'
                  : 'bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/40'
              }`}
            >
              {abortPending ? 'Confirm Abort' : 'Abort'}
            </button>

            {abortPending && (
              <button
                onClick={onAbortCancel}
                className="text-xs text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 self-center"
              >
                cancel
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
