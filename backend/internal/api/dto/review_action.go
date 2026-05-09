package dto

// ReviewActionRequest is the API request body for
// POST /api/v1/executions/:id/review-actions.
type ReviewActionRequest struct {
	EpisodeID string `json:"episode_id,omitempty"` // Optional target episode ID within execution.
	Status    string `json:"status"`               // "approved" | "aborted" | "overridden".
	Actor     string `json:"actor,omitempty"`      // Human actor identity.
	Note      string `json:"note,omitempty"`       // Optional review note.
}
