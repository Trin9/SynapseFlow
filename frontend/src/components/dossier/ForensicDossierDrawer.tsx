// Forensic Dossier Drawer — full-screen overlay that displays a three-column
// episode workspace (Expected Logic | Causal Bridge | Runtime Facts).
//
// State responsibilities:
//   M3.2 — human review actions (approve / override / abort) + loading/error
//   M3.4 — replay percent + replay slice fetch
//   M4.1 — activeFocusKey for cross-column item linkage (local state)
import { useState, useEffect, useRef } from 'react'
import { useGraphStore } from '@/hooks/useGraphStore'
import { getEpisodeDossier, getEpisodeReplay, getEpisode, postReviewAction } from '@/api/episodes'
import { statusStyle, formatDate } from './_shared'
import { ReplaySlider } from './ReplaySlider'
import { FocusLinkOverlay } from './FocusLinkOverlay'
import { HumanReviewPanel } from './HumanReviewPanel'
import { ProcessTracePanel } from './ProcessTracePanel'
import { MemoryRecallInset } from './MemoryRecallInset'
import { ExpectedBehaviorColumn } from './ExpectedBehaviorColumn'
import { VerdictBridgeColumn } from './VerdictBridgeColumn'
import { RuntimeFactsColumn } from './RuntimeFactsColumn'
import { HistoricalComparisonSheet } from '@/components/execution/HistoricalComparisonSheet'
import type { Episode } from '@/types/episode'
import type { EpisodeDossierView, ReplaySliceView } from '@/types/workspace'

// ─── Trigger Banner ───────────────────────────────────────────────────────

/**
 * Dark banner at the top of the dossier summarising what triggered this episode.
 * Prefers dossier.display.banner (server-formatted) when available.
 * Falls back to episode trigger / action_context / investigation_context.
 */
function TriggerBanner({ ep, banner }: { ep: Episode; banner?: string | null }) {
  let title: string | null = null
  let subtitle: string | null = null
  let badge: string | null = null

  if (banner != null) {
    badge = ep.episode_type === 'action_verification' ? 'ACTION' : 'INVESTIGATION'
    title = banner
  } else if (ep.trigger?.type) {
    badge = ep.trigger.type.toUpperCase().replace('_', ' ')
    const payload = ep.trigger.payload ?? {}
    const summary =
      payload['summary'] ?? payload['description'] ?? payload['message'] ?? payload['event']
    if (typeof summary === 'string') title = summary
    const service = payload['service'] ?? payload['service_name']
    const env = payload['env'] ?? payload['environment']
    if (service || env) {
      subtitle = [service, env]
        .filter((x): x is unknown => Boolean(x))
        .map(String)
        .join(', ')
    }
  } else if (ep.action_context) {
    badge = 'ACTION'
    title = ep.action_context.action_name
    if (ep.action_context.action_type) subtitle = `Type: ${ep.action_context.action_type}`
  } else if (ep.investigation_context) {
    badge = 'INVESTIGATION'
    title = ep.investigation_context.hypothesis
  }

  if (!badge && !title) return null

  return (
    <div className="bg-gray-800 text-gray-200 px-6 py-3 text-sm flex gap-2 items-center shrink-0 shadow-inner">
      {badge && (
        <span className="bg-gray-600 text-white text-[10px] font-bold px-1.5 py-0.5 rounded uppercase shrink-0">
          {badge}
        </span>
      )}
      {title && <span className="font-semibold text-white">{title}</span>}
      {subtitle && <span className="text-gray-400 ml-4 font-mono text-xs">{subtitle}</span>}
    </div>
  )
}

// ─── Main Component ───────────────────────────────────────────────────────

export function ForensicDossierDrawer() {
  // ── Store hooks (all before any early return) ──────────────────────────
  const selectedEpisode = useGraphStore((s) => s.selectedEpisode)
  const setSelectedEpisode = useGraphStore((s) => s.setSelectedEpisode)
  const replayPercent = useGraphStore((s) => s.replayPercent)
  const setReplayPercent = useGraphStore((s) => s.setReplayPercent)

  // ── Local state ────────────────────────────────────────────────────────
  const [dossier, setDossier] = useState<EpisodeDossierView | null>(null)
  const [dossierError, setDossierError] = useState<string | null>(null)
  const [dossierRefreshKey, setDossierRefreshKey] = useState(0)
  const [replaySlice, setReplaySlice] = useState<ReplaySliceView | null>(null)

  // M3.2 — review action state
  const [reviewLoading, setReviewLoading] = useState(false)
  const [reviewError, setReviewError] = useState<string | null>(null)
  const [overrideOpen, setOverrideOpen] = useState(false)
  const [overrideNote, setOverrideNote] = useState('')
  const [abortPending, setAbortPending] = useState(false)

  // M4.1 — cross-column focus linkage (local — no need to pollute the global store)
  const [activeFocusKey, setActiveFocusKey] = useState<string | null>(null)

  // M4.2 — ref to the 3-pane panel container for the SVG overlay
  const panelRef = useRef<HTMLDivElement>(null)

  // M5.3 — historical comparison sheet
  const [comparisonOpen, setComparisonOpen] = useState(false)

  // ── Effects ────────────────────────────────────────────────────────────

  // Fetch dossier whenever episode changes or a review action is submitted
  useEffect(() => {
    if (!selectedEpisode) {
      setDossier(null)
      setDossierError(null)
      setReplaySlice(null)
      setActiveFocusKey(null)
      return
    }
    let cancelled = false
    // Reset per-episode UI state
    setReplayPercent(100)
    setActiveFocusKey(null)
    setDossierError(null)
    setReviewLoading(false)
    setReviewError(null)
    setOverrideOpen(false)
    setOverrideNote('')
    setAbortPending(false)
    getEpisodeDossier(selectedEpisode.exec_id, selectedEpisode.id)
      .then((d) => { if (!cancelled) setDossier(d) })
      .catch((e: unknown) => {
        if (!cancelled) {
          const msg = e instanceof Error ? e.message : 'Dossier unavailable'
          setDossierError(msg)
        }
      })
    return () => { cancelled = true }
  }, [selectedEpisode?.id, dossierRefreshKey])

  // M3.4 — Fetch replay slice whenever episode or replayPercent changes
  useEffect(() => {
    if (!selectedEpisode) { setReplaySlice(null); return }
    let cancelled = false
    getEpisodeReplay(selectedEpisode.exec_id, selectedEpisode.id, replayPercent)
      .then((slice) => { if (!cancelled) setReplaySlice(slice) })
      .catch(() => { if (!cancelled) setReplaySlice(null) })
    return () => { cancelled = true }
  }, [selectedEpisode?.id, replayPercent])

  // ── Early return ───────────────────────────────────────────────────────
  if (!selectedEpisode) return null
  const ep = selectedEpisode

  // ── M3.2 Review action handlers ────────────────────────────────────────
  async function handleApprove() {
    setReviewLoading(true)
    setReviewError(null)
    try {
      await postReviewAction(ep.exec_id, { episode_id: ep.id, status: 'approved' })
      // CR-006: refresh selectedEpisode so status badge + isHumanInLoop re-derive correctly
      const updated = await getEpisode(ep.id)
      setSelectedEpisode(updated)
      setDossierRefreshKey((k) => k + 1)
    } catch (e: unknown) {
      setReviewError(e instanceof Error ? e.message : 'Action failed')
    } finally {
      setReviewLoading(false)
    }
  }

  async function handleOverrideSubmit() {
    setReviewLoading(true)
    setReviewError(null)
    try {
      await postReviewAction(ep.exec_id, {
        episode_id: ep.id,
        status: 'overridden',
        note: overrideNote,
      })
      setOverrideOpen(false)
      setOverrideNote('')
      // CR-006: refresh selectedEpisode
      const updated = await getEpisode(ep.id)
      setSelectedEpisode(updated)
      setDossierRefreshKey((k) => k + 1)
    } catch (e: unknown) {
      setReviewError(e instanceof Error ? e.message : 'Action failed')
    } finally {
      setReviewLoading(false)
    }
  }

  async function handleAbort() {
    if (!abortPending) { setAbortPending(true); return }
    setAbortPending(false)
    setReviewLoading(true)
    setReviewError(null)
    try {
      await postReviewAction(ep.exec_id, { episode_id: ep.id, status: 'aborted' })
      // CR-006: refresh selectedEpisode
      const updated = await getEpisode(ep.id)
      setSelectedEpisode(updated)
      setDossierRefreshKey((k) => k + 1)
    } catch (e: unknown) {
      setReviewError(e instanceof Error ? e.message : 'Action failed')
    } finally {
      setReviewLoading(false)
    }
  }

  // M4.1 — toggle helper
  function handleFocusKey(key: string | null) {
    setActiveFocusKey((prev) => (prev === key ? null : key))
  }

  // ── Render ─────────────────────────────────────────────────────────────
  return (
    <div className="fixed inset-0 z-50 flex items-stretch bg-gray-900/60 dark:bg-black/70 backdrop-blur-sm p-8">
      {/* Dossier Container */}
      <div className="w-full max-w-[1600px] mx-auto h-full bg-gray-100 dark:bg-gray-900 rounded-xl shadow-2xl flex flex-col overflow-hidden border border-gray-300 dark:border-gray-700 relative">

        {/* Top Bar */}
        <div className="bg-white dark:bg-gray-900 border-b border-gray-300 dark:border-gray-700 px-6 py-3 flex items-center justify-between shrink-0">
          <div className="flex items-center gap-4 flex-wrap">
            <span
              className={`text-xs font-bold px-2.5 py-1 rounded uppercase border shrink-0 tracking-wider ${
                ep.episode_type === 'action_verification'
                  ? 'bg-blue-50 text-blue-700 border-blue-200'
                  : 'bg-violet-50 text-violet-700 border-violet-200'
              }`}
            >
              {ep.episode_type === 'action_verification'
                ? 'Action Verification'
                : 'Investigation Step'}
            </span>
            <span className="text-sm font-mono text-gray-500 dark:text-gray-400">#{ep.id.slice(0, 8)}</span>
            <span
              className={`text-[10px] font-bold px-2 py-0.5 rounded uppercase shrink-0 ${statusStyle(ep.status)}`}
            >
              {ep.status}
            </span>
            {dossier && (
              <span className="text-[10px] text-gray-400 dark:text-gray-500 font-mono">
                {dossier.episode.label}
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {/* M5.3 — Compare against historical execution */}
            <button
              onClick={() => setComparisonOpen((o) => !o)}
              className={`px-3 py-1.5 text-xs font-medium rounded-lg border transition-colors ${
                comparisonOpen
                  ? 'bg-blue-600 text-white border-blue-600'
                  : 'bg-white dark:bg-gray-800 text-gray-500 dark:text-gray-400 border-gray-300 dark:border-gray-600 hover:border-blue-400 hover:text-blue-600 dark:hover:text-blue-400'
              }`}
            >
              Compare
            </button>
            <button
              onClick={() => setSelectedEpisode(null)}
              className="text-gray-400 dark:text-gray-500 hover:text-gray-700 dark:hover:text-gray-200 text-2xl leading-none w-8 h-8 rounded hover:bg-gray-100 dark:hover:bg-gray-800 flex items-center justify-center transition-colors"
            >
              ×
            </button>
          </div>
        </div>

        {/* Timestamp row */}
        <div className="px-5 py-1.5 text-[11px] text-gray-400 dark:text-gray-500 border-b border-gray-100 dark:border-gray-800 shrink-0 flex gap-4 flex-wrap">
          <span>Created {formatDate(ep.created_at)}</span>
          <span>Updated {formatDate(ep.updated_at)}</span>
          {ep.concluded_at && <span>Concluded {formatDate(ep.concluded_at)}</span>}
          <span className="ml-auto font-mono text-gray-300 dark:text-gray-600">{ep.exec_id.slice(0, 8)}…</span>
        </div>

        <TriggerBanner ep={ep} banner={dossier?.display.banner} />

        {/* M4.3 — Historical Memory side-context inset */}
        <MemoryRecallInset execId={ep.exec_id} episodeId={ep.id} />

        {/* Dossier error notice */}
        {dossierError && (
          <div className="px-5 py-1.5 text-[11px] text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border-b border-amber-100 dark:border-amber-800 shrink-0">
            Dossier unavailable — showing raw episode data. ({dossierError})
          </div>
        )}

        {/* M3.4 — Replay slider */}
        <ReplaySlider
          dossier={dossier}
          replayPercent={replayPercent}
          onPercentChange={setReplayPercent}
          replaySlice={replaySlice}
        />

        {/* M5.1 — Process trace step bar (replay-percent aware) */}
        <ProcessTracePanel
          dossier={dossier}
          visibleTrace={replaySlice?.visible_process_trace ?? []}
        />

        {/* M4.1 — Cross-column linkage hint banner */}
        {activeFocusKey && (
          <div className="px-5 py-1 text-[11px] text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20 border-b border-blue-100 dark:border-blue-800 shrink-0 flex items-center gap-2">
            <span className="font-bold">Linked view:</span>
            <code className="font-mono">{activeFocusKey}</code>
            <span className="text-blue-400 dark:text-blue-500">— highlighted items share this evidence thread across all three columns.</span>
            <button
              onClick={() => setActiveFocusKey(null)}
              className="ml-auto text-blue-400 dark:text-blue-500 hover:text-blue-600 dark:hover:text-blue-300 font-medium"
            >
              clear
            </button>
          </div>
        )}

        {/* 3-Pane Layout */}
        <div ref={panelRef} className="flex-1 flex overflow-hidden p-4 gap-4 relative bg-gray-100 dark:bg-gray-900">
          {/* M4.2 — SVG connection lines for active focus_key */}
          <FocusLinkOverlay activeFocusKey={activeFocusKey} panelRef={panelRef} />
          {/* Left — Expected Logic */}
          <div className="flex-1 min-w-0">
            <ExpectedBehaviorColumn
              ep={ep}
              dossier={dossier}
              activeFocusKey={activeFocusKey}
              onFocusKey={handleFocusKey}
            />
          </div>

          {/* Middle — Causal Bridge */}
          <div className="flex-1 min-w-0">
            <VerdictBridgeColumn
              ep={ep}
              dossier={dossier}
              activeFocusKey={activeFocusKey}
            />
          </div>

          {/* Right — Runtime Facts */}
          <div className="flex-[1.5] min-w-0">
            <RuntimeFactsColumn
              ep={ep}
              dossier={dossier}
              replaySlice={replaySlice}
              activeFocusKey={activeFocusKey}
            />
          </div>
        </div>

        {/* M5.1 — Human Review Panel + Override Popover (replaces M3.2 floating bar) */}
        <div className="relative shrink-0">
          {overrideOpen && (
            <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 bg-white dark:bg-gray-800 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 px-5 py-4 w-96 space-y-3 z-10">
              <p className="text-sm font-semibold text-gray-700 dark:text-gray-200">Override State — add a note</p>
              <textarea
                className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-blue-400 bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-100"
                rows={3}
                placeholder="Describe the corrected state or reason for override..."
                value={overrideNote}
                onChange={(e) => setOverrideNote(e.target.value)}
              />
              <div className="flex gap-2 justify-end">
                <button
                  onClick={() => { setOverrideOpen(false); setOverrideNote('') }}
                  disabled={reviewLoading}
                  className="px-3 py-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 disabled:opacity-50"
                >
                  Cancel
                </button>
                <button
                  onClick={handleOverrideSubmit}
                  disabled={reviewLoading || !overrideNote.trim()}
                  className="px-4 py-1.5 bg-blue-600 text-white text-sm font-medium rounded-lg disabled:opacity-50 hover:bg-blue-700 transition-colors"
                >
                  {reviewLoading ? 'Submitting…' : 'Submit Override'}
                </button>
              </div>
            </div>
          )}
          <HumanReviewPanel
            ep={ep}
            dossier={dossier}
            reviewLoading={reviewLoading}
            reviewError={reviewError}
            abortPending={abortPending}
            onApprove={handleApprove}
            onOverrideToggle={() => { setOverrideOpen((o) => !o); setAbortPending(false) }}
            onAbort={handleAbort}
            onAbortCancel={() => setAbortPending(false)}
          />
        </div>

        {/* M5.3 — Historical comparison sheet (slides in from right) */}
        {comparisonOpen && (
          <HistoricalComparisonSheet
            execId={ep.exec_id}
            onClose={() => setComparisonOpen(false)}
          />
        )}

      </div>
    </div>
  )
}
