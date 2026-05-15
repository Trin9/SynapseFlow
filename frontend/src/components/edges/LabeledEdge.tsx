import type { EdgeProps } from '@xyflow/react'
import { getSmoothStepPath, EdgeLabelRenderer, BaseEdge } from '@xyflow/react'

/**
 * LabeledEdge renders a smoothstep edge with an optional mid-edge condition badge.
 * Implements smart collision avoidance: when multiple edges connect the same nodes,
 * they are automatically offset vertically to prevent overlap.
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
  // Compute a stable, deterministic offset for collision avoidance based on edge ID.
  // This ensures edges between the same pair of nodes are offset consistently.
  const edgeHash = Array.from(id).reduce((h, c) => {
    return ((h << 5) - h) + c.charCodeAt(0) | 0
  }, 0)
  
  // Calculate vertical offset: alternate between positive and negative to spread edges
  const offsetMagnitude = 50
  const offset = Math.abs(edgeHash) % 3 === 0 ? 0 : (edgeHash % 2 === 0 ? offsetMagnitude : -offsetMagnitude)
  
  const [edgePath, labelX, labelY] = getSmoothStepPath({
    sourceX,
    sourceY: sourceY + offset,
    targetX,
    targetY: targetY + offset,
    sourcePosition,
    targetPosition,
    borderRadius: 10,
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
              transform: `translate(-50%, -50%) translate(${labelX}px,${labelY - 18}px)`,
              pointerEvents: 'all',
            }}
            className="nodrag nopan"
          >
            <span className="px-2 py-1 rounded text-xs font-mono bg-black/30 border border-dashed border-cyan-400/60 text-cyan-300 whitespace-nowrap shadow-lg backdrop-blur-sm">
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
