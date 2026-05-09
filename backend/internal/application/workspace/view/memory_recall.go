package view

// MemoryRecallView is a single memory recall item.
// confidence reflects keyword-overlap degree in current implementation.
type MemoryRecallView struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Summary           string `json:"summary"`
	MatchedPattern    string `json:"matched_pattern,omitempty"`
	Confidence        string `json:"confidence,omitempty"` // "high" | "medium" | "low"
	SourceExecutionID string `json:"source_execution_id,omitempty"`
	Caution           string `json:"caution,omitempty"`
	Recommendation    string `json:"recommendation,omitempty"`
}

// MemoryRecallListView wraps the memory recall list response.
// implementation_note is fixed to "keyword_overlap" until vector recall is introduced.
type MemoryRecallListView struct {
	Items              []MemoryRecallView `json:"items"`
	ImplementationNote string             `json:"implementation_note"` // "keyword_overlap"
}
