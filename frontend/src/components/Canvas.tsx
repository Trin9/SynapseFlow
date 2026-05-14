import { useCallback, type DragEvent } from 'react'
import { useEffect, useMemo } from 'react'
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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getExecutionSummaryView, listEpisodeSummariesByExecution } from '@/api/episodes'
import { getSceneManifest } from '@/lib/sceneManifest'

import { useRef } from 'react'

export function Canvas() {
  const queryClient = useQueryClient()
  const appMode = useGraphStore((s) => s.appMode)
  const activeExecutionId = useGraphStore((s) => s.activeExecutionId)
  const useWorkbenchLayout = useGraphStore((s) => s.useWorkbenchLayout)
  const isRunning = useGraphStore((s) => s.isRunning)
  const nodes = useGraphStore((s) => s.nodes)
  const edges = useGraphStore((s) => s.edges)
  const onNodesChange = useGraphStore((s) => s.onNodesChange)
  const onEdgesChange = useGraphStore((s) => s.onEdgesChange)
  const onConnect = useGraphStore((s) => s.onConnect)
  const addNode = useGraphStore((s) => s.addNode)
  const setSelectedNodeId = useGraphStore((s) => s.setSelectedNodeId)
  const setShowHistory = useGraphStore((s) => s.setShowHistory)

  // SuperNode drilldown
  const viewLevel = useGraphStore((s) => s.viewLevel)
  const activeSuperNodeId = useGraphStore((s) => s.activeSuperNodeId)
  const exitDrilldown = useGraphStore((s) => s.exitDrilldown)

  // In REVIEW mode the canvas is read-only: no drag, no connect, no select.
  const isReview = appMode === 'REVIEW'

  const reactFlowRef = useRef<ReactFlowInstance<FlowNode, FlowEdge> | null>(null)
  const lastAutoFitKeyRef = useRef<string | null>(null)

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

      if (!isReview) setShowHistory(false)
      addNode(type, position)
    },
    [addNode, isReview, setShowHistory]
  )

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: FlowNode) => {
      if (!isReview) setShowHistory(false)
      setSelectedNodeId(node.id)
    },
    [isReview, setSelectedNodeId, setShowHistory]
  )

  const onPaneClick = useCallback(() => {
    if (!isReview) setShowHistory(false)
    setSelectedNodeId(null)
  }, [isReview, setSelectedNodeId, setShowHistory])

  const useEpisodeRendererSwap = isReview && useWorkbenchLayout && !!activeExecutionId

  const { data: episodeSummaries = [], isLoading: episodesLoading } = useQuery({
    queryKey: ['review-canvas-episodes', activeExecutionId],
    queryFn: () => listEpisodeSummariesByExecution(activeExecutionId!),
    enabled: useEpisodeRendererSwap,
  })

  const { data: executionSummary } = useQuery({
    queryKey: ['review-canvas-summary', activeExecutionId],
    queryFn: () => getExecutionSummaryView(activeExecutionId!),
    enabled: useEpisodeRendererSwap,
  })

  useEffect(() => {
    if (!useEpisodeRendererSwap || !activeExecutionId) return
    void queryClient.invalidateQueries({
      queryKey: ['review-canvas-episodes', activeExecutionId],
    })
  }, [activeExecutionId, queryClient, useEpisodeRendererSwap])

  useEffect(() => {
    if (!useEpisodeRendererSwap || !activeExecutionId) return
    if (isRunning) return
    // Ensure we fetch the final episode summaries after execution settles.
    void queryClient.invalidateQueries({
      queryKey: ['review-canvas-episodes', activeExecutionId],
    })
  }, [activeExecutionId, isRunning, queryClient, useEpisodeRendererSwap])

  const sceneManifest = getSceneManifest(executionSummary?.dag_id, executionSummary?.dag_name)

  const orderedEpisodeSummaries = useMemo(() => {
    if (!sceneManifest?.curatedEpisodeOrder?.length) return episodeSummaries

    const orderTokens = sceneManifest.curatedEpisodeOrder.map((token) => token.toLowerCase())

    const rank = (value: { episode_id: string; label: string; display: { summary?: string } }) => {
      const haystack = `${value.episode_id} ${value.label} ${value.display.summary ?? ''}`.toLowerCase()
      const idx = orderTokens.findIndex((token) => haystack.includes(token))
      return idx === -1 ? Number.MAX_SAFE_INTEGER : idx
    }

    return [...episodeSummaries].sort((a, b) => {
      const ra = rank(a)
      const rb = rank(b)
      if (ra !== rb) return ra - rb
      return a.label.localeCompare(b.label)
    })
  }, [episodeSummaries, sceneManifest?.curatedEpisodeOrder])

  // SuperNode drilldown: restrict visible nodes/edges to the active super-group
  const isDrilldown = viewLevel === 'drilldown' && activeSuperNodeId != null

  const designEpisodeNodes = useMemo<FlowNode[]>(() => {
    if (useEpisodeRendererSwap) return []
    return nodes.filter((node) => node.data.nodeType === 'super')
  }, [nodes, useEpisodeRendererSwap])

  const designEpisodeEdges = useMemo<FlowEdge[]>(() => {
    if (useEpisodeRendererSwap) return []

    const designEpisodes = nodes.filter((node) => node.data.nodeType === 'super')
    if (designEpisodes.length <= 1) return []

    const ownerByChild = new Map<string, string>()
    for (const episode of designEpisodes) {
      for (const childId of episode.data.childNodeIds ?? []) ownerByChild.set(childId, episode.id)
    }

    const seen = new Set<string>()
    const nextEdges: FlowEdge[] = []
    for (const edge of edges) {
      const fromEpisode = ownerByChild.get(edge.source)
      const toEpisode = ownerByChild.get(edge.target)
      if (!fromEpisode || !toEpisode || fromEpisode === toEpisode) continue

      const key = `${fromEpisode}->${toEpisode}`
      if (seen.has(key)) continue
      seen.add(key)

      nextEdges.push({
        id: `design-episode-${fromEpisode}-${toEpisode}`,
        source: fromEpisode,
        target: toEpisode,
        type: 'labeled',
        animated: true,
        style: { stroke: '#67e8f9', strokeWidth: 2 },
      } as FlowEdge)
    }

    return nextEdges
  }, [edges, nodes, useEpisodeRendererSwap])

  const reviewSwapNodes = useMemo<FlowNode[]>(() => {
    if (!useEpisodeRendererSwap) return []
    const fallbackChildIds = nodes
      .filter((n) => n.data.nodeType !== 'super')
      .map((n) => n.id)

    if (orderedEpisodeSummaries.length === 0) {
      if (!executionSummary) return []
      return [
        {
          id: `episode-fallback-${executionSummary.execution_id}`,
          type: 'episodeOverviewNode',
          position: { x: 120, y: 120 },
          data: {
            label: `${executionSummary.workflow_kind || 'execution'} episode`,
            nodeType: 'llm',
            action: executionSummary.display?.trace_summary || 'Execution is in progress. Episode summary will appear shortly.',
            config: {
              episode_id: '',
              status: executionSummary.status,
              verdict: undefined,
              verdict_label: executionSummary.status === 'failed' ? 'FAIL' : executionSummary.status === 'completed' ? 'PASS' : 'OPEN',
              evidence_count: 0,
              handle_count: 0,
              confidence: '-',
              child_preview: sceneManifest?.childPreview ?? [],
            },
            childNodeIds: fallbackChildIds,
          },
        } as FlowNode,
      ]
    }

    return orderedEpisodeSummaries.map((sv, idx) => ({
      id: `episode-${sv.episode_id}`,
      type: 'episodeOverviewNode',
      position: { x: 80 + idx * 400, y: 100 },
      data: {
        label: sv.label,
        nodeType: 'llm',
        action: sv.display.summary ?? '',
        config: {
          episode_id: sv.episode_id,
          status: sv.status,
          verdict: sv.display.verdict,
          verdict_label: sv.display.verdict_label,
          evidence_count: sv.evidence_count,
          handle_count: sv.handle_count,
          confidence: sv.confidence,
          child_preview: sceneManifest?.childPreview ?? [],
        },
        childNodeIds: (() => {
          const fromSummary = Array.isArray(sv.node_ids)
            ? sv.node_ids.filter((id): id is string => typeof id === 'string' && id.length > 0)
            : []
          if (fromSummary.length > 0) return fromSummary

          const fromPreview = Array.isArray(sceneManifest?.childPreview)
            ? sceneManifest.childPreview
                .map((item) => item?.id)
                .filter((id): id is string => typeof id === 'string' && id.length > 0)
            : []
          if (fromPreview.length > 0) return fromPreview

          // Last-resort fallback so episode cards remain drillable even when
          // backend node mapping is temporarily unavailable.
          return fallbackChildIds
        })(),
      },
    }))
  }, [executionSummary, nodes, orderedEpisodeSummaries, sceneManifest?.childPreview, useEpisodeRendererSwap])

  const reviewSwapEdges = useMemo<FlowEdge[]>(() => {
    if (!useEpisodeRendererSwap) return []
    if (orderedEpisodeSummaries.length <= 1) return []
    const edgesOut: FlowEdge[] = []
    for (let i = 0; i < orderedEpisodeSummaries.length - 1; i += 1) {
      edgesOut.push({
        id: `episode-edge-${i}`,
        source: `episode-${orderedEpisodeSummaries[i].episode_id}`,
        target: `episode-${orderedEpisodeSummaries[i + 1].episode_id}`,
        type: 'labeled',
        animated: true,
        style: { stroke: '#94a3b8', strokeWidth: 2 },
        data: { label: `${i + 1}` },
      } as FlowEdge)
    }
    return edgesOut
  }, [orderedEpisodeSummaries, useEpisodeRendererSwap])

  const hasEpisodeOverview = useEpisodeRendererSwap && reviewSwapNodes.length > 0
  const hasDesignEpisodeOverview = !useEpisodeRendererSwap && designEpisodeNodes.length > 0

  const visibleNodes: FlowNode[] = useEpisodeRendererSwap
    ? hasEpisodeOverview
      ? isDrilldown
        ? (() => {
            const episodeNode = reviewSwapNodes.find((n) => n.id === activeSuperNodeId)
            const childSet = new Set(episodeNode?.data.childNodeIds ?? [])
            return nodes.filter((n) => childSet.has(n.id))
          })()
        : reviewSwapNodes
      : []
    : hasDesignEpisodeOverview
      ? isDrilldown
        ? nodes.filter((n) => {
            const episodeNode = nodes.find((s) => s.id === activeSuperNodeId)
            return (episodeNode?.data.childNodeIds ?? []).includes(n.id)
          })
        : designEpisodeNodes
      : isDrilldown
        ? nodes.filter((n) => {
            if (n.id === activeSuperNodeId) return true
            const superNode = nodes.find((s) => s.id === activeSuperNodeId)
            return (superNode?.data.childNodeIds ?? []).includes(n.id)
          })
        : nodes

  const visibleEdges: FlowEdge[] = useEpisodeRendererSwap
    ? hasEpisodeOverview
      ? isDrilldown
        ? (() => {
            const episodeNode = reviewSwapNodes.find((n) => n.id === activeSuperNodeId)
            const childSet = new Set(episodeNode?.data.childNodeIds ?? [])
            return edges.filter((e) => childSet.has(e.source) && childSet.has(e.target))
          })()
        : reviewSwapEdges
      : []
    : hasDesignEpisodeOverview
      ? isDrilldown
        ? (() => {
            const episodeNode = nodes.find((s) => s.id === activeSuperNodeId)
            const childSet = new Set(episodeNode?.data.childNodeIds ?? [])
            return edges.filter((e) => childSet.has(e.source) && childSet.has(e.target))
          })()
        : designEpisodeEdges
      : isDrilldown
        ? (() => {
            const superNode = nodes.find((s) => s.id === activeSuperNodeId)
            const childSet = new Set(superNode?.data.childNodeIds ?? [])
            childSet.add(activeSuperNodeId!)
            return edges.filter((e) => childSet.has(e.source) && childSet.has(e.target))
          })()
        : edges

  useEffect(() => {
    if (!useEpisodeRendererSwap) return
    if (!reactFlowRef.current) return
    if (visibleNodes.length === 0) return

    const fitKey = `${activeExecutionId ?? 'none'}:${viewLevel}:${activeSuperNodeId ?? 'root'}:${visibleNodes.length}`
    if (lastAutoFitKeyRef.current === fitKey) return
    lastAutoFitKeyRef.current = fitKey

    requestAnimationFrame(() => {
      reactFlowRef.current?.fitView({
        duration: 180,
        padding: 0.2,
        maxZoom: 1.25,
      })
    })
  }, [activeExecutionId, activeSuperNodeId, useEpisodeRendererSwap, viewLevel, visibleNodes.length])

  return (
    <div className="h-full w-full min-h-0 flex flex-col">
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
            {hasEpisodeOverview
              ? reviewSwapNodes.find((n) => n.id === activeSuperNodeId)?.data.label ?? activeSuperNodeId
              : nodes.find((n) => n.id === activeSuperNodeId)?.data.label ?? activeSuperNodeId}
          </span>
          <span className="text-indigo-400 dark:text-indigo-500 ml-1">
            ({visibleNodes.length} child node{visibleNodes.length !== 1 ? 's' : ''})
          </span>
        </div>
      )}

      <div className="flex-1 bg-[#0a0e17]">
        <ReactFlow<FlowNode, FlowEdge>
          className="bg-[#0a0e17]"
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
          panOnDrag={!isReview && !isRunning}
          zoomOnScroll={!isReview && !isRunning}
          zoomOnPinch={!isReview && !isRunning}
          zoomOnDoubleClick={!isReview && !isRunning}
          fitView={!isReview && !isRunning}
          snapToGrid={!isReview}
          snapGrid={[15, 15]}
          defaultEdgeOptions={{
            type: 'labeled',
            animated: true,
            style: { stroke: '#94a3b8', strokeWidth: 2 },
          }}
          proOptions={{ hideAttribution: true }}
        >
          <Background gap={16} size={1} color="rgba(56, 189, 248, 0.15)" />
          <Controls showInteractive={false} />
          <MiniMap
            nodeStrokeWidth={3}
            pannable
            zoomable
            className="!bg-[#0f172a]/95 !border-[#1e293b]"
          />
          {useEpisodeRendererSwap && episodesLoading && (
            <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <p className="text-sm text-gray-400 dark:text-gray-500">Loading episode overview...</p>
            </div>
          )}
          {isReview && useEpisodeRendererSwap && visibleNodes.length === 0 && !episodesLoading && (
            <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <p className="text-sm text-gray-400 dark:text-gray-500">No episode summaries available for this execution yet.</p>
            </div>
          )}
        </ReactFlow>
      </div>
    </div>
  )
}
