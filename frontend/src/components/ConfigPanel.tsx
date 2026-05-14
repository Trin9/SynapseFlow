import { useEffect, useState, useCallback } from 'react'
import { useGraphStore } from '@/hooks/useGraphStore'
import { NODE_TYPE_INFO } from '@/types'

/**
 * Right-side configuration panel that appears when a node is selected.
 * Shows dynamic form fields based on the node type.
 */
export function ConfigPanel() {
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
  const [expectedBehaviorText, setExpectedBehaviorText] = useState('')

  // Sync from store to local state when selection changes
  useEffect(() => {
    if (selectedNode) {
      setLabel(selectedNode.data.label)
      setAction(selectedNode.data.action)
      setConfigJson(JSON.stringify(selectedNode.data.config, null, 2))
      const nextConfig = selectedNode.data.config as Record<string, unknown>
      const expectedBehaviors = Array.isArray(nextConfig.expected_behaviors)
        ? nextConfig.expected_behaviors.filter((item): item is string => typeof item === 'string')
        : []
      setExpectedBehaviorText(expectedBehaviors.join('\n'))
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
    const config = (selectedNode.data.config ?? {}) as Record<string, unknown>
    const candidateNodes = nodes.filter((node) => node.data.nodeType !== 'super')
    const assignedChildIds = new Set(selectedNode.data.childNodeIds ?? [])

    const handleEpisodeSave = () => {
      if (!selectedNodeId) return
      const expectedBehaviors = expectedBehaviorText
        .split('\n')
        .map((item) => item.trim())
        .filter((item) => item.length > 0)

      updateNodeData(selectedNodeId, {
        label,
        action,
        config: {
          ...config,
          expected_behaviors: expectedBehaviors,
        },
      })
    }

    const toggleChildAssignment = (childId: string) => {
      if (!selectedNodeId) return
      const nextChildIds = assignedChildIds.has(childId)
        ? [...assignedChildIds].filter((id) => id !== childId)
        : [...assignedChildIds, childId]

      updateNodeData(selectedNodeId, { childNodeIds: nextChildIds })
    }

    return (
      <div className="w-72 bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-700 flex flex-col overflow-y-auto">
        <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <span className="text-xs font-bold uppercase text-slate-600 dark:text-slate-300">Episode Draft</span>
          <button onClick={() => setSelectedNodeId(null)} className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 text-lg">x</button>
        </div>
        <div className="p-4 space-y-4">
          <div>
            <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">Episode Label</label>
            <input
              type="text"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              onBlur={handleEpisodeSave}
              className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md
                         bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100
                         focus:outline-none focus:ring-2 focus:ring-blue-400 focus:border-transparent"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">Episode Summary</label>
            <textarea
              value={action}
              onChange={(e) => setAction(e.target.value)}
              onBlur={handleEpisodeSave}
              rows={3}
              placeholder="Describe what this episode is supposed to establish before runtime signals arrive."
              className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md
                         bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100
                         focus:outline-none focus:ring-2 focus:ring-blue-400 focus:border-transparent
                         resize-y"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">Expected Behavior</label>
            <textarea
              value={expectedBehaviorText}
              onChange={(e) => setExpectedBehaviorText(e.target.value)}
              onBlur={handleEpisodeSave}
              rows={6}
              placeholder={'One expected behavior per line\nStorefront health check succeeds\nProduct discovery returns at least one product id'}
              className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md
                         bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100
                         focus:outline-none focus:ring-2 focus:ring-blue-400 focus:border-transparent
                         resize-y"
            />
            <p className="text-[10px] text-gray-400 dark:text-gray-500 mt-1">
              This is design-time intent only. Confidence, evidence and handles are filled in during execution.
            </p>
          </div>
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-xs font-medium text-gray-600 dark:text-gray-300">Internal Nodes</label>
              <span className="text-[10px] text-gray-400 dark:text-gray-500">
                {(selectedNode.data.childNodeIds ?? []).length} assigned
              </span>
            </div>
            <div className="max-h-56 overflow-y-auto rounded-md border border-gray-200 dark:border-gray-700 divide-y divide-gray-100 dark:divide-gray-800">
              {candidateNodes.length === 0 && (
                <p className="px-3 py-2 text-xs text-gray-400 dark:text-gray-500">Add workflow nodes first, then assign them to this episode.</p>
              )}
              {candidateNodes.map((node) => {
                const checked = assignedChildIds.has(node.id)
                return (
                  <label key={node.id} className="flex items-start gap-2 px-3 py-2 text-xs cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800/60">
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggleChildAssignment(node.id)}
                      className="mt-0.5"
                    />
                    <span className="min-w-0 flex-1">
                      <span className="block font-medium text-gray-700 dark:text-gray-200 truncate">{node.data.label}</span>
                      <span className="block text-[10px] text-gray-400 dark:text-gray-500 uppercase">{node.data.nodeType}</span>
                    </span>
                  </label>
                )
              })}
            </div>
          </div>
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
