import { memo } from 'react'
import { Handle, Position, type NodeProps, type Node } from '@xyflow/react'
import { NODE_TYPE_INFO, type NodeType } from '@/types'
import { useGraphStore } from '@/hooks/useGraphStore'

// Data shape stored on each flow node
export interface FlowNodeData extends Record<string, unknown> {
  label: string
  nodeType: NodeType
  action: string
  config: Record<string, unknown>
}

type FlowNode = Node<FlowNodeData>

/**
 * Base node component used by all node types.
 * Renders a card with type-specific styling, handles for connections,
 * and displays the node name + type badge.
 */
function BaseNode({ data, selected, id }: NodeProps<FlowNode>) {
  const info = NODE_TYPE_INFO[data.nodeType]
  const status = useGraphStore((s) => s.nodeStatuses[id] ?? 'idle')

  const statusClasses =
    status === 'success'
      ? 'border-green-500 ring-1 ring-green-200'
      : status === 'error'
        ? 'border-red-500 ring-1 ring-red-200'
        : status === 'skipped'
          ? 'opacity-60 border-gray-300'
          : status === 'running'
            ? 'border-blue-400 ring-1 ring-blue-200 animate-pulse'
            : ''

  return (
    <div
      className={`
        min-w-[180px] max-w-[240px] rounded-lg border-2 shadow-sm
        transition-shadow duration-150
        ${info.bgColor} ${info.borderColor}
        ${statusClasses}
        ${selected ? 'shadow-md ring-2 ring-blue-400 ring-offset-1' : ''}
      `}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!w-3 !h-3 !bg-gray-400 !border-2 !border-white"
      />

      {/* Header */}
      <div className="px-3 py-2 border-b border-gray-200/50">
        <div className="flex items-center gap-2">
          <span
            className={`
              text-[10px] font-bold uppercase tracking-wider px-1.5 py-0.5 rounded
              ${info.color} bg-white/60
            `}
          >
            {info.label}
          </span>
        </div>
        <div className="mt-1 text-sm font-medium text-gray-800 truncate">
          {data.label}
        </div>
      </div>

      {/* Body preview */}
      {data.action && (
        <div className="px-3 py-1.5">
          <div className="text-[11px] text-gray-500 font-mono truncate">
            {data.action.slice(0, 40)}{data.action.length > 40 ? '...' : ''}
          </div>
        </div>
      )}

      <Handle
        type="source"
        position={Position.Bottom}
        className="!w-3 !h-3 !bg-gray-400 !border-2 !border-white"
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

// Mapping for React Flow
export const nodeTypes = {
  scriptNode: ScriptNode,
  llmNode: LLMNode,
  mcpNode: MCPNode,
  humanNode: HumanNode,
  routerNode: RouterNode,
}
