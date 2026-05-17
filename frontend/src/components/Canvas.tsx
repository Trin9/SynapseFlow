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

// ---------------------------------------------------------------------------
// Shared left-to-right layout helper for FlowNode / FlowEdge.
// Strips back-edges via DFS before running Kahn topological BFS so that
// agent-loop subgraphs produce a clean branching layout instead of the
// alphabetical fallback we had before.
// ---------------------------------------------------------------------------
function computeFlowLayout(
  nodes: FlowNode[],
  flowEdges: FlowEdge[],
  opts?: { levelWidth?: number; nodeHeight?: number; startX?: number; centerY?: number },
): FlowNode[] {
  if (nodes.length === 0) return nodes

  const LEVEL_WIDTH = opts?.levelWidth ?? 300
  const NODE_HEIGHT = opts?.nodeHeight ?? 140
  const START_X = opts?.startX ?? 80
  const CENTER_Y = opts?.centerY ?? 320

  const nodeIds = nodes.map((n) => n.id)

  // Detect back-edges using DFS coloring.
  const adjFull: Record<string, string[]> = {}
  for (const id of nodeIds) adjFull[id] = []
  for (const e of flowEdges) {
    if (adjFull[e.source] !== undefined) adjFull[e.source].push(e.target)
  }
  const visited = new Set<string>()
  const inStack = new Set<string>()
  const backKeys = new Set<string>()
  function dfs(id: string): void {
    visited.add(id)
    inStack.add(id)
    for (const nb of adjFull[id] ?? []) {
      if (!visited.has(nb)) dfs(nb)
      else if (inStack.has(nb)) backKeys.add(`${id}→${nb}`)
    }
    inStack.delete(id)
  }
  for (const id of nodeIds) if (!visited.has(id)) dfs(id)

  const fwdEdges = flowEdges.filter((e) => !backKeys.has(`${e.source}→${e.target}`))

  // Kahn BFS on forward edges.
  const inDeg: Record<string, number> = {}
  const outMap: Record<string, string[]> = {}
  for (const id of nodeIds) { inDeg[id] = 0; outMap[id] = [] }
  for (const e of fwdEdges) {
    if (outMap[e.source] === undefined || inDeg[e.target] === undefined) continue
    outMap[e.source].push(e.target)
    inDeg[e.target] += 1
  }

  const levels: Record<string, number> = {}
  for (const id of nodeIds) levels[id] = 0
  const queue: string[] = []
  for (const id of nodeIds) if ((inDeg[id] ?? 0) === 0) queue.push(id)

  while (queue.length > 0) {
    const id = queue.shift()!
    const base = levels[id] ?? 0
    for (const to of outMap[id] ?? []) {
      if ((levels[to] ?? 0) < base + 1) levels[to] = base + 1
      inDeg[to] -= 1
      if (inDeg[to] === 0) queue.push(to)
    }
  }

  // Group nodes by level and assign (x, y).
  const groups: Record<number, string[]> = {}
  for (const id of nodeIds) {
    const l = levels[id] ?? 0
    if (!groups[l]) groups[l] = []
    groups[l].push(id)
  }

  // Reduce line crossings with a lightweight barycenter ordering.
  // Each level is sorted by the average index of predecessor nodes in the
  // previous level; ties fall back to semantic node priority.
  const nodeTypeByID = new Map<string, string>()
  for (const n of nodes) nodeTypeByID.set(n.id, n.data.nodeType as string)
  const parentMap: Record<string, string[]> = {}
  for (const id of nodeIds) parentMap[id] = []
  for (const e of fwdEdges) {
    if (parentMap[e.target] !== undefined) parentMap[e.target].push(e.source)
  }

  const typePriority = (nodeType: string | undefined): number => {
    switch (nodeType) {
      case 'start': return 0
      case 'llm': return 1
      case 'router': return 2
      case 'mcp':
      case 'mcp-tool': return 3
      case 'script': return 4
      case 'human': return 5
      case 'end': return 6
      default: return 99
    }
  }

  const orderedLevels = Object.keys(groups).map(Number).sort((a, b) => a - b)
  for (const level of orderedLevels) {
    const ids = groups[level]
    if (!ids || ids.length <= 1) continue
    if (level === orderedLevels[0]) {
      ids.sort((a, b) => {
        const pa = typePriority(nodeTypeByID.get(a))
        const pb = typePriority(nodeTypeByID.get(b))
        if (pa !== pb) return pa - pb
        return a.localeCompare(b)
      })
      continue
    }

    const prev = groups[level - 1] ?? []
    const prevIdx = new Map<string, number>()
    prev.forEach((id, idx) => prevIdx.set(id, idx))

    const barycenter = (id: string): number => {
      const parents = parentMap[id] ?? []
      if (parents.length === 0) return Number.MAX_SAFE_INTEGER
      let sum = 0
      let count = 0
      for (const p of parents) {
        const idx = prevIdx.get(p)
        if (idx !== undefined) {
          sum += idx
          count += 1
        }
      }
      if (count === 0) return Number.MAX_SAFE_INTEGER
      return sum / count
    }

    ids.sort((a, b) => {
      const ba = barycenter(a)
      const bb = barycenter(b)
      if (ba !== bb) return ba - bb
      const pa = typePriority(nodeTypeByID.get(a))
      const pb = typePriority(nodeTypeByID.get(b))
      if (pa !== pb) return pa - pb
      return a.localeCompare(b)
    })
  }

  const posMap: Record<string, { x: number; y: number }> = {}
  for (const [lStr, ids] of Object.entries(groups)) {
    const x = START_X + Number(lStr) * LEVEL_WIDTH
    ids.forEach((id, i) => {
      const total = (ids.length - 1) * NODE_HEIGHT
      posMap[id] = { x, y: CENTER_Y - total / 2 + i * NODE_HEIGHT }
    })
  }

  return nodes.map((n) => ({ ...n, position: posMap[n.id] ?? n.position }))
}

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
  
  // Helper: determine edge color based on source/target node types and condition
  const getEdgeColor = useCallback((edgeInfo: {
    sourceNodeType?: string
    targetNodeType?: string
    hasCondition: boolean
    crossesEpisodeBoundary?: boolean
  }): string => {
    const { sourceNodeType, targetNodeType, hasCondition, crossesEpisodeBoundary } = edgeInfo
    
    if (hasCondition) return '#10b981'  // green for conditional edges
    if (crossesEpisodeBoundary) return '#06b6d4'  // cyan for cross-episode edges
    if (sourceNodeType === 'llm' && targetNodeType === 'router') return '#a855f7'  // purple for LLM→Router
    if (sourceNodeType === 'router') return '#f97316'  // orange for Router output
    if ((sourceNodeType === 'script' || sourceNodeType === 'mcp') && targetNodeType === 'llm') return '#3b82f6'  // blue for evidence→LLM
    return '#6366f1'  // default indigo
  }, [])

  // Helper: apply directional colors to a list of edges
  const applyEdgeColors = useCallback((edgeList: FlowEdge[]): FlowEdge[] => {
    const nodeTypeMap = new Map<string, string>()
    for (const n of nodes) nodeTypeMap.set(n.id, n.data.nodeType as string)
    
    return edgeList.map((e) => ({
      ...e,
      style: {
        ...(e.style ?? {}),
        stroke: getEdgeColor({
          sourceNodeType: nodeTypeMap.get(e.source),
          targetNodeType: nodeTypeMap.get(e.target),
          hasCondition: !!(e.data?.condition),
        }),
      },
    }))
  }, [nodes, getEdgeColor])
  const setSelectedNodeId = useGraphStore((s) => s.setSelectedNodeId)
  const setShowHistory = useGraphStore((s) => s.setShowHistory)
  const enableAutoLayout = useGraphStore((s) => s.enableAutoLayout)

  // SuperNode drilldown
  const viewLevel = useGraphStore((s) => s.viewLevel)
  const activeSuperNodeId = useGraphStore((s) => s.activeSuperNodeId)
  const exitDrilldown = useGraphStore((s) => s.exitDrilldown)

  // In REVIEW mode the canvas is read-only: no drag, no connect, no select.
  const isReview = appMode === 'REVIEW'

  const reactFlowRef = useRef<ReactFlowInstance<FlowNode, FlowEdge> | null>(null)
  const lastAutoFitKeyRef = useRef<string | null>(null)
  const lastDrilldownEpisodeRef = useRef<string | null>(null)
  const pendingFocusEpisodeRef = useRef<string | null>(null)

  const handleBackToOverview = useCallback(() => {
    if (activeSuperNodeId) pendingFocusEpisodeRef.current = activeSuperNodeId
    exitDrilldown()
  }, [activeSuperNodeId, exitDrilldown])

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

  const annotateEpisodeBoundaryNodes = useCallback(
    (candidateNodes: FlowNode[], childNodeIds: string[]): FlowNode[] => {
      if (childNodeIds.length === 0) return candidateNodes

      const childSet = new Set(childNodeIds)
      const incomingFromOutside = new Set<string>()
      const outgoingToOutside = new Set<string>()

      for (const edge of edges) {
        const sourceIn = childSet.has(edge.source)
        const targetIn = childSet.has(edge.target)
        if (!sourceIn && targetIn) incomingFromOutside.add(edge.target)
        if (sourceIn && !targetIn) outgoingToOutside.add(edge.source)
      }

      return candidateNodes.map((node) => {
        if (!childSet.has(node.id)) return node

        const isEntry = incomingFromOutside.has(node.id)
        const isExit = outgoingToOutside.has(node.id)
        const ioRole = isEntry && isExit ? 'entry_exit' : isEntry ? 'entry' : isExit ? 'exit' : ''
        if (!ioRole) return node

        return {
          ...node,
          data: {
            ...node.data,
            config: {
              ...(node.data.config ?? {}),
              __episode_io_role: ioRole,
            },
          },
        }
      })
    },
    [edges]
  )

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

  const designStandaloneNodes = useMemo<FlowNode[]>(() => {
    if (useEpisodeRendererSwap) return []

    const assignedChildIds = new Set<string>()
    for (const episode of nodes.filter((node) => node.data.nodeType === 'super')) {
      for (const childId of episode.data.childNodeIds ?? []) assignedChildIds.add(childId)
    }

    return nodes.filter((node) => node.data.nodeType !== 'super' && !assignedChildIds.has(node.id))
  }, [nodes, useEpisodeRendererSwap])

  const designOverviewEdges = useMemo<FlowEdge[]>(() => {
    if (useEpisodeRendererSwap) return []

    const designEpisodes = nodes.filter((node) => node.data.nodeType === 'super')
    if (designEpisodes.length === 0) return edges

    const ownerByChild = new Map<string, string>()
    const nodeTypeMap = new Map<string, string>()
    for (const n of nodes) nodeTypeMap.set(n.id, n.data.nodeType as string)
    
    for (const episode of designEpisodes) {
      for (const childId of episode.data.childNodeIds ?? []) ownerByChild.set(childId, episode.id)
    }

    const seen = new Set<string>()
    const nextEdges: FlowEdge[] = []

    for (const edge of edges) {
      const source = ownerByChild.get(edge.source) ?? edge.source
      const target = ownerByChild.get(edge.target) ?? edge.target
      if (source === target) continue

      const condition = edge.data?.condition as string | undefined
      const key = `${source}->${target}:${condition ?? ''}`
      if (seen.has(key)) continue
      seen.add(key)

      const crossesEpisodeBoundary = source !== edge.source || target !== edge.target
      const edgeColor = getEdgeColor({
        sourceNodeType: nodeTypeMap.get(edge.source),
        targetNodeType: nodeTypeMap.get(edge.target),
        hasCondition: !!condition,
        crossesEpisodeBoundary,
      })

      nextEdges.push({
        id: `design-overview-${source}-${target}-${nextEdges.length}`,
        source,
        target,
        type: 'labeled',
        data: condition ? { condition } : undefined,
        animated: crossesEpisodeBoundary,
        style: {
          stroke: edgeColor,
          strokeWidth: 2,
          ...(crossesEpisodeBoundary && { strokeDasharray: '6 4' }),
        },
      } as FlowEdge)
    }

    return nextEdges
  }, [edges, nodes, useEpisodeRendererSwap, getEdgeColor])

  // Lay out the combined Episode-shell + standalone-node overview so that
  // positions are derived from the overview-level topology rather than each
  // node's full-graph (or hardcoded) position.  Episode shells are tall
  // (~300 px) and wide (~380 px), so we use generous spacing here.
  // When user manually drags nodes, disable auto-layout to preserve their edits.
  const designOverviewNodes = useMemo<FlowNode[]>(() => {
    if (useEpisodeRendererSwap) return []
    const combined = [...designEpisodeNodes, ...designStandaloneNodes]
    if (combined.length === 0 || !enableAutoLayout) return combined
    return computeFlowLayout(combined, designOverviewEdges, {
      levelWidth: 520,   // Episode cards are ~380 px wide; leave breathing room
      nodeHeight: 340,   // Episode cards can be ~300 px tall
      startX: 80,
      centerY: 380,
    })
  }, [designEpisodeNodes, designStandaloneNodes, designOverviewEdges, useEpisodeRendererSwap, enableAutoLayout])

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
            const childIds = Array.isArray(episodeNode?.data.childNodeIds) ? episodeNode.data.childNodeIds : []
            const childSet = new Set(childIds)
            const drilldownEdges = edges.filter((e) => childSet.has(e.source) && childSet.has(e.target))
            const rawNodes = nodes.filter((n) => childSet.has(n.id))
            const layoutNodes = enableAutoLayout ? computeFlowLayout(rawNodes, drilldownEdges, {
              levelWidth: 400, nodeHeight: 200, startX: 80, centerY: 320,
            }) : rawNodes
            return annotateEpisodeBoundaryNodes(layoutNodes, childIds)
          })()
        : reviewSwapNodes
      : []
    : hasDesignEpisodeOverview
      ? isDrilldown
        ? (() => {
            const episodeNode = nodes.find((s) => s.id === activeSuperNodeId)
            const childIds = Array.isArray(episodeNode?.data.childNodeIds) ? episodeNode.data.childNodeIds : []
            const childSet = new Set(childIds)
            const drilldownEdges = edges.filter((e) => childSet.has(e.source) && childSet.has(e.target))
            const rawNodes = nodes.filter((n) => childIds.includes(n.id))
            const layoutNodes = enableAutoLayout ? computeFlowLayout(rawNodes, drilldownEdges, {
              levelWidth: 400, nodeHeight: 200, startX: 80, centerY: 320,
            }) : rawNodes
            return annotateEpisodeBoundaryNodes(layoutNodes, childIds)
          })()
        : designOverviewNodes
      : isDrilldown
        ? nodes.filter((n) => {
            if (n.id === activeSuperNodeId) return true
            const superNode = nodes.find((s) => s.id === activeSuperNodeId)
            return (superNode?.data.childNodeIds ?? []).includes(n.id)
          })
        : nodes

  const visibleEdgesBeforeColor: FlowEdge[] = useEpisodeRendererSwap
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
        : designOverviewEdges.length > 0 ? designOverviewEdges : designEpisodeEdges
      : isDrilldown
        ? (() => {
            const superNode = nodes.find((s) => s.id === activeSuperNodeId)
            const childSet = new Set(superNode?.data.childNodeIds ?? [])
            childSet.add(activeSuperNodeId!)
            return edges.filter((e) => childSet.has(e.source) && childSet.has(e.target))
          })()
        : edges

  // Apply directional colors to visible edges (except overview/drilldown which already have colors)
  const visibleEdges = useMemo(() => {
    if (hasDesignEpisodeOverview) return visibleEdgesBeforeColor  // already colored
    return applyEdgeColors(visibleEdgesBeforeColor)
  }, [visibleEdgesBeforeColor, hasDesignEpisodeOverview, applyEdgeColors])

  useEffect(() => {
    if (!useEpisodeRendererSwap) return
    if (!reactFlowRef.current) return
    if (visibleNodes.length === 0) return

    if (isDrilldown && activeSuperNodeId) {
      lastDrilldownEpisodeRef.current = activeSuperNodeId
    }

    const fitKey = `${activeExecutionId ?? 'none'}:${viewLevel}:${activeSuperNodeId ?? 'root'}:${visibleNodes.length}`
    if (lastAutoFitKeyRef.current === fitKey) return
    lastAutoFitKeyRef.current = fitKey

    requestAnimationFrame(() => {
      const focusEpisodeId = pendingFocusEpisodeRef.current ?? lastDrilldownEpisodeRef.current
      if (viewLevel === 'overview' && focusEpisodeId && visibleNodes.some((n) => n.id === focusEpisodeId)) {
        pendingFocusEpisodeRef.current = null
        reactFlowRef.current?.fitView({
          nodes: [{ id: focusEpisodeId }],
          duration: 180,
          padding: 0.3,
          maxZoom: 1.25,
        })
        return
      }
      reactFlowRef.current?.fitView({
        duration: 180,
        padding: 0.2,
        maxZoom: 1.25,
      })
    })
  }, [activeExecutionId, activeSuperNodeId, isDrilldown, useEpisodeRendererSwap, viewLevel, visibleNodes.length])

  return (
    <div className="h-full w-full min-h-0 flex flex-col">
      {/* Drilldown breadcrumb bar */}
      {isDrilldown && (
        <div className="shrink-0 border-b border-cyan-700/40 bg-[#0c1220]">
          {(hasEpisodeOverview || hasDesignEpisodeOverview) && (
            <div className="h-24 border-b border-cyan-900/40">
              <ReactFlow<FlowNode, FlowEdge>
                nodes={(hasEpisodeOverview ? reviewSwapNodes : designOverviewNodes).map((n) => ({
                  ...n,
                  className: n.id === activeSuperNodeId ? 'ring-2 ring-cyan-400/80' : '',
                }))}
                edges={hasEpisodeOverview ? reviewSwapEdges : designEpisodeEdges}
                nodeTypes={nodeTypes}
                edgeTypes={edgeTypes}
                fitView
                nodesDraggable={false}
                nodesConnectable={false}
                elementsSelectable={false}
                panOnDrag={false}
                zoomOnPinch={false}
                zoomOnScroll={false}
                zoomOnDoubleClick={false}
                proOptions={{ hideAttribution: true }}
              >
                <Background gap={14} size={1} color="rgba(34, 211, 238, 0.12)" />
              </ReactFlow>
            </div>
          )}
          <div className="flex items-center gap-2 px-4 py-1.5 text-xs">
            <button
              onClick={handleBackToOverview}
              className="text-cyan-300 hover:text-cyan-100 font-medium transition-colors"
            >
              ← Overview
            </button>
            <span className="text-cyan-700">/</span>
            <span className="font-semibold text-cyan-200">
              {hasEpisodeOverview
                ? reviewSwapNodes.find((n) => n.id === activeSuperNodeId)?.data.label ?? activeSuperNodeId
                : nodes.find((n) => n.id === activeSuperNodeId)?.data.label ?? activeSuperNodeId}
            </span>
            <span className="text-cyan-500 ml-1">
              ({visibleNodes.length} child node{visibleNodes.length !== 1 ? 's' : ''})
            </span>
          </div>
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
            style: { stroke: '#6366f1', strokeWidth: 2 },  // indigo: clearer than gray
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
