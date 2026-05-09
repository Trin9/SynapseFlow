package view

import (
	"time"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// ExecutionDisplayView holds display-layer metadata for an execution.
type ExecutionDisplayView struct {
	RunLabel      string `json:"run_label,omitempty"`
	OverviewBadge string `json:"overview_badge,omitempty"`
	TraceTitle    string `json:"trace_title,omitempty"`
	TraceSummary  string `json:"trace_summary,omitempty"`
}

// ExecutionSummaryView is the high-level execution summary returned by
// GET /api/v1/executions/:execution_id/summary.
type ExecutionSummaryView struct {
	ExecutionID  string                 `json:"execution_id"`
	DAGID        string                 `json:"dag_id"`
	DAGName      string                 `json:"dag_name"`
	Status       models.ExecutionStatus `json:"status"`
	StartedAt    time.Time              `json:"started_at"`
	EndedAt      *time.Time             `json:"ended_at,omitempty"`
	DurationMs   int64                  `json:"duration_ms"`
	Mode         string                 `json:"mode"`          // "execution"
	WorkflowKind string                 `json:"workflow_kind"` // "investigation" | "verification"
	Metadata     map[string]string      `json:"metadata,omitempty"`
	Display      ExecutionDisplayView   `json:"display"`
}
