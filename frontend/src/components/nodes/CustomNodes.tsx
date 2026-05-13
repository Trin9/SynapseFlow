import { memo } from 'react'
import { Handle, Position, type NodeProps, type Node } from '@xyflow/react'
import { Layers, ScrollText } from 'lucide-react'
import { NODE_TYPE_INFO, type NodeType, type AnyNodeType } from '@/types'
import { useGraphStore } from '@/hooks/useGraphStore'
import { getEpisode } from '@/api/episodes'
import { cn } from '@/lib/utils'

// Data shape stored on each flow node
export interface FlowNodeData extends Record<string, unknown> {
  label: string
  nodeType: AnyNodeType
  action: string
  config: Record<string, unknown>
  /** SuperNode only — ids of child nodes grouped under this super-node. */
  childNodeIds?: string[]
}

type FlowNode = Node<FlowNodeData>

/**
 * Base node component used by all node types.
 * Renders a card with type-specific styling, handles for connections,
 * and displays the node name + type badge.
 */
function BaseNode({ data, selected, id }: NodeProps<FlowNode>) {
  const info = NODE_TYPE_INFO[data.nodeType as NodeType]
  const status = useGraphStore((s) => s.nodeStatuses[id] ?? 'idle')

  const statusClasses =
    status === 'success'
      ? 'border-green-500 ring-1 ring-green-200 dark:ring-green-900'
      : status === 'error'
        ? 'border-red-500 ring-1 ring-red-200 dark:ring-red-900'
        : status === 'skipped'
          ? 'opacity-60 border-gray-300 dark:border-gray-600'
          : status === 'running'
            ? 'border-blue-400 ring-1 ring-blue-200 dark:ring-blue-900'
            : ''

  return (
    <div
      className={`
        min-w-[180px] max-w-[240px] rounded-lg border-2 shadow-sm
        transition-shadow duration-150
        ${info.bgColor} ${info.borderColor}
        ${statusClasses}
        ${selected ? 'shadow-md ring-2 ring-blue-400 ring-offset-1 dark:ring-offset-gray-900' : ''}
      `}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!w-3 !h-3 !bg-gray-400 !border-2 !border-white dark:!border-gray-800"
      />

      {/* Header */}
      <div className="px-3 py-2 border-b border-gray-200/50 dark:border-gray-700/50">
        <div className="flex items-center gap-2">
          <span
            className={`
              text-[10px] font-bold uppercase tracking-wider px-1.5 py-0.5 rounded
              ${info.color} bg-white/60 dark:bg-black/30
            `}
          >
            {info.label}
          </span>
        </div>
        <div className="mt-1 text-sm font-medium text-gray-800 dark:text-gray-100 truncate">
          {data.label}
        </div>
      </div>

      {/* Body preview */}
      {data.action && (
        <div className="px-3 py-1.5">
          <div className="text-[11px] text-gray-500 dark:text-gray-400 font-mono truncate">
            {data.action.slice(0, 40)}{data.action.length > 40 ? '...' : ''}
          </div>
        </div>
      )}

      <Handle
        type="source"
        position={Position.Bottom}
        className="!w-3 !h-3 !bg-gray-400 !border-2 !border-white dark:!border-gray-800"
      />
    </div>
  )
}

// ─── SuperNode ────────────────────────────────────────────────────────────

/**
 * Canvas-only SuperNode — a collapsible group that holds child node ids.
 * Clicking "View Inside →" triggers drilldown mode in the store.
 * SuperNode is never serialized to the backend DAGConfig.
 */
function SuperNodeBase({ data, selected, id }: NodeProps<FlowNode>) {
  const enterDrilldown = useGraphStore((s) => s.enterDrilldown)
  const childCount = (data.childNodeIds ?? []).length

  return (
    <div
      className={`
        min-w-[200px] max-w-[260px] rounded-xl border-2 shadow-md
        bg-indigo-50 dark:bg-indigo-900/30
        border-indigo-300 dark:border-indigo-600
        transition-shadow duration-150
        ${selected ? 'shadow-lg ring-2 ring-indigo-400 ring-offset-1 dark:ring-offset-gray-900' : ''}
      `}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!w-3 !h-3 !bg-indigo-400 !border-2 !border-white dark:!border-indigo-900"
      />

      {/* Header */}
      <div className="px-3 py-2 border-b border-indigo-200/60 dark:border-indigo-700/60">
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-bold uppercase tracking-wider px-1.5 py-0.5 rounded text-indigo-700 dark:text-indigo-300 bg-white/60 dark:bg-indigo-900/60">
            SuperNode
          </span>
          {childCount > 0 && (
            <span className="text-[10px] text-indigo-500 dark:text-indigo-400 font-mono">
              {childCount} node{childCount !== 1 ? 's' : ''}
            </span>
          )}
        </div>
        <div className="mt-1 text-sm font-semibold text-indigo-900 dark:text-indigo-100 truncate">
          {data.label}
        </div>
      </div>

      {/* Child node id chips preview (max 4) */}
      {childCount > 0 && (
        <div className="px-3 py-2 flex flex-wrap gap-1">
          {(data.childNodeIds ?? []).slice(0, 4).map((cid) => (
            <span
              key={cid}
              className="text-[9px] bg-indigo-100 dark:bg-indigo-800 text-indigo-600 dark:text-indigo-300 px-1.5 py-0.5 rounded font-mono border border-indigo-200 dark:border-indigo-700 truncate max-w-[72px]"
            >
              {cid}
            </span>
          ))}
          {childCount > 4 && (
            <span className="text-[9px] text-indigo-400 dark:text-indigo-500 self-center">
              +{childCount - 4}
            </span>
          )}
        </div>
      )}

      {/* Drilldown button */}
      <div className="px-3 pb-2">
        <button
          onMouseDown={(e) => e.stopPropagation()}
          onClick={(e) => { e.stopPropagation(); enterDrilldown(id) }}
          className="w-full text-[11px] font-medium
            text-indigo-600 dark:text-indigo-400
            hover:text-indigo-800 dark:hover:text-indigo-200
            bg-indigo-100/60 dark:bg-indigo-900/40
            hover:bg-indigo-200/60 dark:hover:bg-indigo-800/60
            rounded-lg px-2 py-1 transition-colors
            border border-indigo-200/60 dark:border-indigo-700/60"
        >
          View Inside →
        </button>
      </div>

      <Handle
        type="source"
        position={Position.Bottom}
        className="!w-3 !h-3 !bg-indigo-400 !border-2 !border-white dark:!border-indigo-900"
      />
    </div>
  )
}

// ─── Review Episode Overview Node (Spike) ───────────────────────────────

function verdictTone(verdict?: string): string {
  switch (verdict) {
    case 'pass': return 'border-emerald-500/40 bg-emerald-500/10 text-emerald-400'
    case 'fail': return 'border-red-500/40 bg-red-500/10 text-red-400'
    case 'inconclusive': return 'border-amber-500/40 bg-amber-500/10 text-amber-400'
    default: return 'border-zinc-700 bg-zinc-800 text-zinc-400'
  }
}

function statusBorderColor(status?: string): string {
  switch (status) {
    case 'completed': return 'border-emerald-500/30'
    case 'running': return 'border-amber-500/50'
    case 'failed': return 'border-red-500/30'
    default: return 'border-zinc-700/50'
  }
}

function childDotColor(type?: string): string {
  switch (type) {
    case 'script': return 'bg-blue-500'
    case 'llm': return 'bg-purple-500'
    case 'mcp': case 'mcp-tool': return 'bg-emerald-500'
    case 'human': return 'bg-amber-500'
    default: return 'bg-cyan-500'
  }
}

function EpisodeOverviewNodeBase({ data, selected, id }: NodeProps<FlowNode>) {
  const setSelectedEpisode = useGraphStore((s) => s.setSelectedEpisode)
  const enterDrilldown = useGraphStore((s) => s.enterDrilldown)

  const verdict = typeof data.config?.verdict === 'string' ? data.config.verdict : undefined
  const verdictLabel =
    typeof data.config?.verdict_label === 'string'
      ? data.config.verdict_label
      : verdict ?? 'open'
  const status = typeof data.config?.status === 'string' ? data.config.status : 'unknown'
  const evidenceCount = typeof data.config?.evidence_count === 'number' ? data.config.evidence_count : 0
  const handleCount = typeof data.config?.handle_count === 'number' ? data.config.handle_count : 0
  const confidence = typeof data.config?.confidence === 'string' ? data.config.confidence : '-'
  const episodeId = typeof data.config?.episode_id === 'string' ? data.config.episode_id : ''
  const childPreview = Array.isArray(data.config?.child_preview)
    ? (data.config.child_preview as Array<{ id?: string; label?: string; type?: string }>).filter(
        (item) => typeof item?.label === 'string'
      )
    : []
  const childNodeIds = Array.isArray(data.childNodeIds)
    ? data.childNodeIds.filter((id): id is string => typeof id === 'string')
    : []

  async function openDossier() {
    if (!episodeId) return
    const episode = await getEpisode(episodeId)
    setSelectedEpisode(episode)
  }

  return (
    <>
      <Handle type="target" position={Position.Left} className="!h-3 !w-3 !border-zinc-500 !bg-zinc-600" />
      <div
        className={cn(
          'relative w-[360px] rounded-2xl border-2 p-4 backdrop-blur-sm transition-all duration-200',
          'bg-[#0c1220]/95 text-zinc-100',
          statusBorderColor(status),
          selected && 'ring-2 ring-cyan-500/60 ring-offset-2 ring-offset-[#0a0e17]',
          'shadow-xl shadow-black/50'
        )}
      >
        {/* Header row: title + verdict badge */}
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <Layers className="h-4 w-4 text-cyan-400 shrink-0" />
              <span className="font-semibold text-[15px] truncate tracking-tight">{data.label}</span>
            </div>
            {data.action && (
              <p className="mt-1 text-[12px] text-zinc-400 line-clamp-2">{data.action}</p>
            )}
          </div>
          <span className={cn(
            'shrink-0 text-[10px] font-bold uppercase px-2 py-1 rounded-md border',
            verdictTone(verdict)
          )}>
            {verdictLabel}
          </span>
        </div>

        {/* Metrics grid */}
        <div className="mt-3 grid grid-cols-3 gap-2 rounded-xl border border-zinc-700/80 bg-zinc-900/50 p-3.5">
          <div className="flex flex-col items-center gap-0.5">
            <span className="text-[10px] text-zinc-500 uppercase tracking-wider">Confidence</span>
            <span className="text-[13px] font-semibold text-zinc-200 capitalize">{confidence}</span>
          </div>
          <div className="flex flex-col items-center gap-0.5">
            <span className="text-[10px] text-zinc-500 uppercase tracking-wider">Evidence</span>
            <span className="text-[13px] font-semibold text-zinc-200">{evidenceCount}</span>
          </div>
          <div className="flex flex-col items-center gap-0.5">
            <span className="text-[10px] text-zinc-500 uppercase tracking-wider">Handles</span>
            <span className="text-[13px] font-semibold text-zinc-200">{handleCount}</span>
          </div>
        </div>

        {/* Child node dot preview */}
        <div className="relative mt-3 h-16 rounded-lg border border-zinc-800 bg-zinc-900/50">
          {childPreview.length > 0 ? (
            <div className="flex h-full items-center justify-center gap-3 px-4">
              {childPreview.slice(0, 6).map((child, idx) => (
                <div
                  key={child.id ?? idx}
                  className={cn('h-3 w-3 rounded-full', childDotColor(child.type), idx % 2 === 0 && '-translate-y-1')}
                  title={child.label}
                />
              ))}
            </div>
          ) : (
            <div className="flex h-full items-center justify-center text-[10px] text-zinc-600">
              {childPreview.length === 0 ? 'No child preview' : ''}
            </div>
          )}
          <button
            onMouseDown={(e) => e.stopPropagation()}
            onClick={(e) => { e.stopPropagation(); enterDrilldown(id) }}
            disabled={childNodeIds.length === 0}
            className="absolute bottom-1.5 right-1.5 h-6 px-2 text-[11px] font-medium rounded bg-cyan-600 text-zinc-950 hover:bg-cyan-500 disabled:opacity-30 disabled:cursor-not-allowed"
          >
            View Inside →
          </button>
        </div>

        {/* Actions row */}
        <div className="mt-3 flex items-center justify-between gap-2">
          <span className="text-xs text-zinc-500">{childNodeIds.length} internal nodes</span>
          <button
            onMouseDown={(e) => e.stopPropagation()}
            onClick={(e) => { e.stopPropagation(); void openDossier() }}
            className="flex items-center gap-1.5 h-7 px-2.5 text-xs font-medium text-cyan-400 hover:text-cyan-300 hover:bg-cyan-500/10 rounded transition-colors"
          >
            <ScrollText className="h-3 w-3" />
            Open Dossier
          </button>
        </div>

      </div>
      <Handle type="source" position={Position.Right} className="!h-3 !w-3 !border-zinc-500 !bg-zinc-600" />
    </>
  )
}

// Export individual node types for React Flow's nodeTypes registry
export const ScriptNode = memo(BaseNode)
export const LLMNode = memo(BaseNode)
export const MCPNode = memo(BaseNode)
export const HumanNode = memo(BaseNode)
export const RouterNode = memo(BaseNode)
export const SuperNodeComponent = memo(SuperNodeBase)
export const EpisodeOverviewNode = memo(EpisodeOverviewNodeBase)

// Mapping for React Flow
export const nodeTypes = {
  scriptNode: ScriptNode,
  llmNode: LLMNode,
  mcpNode: MCPNode,
  humanNode: HumanNode,
  routerNode: RouterNode,
  superNode: SuperNodeComponent,
  episodeOverviewNode: EpisodeOverviewNode,
}
