// Shared primitives used by all dossier column components.
import { type ReactNode, useMemo } from 'react'
import type { EpisodeResult, EpisodeConfidence, EpisodeStatus } from '@/types/episode'
import type { ExpectedBehaviorView } from '@/types/workspace'

// ─── Style helpers ────────────────────────────────────────────────────────

export function resultStyle(result?: EpisodeResult): string {
  switch (result) {
    case 'pass':         return 'bg-green-100 text-green-700 border-green-200'
    case 'fail':         return 'bg-red-100 text-red-700 border-red-200'
    case 'inconclusive': return 'bg-yellow-100 text-yellow-700 border-yellow-200'
    default:             return 'bg-gray-100 text-gray-500 border-gray-200'
  }
}

export function confidenceStyle(confidence?: EpisodeConfidence): string {
  switch (confidence) {
    case 'high':   return 'bg-green-50 text-green-700'
    case 'medium': return 'bg-yellow-50 text-yellow-700'
    case 'low':    return 'bg-red-50 text-red-700'
    default:       return 'bg-gray-50 text-gray-500'
  }
}

export function collectorStyle(t?: string): string {
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

export function statusStyle(status: EpisodeStatus): string {
  switch (status) {
    case 'converged':   return 'bg-green-100 text-green-700'
    case 'escalated':   return 'bg-amber-100 text-amber-700'
    case 'failed':      return 'bg-red-100 text-red-700'
    case 'in_progress': return 'bg-blue-100 text-blue-700'
    default:            return 'bg-gray-100 text-gray-500'
  }
}

export function formatDate(iso?: string): string {
  if (!iso) return '—'
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

// ─── Trust Boundary ───────────────────────────────────────────────────────

export type TrustTone = 'sop_only' | 'mixed' | 'ai_only'

export interface TrustDescriptor {
  tone: TrustTone
  label: string
  detail: string
}

/**
 * Given all expected-behavior items for an episode, computes a TrustDescriptor
 * per focus_key.  Items without a focus_key are ignored.
 * Mirror of Demo's `focusTrustDescriptors` useMemo.
 */
export function useTrustMap(
  expectedBehavior: ExpectedBehaviorView[],
): Record<string, TrustDescriptor> {
  return useMemo(() => {
    const grouped = new Map<string, Set<string>>()
    for (const item of expectedBehavior) {
      if (!item.focus_key) continue
      const set = grouped.get(item.focus_key) ?? new Set<string>()
      set.add(item.source_type ?? 'ai')
      grouped.set(item.focus_key, set)
    }
    const result: Record<string, TrustDescriptor> = {}
    for (const [key, sourceTypes] of grouped.entries()) {
      const hasSOP = sourceTypes.has('sop')
      const hasAI  = sourceTypes.has('ai') || [...sourceTypes].some((t) => t !== 'sop')
      const tone: TrustTone =
        hasSOP && hasAI ? 'mixed' : hasSOP ? 'sop_only' : 'ai_only'
      result[key] =
        tone === 'mixed'
          ? { tone, label: 'Mixed with AI hypothesis',
              detail: 'Grounded in SOP-backed expectations but also references at least one AI hypothesis — treat with care.' }
          : tone === 'ai_only'
          ? { tone, label: 'AI hypothesis only',
              detail: 'Depends on AI-generated reasoning. Requires manual validation before influencing a final decision.' }
          : { tone, label: 'Verified SOP only',
              detail: 'Anchored only to SOP-backed expectations — can be read as hard workflow intent.' }
    }
    return result
  }, [expectedBehavior])
}

/** Tailwind class string for a trust badge (light-theme). */
export function trustBadgeStyle(tone: TrustTone): string {
  switch (tone) {
    case 'sop_only': return 'bg-emerald-50 text-emerald-700 border border-emerald-200'
    case 'mixed':    return 'bg-amber-50 text-amber-700 border border-amber-200'
    case 'ai_only':  return 'bg-purple-50 text-purple-700 border border-purple-200'
  }
}

export function prettyJSON(v: unknown): string {
  if (typeof v === 'string') {
    try { return JSON.stringify(JSON.parse(v), null, 2) } catch { return v }
  }
  try { return JSON.stringify(v, null, 2) } catch { return String(v) }
}

// ─── Section wrapper ──────────────────────────────────────────────────────

/**
 * Framed pane with a labelled header.
 * Optional `hint` text appears right-aligned in the header — used by M4.1
 * to show "linked: <focus_key>" when a cross-column focus is active.
 */
export function Section({
  title,
  hint,
  children,
}: {
  title: string
  hint?: string
  children: ReactNode
}) {
  return (
    <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden h-full flex flex-col bg-white dark:bg-gray-900">
      <div className="px-4 py-2 bg-gray-100 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 shrink-0 flex items-center justify-between">
        <h3 className="text-[11px] font-bold text-gray-600 dark:text-gray-300 uppercase tracking-wider">{title}</h3>
        {hint && (
          <span className="text-[10px] text-blue-500 font-medium italic">{hint}</span>
        )}
      </div>
      <div className="px-4 py-3 flex-1 overflow-y-auto space-y-4">{children}</div>
    </div>
  )
}
