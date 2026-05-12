// Middle pane of the Forensic Dossier — Causal Bridge & Verdict.
// M4.1: items whose focus_key matches activeFocusKey receive a blue ring.
// Demo-migration: Trust Boundary tone badge per bridge item + Semantic Linkage Rail.
import { Section, resultStyle, confidenceStyle, useTrustMap, trustBadgeStyle } from './_shared'
import type { TrustDescriptor } from './_shared'
import type { Episode, EpisodeVerdict } from '@/types/episode'
import type { EpisodeDossierView, VerdictBridgeItemView } from '@/types/workspace'

// ─── Semantic Linkage Rail ────────────────────────────────────────────────

/**
 * Three-pill visual summarising the active focus chain:
 *   Expected Behavior → Causal Bridge → Verdict
 * Mirrors Demo's SemanticLinkageRail component.
 * Only rendered when activeFocusKey is set and a dossier is available.
 */
function SemanticLinkageRail({
  activeFocusKey,
  dossier,
  trust,
}: {
  activeFocusKey: string
  dossier: EpisodeDossierView
  trust?: TrustDescriptor
}) {
  const expectedItem = dossier.expected_behavior.find((e) => e.focus_key === activeFocusKey)
  const factItem = dossier.runtime_facts.find((f) => f.focus_key === activeFocusKey)
  const verdictLabel = dossier.display.verdict_label ?? 'Verdict'

  const pillBase = 'rounded-full border px-3 py-2 text-center text-xs font-medium transition-all'
  const activePillLeft =
    trust?.tone === 'mixed'
      ? 'border-amber-300 bg-amber-50 text-amber-700'
      : trust?.tone === 'ai_only'
      ? 'border-purple-300 bg-purple-50 text-purple-700'
      : 'border-teal-300 bg-teal-50 text-teal-700'
  const activePillRight = 'border-emerald-300 bg-emerald-50 text-emerald-700'
  const inactivePill   = 'border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-700/50 text-gray-500 dark:text-gray-400'

  return (
    <div className="mt-4 rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-3 space-y-2 shrink-0">
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-bold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
          Semantic Linkage Rail
        </span>
        {trust && (
          <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase ${trustBadgeStyle(trust.tone)}`}>
            {trust.label}
          </span>
        )}
      </div>
      {trust && (
        <p className="text-[11px] text-gray-400 dark:text-gray-500 italic leading-relaxed">{trust.detail}</p>
      )}
      <div className="grid grid-cols-[1fr_auto_1fr_auto_1fr] items-center gap-2 pt-1">
        <div className={`${pillBase} ${expectedItem ? activePillLeft : inactivePill}`}>
          {expectedItem?.title ?? 'Expected behavior'}
        </div>
        <span className={`text-sm font-bold ${activeFocusKey ? 'text-teal-500' : 'text-gray-300 dark:text-gray-600'}`}>›</span>
        <div className={`${pillBase} ${factItem ? activePillLeft : inactivePill}`}>
          {factItem?.title ?? 'Runtime facts'}
        </div>
        <span className={`text-sm font-bold ${activeFocusKey ? 'text-teal-500' : 'text-gray-300 dark:text-gray-600'}`}>›</span>
        <div className={`${pillBase} ${activePillRight}`}>
          {verdictLabel}
        </div>
      </div>
    </div>
  )
}

// ─── Sub-components ───────────────────────────────────────────────────────

function VerdictBlock({ verdict }: { verdict: EpisodeVerdict }) {
  const RESULT_ICONS: Record<string, string> = { pass: '✓', fail: '✗', inconclusive: '~' }
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        {verdict.result && (
          <span
            className={`text-sm font-bold px-3 py-1 rounded-lg border ${resultStyle(verdict.result)}`}
          >
            {RESULT_ICONS[verdict.result] ?? ''} {verdict.result.toUpperCase()}
          </span>
        )}
        {verdict.confidence && (
          <span
            className={`text-xs font-semibold px-2.5 py-1 rounded ${confidenceStyle(verdict.confidence)}`}
          >
            {verdict.confidence} confidence
          </span>
        )}
      </div>

      {verdict.conclusion && (
        <p className="text-sm text-gray-800 dark:text-gray-100 leading-relaxed font-medium bg-gray-50 dark:bg-gray-800 p-3 rounded-lg border border-gray-100 dark:border-gray-700">
          {verdict.conclusion}
        </p>
      )}

      {(verdict.causal_chain ?? []).length > 0 && (
        <div>
          <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 block mb-2">Causal Chain</span>
          <ol className="space-y-1 pl-1">
            {verdict.causal_chain!.map((item, i) => (
              <li key={i} className="text-sm text-gray-600 dark:text-gray-400 flex gap-2">
                <span className="text-blue-500 font-bold shrink-0">→</span>
                <span>{item}</span>
              </li>
            ))}
          </ol>
        </div>
      )}

      {(verdict.gaps ?? []).length > 0 && (
        <div className="bg-amber-50 dark:bg-amber-900/20 p-3 rounded-lg border border-amber-100 dark:border-amber-800">
          <span className="text-xs font-bold text-amber-700 dark:text-amber-400 uppercase block mb-1">
            Gaps / Missing Info
          </span>
          <ul className="space-y-1">
            {verdict.gaps!.map((gap, i) => (
              <li key={i} className="text-sm text-amber-800 dark:text-amber-300 flex gap-2">
                <span className="text-amber-500 shrink-0">!</span>
                <span>{gap}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {(verdict.recommendations ?? []).length > 0 && (
        <div className="bg-blue-50 dark:bg-blue-900/20 p-3 rounded-lg border border-blue-100 dark:border-blue-800">
          <span className="text-xs font-bold text-blue-700 dark:text-blue-400 uppercase block mb-1">
            Recommendations
          </span>
          <ul className="space-y-1">
            {verdict.recommendations!.map((rec, i) => (
              <li key={i} className="flex gap-2 items-start">
                <span className="text-blue-500 font-bold shrink-0 mt-0.5">•</span>
                <span className="text-sm text-blue-900 dark:text-blue-300">{rec}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

// M4.1 — passive highlight when focus_key matches
// Demo-migration — trust tone badge + detail text
function VerdictBridgeItem({
  item,
  activeFocusKey,
  trust,
}: {
  item: VerdictBridgeItemView
  activeFocusKey: string | null
  trust?: TrustDescriptor
}) {
  const isActive = item.focus_key != null && item.focus_key === activeFocusKey

  return (
    <div
      data-focus-key={item.focus_key ?? undefined}
      className={[
        'border border-gray-200 dark:border-gray-700 rounded-lg p-3 space-y-1.5 bg-white dark:bg-gray-800 transition-all',
        isActive ? 'ring-2 ring-blue-400' : '',
        trust?.tone === 'mixed'   ? 'bg-amber-50/30 dark:bg-amber-900/10' : '',
        trust?.tone === 'ai_only' ? 'bg-purple-50/30 dark:bg-purple-900/10' : '',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-sm font-medium text-gray-800 dark:text-gray-100">{item.title}</span>
        {trust && (
          <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase shrink-0 ${trustBadgeStyle(trust.tone)}`}>
            {trust.label}
          </span>
        )}
      </div>
      <p className="text-xs text-gray-600 dark:text-gray-400 leading-relaxed">{item.body}</p>
      {trust && (
        <p className="text-[11px] text-gray-400 dark:text-gray-500 italic">{trust.detail}</p>
      )}
    </div>
  )
}

// ─── Column ───────────────────────────────────────────────────────────────

interface VerdictBridgeColumnProps {
  ep: Episode
  dossier: EpisodeDossierView | null
  /** M4.1 — currently highlighted focus_key */
  activeFocusKey: string | null
}

export function VerdictBridgeColumn({
  ep,
  dossier,
  activeFocusKey,
}: VerdictBridgeColumnProps) {
  const hint = activeFocusKey ? `linked: ${activeFocusKey}` : undefined
  const trustMap = useTrustMap(dossier?.expected_behavior ?? [])

  if (dossier) {
    const verdictBridge = dossier.verdict_bridge ?? []
    return (
      <Section title={`2. Causal Bridge (${verdictBridge.length})`} hint={hint}>
        {dossier.display.verdict_label && (
          <div className="flex items-center gap-2 pb-2 border-b border-gray-100 dark:border-gray-700">
            <span
              className={`text-sm font-bold px-3 py-1 rounded-lg border ${resultStyle(
                dossier.display.verdict === 'pass'
                  ? 'pass'
                  : dossier.display.verdict === 'fail'
                  ? 'fail'
                  : dossier.display.verdict === 'inconclusive'
                  ? 'inconclusive'
                  : undefined,
              )}`}
            >
              {dossier.display.verdict_label}
            </span>
            {dossier.display.summary && (
              <span className="text-xs text-gray-500 dark:text-gray-400 leading-relaxed">
                {dossier.display.summary}
              </span>
            )}
          </div>
        )}
        {verdictBridge.length > 0 ? (
          verdictBridge.map((item) => (
            <VerdictBridgeItem
              key={item.id}
              item={item}
              activeFocusKey={activeFocusKey}
              trust={item.focus_key ? trustMap[item.focus_key] : undefined}
            />
          ))
        ) : (
          <div className="text-center py-10 text-gray-400 dark:text-gray-500 text-sm">
            No causal bridge entries.
          </div>
        )}
        {activeFocusKey && (
          <SemanticLinkageRail
            activeFocusKey={activeFocusKey}
            dossier={dossier}
            trust={trustMap[activeFocusKey]}
          />
        )}
      </Section>
    )
  }

  return (
    <Section title="2. Causal Bridge & Verdict">
      {ep.verdict ? (
        <VerdictBlock verdict={ep.verdict} />
      ) : (
        <div className="flex flex-col items-center justify-center h-full text-gray-400 dark:text-gray-500 gap-3">
          <div className="w-8 h-8 rounded-full border-2 border-gray-300 dark:border-gray-600 border-t-blue-500 animate-spin" />
          <p className="text-sm font-medium animate-pulse">AI is determining verdict...</p>
        </div>
      )}
    </Section>
  )
}
