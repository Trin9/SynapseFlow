package view

// TriggerContextFieldView is a single key-value field in a trigger context section.
type TriggerContextFieldView struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Range [2]int `json:"range,omitempty"` // optional [start_percent, end_percent]
}

// TriggerContextSectionView groups related trigger context fields.
type TriggerContextSectionView struct {
	Title  string                    `json:"title"`
	Fields []TriggerContextFieldView `json:"fields"`
}

// TriggerContextView is the full trigger context payload.
type TriggerContextView struct {
	Title    string                      `json:"title"`
	Summary  string                      `json:"summary"`
	Sections []TriggerContextSectionView `json:"sections"`
}
