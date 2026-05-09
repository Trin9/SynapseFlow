package view

// ProcessTraceEntryView is a single step in the process trace timeline.
// stage values: "Action" | "Round N" | "Verdict" | "Human Review" | "Circuit Breaker"
type ProcessTraceEntryView struct {
	ID     string   `json:"id"`
	Stage  string   `json:"stage"`
	Title  string   `json:"title"`
	Detail string   `json:"detail,omitempty"`
	Status string   `json:"status"` // "success" | "failed" | "running" | "pending"
	Chips  []string `json:"chips,omitempty"`
	Range  [2]int   `json:"range"` // [start_percent, end_percent]
}
