import { create } from 'zustand'
import {
  type Node,
  type Edge,
  type OnNodesChange,
  type OnEdgesChange,
  type OnConnect,
  applyNodeChanges,
  applyEdgeChanges,
  addEdge,
} from '@xyflow/react'
import type { NodeType, AnyNodeType, DAGConfig, WorkflowNode, WorkflowEdge, ExecutionNodesResponse } from '@/types'
import type { FlowNodeData } from '@/components/nodes/CustomNodes'
import type { Episode } from '@/types/episode'

// ---------------------------------------------------------------------------
// DAG layout: BFS-based level assignment, horizontal layout (left → right)
// Handles fan-out/fan-in correctly by assigning the maximum depth per node.
// ---------------------------------------------------------------------------
function computeDAGLayout(
  nodes: FlowNode[],
  dagEdges: WorkflowEdge[],
): FlowNode[] {
  const LEVEL_WIDTH = 270   // horizontal spacing between levels
  const NODE_HEIGHT = 120   // vertical spacing between nodes in the same level
  const START_X = 80
  const CENTER_Y = 320

  // Build adjacency maps
  const inDegree: Record<string, string[]> = {}
  const outEdgesMap: Record<string, string[]> = {}
  for (const n of nodes) {
    inDegree[n.id] = []
    outEdgesMap[n.id] = []
  }
  for (const e of dagEdges) {
    if (outEdgesMap[e.from] !== undefined) outEdgesMap[e.from].push(e.to)
    if (inDegree[e.to] !== undefined) inDegree[e.to].push(e.from)
  }

  // Assign levels: use longest-path (critical path) depth via BFS/relaxation
  const levels: Record<string, number> = {}
  for (const n of nodes) levels[n.id] = 0

  // Topological relaxation — iterate until stable (handles any DAG depth)
  let changed = true
  while (changed) {
    changed = false
    for (const e of dagEdges) {
      const newLevel = (levels[e.from] ?? 0) + 1
      if ((levels[e.to] ?? 0) < newLevel) {
        levels[e.to] = newLevel
        changed = true
      }
    }
  }

  // Group nodes by level
  const levelGroups: Record<number, string[]> = {}
  for (const n of nodes) {
    const l = levels[n.id] ?? 0
    if (!levelGroups[l]) levelGroups[l] = []
    levelGroups[l].push(n.id)
  }

  // Assign positions: levels left→right, nodes within a level centered vertically
  const posMap: Record<string, { x: number; y: number }> = {}
  for (const [levelStr, nodeIds] of Object.entries(levelGroups)) {
    const level = Number(levelStr)
    const x = START_X + level * LEVEL_WIDTH
    nodeIds.forEach((id, i) => {
      const totalHeight = (nodeIds.length - 1) * NODE_HEIGHT
      const y = CENTER_Y - totalHeight / 2 + i * NODE_HEIGHT
      posMap[id] = { x, y }
    })
  }

  return nodes.map((n) => ({ ...n, position: posMap[n.id] ?? n.position }))
}

export type FlowNode = Node<FlowNodeData>
export type FlowEdge = Edge

export type AppMode = 'BUILDER' | 'REVIEW'

interface GraphState {
  // Layout feature flag (Phase 0 / Phase A)
  useWorkbenchLayout: boolean
  setUseWorkbenchLayout: (enabled: boolean) => void

  // Mode toggle
  appMode: AppMode
  setAppMode: (mode: AppMode) => void
  /** Switch to REVIEW mode and set the execution being reviewed. */
  enterReviewMode: (executionId: string) => void
  /** Return to BUILDER mode, clearing any review-specific state. */
  exitReviewMode: () => void

  // React Flow state
  nodes: FlowNode[]
  edges: FlowEdge[]
  onNodesChange: OnNodesChange<FlowNode>
  onEdgesChange: OnEdgesChange
  onConnect: OnConnect

  // Selection
  selectedNodeId: string | null
  setSelectedNodeId: (id: string | null) => void

  // Node operations
  addNode: (type: AnyNodeType, position: { x: number; y: number }) => void
  updateNodeData: (nodeId: string, data: Partial<FlowNodeData>) => void
  deleteNode: (nodeId: string) => void

  // Workflow name
  workflowId: string | null
  workflowName: string
  setWorkflowId: (id: string | null) => void
  setWorkflowName: (name: string) => void

  // Execution state
  isRunning: boolean
  activeExecutionId: string | null
  executionResult: ExecutionNodesResponse | null
  nodeStatuses: Record<string, 'idle' | 'running' | 'success' | 'error' | 'skipped'>
  setIsRunning: (running: boolean) => void
  setActiveExecutionId: (id: string | null) => void
  setExecutionResult: (result: ExecutionNodesResponse | null) => void

  // History panel
  showHistory: boolean
  setShowHistory: (show: boolean) => void

  // Library panel
  showLibrary: boolean
  setShowLibrary: (show: boolean) => void

  // Episode detail overlay (Scope 3)
  selectedEpisode: Episode | null
  setSelectedEpisode: (ep: Episode | null) => void

  // M3.4 — replay position (0–100)
  replayPercent: number
  setReplayPercent: (n: number) => void

  // SuperNode drilldown view level
  viewLevel: 'overview' | 'drilldown'
  activeSuperNodeId: string | null
  enterDrilldown: (nodeId: string) => void
  exitDrilldown: () => void

  // Serialization
  toDAGConfig: () => DAGConfig
  loadFromDAGConfig: (dag: DAGConfig) => void

  // Clear
  clearCanvas: () => void
}

let nodeIdCounter = 0

function generateNodeId(): string {
  nodeIdCounter++
  return `node_${nodeIdCounter}_${Date.now()}`
}

export const useGraphStore = create<GraphState>((set, get) => ({
  useWorkbenchLayout: false,
  setUseWorkbenchLayout: (enabled) => set({ useWorkbenchLayout: enabled }),

  appMode: 'BUILDER',
  setAppMode: (mode) => set({ appMode: mode }),

  enterReviewMode: (executionId) => {
    set({
      appMode: 'REVIEW',
      activeExecutionId: executionId,
      // Reset any in-progress run state so the canvas shows review colours cleanly.
      isRunning: false,
      // CR-016: open history panel and collapse library; clear any leftover node selection.
      showHistory: true,
      showLibrary: false,
      selectedNodeId: null,
    })
  },

  exitReviewMode: () => {
    set({
      appMode: 'BUILDER',
      activeExecutionId: null,
      executionResult: null,
      nodeStatuses: {},
      isRunning: false,
      // CR-016: clear review-specific overlay state so the canvas is truly reset.
      selectedEpisode: null,
      replayPercent: 100,
      viewLevel: 'overview',
      activeSuperNodeId: null,
    })
  },

  nodes: [],
  edges: [],

  onNodesChange: (changes) => {
    set({ nodes: applyNodeChanges(changes, get().nodes) })
  },

  onEdgesChange: (changes) => {
    set({ edges: applyEdgeChanges(changes, get().edges) })
  },

  onConnect: (connection) => {
    set({ edges: addEdge(connection, get().edges) })
  },

  selectedNodeId: null,
  setSelectedNodeId: (id) => set({ selectedNodeId: id }),

  addNode: (type, position) => {
    const id = generateNodeId()
    const defaultLabels: Record<AnyNodeType, string> = {
      script: 'Script Node',
      llm: 'LLM Analysis',
      mcp: 'MCP Tool',
      human: 'Manual Review',
      router: 'Router',
      super: 'Super Group',
    }

    const newNode: FlowNode = {
      id,
      type: type === 'super' ? 'superNode' : `${type}Node`,
      position,
      data: {
        label: defaultLabels[type],
        nodeType: type,
        action: '',
        config: {},
        ...(type === 'super' ? { childNodeIds: [] } : {}),
      },
    }

    set({ nodes: [...get().nodes, newNode] })
  },

  updateNodeData: (nodeId, data) => {
    set({
      nodes: get().nodes.map((node) =>
        node.id === nodeId
          ? { ...node, data: { ...node.data, ...data } }
          : node
      ),
    })
  },

  deleteNode: (nodeId) => {
    set({
      nodes: get().nodes.filter((n) => n.id !== nodeId),
      edges: get().edges.filter((e) => e.source !== nodeId && e.target !== nodeId),
      selectedNodeId: get().selectedNodeId === nodeId ? null : get().selectedNodeId,
    })
  },

  workflowId: null,
  workflowName: 'Untitled Workflow',
  setWorkflowId: (id) => set({ workflowId: id }),
  setWorkflowName: (name) => set({ workflowName: name }),

  isRunning: false,
  activeExecutionId: null,
  executionResult: null,
  nodeStatuses: {},
  setIsRunning: (running) => {
    if (running) {
      // Best-effort: mark all nodes as running until results arrive.
      const next: Record<string, 'idle' | 'running' | 'success' | 'error' | 'skipped'> = {}
      for (const n of get().nodes) next[n.id] = 'running'
      set({ isRunning: true, nodeStatuses: next })
      return
    }
    set({ isRunning: false })
  },
  setActiveExecutionId: (id) => set({ activeExecutionId: id }),

  showHistory: false,
  setShowHistory: (show) => set({ showHistory: show }),

  showLibrary: false,
  setShowLibrary: (show) => set({ showLibrary: show }),

  selectedEpisode: null,
  setSelectedEpisode: (ep) => set({ selectedEpisode: ep }),

  replayPercent: 100,
  setReplayPercent: (n) => set({ replayPercent: n }),

  viewLevel: 'overview',
  activeSuperNodeId: null,
  enterDrilldown: (nodeId) => set({ viewLevel: 'drilldown', activeSuperNodeId: nodeId }),
  exitDrilldown: () => set({ viewLevel: 'overview', activeSuperNodeId: null }),

  setExecutionResult: (result) => {
    if (!result) {
      // Reset to idle
      const next: Record<string, 'idle' | 'running' | 'success' | 'error' | 'skipped'> = {}
      for (const n of get().nodes) next[n.id] = 'idle'
      set({ executionResult: null, nodeStatuses: next })
      return
    }

    const next: Record<string, 'idle' | 'running' | 'success' | 'error' | 'skipped'> = {}
    const base: 'idle' | 'running' = result.status === 'running' ? 'running' : 'idle'
    for (const n of get().nodes) next[n.id] = base
    for (const r of (result.results ?? [])) next[r.node_id] = r.status

    set({ executionResult: result, nodeStatuses: next })
  },

  toDAGConfig: () => {
    const { nodes, edges, workflowName, workflowId } = get()

    // Super nodes are canvas-only grouping constructs — never sent to the backend.
    const workflowNodes: WorkflowNode[] = nodes
      .filter((node) => node.data.nodeType !== 'super')
      .map((node) => ({
        id: node.id,
        name: node.data.label,
        type: node.data.nodeType as NodeType,
        action: node.data.action,
        config: node.data.config,
      }))

    const workflowEdges: WorkflowEdge[] = edges.map((edge) => {
      const condition = edge.data?.condition as string | undefined
      const entry: WorkflowEdge = { from: edge.source, to: edge.target }
      if (condition) entry.condition = condition
      return entry
    })

    return {
      id: workflowId ?? undefined,
      name: workflowName,
      nodes: workflowNodes,
      edges: workflowEdges,
    }
  },

  loadFromDAGConfig: (dag) => {
    const rawNodes: FlowNode[] = (dag.nodes ?? []).map((n, idx) => ({
      id: n.id,
      type: `${n.type}Node`,
      position: { x: 80, y: 80 + idx * 110 },  // overwritten by computeDAGLayout below
      data: {
        label: n.name,
        nodeType: n.type,
        action: n.action,
        config: (n.config ?? {}) as Record<string, unknown>,
      },
    }))

    const dagEdges = dag.edges ?? []

    const edges: FlowEdge[] = dagEdges.map((e) => ({
      id: `e_${e.from}_${e.to}`,
      source: e.from,
      target: e.to,
      type: e.condition ? 'labeled' : undefined,
      data: e.condition ? { condition: e.condition } : undefined,
    }))

    // Apply BFS-based layout so fan-out nodes don't overlap
    const nodes = computeDAGLayout(rawNodes, dagEdges)

    set({
      nodes,
      edges,
      workflowId: dag.id ?? null,
      workflowName: dag.name ?? 'Untitled Workflow',
      selectedNodeId: null,
      executionResult: null,
      activeExecutionId: null,
      isRunning: false,
      nodeStatuses: {},
    })
    nodeIdCounter = nodes.length
  },

  clearCanvas: () => {
    set({
      nodes: [],
      edges: [],
      selectedNodeId: null,
      executionResult: null,
      activeExecutionId: null,
      workflowId: null,
      nodeStatuses: {},
      viewLevel: 'overview',
      activeSuperNodeId: null,
    })
    nodeIdCounter = 0
  },
}))
