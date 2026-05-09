package view

// RuntimeFactView is a single evidence fact in the Dossier right column.
// focus_key is the key field for three-column linkage (M4.1).
type RuntimeFactView struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	FocusKey       string `json:"focus_key,omitempty"`
	SourceType     string `json:"source_type,omitempty"` // "json" | "log" | "code" | "text"
	Collector      string `json:"collector,omitempty"`   // "node_type:node_name"
	Handle         string `json:"handle,omitempty"`      // "state:key"
	Revision       string `json:"revision,omitempty"`
	TimeWindow     string `json:"time_window,omitempty"`
	SourceName     string `json:"source_name,omitempty"`
	Content        string `json:"content,omitempty"`
	ContentRef     string `json:"content_ref,omitempty"`
	HighlightLines []int  `json:"highlight_lines,omitempty"`
}
