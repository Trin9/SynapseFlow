package dto

import (
	"time"

	workspaceView "github.com/Trin9/SynapseFlow/backend/internal/application/workspace/view"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// APIError is the unified error response format.
type APIError struct {
	Error   string      `json:"error"`
	Code    string      `json:"code"`
	Details interface{} `json:"details,omitempty"`
}

// HealthDepsResponse describes dependency health checks.
type HealthDepsResponse struct {
	MCP string `json:"mcp"`
	DB  string `json:"db"`
}

// HealthResponse is the response payload for health check APIs.
type HealthResponse struct {
	Status  string             `json:"status"`
	Service string             `json:"service"`
	Version string             `json:"version"`
	Deps    HealthDepsResponse `json:"deps"`
}

// LiveResponse is the response payload for liveness checks.
type LiveResponse struct {
	Status string `json:"status"`
}

// IssueTokenRequest is the request payload for JWT token issuance.
type IssueTokenRequest struct {
	APIKey string `json:"api_key" binding:"required"`
}

// IssueTokenResponse is the response payload for JWT token issuance.
type IssueTokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	Role      string `json:"role"`
	Subject   string `json:"subject"`
}

// DeleteDAGResponse is the response payload after DAG deletion.
type DeleteDAGResponse struct {
	Message string `json:"message"`
}

// RunExecutionResponse is the response payload for starting or resuming executions.
type RunExecutionResponse struct {
	ExecutionID string                 `json:"execution_id"`
	Status      models.ExecutionStatus `json:"status"`
}

// ExecutionNodesResponse is the polling payload for node-level execution status.
type ExecutionNodesResponse struct {
	ExecutionID string                 `json:"execution_id"`
	Status      models.ExecutionStatus `json:"status"`
	Results     []models.NodeResult    `json:"results"`
	Error       string                 `json:"error"`
	StartedAt   time.Time              `json:"started_at"`
	EndedAt     *time.Time             `json:"ended_at"`
	DurationMS  int64                  `json:"duration_ms"`
}

// ResumeExecutionRequest is the optional request payload for execution resume.
type ResumeExecutionRequest struct {
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Detail string `json:"detail"`
}

// EpisodeSummariesResponse is the list payload for episode summaries.
type EpisodeSummariesResponse struct {
	Episodes []workspaceView.EpisodeSummaryView `json:"episodes"`
}

// EpisodesResponse is the list payload for full episode objects.
type EpisodesResponse struct {
	Episodes []*models.Episode `json:"episodes"`
}

// ReviewActionResponse is the response payload for review action writes.
type ReviewActionResponse struct {
	OK bool `json:"ok"`
}
