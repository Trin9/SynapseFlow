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
import type { NodeType, DAGConfig, WorkflowNode, WorkflowEdge, ExecutionNodesResponse } from '@/types'
import type { FlowNodeData } from '@/components/nodes/CustomNodes'

export type FlowNode = Node<FlowNodeData>
export type FlowEdge = Edge

interface GraphState {
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
  addNode: (type: NodeType, position: { x: number; y: number }) => void
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
    const defaultLabels: Record<NodeType, string> = {
      script: 'Script Node',
      llm: 'LLM Analysis',
      mcp: 'MCP Tool',
      human: 'Manual Review',
      router: 'Router',
    }

    const newNode: FlowNode = {
      id,
      type: `${type}Node`,
      position,
      data: {
        label: defaultLabels[type],
        nodeType: type,
        action: '',
        config: {},
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

    const workflowNodes: WorkflowNode[] = nodes.map((node) => ({
      id: node.id,
      name: node.data.label,
      type: node.data.nodeType,
      action: node.data.action,
      config: node.data.config,
    }))

    const workflowEdges: WorkflowEdge[] = edges.map((edge) => ({
      from: edge.source,
      to: edge.target,
    }))

    return {
      id: workflowId ?? undefined,
      name: workflowName,
      nodes: workflowNodes,
      edges: workflowEdges,
    }
  },

  loadFromDAGConfig: (dag) => {
    const nodes: FlowNode[] = (dag.nodes ?? []).map((n, idx) => ({
      id: n.id,
      type: `${n.type}Node`,
      position: { x: 80, y: 80 + idx * 110 },
      data: {
        label: n.name,
        nodeType: n.type,
        action: n.action,
        config: (n.config ?? {}) as Record<string, unknown>,
      },
    }))

    const edges: FlowEdge[] = (dag.edges ?? []).map((e) => ({
      id: `e_${e.from}_${e.to}`,
      source: e.from,
      target: e.to,
    }))

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
    })
    nodeIdCounter = 0
  },
}))
