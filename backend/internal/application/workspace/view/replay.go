package view

import "github.com/Trin9/SynapseFlow/backend/pkg/models"

// ReplayCheckpointView describes the narrative at a given replay position.
type ReplayCheckpointView struct {
	Label    string `json:"label"`
	Headline string `json:"headline"`
	Detail   string `json:"detail"`
}

// ReplaySliceView is returned by
// GET /api/v1/executions/:execution_id/episodes/:episode_id/replay?percent=N.
type ReplaySliceView struct {
	EpisodeID             string                         `json:"episode_id"`
	Percent               int                            `json:"percent"`
	Checkpoint            ReplayCheckpointView           `json:"checkpoint"`
	VisibleProcessTrace   []models.ProcessTraceEntryView `json:"visible_process_trace"`
	VisibleHandles        []any                          `json:"visible_handles"`
	VisibleStateFields    []any                          `json:"visible_state_fields"`
	VisibleRuntimeFactIDs []string                       `json:"visible_runtime_fact_ids"`
}
