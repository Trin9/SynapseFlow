package workspace

// ComparisonSummaryView is returned by
// GET /api/v1/executions/:execution_id/comparison-targets/:historical_execution_id.
type ComparisonSummaryView struct {
	ExecutionID     string   `json:"execution_id"`
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	Outcome         string   `json:"outcome,omitempty"`
	ComparedAgainst string   `json:"compared_against,omitempty"`
	Highlights      []string `json:"highlights,omitempty"`
	Caution         string   `json:"caution,omitempty"`
}
