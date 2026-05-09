package view

import "time"

// ReviewStateView is returned by
// GET /api/v1/executions/:execution_id/review-state.
type ReviewStateView struct {
	Status       string     `json:"status"` // "pending" | "approved" | "overridden" | "aborted"
	Actor        string     `json:"actor,omitempty"`
	ActedAt      *time.Time `json:"acted_at,omitempty"`
	ActionLabel  string     `json:"action_label,omitempty"`
	Note         string     `json:"note,omitempty"`
	TicketID     string     `json:"ticket_id,omitempty"`
	Queue        string     `json:"queue,omitempty"`
	ResumeCursor string     `json:"resume_cursor,omitempty"`
}
