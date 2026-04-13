// Node types matching backend models.NodeType
export type NodeType = 'script' | 'llm' | 'mcp' | 'human' | 'router'

// Matches backend models.Node
export interface WorkflowNode {
  id: string
  name: string
  type: NodeType
  action: string
  config?: Record<string, unknown>
  depends_on?: string[]
}

// Matches backend models.Edge
export interface WorkflowEdge {
  from: string
  to: string
  condition?: string
}

// Matches backend models.DAGConfig
export interface DAGConfig {
  id?: string
  name: string
  description?: string
  nodes: WorkflowNode[]
  edges: WorkflowEdge[]
}

// Matches backend NodeResult
export interface NodeResult {
  node_id: string
  node_name: string
  node_type: NodeType
  status: 'success' | 'error' | 'skipped'
  output?: string
  error?: string
  duration_ms: number
  tokens_in?: number
  tokens_out?: number
}

export type ExecutionStatus = 'pending' | 'running' | 'completed' | 'failed' | 'timeout' | 'suspended'

// Response from POST /run and POST /dags/:id/run
export interface ExecutionStartResponse {
  execution_id: string
  status: ExecutionStatus
}

// Response from GET /executions/:id/nodes
export interface ExecutionNodesResponse {
  execution_id: string
  status: ExecutionStatus
  results: NodeResult[]
  error?: string
  started_at?: string
  ended_at?: string
  duration_ms?: number
}

// Node type metadata for the sidebar
export interface NodeTypeInfo {
  type: NodeType
  label: string
  description: string
  color: string
  bgColor: string
  borderColor: string
  icon: string
}

export const NODE_TYPE_INFO: Record<NodeType, NodeTypeInfo> = {
  script: {
    type: 'script',
    label: 'Script',
    description: 'Execute bash commands (Hard Node)',
    color: 'text-gray-700',
    bgColor: 'bg-node-script-light',
    borderColor: 'border-node-script-border',
    icon: 'Terminal',
  },
  llm: {
    type: 'llm',
    label: 'LLM',
    description: 'AI reasoning & analysis (Soft Node)',
    color: 'text-blue-700',
    bgColor: 'bg-node-llm-light',
    borderColor: 'border-node-llm-border',
    icon: 'Brain',
  },
  mcp: {
    type: 'mcp',
    label: 'MCP Tool',
    description: 'External tool via MCP protocol',
    color: 'text-violet-700',
    bgColor: 'bg-node-mcp-light',
    borderColor: 'border-node-mcp-border',
    icon: 'Plug',
  },
  human: {
    type: 'human',
    label: 'Human',
    description: 'Manual approval checkpoint',
    color: 'text-amber-700',
    bgColor: 'bg-node-human-light',
    borderColor: 'border-node-human-border',
    icon: 'User',
  },
  router: {
    type: 'router',
    label: 'Router',
    description: 'Conditional branching node',
    color: 'text-emerald-700',
    bgColor: 'bg-node-router-light',
    borderColor: 'border-node-router-border',
    icon: 'GitBranch',
  },
}
