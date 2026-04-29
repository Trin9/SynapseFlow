import type { EdgeProps } from '@xyflow/react'
import { getSmoothStepPath, EdgeLabelRenderer, BaseEdge } from '@xyflow/react'

/**
 * LabeledEdge renders a smoothstep edge with an optional mid-edge condition badge.
 * The condition string is read from `data.condition` (set by loadFromDAGConfig).
 */
export function LabeledEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
  style,
  markerEnd,
}: EdgeProps) {
  const [edgePath, labelX, labelY] = getSmoothStepPath({
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
  })

  const condition = data?.condition as string | undefined

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        style={style}
        markerEnd={markerEnd}
      />
      {condition && (
        <EdgeLabelRenderer>
          <div
            style={{
              position: 'absolute',
              transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)`,
              pointerEvents: 'all',
            }}
            className="nodrag nopan"
          >
            <span className="px-1.5 py-0.5 rounded text-xs font-mono bg-emerald-50 border border-emerald-200 text-emerald-700 whitespace-nowrap shadow-sm">
              {condition}
            </span>
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  )
}

export const edgeTypes = {
  labeled: LabeledEdge,
}
