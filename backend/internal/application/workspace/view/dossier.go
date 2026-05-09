package view

import "github.com/Trin9/SynapseFlow/backend/pkg/models"

// ExpectedBehaviorView is a single entry in the Dossier left column.
// source_type: "sop" (Verified SOP) | "ai" (AI Hypothesized)
type ExpectedBehaviorView struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	FocusKey     string `json:"focus_key,omitempty"`
	SourceType   string `json:"source_type,omitempty"`   // "sop" | "ai"
	SourceLabel  string `json:"source_label,omitempty"`  // "Verified SOP" | "AI Hypothesized"
	SourceDetail string `json:"source_detail,omitempty"` // explanation of source
}

// VerdictBridgeItemView is a single entry in the Dossier middle column.
type VerdictBridgeItemView struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	FocusKey string `json:"focus_key,omitempty"`
}

// DossierEpisodeRefView is the lightweight episode reference in a dossier.
type DossierEpisodeRefView struct {
	EpisodeID string `json:"episode_id"`
	Label     string `json:"label"`
}

// DossierDisplayView holds the display-layer state of a dossier.
type DossierDisplayView struct {
	Verdict      string  `json:"verdict,omitempty"`
	VerdictLabel string  `json:"verdict_label,omitempty"`
	Summary      string  `json:"summary,omitempty"`
	Banner       *string `json:"banner"` // null when no banner
}

// EpisodeDossierView is the full dossier payload returned by
// GET /api/v1/executions/:execution_id/episodes/:episode_id/dossier.
type EpisodeDossierView struct {
	Episode          DossierEpisodeRefView      `json:"episode"`
	Display          DossierDisplayView         `json:"display"`
	ExpectedBehavior []ExpectedBehaviorView     `json:"expected_behavior"`
	VerdictBridge    []VerdictBridgeItemView    `json:"verdict_bridge"`
	RuntimeFacts     []models.RuntimeFactView   `json:"runtime_facts"`
	Handles          []models.EpisodeHandle     `json:"handles"`
	MemoryRecalls    []MemoryRecallView         `json:"memory_recalls"`
	HumanAuditTrail  []models.HumanIntervention `json:"human_audit_trail"`
}
