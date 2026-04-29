import type {
  Episode,
  EpisodeEvidence,
  EpisodeVerdict,
  EpisodeHandle,
  EvidenceCollectorSpec,
  EpisodeResult,
  EpisodeConfidence,
} from '@/types/episode'
// ─── helpers ──────────────────────────────────────────────────────────────

function confidenceColor(confidence?: EpisodeConfidence): string {
  switch (confidence) {
    case 'high':   return 'text-green-700 bg-green-100'
    case 'medium': return 'text-yellow-700 bg-yellow-100'
    case 'low':    return 'text-red-700 bg-red-100'
    default:       return 'text-gray-500 bg-gray-100'
  }
}

function resultColor(result?: EpisodeResult): string {
  switch (result) {
    case 'pass':         return 'bg-green-100 text-green-700'
    case 'fail':         return 'bg-red-100 text-red-700'
    case 'inconclusive': return 'bg-yellow-100 text-yellow-700'
    default:             return 'bg-gray-100 text-gray-500'
  }
}

function evidenceBadgeColor(type: EpisodeEvidence['type']): string {
  switch (type) {
    case 'fact':             return 'bg-gray-100 text-gray-700'
    case 'inference':        return 'bg-blue-100 text-blue-700'
    case 'human_correction': return 'bg-amber-100 text-amber-700'
  }
}

function collectorTypeColor(t?: string): string {
  switch (t) {
    case 'script':      return 'bg-emerald-50 text-emerald-700 border-emerald-200'
    case 'llm_query':   return 'bg-purple-50 text-purple-700 border-purple-200'
    case 'log_query':   return 'bg-indigo-50 text-indigo-600 border-indigo-200'
    case 'db_query':    return 'bg-teal-50 text-teal-600 border-teal-200'
    case 'api_call':    return 'bg-violet-50 text-violet-600 border-violet-200'
    case 'code_search': return 'bg-orange-50 text-orange-600 border-orange-200'
    case 'manual':      return 'bg-amber-50 text-amber-600 border-amber-200'
    default:            return 'bg-gray-50 text-gray-500 border-gray-200'
  }
}

function episodeTypeLabel(type: Episode['episode_type']): string {
  switch (type) {
    case 'action_verification': return 'Action Verification'
    case 'investigation_step':  return 'Investigation Step'
  }
}

function formatDate(iso?: string): string {
  if (!iso) return '—'
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

function handleTypeShort(t: string): string {
  switch (t) {
    case 'order_id':        return 'order'
    case 'session_id':      return 'session'
    case 'trace_id':        return 'trace'
    case 'request_id':      return 'req'
    case 'git_ref':         return 'git'
    case 'deploy_revision': return 'deploy'
    case 'file_location':   return 'file'
    case 'pod_name':        return 'pod'
    default:                return t
  }
}

// ─── CollectorSpecRow ──────────────────────────────────────────────────────

function CollectorSpecRow({ spec }: { spec: EvidenceCollectorSpec }) {
  const colorClass = collectorTypeColor(spec.collector_type)
  return (
    <div className="mt-1 flex flex-wrap gap-1 items-start">
      {spec.collector_type && (
        <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded border uppercase shrink-0 ${colorClass}`}>
          {spec.collector_type.replace('_', ' ')}
        </span>
      )}
      {spec.params && Object.keys(spec.params).length > 0 && (
        <span className="text-[10px] font-mono text-gray-500 bg-gray-50 rounded px-1.5 py-0.5 border border-gray-100 break-all">
          {Object.entries(spec.params)
            .map(([k, v]) => `${k}: ${String(v)}`)
            .join('  ·  ')}
        </span>
      )}
      {spec.raw_command && !spec.params && (
        <span className="text-[10px] font-mono text-gray-400 bg-gray-50 rounded px-1.5 py-0.5 border border-gray-100 break-all max-w-full">
          {spec.raw_command.length > 120
            ? spec.raw_command.slice(0, 120) + '…'
            : spec.raw_command}
        </span>
      )}
    </div>
  )
}

// ─── EvidenceRow ───────────────────────────────────────────────────────────

function EvidenceRow({ ev }: { ev: EpisodeEvidence }) {
  return (
    <div className="py-1.5 border-b border-gray-50 last:border-0">
      <div className="flex items-center gap-2">
        <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase shrink-0 ${evidenceBadgeColor(ev.type)}`}>
          {ev.type}
        </span>
        <span className="text-xs font-medium text-gray-700 truncate">{ev.label ?? ev.node_id}</span>
        <span className="text-[10px] text-gray-400 ml-auto shrink-0">{ev.node_type}</span>
      </div>
      {ev.collector_spec && <CollectorSpecRow spec={ev.collector_spec} />}
      {ev.content && (
        <pre className="mt-1 text-[10px] text-gray-500 bg-gray-50 rounded p-1.5 overflow-x-auto max-h-20 font-mono whitespace-pre-wrap">
          {ev.content.length > 400 ? ev.content.slice(0, 400) + '…' : ev.content}
        </pre>
      )}
      {ev.content_ref && !ev.content && (
        <span className="text-[10px] text-blue-500 mt-0.5 block">{ev.content_ref}</span>
      )}
    </div>
  )
}

// ─── HandlesSection ────────────────────────────────────────────────────────

function HandlesSection({ handles }: { handles: EpisodeHandle[] }) {
  if (!handles.length) return null
  return (
    <div className="mb-2">
      <span className="text-[10px] font-semibold text-gray-500 uppercase">Handles</span>
      <div className="mt-1 flex flex-wrap gap-1">
        {handles.map((h, i) => (
          <span
            key={i}
            className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded border border-gray-200 bg-gray-50 text-[10px]"
            title={`source: ${h.source}`}
          >
            <span className="font-semibold text-gray-500">{handleTypeShort(h.type)}</span>
            <span className="font-mono text-gray-700">{h.value}</span>
          </span>
        ))}
      </div>
    </div>
  )
}

// ─── VerdictSection ────────────────────────────────────────────────────────

function VerdictSection({ verdict }: { verdict: EpisodeVerdict }) {
  return (
    <div className="mt-3 pt-3 border-t border-gray-100">
      <div className="flex items-center gap-2 mb-1 flex-wrap">
        <span className="text-[10px] font-bold uppercase text-gray-500">Verdict</span>
        {verdict.result && (
          <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${resultColor(verdict.result)}`}>
            {verdict.result}
          </span>
        )}
        {verdict.confidence && (
          <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${confidenceColor(verdict.confidence)}`}>
            {verdict.confidence} confidence
          </span>
        )}
        {verdict.decided_by && (
          <span className="text-[10px] text-gray-400 ml-auto">by {verdict.decided_by}</span>
        )}
      </div>

      {verdict.conclusion && (
        <p className="text-xs text-gray-700 leading-relaxed">{verdict.conclusion}</p>
      )}

      {(verdict.causal_chain ?? []).length > 0 && (
        <div className="mt-2">
          <span className="text-[10px] font-semibold text-gray-500 uppercase">Causal chain</span>
          <ul className="mt-1 space-y-0.5">
            {verdict.causal_chain!.map((item, i) => (
              <li key={i} className="text-xs text-gray-600 flex gap-1">
                <span className="text-gray-400">›</span>
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {(verdict.gaps ?? []).length > 0 && (
        <div className="mt-2">
          <span className="text-[10px] font-semibold text-amber-600 uppercase">Gaps / missing info</span>
          <ul className="mt-1 space-y-0.5">
            {verdict.gaps!.map((gap, i) => (
              <li key={i} className="text-xs text-amber-700 flex gap-1">
                <span className="text-amber-400">!</span>
                <span>{gap}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {(verdict.recommendations ?? []).length > 0 && (
        <div className="mt-2">
          <span className="text-[10px] font-semibold text-blue-600 uppercase">Recommendations</span>
          <ul className="mt-1 space-y-0.5">
            {verdict.recommendations!.map((rec, i) => (
              <li key={i} className="text-xs text-blue-700 flex gap-1">
                <span className="text-blue-400">→</span>
                <span>{rec}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

// ─── EpisodeCard ───────────────────────────────────────────────────────────

interface EpisodeCardProps {
  episode: Episode
  defaultOpen?: boolean
  onOpenDetail?: (ep: Episode) => void
}

export function EpisodeCard({ episode, defaultOpen = false, onOpenDetail }: EpisodeCardProps) {
  const evidenceCount = episode.evidence?.length ?? 0
  const handles = episode.handles ?? []
  const hasVerdict = !!episode.verdict
  const result = episode.verdict?.result

  // Summary line for collapsed header
  const actionLabel = episode.action_context?.action_name ?? ''

  return (
    <div className="border border-gray-200 rounded-md bg-white text-sm overflow-hidden">
      <details open={defaultOpen}>
        <summary className="list-none cursor-pointer select-none">
          <div className="px-3 py-2 flex items-center gap-2 hover:bg-gray-50 transition-colors flex-wrap">
            {/* Episode type badge */}
            <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase shrink-0 ${
              episode.episode_type === 'action_verification'
                ? 'bg-blue-100 text-blue-700'
                : 'bg-violet-100 text-violet-700'
            }`}>
              {episodeTypeLabel(episode.episode_type)}
            </span>

            {/* Verdict result or open badge */}
            <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded shrink-0 ${
              hasVerdict ? resultColor(result) : 'bg-gray-100 text-gray-500'
            }`}>
              {hasVerdict ? (result ?? 'verdict') : 'open'}
            </span>

            {/* Action name if present */}
            {actionLabel && (
              <span className="text-xs text-gray-600 font-medium truncate max-w-[160px]">
                {actionLabel}
              </span>
            )}

            {/* Handle pills (first 2 only, save space) */}
            {handles.slice(0, 2).map((h, i) => (
              <span key={i} className="text-[9px] font-mono px-1 py-0.5 bg-gray-100 text-gray-500 rounded shrink-0">
                {handleTypeShort(h.type)}:{h.value.slice(0, 10)}{h.value.length > 10 ? '…' : ''}
              </span>
            ))}

            <span className="text-xs text-gray-500 ml-auto shrink-0">
              {evidenceCount}ev
            </span>
            <span className="text-[10px] text-gray-400 shrink-0"
              title={episode.id}>
              {episode.id.slice(0, 8)}…
            </span>
            {onOpenDetail && (
              <button
                onClick={(e) => { e.preventDefault(); e.stopPropagation(); onOpenDetail(episode) }}
                className="text-[10px] text-blue-500 hover:text-blue-700 font-medium px-1.5 py-0.5 rounded hover:bg-blue-50 shrink-0 transition-colors"
              >
                View →
              </button>
            )}
            <span className="text-xs text-gray-400 shrink-0">›</span>
          </div>
        </summary>

        {/* ── Detail body (Scope 3) ── */}
        <div className="px-3 pb-3 border-t border-gray-50">

          {/* Timestamps */}
          <div className="text-[10px] text-gray-400 mt-2 mb-2">
            Created {formatDate(episode.created_at)}
            {episode.concluded_at && (
              <span className="ml-2">· Concluded {formatDate(episode.concluded_at)}</span>
            )}
            {episode.status && (
              <span className={`ml-2 font-semibold ${
                episode.status === 'converged' ? 'text-green-600' :
                episode.status === 'escalated' ? 'text-amber-600' :
                episode.status === 'failed' ? 'text-red-600' : 'text-gray-500'
              }`}>
                [{episode.status}]
              </span>
            )}
          </div>

          {/* Section: Handles */}
          {handles.length > 0 && <HandlesSection handles={handles} />}

          {/* Section: Evidence */}
          {evidenceCount > 0 && (
            <div>
              <span className="text-[10px] font-semibold text-gray-500 uppercase">
                Evidence ({evidenceCount})
              </span>
              <div className="mt-1">
                {episode.evidence!.map((ev) => (
                  <EvidenceRow key={ev.id} ev={ev} />
                ))}
              </div>
            </div>
          )}

          {/* Section: Verdict */}
          {episode.verdict && <VerdictSection verdict={episode.verdict} />}
        </div>
      </details>
    </div>
  )
}
