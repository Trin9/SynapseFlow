import { memo } from 'react'
import { Handle, Position, type NodeProps, type Node } from '@xyflow/react'
import { NODE_TYPE_INFO, type NodeType, type AnyNodeType } from '@/types'
import { useGraphStore } from '@/hooks/useGraphStore'

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
            ? 'border-blue-400 ring-1 ring-blue-200 dark:ring-blue-900 animate-pulse'
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

// Export individual node types for React Flow's nodeTypes registry
export const ScriptNode = memo(BaseNode)
export const LLMNode = memo(BaseNode)
export const MCPNode = memo(BaseNode)
export const HumanNode = memo(BaseNode)
export const RouterNode = memo(BaseNode)
export const SuperNodeComponent = memo(SuperNodeBase)

// Mapping for React Flow
export const nodeTypes = {
  scriptNode: ScriptNode,
  llmNode: LLMNode,
  mcpNode: MCPNode,
  humanNode: HumanNode,
  routerNode: RouterNode,
  superNode: SuperNodeComponent,
}
