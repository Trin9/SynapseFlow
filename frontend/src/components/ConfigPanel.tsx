import { useEffect, useState, useCallback } from 'react'
import { useGraphStore } from '@/hooks/useGraphStore'
import { NODE_TYPE_INFO } from '@/types'

/**
 * Right-side configuration panel that appears when a node is selected.
 * Shows dynamic form fields based on the node type.
 */
export function ConfigPanel() {
  const appMode = useGraphStore((s) => s.appMode)
  const selectedNodeId = useGraphStore((s) => s.selectedNodeId)
  const nodes = useGraphStore((s) => s.nodes)
  const updateNodeData = useGraphStore((s) => s.updateNodeData)
  const deleteNode = useGraphStore((s) => s.deleteNode)
  const setSelectedNodeId = useGraphStore((s) => s.setSelectedNodeId)

  const selectedNode = nodes.find((n) => n.id === selectedNodeId)

  // Local form state (syncs to store on change)
  const [label, setLabel] = useState('')
  const [action, setAction] = useState('')
  const [configJson, setConfigJson] = useState('')

  // Sync from store to local state when selection changes
  useEffect(() => {
    if (selectedNode) {
      setLabel(selectedNode.data.label)
      setAction(selectedNode.data.action)
      setConfigJson(JSON.stringify(selectedNode.data.config, null, 2))
    }
  }, [selectedNode])

  const handleSave = useCallback(() => {
    if (!selectedNodeId) return
    let config: Record<string, unknown> = {}
    try {
      config = JSON.parse(configJson)
    } catch {
      // Keep existing config if JSON is invalid
    }
    updateNodeData(selectedNodeId, { label, action, config })
  }, [selectedNodeId, label, action, configJson, updateNodeData])

  const handleDelete = useCallback(() => {
    if (!selectedNodeId) return
    deleteNode(selectedNodeId)
  }, [selectedNodeId, deleteNode])

  if (appMode === 'REVIEW') {
    return null
  }

  if (!selectedNode) {
    return (
      <div className="w-72 bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-700 flex items-center justify-center">
        <p className="text-sm text-gray-400 dark:text-gray-500">Select a node to configure</p>
      </div>
    )
  }

  const nodeType = selectedNode.data.nodeType
  // Super nodes don't have type metadata — show a minimal panel
  if (nodeType === 'super') {
    return (
      <div className="w-72 bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-700 flex flex-col overflow-y-auto">
        <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <span className="text-xs font-bold uppercase text-slate-600 dark:text-slate-300">SuperNode</span>
          <button onClick={() => setSelectedNodeId(null)} className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 text-lg">x</button>
        </div>
        <div className="p-4 space-y-4">
          <div>
            <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">Group Label</label>
            <input
              type="text"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              onBlur={handleSave}
              className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md
                         bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100
                         focus:outline-none focus:ring-2 focus:ring-blue-400 focus:border-transparent"
            />
          </div>
          <p className="text-[11px] text-gray-400 dark:text-gray-500">
            Child nodes: {(selectedNode.data.childNodeIds ?? []).length}
          </p>
        </div>
        <div className="px-4 py-3 border-t border-gray-200 dark:border-gray-700">
          <button
            onClick={handleDelete}
            className="w-full px-3 py-1.5 text-sm text-red-600 border border-red-300
                       rounded-md hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
          >
            Delete Node
          </button>
        </div>
      </div>
    )
  }

  const info = NODE_TYPE_INFO[nodeType]

  return (
    <div className="w-72 bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-700 flex flex-col overflow-y-auto">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <div>
          <span className={`text-xs font-bold uppercase ${info.color}`}>
            {info.label} Node
          </span>
          <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">{selectedNode.id}</p>
        </div>
        <button
          onClick={() => setSelectedNodeId(null)}
          className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 text-lg"
        >
          x
        </button>
      </div>

      {/* Form */}
      <div className="flex-1 p-4 space-y-4">
        {/* Name */}
        <div>
          <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">
            Node Name
          </label>
          <input
            type="text"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            onBlur={handleSave}
            className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md
                       bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100
                       focus:outline-none focus:ring-2 focus:ring-blue-400 focus:border-transparent"
          />
        </div>

        {/* Action - different labels based on node type */}
        <div>
          <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">
            {selectedNode.data.nodeType === 'script' ? 'Bash Command' :
             selectedNode.data.nodeType === 'llm' ? 'Prompt Template' :
             selectedNode.data.nodeType === 'mcp' ? 'MCP Tool Name' :
             selectedNode.data.nodeType === 'human' ? 'Review Instructions' :
             'Routing Expression'}
          </label>
          <textarea
            value={action}
            onChange={(e) => setAction(e.target.value)}
            onBlur={handleSave}
            rows={selectedNode.data.nodeType === 'llm' ? 6 : 3}
            placeholder={
              selectedNode.data.nodeType === 'script' ? 'echo "hello world"' :
              selectedNode.data.nodeType === 'llm' ? 'Analyze the following data: {{prev_node_id}}' :
              selectedNode.data.nodeType === 'mcp' ? 'query_elasticsearch' :
              selectedNode.data.nodeType === 'human' ? 'Review the analysis before proceeding' :
              '{{analyze}}.severity == "critical"'
            }
            className="w-full px-3 py-1.5 text-sm font-mono border border-gray-300 dark:border-gray-600 rounded-md
                       bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100
                       focus:outline-none focus:ring-2 focus:ring-blue-400 focus:border-transparent
                       resize-y"
          />
          {selectedNode.data.nodeType !== 'human' && (
            <p className="text-[10px] text-gray-400 dark:text-gray-500 mt-1">
              Use {'{{node_id}}'} to reference outputs from upstream nodes
            </p>
          )}
        </div>

        {/* Config JSON */}
        <div>
          <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">
            Config (JSON)
          </label>
          <textarea
            value={configJson}
            onChange={(e) => setConfigJson(e.target.value)}
            onBlur={handleSave}
            rows={4}
            placeholder='{"timeout_seconds": 30}'
            className="w-full px-3 py-1.5 text-sm font-mono border border-gray-300 dark:border-gray-600 rounded-md
                       bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100
                       focus:outline-none focus:ring-2 focus:ring-blue-400 focus:border-transparent
                       resize-y"
          />
        </div>
      </div>

      {/* Footer actions */}
      <div className="px-4 py-3 border-t border-gray-200 dark:border-gray-700">
        <button
          onClick={handleDelete}
          className="w-full px-3 py-1.5 text-sm text-red-600 border border-red-300
                     rounded-md hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
        >
          Delete Node
        </button>
      </div>
    </div>
  )
}
