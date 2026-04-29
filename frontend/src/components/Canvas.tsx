import { useCallback, type DragEvent } from 'react'
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  type ReactFlowInstance,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'

import { nodeTypes } from './nodes/CustomNodes'
import { edgeTypes } from './edges/LabeledEdge'
import { useGraphStore, type FlowNode, type FlowEdge } from '@/hooks/useGraphStore'
import type { AnyNodeType } from '@/types'

import { useRef } from 'react'

export function Canvas() {
  const appMode = useGraphStore((s) => s.appMode)
  const nodes = useGraphStore((s) => s.nodes)
  const edges = useGraphStore((s) => s.edges)
  const onNodesChange = useGraphStore((s) => s.onNodesChange)
  const onEdgesChange = useGraphStore((s) => s.onEdgesChange)
  const onConnect = useGraphStore((s) => s.onConnect)
  const addNode = useGraphStore((s) => s.addNode)
  const setSelectedNodeId = useGraphStore((s) => s.setSelectedNodeId)

  // SuperNode drilldown
  const viewLevel = useGraphStore((s) => s.viewLevel)
  const activeSuperNodeId = useGraphStore((s) => s.activeSuperNodeId)
  const exitDrilldown = useGraphStore((s) => s.exitDrilldown)

  const reactFlowRef = useRef<ReactFlowInstance<FlowNode, FlowEdge> | null>(null)

  const onDragOver = useCallback((event: DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'
  }, [])

  const onDrop = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      event.preventDefault()

      const type = event.dataTransfer.getData('application/synapse-node-type') as AnyNodeType
      if (!type) return

      const reactFlowInstance = reactFlowRef.current
      if (!reactFlowInstance) return

      const position = reactFlowInstance.screenToFlowPosition({
        x: event.clientX,
        y: event.clientY,
      })

      addNode(type, position)
    },
    [addNode]
  )

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: FlowNode) => {
      setSelectedNodeId(node.id)
    },
    [setSelectedNodeId]
  )

  const onPaneClick = useCallback(() => {
    setSelectedNodeId(null)
  }, [setSelectedNodeId])

  // In REVIEW mode the canvas is read-only: no drag, no connect, no select.
  const isReview = appMode === 'REVIEW'

  // SuperNode drilldown: restrict visible nodes/edges to the active super-group
  const isDrilldown = viewLevel === 'drilldown' && activeSuperNodeId != null

  const visibleNodes: FlowNode[] = isDrilldown
    ? nodes.filter((n) => {
        if (n.id === activeSuperNodeId) return true
        const superNode = nodes.find((s) => s.id === activeSuperNodeId)
        return (superNode?.data.childNodeIds ?? []).includes(n.id)
      })
    : nodes

  const visibleEdges: FlowEdge[] = isDrilldown
    ? (() => {
        const superNode = nodes.find((s) => s.id === activeSuperNodeId)
        const childSet = new Set(superNode?.data.childNodeIds ?? [])
        childSet.add(activeSuperNodeId!)
        return edges.filter((e) => childSet.has(e.source) && childSet.has(e.target))
      })()
    : edges

  return (
    <div className="flex-1 flex flex-col">
      {/* Drilldown breadcrumb bar */}
      {isDrilldown && (
        <div className="flex items-center gap-2 px-4 py-1.5 bg-indigo-50 dark:bg-indigo-900/30 border-b border-indigo-200 dark:border-indigo-700 text-xs shrink-0">
          <button
            onClick={exitDrilldown}
            className="text-indigo-500 dark:text-indigo-400 hover:text-indigo-700 dark:hover:text-indigo-200 font-medium transition-colors"
          >
            ← Overview
          </button>
          <span className="text-indigo-300 dark:text-indigo-600">/</span>
          <span className="font-semibold text-indigo-700 dark:text-indigo-300">
            {nodes.find((n) => n.id === activeSuperNodeId)?.data.label ?? activeSuperNodeId}
          </span>
          <span className="text-indigo-400 dark:text-indigo-500 ml-1">
            ({visibleNodes.length - 1} child node{visibleNodes.length - 1 !== 1 ? 's' : ''})
          </span>
        </div>
      )}

      <div className="flex-1">
        <ReactFlow<FlowNode, FlowEdge>
          nodes={visibleNodes}
          edges={visibleEdges}
          onNodesChange={isReview ? undefined : onNodesChange}
          onEdgesChange={isReview ? undefined : onEdgesChange}
          onConnect={isReview ? undefined : onConnect}
          onInit={(instance) => { reactFlowRef.current = instance }}
          onDrop={isReview ? undefined : onDrop}
          onDragOver={isReview ? undefined : onDragOver}
          onNodeClick={isReview ? undefined : onNodeClick}
          onPaneClick={isReview ? undefined : onPaneClick}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          nodesDraggable={!isReview}
          nodesConnectable={!isReview}
          elementsSelectable={!isReview}
          fitView
          snapToGrid={!isReview}
          snapGrid={[15, 15]}
          defaultEdgeOptions={{
            type: 'labeled',
            animated: true,
            style: { stroke: '#94a3b8', strokeWidth: 2 },
          }}
          proOptions={{ hideAttribution: true }}
        >
          <Background gap={15} size={1} />
          <Controls showInteractive={false} />
          <MiniMap
            nodeStrokeWidth={3}
            pannable
            zoomable
            className="!bg-gray-100 dark:!bg-gray-800 !border-gray-200 dark:!border-gray-700"
          />
          {isReview && nodes.length === 0 && (
            <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <p className="text-sm text-gray-400 dark:text-gray-500">Load a workflow to view its execution state.</p>
            </div>
          )}
        </ReactFlow>
      </div>
    </div>
  )
}
