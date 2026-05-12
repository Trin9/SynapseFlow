// Left pane of the Forensic Dossier — Expected Logic & Context.
// M4.1: clicking an item with a focus_key toggles cross-column highlight.
import { Section, prettyJSON } from './_shared'
import type {
  Episode,
  ActionContext,
  InvestigationContext,
  EpisodeHandle,
} from '@/types/episode'
import type { EpisodeDossierView, ExpectedBehaviorView } from '@/types/workspace'

// ─── Sub-components ───────────────────────────────────────────────────────

function ActionBlock({ ctx }: { ctx: ActionContext }) {
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 w-14 shrink-0">Name</span>
        <span className="text-sm font-mono font-semibold text-gray-800 dark:text-gray-100">{ctx.action_name}</span>
        {ctx.action_type && (
          <span className="text-[10px] px-1.5 py-0.5 bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 border border-blue-100 dark:border-blue-800 rounded uppercase font-bold">
            {ctx.action_type}
          </span>
        )}
      </div>
      {ctx.action_input && (
        <div>
          <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 block mb-1">Input</span>
          <pre className="text-xs font-mono bg-gray-50 dark:bg-gray-800 rounded p-3 overflow-x-auto border border-gray-100 dark:border-gray-700 text-gray-700 dark:text-gray-300 whitespace-pre-wrap max-h-48">
            {prettyJSON(ctx.action_input)}
          </pre>
        </div>
      )}
      {ctx.action_output && (
        <div>
          <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 block mb-1">Output</span>
          <pre className="text-xs font-mono bg-gray-50 dark:bg-gray-800 rounded p-3 overflow-x-auto border border-gray-100 dark:border-gray-700 text-gray-700 dark:text-gray-300 whitespace-pre-wrap max-h-48">
            {prettyJSON(ctx.action_output)}
          </pre>
        </div>
      )}
    </div>
  )
}

function InvestigationBlock({ ctx }: { ctx: InvestigationContext }) {
  return (
    <div className="space-y-3">
      <div>
        <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 block mb-1">Hypothesis</span>
        <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed">{ctx.hypothesis}</p>
      </div>
      {(ctx.known_signals ?? []).length > 0 && (
        <div>
          <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 block mb-1">Known Signals</span>
          <ul className="space-y-0.5">
            {ctx.known_signals!.map((s, i) => (
              <li key={i} className="text-xs text-gray-600 dark:text-gray-400 flex gap-1.5">
                <span className="text-gray-400 dark:text-gray-500 shrink-0">›</span>
                <span>{s}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
      {ctx.retrieval_plan && (
        <div>
          <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 block mb-1">Retrieval Plan</span>
          <pre className="text-xs font-mono text-gray-600 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 p-2.5 rounded border border-gray-100 dark:border-gray-700 whitespace-pre-wrap">
            {ctx.retrieval_plan}
          </pre>
        </div>
      )}
    </div>
  )
}

function HandlesBlock({ handles }: { handles: EpisodeHandle[] }) {
  return (
    <div className="mt-4">
      <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider block mb-2">
        Extracted Handles
      </span>
      <table className="w-full text-xs border-collapse">
        <thead>
          <tr className="text-[10px] text-gray-400 dark:text-gray-500 uppercase border-b border-gray-100 dark:border-gray-700">
            <th className="text-left font-semibold pb-1 pr-2">Type</th>
            <th className="text-left font-semibold pb-1 pr-2">Value</th>
            <th className="text-left font-semibold pb-1">Source</th>
          </tr>
        </thead>
        <tbody>
          {handles.map((h, i) => (
            <tr key={i} className="border-b border-gray-50 dark:border-gray-800 last:border-0">
              <td className="py-1 pr-2 font-semibold text-gray-500 dark:text-gray-400 align-top">{h.type}</td>
              <td className="py-1 pr-2 font-mono text-gray-800 dark:text-gray-100 align-top">{h.value}</td>
              <td className="py-1 text-gray-500 dark:text-gray-400 align-top">{h.source}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// M4.1 — click to set/clear the cross-column focus key
function ExpectedBehaviorItem({
  item,
  activeFocusKey,
  onFocusKey,
}: {
  item: ExpectedBehaviorView
  activeFocusKey: string | null
  onFocusKey: (key: string | null) => void
}) {
  const isActive = item.focus_key != null && item.focus_key === activeFocusKey
  const isClickable = item.focus_key != null

  const sourceColor =
    item.source_type === 'sop'
      ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
      : 'bg-amber-50 text-amber-700 border border-amber-200'

  return (
    <div
      data-focus-key={item.focus_key ?? undefined}
      onClick={
        isClickable
          ? () => onFocusKey(isActive ? null : item.focus_key!)
          : undefined
      }
      className={[
        'border border-gray-200 dark:border-gray-700 rounded-lg p-3 space-y-1.5 bg-white dark:bg-gray-800 transition-all',
        isClickable ? 'cursor-pointer hover:border-blue-300 dark:hover:border-blue-600' : '',
        isActive ? 'ring-2 ring-blue-400' : '',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <div className="flex items-center gap-2 flex-wrap">
        {item.source_type && (
          <span
            className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase shrink-0 ${sourceColor}`}
          >
            {item.source_label ?? item.source_type}
          </span>
        )}
        <span className="text-sm font-medium text-gray-800 dark:text-gray-100">{item.title}</span>
      </div>
      <p className="text-xs text-gray-600 dark:text-gray-400 leading-relaxed">{item.body}</p>
      {item.source_detail && (
        <p className="text-[11px] text-gray-400 dark:text-gray-500 italic">{item.source_detail}</p>
      )}
    </div>
  )
}

// ─── Column ───────────────────────────────────────────────────────────────

interface ExpectedBehaviorColumnProps {
  ep: Episode
  dossier: EpisodeDossierView | null
  /** M4.1 — currently highlighted focus_key (null = none) */
  activeFocusKey: string | null
  /** M4.1 — called when user clicks a linkable item */
  onFocusKey: (key: string | null) => void
}

export function ExpectedBehaviorColumn({
  ep,
  dossier,
  activeFocusKey,
  onFocusKey,
}: ExpectedBehaviorColumnProps) {
  const hint = activeFocusKey ? `linked: ${activeFocusKey}` : undefined

  if (dossier) {
    const expected = dossier.expected_behavior ?? []
    return (
      <Section title={`1. Expected Logic (${expected.length})`} hint={hint}>
        {expected.length > 0 ? (
          expected.map((item) => (
            <ExpectedBehaviorItem
              key={item.id}
              item={item}
              activeFocusKey={activeFocusKey}
              onFocusKey={onFocusKey}
            />
          ))
        ) : (
          <div className="text-center py-10 text-gray-400 dark:text-gray-500 text-sm">
            No expected behaviors recorded.
          </div>
        )}
      </Section>
    )
  }

  return (
    <Section title="1. Expected Logic & Context">
      {ep.action_context && <ActionBlock ctx={ep.action_context} />}
      {ep.investigation_context && <InvestigationBlock ctx={ep.investigation_context} />}
      {(ep.handles ?? []).length > 0 && <HandlesBlock handles={ep.handles!} />}
    </Section>
  )
}
