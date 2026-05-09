package view

import "github.com/Trin9/SynapseFlow/backend/pkg/models"

// EpisodeDisplayView holds display-layer metadata for an episode card.
type EpisodeDisplayView struct {
	Verdict      string  `json:"verdict,omitempty"`       // display verdict (may reflect human review)
	VerdictLabel string  `json:"verdict_label,omitempty"` // display label (human-adjusted)
	Summary      string  `json:"summary,omitempty"`
	Banner       *string `json:"banner"` // nil when not present
}

// EpisodeSummaryView is a compact episode card used by
// GET /api/v1/executions/:execution_id/episodes?view=summary.
type EpisodeSummaryView struct {
	EpisodeID            string                   `json:"episode_id"`
	Label                string                   `json:"label"`
	Status               models.EpisodeStatus     `json:"status"`
	Display              EpisodeDisplayView       `json:"display"`
	Confidence           models.EpisodeConfidence `json:"confidence,omitempty"`
	EvidenceCount        int                      `json:"evidence_count"`
	HandleCount          int                      `json:"handle_count"`
	DefaultReplayPercent int                      `json:"default_replay_percent"`
}
