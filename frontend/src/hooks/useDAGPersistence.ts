import { useState, useCallback } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createDAG, getDAG, listDAGs, updateDAG } from '@/api/client'
import { useGraphStore } from '@/hooks/useGraphStore'

/**
 * Encapsulates DAG save / load / list logic.
 * Replaces the manual refreshDAGs + handleSave + handleLoad logic in the old Toolbar.
 */
export function useDAGPersistence() {
  const workflowId = useGraphStore((s) => s.workflowId)
  const setWorkflowId = useGraphStore((s) => s.setWorkflowId)
  const toDAGConfig = useGraphStore((s) => s.toDAGConfig)
  const loadFromDAGConfig = useGraphStore((s) => s.loadFromDAGConfig)

  const queryClient = useQueryClient()

  const [selectedLoadId, setSelectedLoadId] = useState<string>('')
  const [saveLoadError, setSaveLoadError] = useState<string | null>(null)

  // Automatically fetches and keeps the DAG list fresh
  const { data: dags = [] } = useQuery({
    queryKey: ['dags'],
    queryFn: listDAGs,
  })

  const handleSave = useCallback(async (): Promise<string | null> => {
    const dag = toDAGConfig()
    if (dag.nodes.length === 0) {
      setSaveLoadError('Nothing to save (add nodes first)')
      setTimeout(() => setSaveLoadError(null), 3000)
      return 'Nothing to save (add nodes first)'
    }

    try {
      if (workflowId) {
        const updated = await updateDAG(workflowId, dag)
        loadFromDAGConfig(updated)
      } else {
        const created = await createDAG(dag)
        loadFromDAGConfig(created)
        setWorkflowId(created.id ?? null)
      }
      // Re-fetch the DAG list so the Load dropdown stays in sync
      await queryClient.invalidateQueries({ queryKey: ['dags'] })
      return null
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Save failed'
      setSaveLoadError(msg)
      setTimeout(() => setSaveLoadError(null), 5000)
      return msg
    }
  }, [toDAGConfig, workflowId, loadFromDAGConfig, setWorkflowId, queryClient])

  const handleLoad = useCallback(async (): Promise<string | null> => {
    if (!selectedLoadId) return null
    try {
      const dag = await getDAG(selectedLoadId)
      loadFromDAGConfig(dag)
      return null
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Load failed'
      setSaveLoadError(msg)
      setTimeout(() => setSaveLoadError(null), 5000)
      return msg
    }
  }, [selectedLoadId, loadFromDAGConfig])

  return {
    dags,
    selectedLoadId,
    setSelectedLoadId,
    handleSave,
    handleLoad,
    saveLoadError,
  }
}
