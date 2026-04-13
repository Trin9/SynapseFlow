import { type DragEvent } from 'react'
import { NODE_TYPE_INFO, type NodeType } from '@/types'

const nodeTypeList: NodeType[] = ['script', 'llm', 'mcp', 'human', 'router']

/**
 * Sidebar with draggable node type buttons.
 * Users drag a node type from here onto the canvas to create a new node.
 */
export function Sidebar() {
  const onDragStart = (event: DragEvent<HTMLDivElement>, nodeType: NodeType) => {
    event.dataTransfer.setData('application/synapse-node-type', nodeType)
    event.dataTransfer.effectAllowed = 'move'
  }

  return (
    <div className="w-56 bg-white border-r border-gray-200 flex flex-col">
      <div className="px-4 py-3 border-b border-gray-200">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
          Node Types
        </h2>
        <p className="text-xs text-gray-400 mt-0.5">
          Drag onto canvas
        </p>
      </div>

      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        {nodeTypeList.map((type) => {
          const info = NODE_TYPE_INFO[type]
          return (
            <div
              key={type}
              draggable
              onDragStart={(e) => onDragStart(e, type)}
              className={`
                cursor-grab active:cursor-grabbing
                rounded-lg border-2 p-3
                ${info.bgColor} ${info.borderColor}
                hover:shadow-md transition-shadow duration-150
                select-none
              `}
            >
              <div className="flex items-center gap-2">
                <span className={`text-xs font-bold uppercase ${info.color}`}>
                  {info.label}
                </span>
              </div>
              <p className="text-[11px] text-gray-500 mt-1">
                {info.description}
              </p>
            </div>
          )
        })}
      </div>
    </div>
  )
}
