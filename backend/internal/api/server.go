package api

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"time"

	appDAG "github.com/Trin9/SynapseFlow/backend/internal/application/dag"
	appExecution "github.com/Trin9/SynapseFlow/backend/internal/application/execution"
	appOps "github.com/Trin9/SynapseFlow/backend/internal/application/ops"
	appSystem "github.com/Trin9/SynapseFlow/backend/internal/application/system"
	appWorkspace "github.com/Trin9/SynapseFlow/backend/internal/application/workspace"
	workspaceView "github.com/Trin9/SynapseFlow/backend/internal/application/workspace/view"
	"github.com/Trin9/SynapseFlow/backend/internal/auth"
	"github.com/Trin9/SynapseFlow/backend/internal/config"
	"github.com/Trin9/SynapseFlow/backend/internal/engine"
	"github.com/Trin9/SynapseFlow/backend/internal/mcp"
	"github.com/Trin9/SynapseFlow/backend/internal/memory"
	"github.com/Trin9/SynapseFlow/backend/internal/metrics"
	"github.com/Trin9/SynapseFlow/backend/internal/notify"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/logger"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

// Server holds the HTTP server state and storage backends.
type Server struct {
	router    *gin.Engine
	httpSrv   *http.Server
	scheduler *engine.Scheduler
	mcpMgr    mcp.ToolCaller
	memory    memory.ExperienceStore
	extractor *memory.Extractor
	db        *sql.DB
	config    config.Config

	// Sprint 4: observability, security, notifications
	metricsCollector *metrics.Collector
	notifier         notify.Sender
	slackWebhookURL  string // populated from DAG config or env

	// apiKeys maps raw key string → Identity (populated from env SYNAPSE_API_KEYS)
	// Format: comma-separated "key:role:subject" triples.
	// If empty, server runs in open/dev mode (any request gets admin identity).
	apiKeys map[string]*auth.Identity

	// jwtSecret is the HMAC-SHA256 signing key for JWT tokens (Sprint 9).
	// Set via SYNAPSE_JWT_SECRET env var. When non-empty, Bearer tokens are
	// verified as signed JWTs; when empty, the legacy "role:subject" plaintext
	// mode is used (dev/open environments only).
	jwtSecret []byte

	dags     store.DAGStore
	execs    store.ExecutionStore
	audits   store.AuditStore
	episodes store.EpisodeStore

	// episodeWriter is the shared write-permission layer for Episode objects.
	// Injected into executors at startup; also used by API handlers (e.g. resume).
	episodeWriter *engine.EpisodeWriter
	dagService    *appDAG.Service
	execService   *appExecution.Service
	opsService    *appOps.Service
	systemSvc     *appSystem.Service
	workspaceSvc  *appWorkspace.Service
}

// apiError is the unified error response format.
// details is optional and should be JSON-serializable.
type apiError struct {
	Error   string      `json:"error"`
	Code    string      `json:"code"`
	Details interface{} `json:"details,omitempty"`
}

// healthDepsResponse describes dependency health checks.
type healthDepsResponse struct {
	MCP string `json:"mcp"` // MCP manager health status.
	DB  string `json:"db"`  // Database health status.
}

// healthResponse is the response payload for health check APIs.
type healthResponse struct {
	Status  string             `json:"status"`  // Overall service status.
	Service string             `json:"service"` // Service name.
	Version string             `json:"version"` // Service version.
	Deps    healthDepsResponse `json:"deps"`    // Dependency health statuses.
}

// liveResponse is the response payload for liveness checks.
type liveResponse struct {
	Status string `json:"status"` // Process liveness status.
}

// issueTokenRequest is the request payload for JWT token issuance.
type issueTokenRequest struct {
	APIKey string `json:"api_key" binding:"required"` // API key used to exchange for JWT; required.
}

// issueTokenResponse is the response payload for JWT token issuance.
type issueTokenResponse struct {
	Token     string `json:"token"`      // Signed JWT token.
	ExpiresIn int    `json:"expires_in"` // Token TTL in seconds.
	Role      string `json:"role"`       // Role associated with the API key identity.
	Subject   string `json:"subject"`    // Subject associated with the API key identity.
}

// deleteDAGResponse is the response payload after DAG deletion.
type deleteDAGResponse struct {
	Message string `json:"message"` // Deletion result message.
}

// runExecutionResponse is the response payload for starting or resuming executions.
type runExecutionResponse struct {
	ExecutionID string                 `json:"execution_id"` // Execution identifier.
	Status      models.ExecutionStatus `json:"status"`       // Current execution status.
}

// executionNodesResponse is the polling payload for node-level execution status.
type executionNodesResponse struct {
	ExecutionID string                 `json:"execution_id"` // Execution identifier.
	Status      models.ExecutionStatus `json:"status"`       // Current execution status.
	Results     []models.NodeResult    `json:"results"`      // Node execution results.
	Error       string                 `json:"error"`        // Execution error message when failed.
	StartedAt   time.Time              `json:"started_at"`   // Execution start time.
	EndedAt     *time.Time             `json:"ended_at"`     // Execution end time when terminal.
	DurationMS  int64                  `json:"duration_ms"`  // Execution duration in milliseconds.
}

// resumeExecutionRequest is the optional request payload for execution resume.
type resumeExecutionRequest struct {
	Actor  string `json:"actor"`  // Human actor who resumes execution; defaults to "operator".
	Action string `json:"action"` // Human intervention action; defaults to "resumed".
	Detail string `json:"detail"` // Optional detail for the resume operation.
}

// episodeSummariesResponse is the list payload for episode summaries.
type episodeSummariesResponse struct {
	Episodes []workspaceView.EpisodeSummaryView `json:"episodes"` // Episode summary items.
}

// episodesResponse is the list payload for full episode objects.
type episodesResponse struct {
	Episodes []*models.Episode `json:"episodes"` // Episode items.
}

// reviewActionResponse is the response payload for review action writes.
type reviewActionResponse struct {
	OK bool `json:"ok"` // Whether the review action was recorded successfully.
}

type ServerOption func(*Server)

func WithMCPManager(mgr mcp.ToolCaller) ServerOption {
	return func(s *Server) {
		s.mcpMgr = mgr
	}
}

// NewServer and setupRouter moved to
// server_bootstrap.go and server_router.go.

// Run starts the HTTP server on the given address.
func (s *Server) Run(addr string) error {
	logger.L().Infow("Starting Synapse server", "addr", addr)
	s.httpSrv = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.httpSrv.Serve(ln)
}

// Close gracefully shuts down the HTTP server and releases MCP resources.
func (s *Server) Close(ctx context.Context) error {
	var err error
	if s.httpSrv != nil {
		err = s.httpSrv.Shutdown(ctx)
	}
	if s.db != nil {
		if cerr := s.db.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	// If the configured MCP manager supports Close, call it.
	if c, ok := s.mcpMgr.(interface{ Close(context.Context) error }); ok {
		if cerr := c.Close(ctx); cerr != nil {
			if err != nil {
				return fmt.Errorf("shutdown http: %v; close mcp: %w", err, cerr)
			}
			return cerr
		}
	}
	return err
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// System/auth and DAG handlers moved to
// server_system_auth.go and server_dag.go.

// List Experiences returns stored memory experiences.
// @Summary List Experiences
// @Description Returns stored memory experiences.
// @Tags Memory
// @Produce json
// @Success 200 {array} object "Experience list"
// @Failure 500 {object} apiError
// @Router /api/v1/experiences [get]
func (s *Server) handleListExperiences(c *gin.Context) {
	experiences, err := s.opsService.ListExperiences(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "memory_error", "failed to list experiences", err.Error())
		return
	}
	c.JSON(http.StatusOK, experiences)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// handleListAudit returns the in-memory audit trail (admin only).
// @Summary List Audit Entries
// @Description Returns the audit trail entries.
// @Tags Audit
// @Produce json
// @Success 200 {array} object "Audit entry list"
// @Failure 500 {object} apiError
// @Router /api/v1/audit [get]
func (s *Server) handleListAudit(c *gin.Context) {
	entries, err := s.opsService.ListAuditEntries(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "audit_list_failed", "failed to list audit entries", err.Error())
		return
	}
	c.JSON(http.StatusOK, entries)
}

// parseAPIKeys parses the SYNAPSE_API_KEYS environment variable.
// Format: comma-separated "key:role:subject" triples.
// Example: "secret123:admin:ci-bot,readkey:viewer:monitoring"
// Common helper functions moved to server_helpers.go.
