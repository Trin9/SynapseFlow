export interface SceneManifestItem {
  title?: string
  description: string
  curatedEpisodeOrder?: string[]
  childPreview?: Array<{ id: string; label: string }>
  defaultComparisonTarget?: string
  recommendedReplayPercent?: number
}

const sceneManifestByDagId: Record<string, SceneManifestItem> = {
  boutique_checkout_consistency_audit: {
    title: 'Boutique Checkout Consistency Audit',
    description:
      'Verification-oriented workflow for storefront health, cart continuity, and checkout closure consistency.',
    curatedEpisodeOrder: ['bootstrap', 'cart-validation', 'closure-verdict'],
    childPreview: [
      { id: 'bootstrap', label: 'Bootstrap readiness checks' },
      { id: 'cart-validation', label: 'Cart state verification' },
      { id: 'closure-verdict', label: 'Business closure verdict' },
    ],
    recommendedReplayPercent: 72,
  },
  checkout_payment_unreachable_agent_loop: {
    title: 'Checkout Payment Unreachable Loop',
    description:
      'Investigation-oriented workflow for reproducing payment unreachable loops and tracing escalation decisions.',
    curatedEpisodeOrder: ['trigger-analysis', 'retry-loop', 'human-handoff'],
    childPreview: [
      { id: 'trigger-analysis', label: 'Trigger context reconstruction' },
      { id: 'retry-loop', label: 'Bounded retry loop trace' },
      { id: 'human-handoff', label: 'Escalation and human review handoff' },
    ],
    recommendedReplayPercent: 84,
  },
}

export function getSceneManifest(dagId?: string, dagName?: string): SceneManifestItem | null {
  if (dagId && sceneManifestByDagId[dagId]) return sceneManifestByDagId[dagId]

  if (!dagName) return null
  const normalizedName = dagName.toLowerCase().replace(/\s+/g, '_')
  if (sceneManifestByDagId[normalizedName]) return sceneManifestByDagId[normalizedName]

  return null
}
