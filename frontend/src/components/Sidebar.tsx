import { type DragEvent } from 'react'
import { NODE_TYPE_INFO, type NodeType } from '@/types'
import { useGraphStore } from '@/hooks/useGraphStore'

const nodeTypeList: NodeType[] = ['script', 'llm', 'mcp', 'human', 'router']

// Dark-theme palette for each node type (used in the dark sidebar)
const DARK_NODE_STYLE: Record<NodeType, { bg: string; border: string; label: string }> = {
  script:  { bg: 'bg-slate-800/60',    border: 'border-slate-600',      label: 'text-slate-200' },
  llm:     { bg: 'bg-blue-950/70',     border: 'border-blue-600/70',    label: 'text-blue-300'  },
  mcp:     { bg: 'bg-violet-950/70',   border: 'border-violet-600/70',  label: 'text-violet-300' },
  human:   { bg: 'bg-amber-950/70',    border: 'border-amber-600/70',   label: 'text-amber-300'  },
  router:  { bg: 'bg-emerald-950/70',  border: 'border-emerald-600/70', label: 'text-emerald-300' },
}

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
    <div className="w-56 bg-[#0c1220] border-r border-cyan-900/35 flex flex-col">
      <div className="px-4 py-3 border-b border-cyan-900/35">
        <h2 className="text-sm font-semibold text-cyan-200 uppercase tracking-wider">
          Node Types
        </h2>
        <p className="text-xs text-cyan-500/80 mt-0.5">
          Drag onto canvas
        </p>
      </div>

      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        {nodeTypeList.map((type) => {
          const info = NODE_TYPE_INFO[type]
          const dark = DARK_NODE_STYLE[type]
          return (
            <div
              key={type}
              draggable
              onDragStart={(e) => onDragStart(e, type)}
              className={`
                cursor-grab active:cursor-grabbing
                rounded-lg border-2 p-3
                ${dark.bg} ${dark.border}
                hover:brightness-110 transition-all duration-150
                select-none
              `}
            >
              <div className="flex items-center gap-2">
                <span className={`text-xs font-bold uppercase ${dark.label}`}>
                  {info.label}
                </span>
              </div>
              <p className="text-[11px] text-slate-400 mt-1">
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
                     bg-slate-900/80 border-cyan-900/45
                     hover:shadow-md transition-shadow duration-150 select-none"
        >
          <div className="flex items-center gap-2">
            <span className="text-xs font-bold uppercase text-cyan-300">
              Episode
            </span>
          </div>
          <p className="text-[11px] text-cyan-500/80 mt-1">
            Design-time episode shell with expected behavior and internal nodes
          </p>
        </div>
      </div>
    </div>
  )
}
