import type { Episode, EpisodeEvidence, EpisodeVerdict } from '@/types/episode'

// ─── helpers ──────────────────────────────────────────────────────────────

function confidencePct(confidence: number): string {
  const pct = confidence <= 1.0 ? Math.round(confidence * 100) : Math.round(confidence)
  return `${pct}%`
}

function confidenceColor(confidence: number): string {
  const pct = confidence <= 1.0 ? confidence * 100 : confidence
  if (pct >= 80) return 'text-green-700 bg-green-100'
  if (pct >= 50) return 'text-yellow-700 bg-yellow-100'
  return 'text-red-700 bg-red-100'
}

function evidenceBadgeColor(type: EpisodeEvidence['type']): string {
  switch (type) {
    case 'fact': return 'bg-gray-100 text-gray-700'
    case 'inference': return 'bg-blue-100 text-blue-700'
    case 'human_correction': return 'bg-amber-100 text-amber-700'
  }
}

function episodeTypeLabel(type: Episode['episode_type']): string {
  switch (type) {
    case 'action_verification': return 'Action Verification'
    case 'investigation_step': return 'Investigation Step'
  }
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

// ─── VerdictSection ────────────────────────────────────────────────────────

function VerdictSection({ verdict }: { verdict: EpisodeVerdict }) {
  return (
    <div className="mt-3 pt-3 border-t border-gray-100">
      <div className="flex items-center gap-2 mb-1">
        <span className="text-[10px] font-bold uppercase text-gray-500">Verdict</span>
        <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${confidenceColor(verdict.confidence)}`}>
          {confidencePct(verdict.confidence)} confidence
        </span>
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
    </div>
  )
}

// ─── EvidenceRow ───────────────────────────────────────────────────────────

function EvidenceRow({ ev }: { ev: EpisodeEvidence }) {
  return (
    <div className="py-1.5 border-b border-gray-50 last:border-0">
      <div className="flex items-center gap-2">
        <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase ${evidenceBadgeColor(ev.type)}`}>
          {ev.type}
        </span>
        <span className="text-xs font-medium text-gray-700 truncate">{ev.label ?? ev.node_id}</span>
        <span className="text-[10px] text-gray-400 ml-auto shrink-0">{ev.node_type}</span>
      </div>
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

// ─── EpisodeCard ───────────────────────────────────────────────────────────

interface EpisodeCardProps {
  episode: Episode
  defaultOpen?: boolean
}

export function EpisodeCard({ episode, defaultOpen = false }: EpisodeCardProps) {
  const evidenceCount = episode.evidence?.length ?? 0
  const hasVerdict = !!episode.verdict

  return (
    <div className="border border-gray-200 rounded-md bg-white text-sm overflow-hidden">
      <details open={defaultOpen}>
        <summary className="list-none cursor-pointer select-none">
          <div className="px-3 py-2 flex items-center gap-2 hover:bg-gray-50 transition-colors">
            {/* Episode type badge */}
            <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase ${
              episode.episode_type === 'action_verification'
                ? 'bg-blue-100 text-blue-700'
                : 'bg-violet-100 text-violet-700'
            }`}>
              {episodeTypeLabel(episode.episode_type)}
            </span>

            {/* Verdict status */}
            <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded ${
              hasVerdict ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
            }`}>
              {hasVerdict ? 'verdict' : 'open'}
            </span>

            <span className="text-xs text-gray-500">
              {evidenceCount} evidence{evidenceCount !== 1 ? 's' : ''}
            </span>

            <span className="text-[10px] text-gray-400 ml-auto truncate max-w-[140px]" title={episode.id}>
              {episode.id.slice(0, 8)}…
            </span>
            <span className="text-xs text-gray-400">›</span>
          </div>
        </summary>

        <div className="px-3 pb-3">
          {/* Metadata */}
          <div className="text-[10px] text-gray-400 mb-2">
            Created {formatDate(episode.created_at)}
            {episode.handles && Object.keys(episode.handles).length > 0 && (
              <span className="ml-2">
                {Object.entries(episode.handles).map(([k, v]) => (
                  <span key={k} className="mr-1">
                    <span className="text-gray-500">{k}:</span> {v}
                  </span>
                ))}
              </span>
            )}
          </div>

          {/* Evidence list */}
          {evidenceCount > 0 && (
            <div>
              <span className="text-[10px] font-semibold text-gray-500 uppercase">Evidence</span>
              <div className="mt-1">
                {episode.evidence!.map((ev) => (
                  <EvidenceRow key={ev.id} ev={ev} />
                ))}
              </div>
            </div>
          )}

          {/* Verdict */}
          {episode.verdict && <VerdictSection verdict={episode.verdict} />}
        </div>
      </details>
    </div>
  )
}
