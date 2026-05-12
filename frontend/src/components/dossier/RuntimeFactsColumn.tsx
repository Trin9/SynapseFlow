// Right pane of the Forensic Dossier — Runtime Facts / Evidence.
// M3.4: facts not in visible_runtime_fact_ids are dimmed (opacity-40).
// M4.1: facts whose focus_key matches activeFocusKey receive a blue ring.
// Demo-migration: syntax highlighting for json/log/code source_types.
import { Section, collectorStyle, prettyJSON } from './_shared'
import type { Episode, EpisodeEvidence, EvidenceCollectorSpec } from '@/types/episode'
import type {
  EpisodeDossierView,
  RuntimeFactView,
  ReplaySliceView,
} from '@/types/workspace'

// ─── Syntax highlighting ───────────────────────────────────────────────────

type SourceType = 'json' | 'log' | 'code' | 'text' | string

function highlightLine(text: string, sourceType: SourceType): string {
  if (sourceType === 'json') {
    return text
      .replace(/("([^"]+)"\s*):/g, '<span class="text-teal-700">$1</span>:')
      .replace(/:\s*("([^"]+)")/g, ': <span class="text-emerald-700">$1</span>')
      .replace(/:\s*(\d+(?:\.\d+)?)/g, ': <span class="text-amber-700">$1</span>')
      .replace(/:\s*(true|false|null)/g, ': <span class="text-purple-700">$1</span>')
  }
  if (sourceType === 'log') {
    return text
      .replace(/(\[\d[\d:.]+\])/g, '<span class="text-gray-400">$1</span>')
      .replace(/\b(INFO)\b/g, '<span class="text-blue-600 font-semibold">$1</span>')
      .replace(/\b(DEBUG)\b/g, '<span class="text-gray-500">$1</span>')
      .replace(/\b(WARN(?:ING)?)\b/g, '<span class="text-amber-600 font-semibold">$1</span>')
      .replace(/\b(ERROR|FATAL)\b/g, '<span class="text-red-600 font-semibold">$1</span>')
  }
  if (sourceType === 'code') {
    return text
      .replace(/\b(class|def|if|elif|else|return|from|import|for|in|while|with|as|pass|raise|try|except|finally|yield|lambda|and|or|not|is|None|True|False)\b/g,
        '<span class="text-pink-600 font-semibold">$1</span>')
      .replace(/"([^"]*)"/g, '<span class="text-emerald-700">"$1"</span>')
      .replace(/'([^']*)'/g, "<span class=\"text-emerald-700\">'$1'</span>")
      .replace(/\b([A-Z][a-zA-Z0-9]+)\b/g, '<span class="text-teal-700">$1</span>')
  }
  return text
}

// Renders a single pre block with per-line syntax highlighting and optional
// line numbers. highlight_lines (1-indexed) receives a tinted background.
function SyntaxCodeBlock({
  content,
  sourceType,
  highlightLines,
}: {
  content: string
  sourceType: SourceType
  highlightLines?: number[]
}) {
  const lines = content.split('\n')
  const highlighted = new Set(highlightLines ?? [])
  const useSyntax = sourceType === 'json' || sourceType === 'log' || sourceType === 'code'

  return (
    <div className="relative overflow-x-auto max-h-64 rounded border border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-900">
      <pre className="p-2 font-mono text-[11px]">
        {lines.map((line, i) => {
          const lineNum = i + 1
          const isHighlighted = highlighted.has(lineNum)
          return (
            <div
              key={i}
              className={[
                'flex',
                isHighlighted ? 'rounded bg-teal-50 dark:bg-teal-900/30 ring-1 ring-teal-300 dark:ring-teal-700' : '',
              ].filter(Boolean).join(' ')}
            >
              <span className="mr-3 w-6 select-none text-right shrink-0 text-gray-400 dark:text-gray-600">
                {lineNum}
              </span>
              {useSyntax ? (
                <code
                  className="flex-1 whitespace-pre text-gray-700 dark:text-gray-300"
                  // eslint-disable-next-line react/no-danger
                  dangerouslySetInnerHTML={{ __html: highlightLine(line, sourceType) }}
                />
              ) : (
                <code className="flex-1 whitespace-pre text-gray-700 dark:text-gray-300">{line}</code>
              )}
            </div>
          )
        })}
      </pre>
    </div>
  )
}

// ─── Sub-components ───────────────────────────────────────────────────────

function CollectorSpecBadges({ spec }: { spec: EvidenceCollectorSpec }) {
  return (
    <div className="flex flex-wrap gap-1.5 items-start mt-1">
      {spec.collector_type && (
        <span
          className={`text-[10px] font-bold px-2 py-0.5 rounded border uppercase ${collectorStyle(spec.collector_type)}`}
        >
          {spec.collector_type.replace('_', ' ')}
        </span>
      )}
      {spec.raw_command && (
        <code
          className="text-[11px] font-mono text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 px-2 py-0.5 rounded break-all"
          title={spec.raw_command}
        >
          {spec.raw_command.length > 60
            ? spec.raw_command.slice(0, 60) + '...'
            : spec.raw_command}
        </code>
      )}
    </div>
  )
}

function EvidenceItem({ ev }: { ev: EpisodeEvidence }) {
  const typeColors: Record<string, string> = {
    fact: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300',
    inference: 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400',
    human_correction: 'bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400',
  }
  const typeColor = typeColors[ev.type] ?? 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'

  const displayContent = ev.content
    ? ev.content.trimStart().startsWith('{') || ev.content.trimStart().startsWith('[')
      ? prettyJSON(ev.content)
      : ev.content
    : null

  return (
    <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-3 space-y-2 bg-white dark:bg-gray-800 relative group">
      <div className="flex items-center gap-2 flex-wrap pr-8">
        <span
          className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase shrink-0 ${typeColor}`}
        >
          {ev.type}
        </span>
        <span className="text-sm font-medium text-gray-800 dark:text-gray-100">{ev.label ?? ev.node_id}</span>
      </div>
      {ev.collector_spec && <CollectorSpecBadges spec={ev.collector_spec} />}
      {displayContent && (
        <pre className="text-[11px] font-mono bg-gray-50 dark:bg-gray-900 rounded p-2 overflow-x-auto max-h-64 text-gray-700 dark:text-gray-300 border border-gray-100 dark:border-gray-700 whitespace-pre-wrap">
          {displayContent}
        </pre>
      )}
      {ev.content_ref && !ev.content && (
        <span className="text-xs text-blue-500 block">{ev.content_ref}</span>
      )}
    </div>
  )
}

// M3.4: isVisible dims the card; M4.1: isActive adds blue ring
function RuntimeFactItem({
  fact,
  isVisible = true,
  isActive = false,
}: {
  fact: RuntimeFactView
  isVisible?: boolean
  isActive?: boolean
}) {
  const typeColor =
    fact.source_type === 'text'
      ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400'
      : fact.source_type === 'json'
      ? 'bg-teal-100 dark:bg-teal-900/30 text-teal-700 dark:text-teal-400'
      : fact.source_type === 'log'
      ? 'bg-indigo-100 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-400'
      : fact.source_type === 'code'
      ? 'bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-400'
      : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'

  const collectorMethod = fact.collector?.split(':')[0]

  const displayContent = fact.content
    ? fact.content.trimStart().startsWith('{') || fact.content.trimStart().startsWith('[')
      ? prettyJSON(fact.content)
      : fact.content
    : fact.summary

  // Determine effective source type for syntax highlighting:
  // if the content is clearly JSON regardless of declared source_type, use 'json'
  const effectiveType: SourceType =
    fact.content &&
    (fact.content.trimStart().startsWith('{') || fact.content.trimStart().startsWith('['))
      ? 'json'
      : (fact.source_type ?? 'text')

  return (
    <div
      data-focus-key={fact.focus_key ?? undefined}
      className={[
        'border border-gray-200 dark:border-gray-700 rounded-lg p-3 space-y-2 bg-white dark:bg-gray-800 transition-all',
        isVisible ? '' : 'opacity-40',
        isActive ? 'ring-2 ring-blue-400' : '',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <div className="flex items-center gap-2 flex-wrap">
        {fact.source_type && (
          <span
            className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase shrink-0 ${typeColor}`}
          >
            {fact.source_type}
          </span>
        )}
        <span className="text-sm font-medium text-gray-800 dark:text-gray-100">{fact.title}</span>
      </div>
      {collectorMethod && (
        <span
          className={`inline-block text-[10px] font-bold px-2 py-0.5 rounded border uppercase ${collectorStyle(collectorMethod)}`}
        >
          {collectorMethod.replace('_', ' ')}
        </span>
      )}
      {displayContent && (
        <SyntaxCodeBlock
          content={displayContent}
          sourceType={effectiveType}
          highlightLines={fact.highlight_lines}
        />
      )}
      {fact.content_ref && !fact.content && (
        <span className="text-xs text-blue-500 block">{fact.content_ref}</span>
      )}
    </div>
  )
}

// ─── Column ───────────────────────────────────────────────────────────────

interface RuntimeFactsColumnProps {
  ep: Episode
  dossier: EpisodeDossierView | null
  /** M3.4 — replay slice filters which facts are visible */
  replaySlice: ReplaySliceView | null
  /** M4.1 — currently highlighted focus_key */
  activeFocusKey: string | null
}

export function RuntimeFactsColumn({
  ep,
  dossier,
  replaySlice,
  activeFocusKey,
}: RuntimeFactsColumnProps) {
  const hint = activeFocusKey ? `linked: ${activeFocusKey}` : undefined

  if (dossier) {
    const runtimeFacts = dossier.runtime_facts ?? []
    return (
      <Section title={`3. Runtime Facts (${runtimeFacts.length})`} hint={hint}>
        {runtimeFacts.length > 0 ? (
          runtimeFacts.map((fact) => (
            <RuntimeFactItem
              key={fact.id}
              fact={fact}
              isVisible={
                !replaySlice || replaySlice.visible_runtime_fact_ids.includes(fact.id)
              }
              isActive={
                fact.focus_key != null && fact.focus_key === activeFocusKey
              }
            />
          ))
        ) : (
          <div className="text-center py-10 text-gray-400 dark:text-gray-500 text-sm">
            No runtime facts collected.
          </div>
        )}
      </Section>
    )
  }

  return (
    <Section title={`3. Runtime Facts / Evidence (${(ep.evidence ?? []).length})`}>
      {(ep.evidence ?? []).length > 0 ? (
        ep.evidence!.map((ev) => <EvidenceItem key={ev.id} ev={ev} />)
      ) : (
          <div className="text-center py-10 text-gray-400 dark:text-gray-500 text-sm">
          Waiting for evidence collection...
          </div>
      )}
    </Section>
  )
}
