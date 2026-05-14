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
import type {
  NodeType,
  AnyNodeType,
  DAGConfig,
  DesignEpisodeSpec,
  WorkflowNode,
  WorkflowEdge,
  ExecutionNodesResponse,
} from '@/types'
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
  const inDegreeCount: Record<string, number> = {}
  const outEdgesMap: Record<string, string[]> = {}
  for (const n of nodes) {
    inDegreeCount[n.id] = 0
    outEdgesMap[n.id] = []
  }
  for (const e of dagEdges) {
    if (outEdgesMap[e.from] === undefined || inDegreeCount[e.to] === undefined) continue
    outEdgesMap[e.from].push(e.to)
    inDegreeCount[e.to] += 1
  }

  // Assign levels with Kahn topological traversal (cycle-safe).
  const levels: Record<string, number> = {}
  for (const n of nodes) levels[n.id] = 0

  const queue: string[] = []
  for (const n of nodes) {
    if ((inDegreeCount[n.id] ?? 0) === 0) queue.push(n.id)
  }

  let processed = 0
  while (queue.length > 0) {
    const id = queue.shift()!
    processed += 1
    const baseLevel = levels[id] ?? 0
    for (const to of outEdgesMap[id] ?? []) {
      const newLevel = baseLevel + 1
      if ((levels[to] ?? 0) < newLevel) levels[to] = newLevel
      inDegreeCount[to] -= 1
      if (inDegreeCount[to] === 0) queue.push(to)
    }
  }

  // If cycles exist (agent loops), keep layout deterministic instead of hanging.
  if (processed < nodes.length) {
    const unresolved = nodes
      .map((n) => n.id)
      .filter((id) => (inDegreeCount[id] ?? 0) > 0)
      .sort((a, b) => a.localeCompare(b))
    unresolved.forEach((id, idx) => {
      levels[id] = Math.max(levels[id] ?? 0, idx)
    })
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

const DESIGN_EPISODE_METADATA_KEY = 'synapse.design_episode_specs'

interface SerializedDesignEpisode {
  id: string
  label: string
  summary?: string
  config?: Record<string, unknown>
  childNodeIds?: string[]
  position?: { x: number; y: number }
}

function sanitizeChildNodeIds(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string' && item.length > 0)
}

function deserializeDesignEpisodeSpecs(specs?: DesignEpisodeSpec[]): SerializedDesignEpisode[] {
  if (!Array.isArray(specs)) return []
  return specs
    .filter((item): item is DesignEpisodeSpec => !!item && typeof item.id === 'string' && typeof item.label === 'string')
    .map((item) => ({
      id: item.id,
      label: item.label,
      summary: typeof item.summary === 'string' ? item.summary : '',
      config: item.config ?? {
        expected_behaviors: Array.isArray(item.expected_behaviors) ? item.expected_behaviors : [],
      },
      childNodeIds: Array.isArray(item.node_ids) ? item.node_ids : [],
      position: undefined,
    }))
}

function parseDesignEpisodeNodes(dag: DAGConfig): FlowNode[] {
  let parsed: SerializedDesignEpisode[] = deserializeDesignEpisodeSpecs(dag.episodes)

  if (parsed.length === 0) {
    const raw = dag.metadata?.[DESIGN_EPISODE_METADATA_KEY]
    if (raw) {
      try {
        const legacy = JSON.parse(raw) as SerializedDesignEpisode[]
        if (Array.isArray(legacy)) parsed = legacy
      } catch {
        parsed = []
      }
    }
  }

  return parsed
    .filter((item): item is SerializedDesignEpisode => !!item && typeof item.id === 'string' && typeof item.label === 'string')
    .map((item, idx) => ({
      id: item.id,
      type: 'superNode',
      position: item.position ?? { x: 120 + idx * 380, y: 120 },
      data: {
        label: item.label,
        nodeType: 'super',
        action: typeof item.summary === 'string' ? item.summary : '',
        config: item.config ?? {},
        childNodeIds: sanitizeChildNodeIds(item.childNodeIds),
      },
    }))
}

function serializeDesignEpisodeNodes(nodes: FlowNode[]): string {
  const serialized: SerializedDesignEpisode[] = nodes
    .filter((node) => node.data.nodeType === 'super')
    .map((node) => ({
      id: node.id,
      label: node.data.label,
      summary: node.data.action,
      config: node.data.config,
      childNodeIds: sanitizeChildNodeIds(node.data.childNodeIds),
      position: node.position,
    }))
  return JSON.stringify(serialized)
}

function buildDesignEpisodeSpecs(nodes: FlowNode[]): DesignEpisodeSpec[] {
  return nodes
    .filter((node) => node.data.nodeType === 'super')
    .map((node) => {
      const config = node.data.config ?? {}
      const expectedBehaviors = Array.isArray(config.expected_behaviors)
        ? config.expected_behaviors.filter((item): item is string => typeof item === 'string')
        : []
      return {
        id: node.id,
        label: node.data.label,
        summary: node.data.action,
        expected_behaviors: expectedBehaviors,
        node_ids: sanitizeChildNodeIds(node.data.childNodeIds),
        config,
      }
    })
}

interface GraphState {
  // Layout — always Workbench (Classic/Workbench toggle removed in Batch 3).
  useWorkbenchLayout: boolean

  // Trigger Context sidebar visibility (moved from WorkbenchLayout local state to store
  // so the header bar can toggle it without prop drilling through App).
  showTriggerCtx: boolean
  setShowTriggerCtx: (show: boolean) => void

  // Mode toggle
  appMode: AppMode
  setAppMode: (mode: AppMode) => void
  /** Switch to REVIEW mode and set the execution being reviewed. */
  enterReviewMode: (executionId?: string | null) => void
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
  workflowMetadata: Record<string, string>
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

  // Comparison intent (used to open comparison sheet from library/history actions)
  openComparisonOnDossier: boolean
  setOpenComparisonOnDossier: (open: boolean) => void

  // M3.4 — replay position (0–100)
  replayPercent: number
  setReplayPercent: (n: number) => void

  // B2 — focused episode for the bottom ProcessTraceTray (separate from Dossier open)
  focusedEpisodeId: string | null
  setFocusedEpisodeId: (id: string | null) => void

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
  // Always Workbench — Classic layout has been removed (Batch 3 simplification).
  useWorkbenchLayout: true,

  showTriggerCtx: false,
  setShowTriggerCtx: (show) => set({ showTriggerCtx: show }),

  appMode: 'BUILDER',
  setAppMode: (mode) => set({ appMode: mode }),

  enterReviewMode: (executionId) => {
    set({
      appMode: 'REVIEW',
      activeExecutionId: executionId ?? get().activeExecutionId,
      // Reset any in-progress run state so the canvas shows review colours cleanly.
      isRunning: false,
      // Keep side drawers closed by default when entering review.
      showHistory: false,
      showLibrary: false,
      showTriggerCtx: false,
      selectedNodeId: null,
      // B2: close any open dossier from a previous execution so it doesn't show stale data.
      selectedEpisode: null,
      replayPercent: 100,
      openComparisonOnDossier: false,
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
      focusedEpisodeId: null,
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
      super: 'Design Episode',
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
  workflowMetadata: {},
  setWorkflowId: (id) => set({ workflowId: id }),
  setWorkflowName: (name) => set({ workflowName: name }),

  isRunning: false,
  activeExecutionId: null,
  executionResult: null,
  nodeStatuses: {},
  setIsRunning: (running) => {
    if (running) {
      const next: Record<string, 'idle' | 'running' | 'success' | 'error' | 'skipped'> = {}
      for (const n of get().nodes) next[n.id] = get().nodeStatuses[n.id] ?? 'idle'
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

  openComparisonOnDossier: false,
  setOpenComparisonOnDossier: (open) => set({ openComparisonOnDossier: open }),

  replayPercent: 100,
  setReplayPercent: (n) => set({ replayPercent: n }),

  focusedEpisodeId: null,
  setFocusedEpisodeId: (id) => set({ focusedEpisodeId: id }),

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

    const prev = get().nodeStatuses
    const next: Record<string, 'idle' | 'running' | 'success' | 'error' | 'skipped'> = { ...prev }

    // Keep statuses sticky between polling snapshots; only update nodes that
    // are present in the latest results payload.
    for (const n of get().nodes) {
      if (!next[n.id]) next[n.id] = 'idle'
    }
    for (const r of (result.results ?? [])) next[r.node_id] = r.status

    // Once execution is terminal, clear any leftover running marks.
    if (result.status !== 'running') {
      for (const [id, st] of Object.entries(next)) {
        if (st === 'running') next[id] = 'idle'
      }
    }

    set({ executionResult: result, nodeStatuses: next })
  },

  toDAGConfig: () => {
    const { nodes, edges, workflowName, workflowId, workflowMetadata } = get()

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

    const metadata: Record<string, string> = {
      ...workflowMetadata,
      [DESIGN_EPISODE_METADATA_KEY]: serializeDesignEpisodeNodes(nodes),
    }

    return {
      id: workflowId ?? undefined,
      name: workflowName,
      metadata,
      episodes: buildDesignEpisodeSpecs(nodes),
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

    // Apply BFS-based layout so fan-out nodes don't overlap, then append
    // design-time episode shells restored from DAG metadata.
    const layoutNodes = computeDAGLayout(rawNodes, dagEdges)
    const nodes = [...layoutNodes, ...parseDesignEpisodeNodes(dag)]

    set({
      nodes,
      edges,
      workflowId: dag.id ?? null,
      workflowName: dag.name ?? 'Untitled Workflow',
      workflowMetadata: dag.metadata ?? {},
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
      workflowMetadata: {},
      nodeStatuses: {},
      focusedEpisodeId: null,
      viewLevel: 'overview',
      activeSuperNodeId: null,
    })
    nodeIdCounter = 0
  },
}))
