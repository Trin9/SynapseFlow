package api

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"

	appDAG "github.com/Trin9/SynapseFlow/backend/internal/application/dag"
	appAudit "github.com/Trin9/SynapseFlow/backend/internal/application/audit"
	appExecution "github.com/Trin9/SynapseFlow/backend/internal/application/execution"
	appOps "github.com/Trin9/SynapseFlow/backend/internal/application/ops"
	appSystem "github.com/Trin9/SynapseFlow/backend/internal/application/system"
	appWorkspace "github.com/Trin9/SynapseFlow/backend/internal/application/workspace"
	"github.com/Trin9/SynapseFlow/backend/internal/auth"
	"github.com/Trin9/SynapseFlow/backend/internal/config"
	"github.com/Trin9/SynapseFlow/backend/internal/engine"
	"github.com/Trin9/SynapseFlow/backend/internal/mcp"
	"github.com/Trin9/SynapseFlow/backend/internal/memory"
	"github.com/Trin9/SynapseFlow/backend/internal/metrics"
	"github.com/Trin9/SynapseFlow/backend/internal/notify"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/logger"
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
	auditSvc      *appAudit.Service
	execService   *appExecution.Service
	opsService    *appOps.Service
	systemSvc     *appSystem.Service
	workspaceSvc  *appWorkspace.Service
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
// @Failure 500 {object} dto.APIError
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
// @Failure 500 {object} dto.APIError
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
