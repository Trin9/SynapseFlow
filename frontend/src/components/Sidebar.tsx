import { type DragEvent } from 'react'
import { NODE_TYPE_INFO, type NodeType } from '@/types'
import { useGraphStore } from '@/hooks/useGraphStore'

const nodeTypeList: NodeType[] = ['script', 'llm', 'mcp', 'human', 'router']

/**
 * Sidebar with draggable node type buttons.
 * Users drag a node type from here onto the canvas to create a new node.
 */
export function Sidebar() {
  const appMode = useGraphStore((s) => s.appMode)

  if (appMode === 'REVIEW') {
    return null
  }

  const onDragStart = (event: DragEvent<HTMLDivElement>, nodeType: NodeType | 'super') => {
    event.dataTransfer.setData('application/synapse-node-type', nodeType)
    event.dataTransfer.effectAllowed = 'move'
  }

  return (
    <div className="w-56 bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-700 flex flex-col">
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 uppercase tracking-wider">
          Node Types
        </h2>
        <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
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
              <p className="text-[11px] text-gray-500 dark:text-gray-400 mt-1">
                {info.description}
              </p>
            </div>
          )
        })}

        {/* Design Episode entry */}
        <div
          draggable
          onDragStart={(e) => onDragStart(e, 'super')}
          className="cursor-grab active:cursor-grabbing rounded-lg border-2 p-3
                     bg-slate-50 dark:bg-slate-800/60 border-slate-300 dark:border-slate-600
                     hover:shadow-md transition-shadow duration-150 select-none"
        >
          <div className="flex items-center gap-2">
            <span className="text-xs font-bold uppercase text-slate-600 dark:text-slate-300">
              Episode
            </span>
          </div>
          <p className="text-[11px] text-gray-500 dark:text-gray-400 mt-1">
            Design-time episode shell with expected behavior and internal nodes
          </p>
        </div>
      </div>
    </div>
  )
}
