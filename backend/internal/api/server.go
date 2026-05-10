package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	appDAG "github.com/Trin9/SynapseFlow/backend/internal/application/dag"
	appExecution "github.com/Trin9/SynapseFlow/backend/internal/application/execution"
	appOps "github.com/Trin9/SynapseFlow/backend/internal/application/ops"
	appWorkspace "github.com/Trin9/SynapseFlow/backend/internal/application/workspace"
	workspaceView "github.com/Trin9/SynapseFlow/backend/internal/application/workspace/view"
	"github.com/Trin9/SynapseFlow/backend/internal/auth"
	"github.com/Trin9/SynapseFlow/backend/internal/config"
	"github.com/Trin9/SynapseFlow/backend/internal/engine"
	"github.com/Trin9/SynapseFlow/backend/internal/llm"
	"github.com/Trin9/SynapseFlow/backend/internal/mcp"
	"github.com/Trin9/SynapseFlow/backend/internal/memory"
	"github.com/Trin9/SynapseFlow/backend/internal/metrics"
	"github.com/Trin9/SynapseFlow/backend/internal/notify"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/logger"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// NewServer creates and configures the HTTP server with all routes.
func NewServer(opts ...ServerOption) *Server {
	log := logger.L()
	cfg := config.Load()

	s := &Server{
		config:           cfg,
		metricsCollector: metrics.NewCollector(),
		apiKeys:          parseAPIKeys(os.Getenv("SYNAPSE_API_KEYS")),
		jwtSecret:        []byte(os.Getenv("SYNAPSE_JWT_SECRET")),
	}

	if cfg.EnableDBStorage {
		db, err := store.OpenPostgres(context.Background(), cfg.DatabaseURL, cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxIdle, cfg.DBConnMaxLife)
		if err != nil {
			log.Warnw("database unavailable, falling back to in-memory stores", "error", err)
		} else {
			if err := store.RunMigrations(context.Background(), db, cfg.MigrationsPath); err != nil {
				log.Warnw("database migrations failed, falling back to in-memory stores", "error", err)
				_ = db.Close()
			} else {
				stores := store.NewPostgresStores(db)
				s.db = db
				s.dags = stores.DAGs
				s.execs = stores.Executions
				s.audits = stores.Audits
				s.episodes = stores.Episodes
				s.memory = stores.MemoryStore()
			}
		}
	}

	if s.dags == nil {
		s.dags = store.NewMemoryDAGStore()
	}
	if s.execs == nil {
		s.execs = store.NewMemoryExecutionStore()
	}
	if s.audits == nil {
		s.audits = store.NewMemoryAuditStore()
	}
	if s.episodes == nil {
		s.episodes = store.NewMemoryEpisodeStore()
	}
	if s.memory == nil {
		s.memory = memory.NewInMemoryStore()
	}

	// Notification: Slack webhook URL from env (may be overridden per-DAG)
	if slackURL := os.Getenv("SYNAPSE_SLACK_WEBHOOK_URL"); slackURL != "" {
		s.notifier = &notify.SlackSender{}
		s.slackWebhookURL = slackURL
	}

	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.mcpMgr == nil {
		s.mcpMgr = mcp.NewManager(mcp.DefaultServersConfigPath())
	}

	// Configure executors based on environment
	episodeWriter := engine.NewEpisodeWriter(s.episodes)
	s.episodeWriter = episodeWriter
	executors := map[models.NodeType]engine.NodeExecutor{
		models.NodeTypeScript: &engine.ScriptExecutor{Writer: episodeWriter},
		models.NodeTypeHuman:  &engine.HumanExecutor{Writer: episodeWriter},
		models.NodeTypeRouter: &engine.RouterExecutor{},
		models.NodeTypeMCP:    &engine.MCPExecutor{MCP: s.mcpMgr, Writer: episodeWriter},
		models.NodeTypeWebInteraction: &engine.WebNodeExecutor{
			Writer: episodeWriter,
		},
	}

	// Use real LLM executor if API key is set, otherwise mock
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey != "" {
		// Build LLM client chain: JSON enforcing → fallback across providers
		var clients []llm.LLMClient

		// Primary provider: check LLM_PROVIDER env (default "openai")
		provider := os.Getenv("LLM_PROVIDER")
		if provider == "" {
			provider = "openai"
		}

		switch provider {
		case "anthropic":
			apiURL := os.Getenv("LLM_API_URL")
			model := os.Getenv("LLM_MODEL")
			if model == "" {
				model = "claude-sonnet-4-20250514"
			}
			clients = append(clients, llm.NewAnthropicClient(llm.ProviderConfig{
				APIURL: apiURL,
				APIKey: apiKey,
				Model:  model,
			}))
			log.Infow("LLM primary: Anthropic", "model", model)
		default: // "openai" or any OpenAI-compatible
			apiURL := os.Getenv("LLM_API_URL")
			if apiURL == "" {
				apiURL = "https://api.openai.com/v1/chat/completions"
			}
			model := os.Getenv("LLM_MODEL")
			if model == "" {
				model = "gpt-4o-mini"
			}
			clients = append(clients, llm.NewOpenAIClient(llm.ProviderConfig{
				APIURL: apiURL,
				APIKey: apiKey,
				Model:  model,
			}))
			log.Infow("LLM primary: OpenAI-compatible", "model", model, "api_url", apiURL)
		}

		// Optional fallback provider
		fallbackKey := os.Getenv("LLM_FALLBACK_API_KEY")
		if fallbackKey != "" {
			fallbackProvider := os.Getenv("LLM_FALLBACK_PROVIDER")
			fallbackURL := os.Getenv("LLM_FALLBACK_API_URL")
			fallbackModel := os.Getenv("LLM_FALLBACK_MODEL")

			switch fallbackProvider {
			case "anthropic":
				clients = append(clients, llm.NewAnthropicClient(llm.ProviderConfig{
					APIURL: fallbackURL,
					APIKey: fallbackKey,
					Model:  fallbackModel,
				}))
				log.Infow("LLM fallback: Anthropic", "model", fallbackModel)
			default:
				if fallbackURL == "" {
					fallbackURL = "https://api.openai.com/v1/chat/completions"
				}
				clients = append(clients, llm.NewOpenAIClient(llm.ProviderConfig{
					APIURL: fallbackURL,
					APIKey: fallbackKey,
					Model:  fallbackModel,
				}))
				log.Infow("LLM fallback: OpenAI-compatible", "model", fallbackModel)
			}
		}

		// Wrap in fallback (if multiple) then JSON enforcement
		var client llm.LLMClient
		if len(clients) > 1 {
			client = llm.NewFallbackClient(clients...)
		} else {
			client = clients[0]
		}
		client = llm.NewJSONEnforcingClient(client)

		executors[models.NodeTypeLLM] = &engine.LLMExecutor{Client: client, Writer: episodeWriter}
	} else {
		executors[models.NodeTypeLLM] = &engine.MockLLMExecutor{Writer: episodeWriter}
		log.Infow("Using mock LLM executor (set LLM_API_KEY for real LLM)")
	}

	s.extractor = &memory.Extractor{Store: s.memory}
	retriever := &memory.Retriever{Store: s.memory}
	s.scheduler = engine.NewScheduler(executors, retriever)
	s.scheduler.SetEpisodeStore(s.episodes)
	s.execService = &appExecution.Service{
		Scheduler:        s.scheduler,
		DAGs:             s.dags,
		Executions:       s.execs,
		Episodes:         s.episodes,
		EpisodeWriter:    s.episodeWriter,
		MetricsCollector: s.metricsCollector,
		Notifier:         s.notifier,
		GetNotifier: func() notify.Sender {
			return s.notifier
		},
		Extractor:                  s.extractor,
		ResolveSlackURL:            s.resolveSlackURL,
		BuildExecutionNotification: buildExecutionNotification,
	}
	s.dagService = &appDAG.Service{DAGs: s.dags}
	s.opsService = &appOps.Service{Audits: s.audits, Memory: s.memory}
	s.workspaceSvc = &appWorkspace.Service{
		Executions:              s.execs,
		Episodes:                s.episodes,
		MemoryStore:             s.memory,
		EpisodeWriter:           s.episodeWriter,
		BuildTriggerContextView: buildTriggerContextView,
		BuildReplaySliceView:    buildReplaySliceView,
		BuildComparisonSummary:  buildComparisonSummary,
		BuildEpisodeDossier:     buildEpisodeDossier,
		BuildMemoryRecalls:      buildMemoryRecallsForEpisode,
		LogMemoryRecallWarning: func(episodeID string, err error) {
			logger.L().Warnw("memory recall search failed; dossier served with empty recalls",
				"episode_id", episodeID, "error", err)
		},
	}

	s.setupRouter()
	return s
}

// setupRouter configures all HTTP routes.
func (s *Server) setupRouter() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS middleware for frontend
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Health check and metrics are always public
	r.GET("/health", s.handleHealth)
	r.GET("/health/live", s.handleLive)
	r.GET("/metrics", s.handleMetrics)

	// Token issuance (public – authentication happens inside the handler)
	r.POST("/api/v1/auth/token", s.handleIssueToken)

	// Webhook endpoint: authenticated via API key (machine-to-machine)
	r.POST("/api/v1/webhook/alert", s.authMiddleware(), s.handleWebhookAlert)

	// API v1 – all routes require authentication
	v1 := r.Group("/api/v1", s.authMiddleware())
	{
		// Tool discovery (viewer+)
		v1.GET("/tools", s.handleListTools)

		// DAG management
		v1.POST("/dags", requireRole(auth.RoleAdmin), s.auditMiddleware("create_dag", "dag"), s.validateDAGMiddleware(), s.handleCreateDAG)
		v1.GET("/dags", s.handleListDAGs)
		v1.GET("/dags/:id", s.handleGetDAG)
		v1.PUT("/dags/:id", requireRole(auth.RoleAdmin), s.auditMiddleware("update_dag", "dag"), s.validateDAGMiddleware(), s.handleUpdateDAG)
		v1.DELETE("/dags/:id", requireRole(auth.RoleAdmin), s.auditMiddleware("delete_dag", "dag"), s.handleDeleteDAG)

		// Execution (operator+)
		v1.POST("/dags/:id/run", requireRole(auth.RoleOperator), s.auditMiddleware("run_dag", "execution"), s.handleRunDAG)
		v1.POST("/run", requireRole(auth.RoleOperator), s.auditMiddleware("run_inline", "execution"), s.validateDAGMiddleware(), s.handleRunInline)
		v1.POST("/executions/:id/resume", requireRole(auth.RoleOperator), s.auditMiddleware("resume", "execution"), s.handleResumeExecution)
		v1.GET("/executions/:id", s.handleGetExecution)
		v1.GET("/executions/:id/nodes", s.handleGetExecutionNodes)
		v1.GET("/executions", s.handleListExecutions)

		// Memory
		v1.GET("/experiences", s.handleListExperiences)

		// Episodes (Sprint 7)
		v1.GET("/executions/:id/episodes", s.handleListEpisodes)
		v1.GET("/episodes/:id", s.handleGetEpisode)

		// Execution Workspace (M1.4) — read-only workspace endpoints.
		v1.GET("/executions/:id/summary", s.handleGetExecutionSummary)
		v1.GET("/executions/:id/trigger-context", s.handleGetTriggerContext)
		v1.GET("/executions/:id/review-state", s.handleGetReviewState)
		v1.POST("/executions/:id/review-actions", requireRole(auth.RoleOperator), s.handlePostReviewAction)
		v1.GET("/executions/:id/episodes/:episode_id/replay", s.handleGetEpisodeReplay)
		v1.GET("/executions/:id/episodes/:episode_id/dossier", s.handleGetEpisodeDossier)
		v1.GET("/executions/:id/episodes/:episode_id/memory-recalls", s.handleGetEpisodeMemoryRecalls)
		v1.GET("/executions/:id/comparison-targets/:historical_id", s.handleGetComparisonTarget)

		// Audit log (admin only)
		v1.GET("/audit", requireRole(auth.RoleAdmin), s.handleListAudit)
	}

	s.router = r
}

// List Tools returns all available MCP tools.
// @Summary List Tools
// @Description Returns all available MCP tools.
// @Tags Tools
// @Produce json
// @Success 200 {array} object "MCP tool list"
// @Failure 500 {object} apiError
// @Router /api/v1/tools [get]
func (s *Server) handleListTools(c *gin.Context) {
	tools, err := s.mcpMgr.ListTools(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "tools_error", "failed to list tools", err.Error())
		return
	}
	c.JSON(http.StatusOK, tools)
}

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

// Health Check returns service and dependency health status.
// @Summary Health Check
// @Description Returns service and dependency health status.
// @Tags System
// @Produce json
// @Success 200 {object} healthResponse
// @Router /health [get]
func (s *Server) handleHealth(c *gin.Context) {
	// Enhanced: check MCP connectivity
	mcpStatus := "ok"
	if _, err := s.mcpMgr.ListTools(c.Request.Context()); err != nil {
		mcpStatus = "degraded: " + err.Error()
	}
	dbStatus := "disabled"
	if s.db != nil {
		if err := s.db.PingContext(c.Request.Context()); err != nil {
			dbStatus = "degraded: " + err.Error()
		} else {
			dbStatus = "ok"
		}
	}
	c.JSON(http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "synapse",
		Version: "0.1.0",
		Deps: healthDepsResponse{
			MCP: mcpStatus,
			DB:  dbStatus,
		},
	})
}

// handleLive is the Kubernetes liveness probe endpoint.
// It always returns 200 as long as the process is running.
// @Summary Live Check
// @Description Returns process liveness status.
// @Tags System
// @Produce json
// @Success 200 {object} liveResponse
// @Router /health/live [get]
func (s *Server) handleLive(c *gin.Context) {
	c.JSON(http.StatusOK, liveResponse{Status: "ok"})
}

// handleIssueToken exchanges a valid API key for a signed JWT.
// POST /api/v1/auth/token
// Body: {"api_key": "<key>"}
// Response: {"token": "<jwt>", "expires_in": 3600, "role": "...", "subject": "..."}
// Returns 501 when SYNAPSE_JWT_SECRET is not configured.
// @Summary Issue Token
// @Description Exchanges a valid API key for a signed JWT.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body issueTokenRequest true "Issue token request"
// @Success 200 {object} issueTokenResponse
// @Failure 400 {object} apiError
// @Failure 401 {object} apiError
// @Failure 500 {object} apiError
// @Failure 501 {object} apiError
// @Router /api/v1/auth/token [post]
func (s *Server) handleIssueToken(c *gin.Context) {
	if len(s.jwtSecret) == 0 {
		writeError(c, http.StatusNotImplemented, "jwt_not_configured",
			"JWT signing is not configured (set SYNAPSE_JWT_SECRET)", nil)
		return
	}
	var req issueTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid request body", err.Error())
		return
	}
	id, ok := s.lookupAPIKey(req.APIKey)
	if !ok {
		writeError(c, http.StatusUnauthorized, "invalid_key", "invalid API key", nil)
		return
	}
	const ttl = time.Hour
	token, err := auth.IssueJWT(s.jwtSecret, id, ttl)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "jwt_error", "failed to issue token", err.Error())
		return
	}
	c.JSON(http.StatusOK, issueTokenResponse{
		Token:     token,
		ExpiresIn: int(ttl.Seconds()),
		Role:      string(id.Role),
		Subject:   id.Subject,
	})
}

// Metrics returns Prometheus metrics output.
// @Summary Metrics
// @Description Returns Prometheus metrics output.
// @Tags System
// @Produce plain
// @Success 200 {string} string "Prometheus metrics"
// @Router /metrics [get]
func (s *Server) handleMetrics(c *gin.Context) {
	c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	c.String(http.StatusOK, s.metricsCollector.RenderPrometheus())
}

// --- DAG CRUD ---

// Create DAG creates a new DAG configuration.
// @Summary Create DAG
// @Description Creates a new DAG configuration.
// @Tags DAG
// @Accept json
// @Produce json
// @Param request body models.DAGConfig true "DAG configuration"
// @Success 201 {object} object "Created DAG"
// @Failure 400 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/dags [post]
func (s *Server) handleCreateDAG(c *gin.Context) {
	dag, ok := getValidatedDAG(c)
	if !ok {
		return
	}

	if dag.ID == "" {
		dag.ID = generateID()
	}
	if err := s.dagService.CreateDAG(c.Request.Context(), dag); err != nil {
		writeError(c, http.StatusInternalServerError, "dag_create_failed", "failed to create DAG", err.Error())
		return
	}

	c.JSON(http.StatusCreated, dag)
}

// List DAGs returns all DAG configurations.
// @Summary List DAGs
// @Description Returns all DAG configurations.
// @Tags DAG
// @Produce json
// @Success 200 {array} object "DAG list"
// @Failure 500 {object} apiError
// @Router /api/v1/dags [get]
func (s *Server) handleListDAGs(c *gin.Context) {
	list, err := s.dagService.ListDAGs(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "dag_list_failed", "failed to list DAGs", err.Error())
		return
	}
	c.JSON(http.StatusOK, list)
}

// Get DAG returns a DAG configuration by ID.
// @Summary Get DAG
// @Description Returns a DAG configuration by ID.
// @Tags DAG
// @Produce json
// @Param id path string true "DAG ID"
// @Success 200 {object} object "DAG"
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/dags/{id} [get]
func (s *Server) handleGetDAG(c *gin.Context) {
	id := c.Param("id")
	dag, err := s.dagService.GetDAG(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, appDAG.ErrDAGNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "dag_get_failed", "failed to get DAG", err.Error())
		return
	}
	c.JSON(http.StatusOK, dag)
}

// Update DAG updates an existing DAG configuration.
// @Summary Update DAG
// @Description Updates an existing DAG configuration.
// @Tags DAG
// @Accept json
// @Produce json
// @Param id path string true "DAG ID"
// @Param request body models.DAGConfig true "DAG configuration"
// @Success 200 {object} object "Updated DAG"
// @Failure 400 {object} apiError
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/dags/{id} [put]
func (s *Server) handleUpdateDAG(c *gin.Context) {
	id := c.Param("id")
	dag, ok := getValidatedDAG(c)
	if !ok {
		return
	}
	if err := s.dagService.UpdateDAG(c.Request.Context(), id, dag); err != nil {
		if errors.Is(err, appDAG.ErrDAGNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "dag_update_failed", "failed to update DAG", err.Error())
		return
	}

	c.JSON(http.StatusOK, dag)
}

// Delete DAG deletes a DAG configuration by ID.
// @Summary Delete DAG
// @Description Deletes a DAG configuration by ID.
// @Tags DAG
// @Produce json
// @Param id path string true "DAG ID"
// @Success 200 {object} deleteDAGResponse
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/dags/{id} [delete]
func (s *Server) handleDeleteDAG(c *gin.Context) {
	id := c.Param("id")
	if err := s.dagService.DeleteDAG(c.Request.Context(), id); err != nil {
		if errors.Is(err, appDAG.ErrDAGNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "dag_delete_failed", "failed to delete DAG", err.Error())
		return
	}
	c.JSON(http.StatusOK, deleteDAGResponse{Message: "DAG deleted"})
}

// --- Execution ---

// Run DAG starts execution for a saved DAG.
// @Summary Run DAG
// @Description Starts execution for a saved DAG.
// @Tags Execution
// @Produce json
// @Param id path string true "DAG ID"
// @Success 202 {object} runExecutionResponse
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/dags/{id}/run [post]
func (s *Server) handleRunDAG(c *gin.Context) {
	id := c.Param("id")
	dag, err := s.dagService.GetDAG(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, appDAG.ErrDAGNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "dag_get_failed", "failed to get DAG", err.Error())
		return
	}

	s.runWorkflow(c, dag)
}

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

func generateID() string {
	return uuid.New().String()
}

func writeError(c *gin.Context, status int, code string, message string, details interface{}) {
	c.JSON(status, apiError{
		Error:   message,
		Code:    code,
		Details: details,
	})
}

func (s *Server) validateDAGMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		dag := new(models.DAGConfig)
		if err := c.ShouldBindJSON(dag); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", "invalid JSON", err.Error())
			c.Abort()
			return
		}

		if _, err := engine.ParseDAG(dag); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_dag", "invalid DAG", err.Error())
			c.Abort()
			return
		}

		c.Set("validated_dag", dag)
		c.Next()
	}
}

func getValidatedDAG(c *gin.Context) (*models.DAGConfig, bool) {
	v, ok := c.Get("validated_dag")
	if !ok {
		writeError(c, http.StatusInternalServerError, "internal", "internal error", "validated DAG missing")
		return nil, false
	}
	dag, ok := v.(*models.DAGConfig)
	if !ok || dag == nil {
		writeError(c, http.StatusInternalServerError, "internal", "internal error", "validated DAG wrong type")
		return nil, false
	}
	return dag, true
}

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
func parseAPIKeys(raw string) map[string]*auth.Identity {
	out := make(map[string]*auth.Identity)
	if raw == "" {
		return out
	}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) != 3 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		role := auth.Role(strings.ToLower(strings.TrimSpace(parts[1])))
		subject := strings.TrimSpace(parts[2])
		if key == "" || subject == "" || !auth.IsValidRole(role) {
			continue
		}
		out[key] = &auth.Identity{Subject: subject, Role: role, Mode: "apikey", APIKey: key}
	}
	return out
}

// resolveSlackURL picks the Slack webhook URL for this DAG run.
// DAG metadata key "slack_webhook_url" takes precedence over the server-level default.
func (s *Server) resolveSlackURL(dag *models.DAGConfig) string {
	if dag != nil {
		if url, ok := dag.Metadata["slack_webhook_url"]; ok && url != "" {
			return url
		}
	}
	return s.slackWebhookURL
}

func buildExecutionNotification(exec *models.Execution, dag *models.DAGConfig, duration time.Duration) string {
	status := "unknown"
	if exec != nil {
		status = string(exec.Status)
	}
	message := fmt.Sprintf("*Synapse Execution %s*\nDAG: %s\nStatus: %s\nDuration: %s",
		executionID(exec), dagName(dag, exec), status, duration.Round(time.Millisecond))

	if summary := strings.TrimSpace(alertSummary(exec)); summary != "" {
		message += "\nAlert: " + summary
	}
	if conclusion := strings.TrimSpace(executionConclusion(exec)); conclusion != "" {
		message += "\nConclusion: " + conclusion
	}
	if detailsURL := strings.TrimSpace(executionDetailsURL(dag, exec)); detailsURL != "" {
		message += "\nDetails: " + detailsURL
	}
	if exec != nil && exec.Error != "" {
		message += "\nError: " + exec.Error
	}
	return message
}

func executionID(exec *models.Execution) string {
	if exec == nil || exec.ID == "" {
		return "unknown"
	}
	return exec.ID
}

func dagName(dag *models.DAGConfig, exec *models.Execution) string {
	if dag != nil && dag.Name != "" {
		return dag.Name
	}
	if exec != nil && exec.DAGName != "" {
		return exec.DAGName
	}
	return "unknown"
}

func alertSummary(exec *models.Execution) string {
	if exec == nil || exec.State == nil {
		return ""
	}
	if summary := exec.State.GetString("alert_summary"); summary != "" {
		return summary
	}
	service := exec.State.GetString("service_name")
	alertName := firstNonEmptyString(exec.State.GetString("alert_name"), exec.State.GetString("alert_type"))
	return strings.TrimSpace(strings.TrimSpace(service + " " + alertName))
}

func executionConclusion(exec *models.Execution) string {
	if exec == nil {
		return ""
	}
	for i := len(exec.Results) - 1; i >= 0; i-- {
		result := exec.Results[i]
		if result.Output == "" {
			continue
		}
		conclusion := extractConclusion(result.Output)
		if conclusion != "" {
			return conclusion
		}
	}
	return ""
}

func extractConclusion(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(output), &payload); err == nil {
		for _, key := range []string{"root_cause", "summary", "conclusion", "message"} {
			if value, ok := payload[key]; ok {
				if text := strings.TrimSpace(fmt.Sprintf("%v", value)); text != "" {
					return text
				}
			}
		}
	}
	return output
}

func executionDetailsURL(dag *models.DAGConfig, exec *models.Execution) string {
	if exec == nil {
		return ""
	}
	baseURL := ""
	if dag != nil {
		baseURL = firstNonEmptyString(dag.Metadata["execution_details_base_url"], dag.Metadata["frontend_base_url"])
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/executions/" + exec.ID
}

// Execution/workspace handlers and view builders moved to
// server_execution.go and server_workspace.go.
