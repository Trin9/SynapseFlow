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

	"github.com/Trin9/SynapseFlow/backend/internal/api/dto"
	appExecution "github.com/Trin9/SynapseFlow/backend/internal/application/execution"
	appWorkspace "github.com/Trin9/SynapseFlow/backend/internal/application/workspace"
	workspaceView "github.com/Trin9/SynapseFlow/backend/internal/application/workspace/view"
	"github.com/Trin9/SynapseFlow/backend/internal/auth"
	"github.com/Trin9/SynapseFlow/backend/internal/config"
	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
	"github.com/Trin9/SynapseFlow/backend/internal/engine"
	"github.com/Trin9/SynapseFlow/backend/internal/llm"
	"github.com/Trin9/SynapseFlow/backend/internal/mcp"
	"github.com/Trin9/SynapseFlow/backend/internal/memory"
	"github.com/Trin9/SynapseFlow/backend/internal/metrics"
	"github.com/Trin9/SynapseFlow/backend/internal/notify"
	projectionWorkspace "github.com/Trin9/SynapseFlow/backend/internal/projection/workspace"
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
	execService   *appExecution.Service
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
	Episodes []models.EpisodeSummaryView `json:"episodes"` // Episode summary items.
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
	dag.CreatedAt = time.Now()
	dag.UpdatedAt = dag.CreatedAt
	if err := s.dags.Create(c.Request.Context(), dag); err != nil {
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
	list, err := s.dags.List(c.Request.Context())
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
	dag, err := s.dags.Get(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
		return
	}
	if err != nil {
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
	if _, err := s.dags.Get(c.Request.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
		return
	} else if err != nil {
		writeError(c, http.StatusInternalServerError, "dag_get_failed", "failed to get DAG", err.Error())
		return
	}

	dag, ok := getValidatedDAG(c)
	if !ok {
		return
	}

	dag.ID = id
	dag.UpdatedAt = time.Now()
	if err := s.dags.Update(c.Request.Context(), dag); err != nil {
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
	if err := s.dags.Delete(c.Request.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
		return
	} else if err != nil {
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
	dag, err := s.dags.Get(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "dag_get_failed", "failed to get DAG", err.Error())
		return
	}

	s.runWorkflow(c, dag)
}

// handleRunInline accepts a full DAGConfig in the request body and executes it immediately.
// @Summary Run Inline DAG
// @Description Accepts a full DAG configuration and executes it immediately.
// @Tags Execution
// @Accept json
// @Produce json
// @Param request body object true "DAG configuration"
// @Success 202 {object} runExecutionResponse
// @Failure 400 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/run [post]
func (s *Server) handleRunInline(c *gin.Context) {
	dag, ok := getValidatedDAG(c)
	if !ok {
		return
	}

	if dag.ID == "" {
		dag.ID = generateID()
	}

	s.runWorkflow(c, dag)
}

func (s *Server) runWorkflow(c *gin.Context, dag *models.DAGConfig) {
	if s.execService == nil {
		writeError(c, http.StatusInternalServerError, "internal", "execution service unavailable", nil)
		return
	}
	exec, err := s.execService.RunWorkflow(dag, nil, "api")
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_dag", "invalid DAG", err.Error())
		return
	}
	c.JSON(http.StatusAccepted, runExecutionResponse{
		ExecutionID: exec.ID,
		Status:      exec.Status,
	})
}

func (s *Server) startExecution(dag *models.DAGConfig, initialState *models.GlobalState, source string) *models.Execution {
	if s.execService == nil {
		logger.L().Errorw("execution service unavailable")
		return &models.Execution{
			ID:        generateID(),
			DAGID:     dag.ID,
			DAGName:   dag.Name,
			Status:    models.StatusFailed,
			Error:     "execution service unavailable",
			StartedAt: time.Now(),
		}
	}
	return s.execService.StartExecution(dag, initialState, source)
}

// Get Execution returns execution details by ID.
// @Summary Get Execution
// @Description Returns execution details by ID.
// @Tags Execution
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} object "Execution"
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id} [get]
func (s *Server) handleGetExecution(c *gin.Context) {
	id := c.Param("id")
	exec, err := s.execs.Get(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "Execution not found", nil)
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "execution_get_failed", "failed to get execution", err.Error())
		return
	}

	c.JSON(http.StatusOK, exec)
}

// List Executions returns executions with optional filters.
// @Summary List Executions
// @Description Returns executions with optional filters.
// @Tags Execution
// @Produce json
// @Param view query string false "Set to summary for summary view"
// @Param dag_id query string false "Filter by DAG ID"
// @Param status query string false "Filter by execution status"
// @Param limit query int false "Pagination limit (for dag_id filter)"
// @Param offset query int false "Pagination offset (for dag_id filter)"
// @Success 200 {array} object "Execution list"
// @Failure 500 {object} apiError
// @Router /api/v1/executions [get]
func (s *Server) handleListExecutions(c *gin.Context) {
	ctx := c.Request.Context()
	viewSummary := c.Query("view") == "summary"

	// Optional filter: ?dag_id=<id>
	if dagID := c.Query("dag_id"); dagID != "" {
		limitStr := c.DefaultQuery("limit", "0")
		offsetStr := c.DefaultQuery("offset", "0")
		limit := 0
		offset := 0
		fmt.Sscanf(limitStr, "%d", &limit)
		fmt.Sscanf(offsetStr, "%d", &offset)
		list, err := s.execs.ListByDAGID(ctx, dagID, limit, offset)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "execution_list_failed", "failed to list executions by dag", err.Error())
			return
		}
		if list == nil {
			list = []*models.Execution{}
		}
		if viewSummary {
			c.JSON(http.StatusOK, projectExecutionList(list))
			return
		}
		c.JSON(http.StatusOK, list)
		return
	}
	// Optional filter: ?status=<status>
	if statusStr := c.Query("status"); statusStr != "" {
		list, err := s.execs.ListByStatus(ctx, models.ExecutionStatus(statusStr))
		if err != nil {
			writeError(c, http.StatusInternalServerError, "execution_list_failed", "failed to list executions by status", err.Error())
			return
		}
		if list == nil {
			list = []*models.Execution{}
		}
		if viewSummary {
			c.JSON(http.StatusOK, projectExecutionList(list))
			return
		}
		c.JSON(http.StatusOK, list)
		return
	}
	list, err := s.execs.List(ctx)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "execution_list_failed", "failed to list executions", err.Error())
		return
	}
	if viewSummary {
		c.JSON(http.StatusOK, projectExecutionList(list))
		return
	}
	c.JSON(http.StatusOK, list)
}

// Get Execution Nodes returns node-level results for one execution.
// @Summary Get Execution Nodes
// @Description Returns node-level results for one execution.
// @Tags Execution
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} object "Execution node results"
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/nodes [get]
func (s *Server) handleGetExecutionNodes(c *gin.Context) {
	id := c.Param("id")
	exec, err := s.execs.Get(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "Execution not found", nil)
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "execution_get_failed", "failed to get execution", err.Error())
		return
	}

	results := exec.Results
	if results == nil {
		results = make([]models.NodeResult, 0)
	}

	// Keep response stable for frontend polling: include status and results.
	c.JSON(http.StatusOK, executionNodesResponse{
		ExecutionID: exec.ID,
		Status:      exec.Status,
		Results:     results,
		Error:       exec.Error,
		StartedAt:   exec.StartedAt,
		EndedAt:     exec.EndedAt,
		DurationMS:  exec.Duration.Milliseconds(),
	})
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
	if s.memory == nil {
		c.JSON(http.StatusOK, []models.Experience{})
		return
	}

	experiences, err := s.memory.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "memory_error", "failed to list experiences", err.Error())
		return
	}
	if experiences == nil {
		experiences = []models.Experience{}
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

// handleResumeExecution resumes a suspended (human-in-the-loop) execution.
// @Summary Resume Execution
// @Description Resumes a suspended execution.
// @Tags Execution
// @Accept json
// @Produce json
// @Param id path string true "Execution ID"
// @Param request body resumeExecutionRequest false "Optional human intervention context"
// @Success 202 {object} runExecutionResponse
// @Failure 404 {object} apiError
// @Failure 409 {object} apiError
// @Failure 422 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/resume [post]
func (s *Server) handleResumeExecution(c *gin.Context) {
	id := c.Param("id")

	var resumeBody resumeExecutionRequest
	_ = c.ShouldBindJSON(&resumeBody)

	exec, err := s.execService.ResumeExecution(c.Request.Context(), appExecution.ResumeInput{
		ExecutionID: id,
		Actor:       resumeBody.Actor,
		Action:      resumeBody.Action,
		Detail:      resumeBody.Detail,
	})
	if err != nil {
		switch {
		case errors.Is(err, appExecution.ErrExecutionNotFound):
			writeError(c, http.StatusNotFound, "not_found", "Execution not found", nil)
			return
		case errors.Is(err, appExecution.ErrCheckpointGet):
			writeError(c, http.StatusInternalServerError, "checkpoint_get_failed", "failed to load checkpoint", err.Error())
			return
		case errors.Is(err, appExecution.ErrDAGNotFoundForResume):
			writeError(c, http.StatusUnprocessableEntity, "dag_not_found", "original DAG not available for resume", nil)
			return
		case errors.Is(err, appExecution.ErrDAGGet):
			writeError(c, http.StatusInternalServerError, "dag_get_failed", "failed to get DAG", err.Error())
			return
		case errors.Is(err, appExecution.ErrExecutionUpdate):
			writeError(c, http.StatusInternalServerError, "execution_update_failed", "failed to update execution", err.Error())
			return
		default:
			var notSuspended appExecution.NotSuspendedError
			if errors.As(err, &notSuspended) {
				writeError(c, http.StatusConflict, "not_suspended", "execution is not suspended", notSuspended.Status)
				return
			}
			writeError(c, http.StatusInternalServerError, "execution_get_failed", "failed to get execution", err.Error())
			return
		}
	}
	c.JSON(http.StatusAccepted, runExecutionResponse{
		ExecutionID: exec.ID,
		Status:      exec.Status,
	})
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
	entries, err := s.audits.List(c.Request.Context())
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

// ---------------------------------------------------------------------------
// Episode handlers (Sprint 7)
// ---------------------------------------------------------------------------

// handleListEpisodes returns all episodes for a given execution.
//
//	GET /api/v1/executions/:id/episodes[?view=summary]
//
// When ?view=summary is set, returns EpisodeSummaryView list instead of raw Episodes.
// @Summary List Episodes
// @Description Returns all episodes for an execution; supports summary view.
// @Tags Episodes
// @Produce json
// @Param id path string true "Execution ID"
// @Param view query string false "Set to summary for summary view"
// @Success 200 {object} episodesResponse
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/episodes [get]
func (s *Server) handleListEpisodes(c *gin.Context) {
	execID := c.Param("id")
	ctx := c.Request.Context()
	if c.Query("view") == "summary" {
		episodes, err := s.episodes.ListByExecution(ctx, execID)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "episode_list_error", "failed to list episode summaries", err.Error())
			return
		}
		summaries := make([]models.EpisodeSummaryView, len(episodes))
		for i, ep := range episodes {
			summaries[i] = projectionWorkspace.EpisodeToSummary(ep)
		}
		if summaries == nil {
			summaries = []models.EpisodeSummaryView{}
		}
		c.JSON(http.StatusOK, episodeSummariesResponse{Episodes: summaries})
		return
	}
	episodes, err := s.episodes.ListByExecution(ctx, execID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "episode_list_error", "failed to list episodes", err.Error())
		return
	}
	if episodes == nil {
		episodes = []*models.Episode{}
	}
	c.JSON(http.StatusOK, episodesResponse{Episodes: episodes})
}

// handleGetEpisode returns a single episode by ID.
//
//	GET /api/v1/episodes/:id
//
// @Summary Get Episode
// @Description Returns a single episode by ID.
// @Tags Episodes
// @Produce json
// @Param id path string true "Episode ID"
// @Success 200 {object} object "Episode"
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/episodes/{id} [get]
func (s *Server) handleGetEpisode(c *gin.Context) {
	id := c.Param("id")
	ep, err := s.episodes.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(c, http.StatusNotFound, "episode_not_found", "episode not found", id)
			return
		}
		writeError(c, http.StatusInternalServerError, "episode_get_error", "failed to get episode", err.Error())
		return
	}
	c.JSON(http.StatusOK, ep)
}

// ---------------------------------------------------------------------------
// Execution Workspace handlers (M1.4)
// ---------------------------------------------------------------------------

// handleGetExecutionSummary returns a high-level summary view of a single execution.
//
//	GET /api/v1/executions/:id/summary
//
// @Summary Get Execution Summary
// @Description Returns a high-level summary view of one execution.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} object "Execution summary"
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/summary [get]
func (s *Server) handleGetExecutionSummary(c *gin.Context) {
	id := c.Param("id")
	summary, err := s.workspaceSvc.GetExecutionSummary(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrExecutionNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "summary_error", "failed to get execution summary", err.Error())
		return
	}
	c.JSON(http.StatusOK, summary)
}

// handleGetTriggerContext returns the trigger context view for an execution,
// built from the first episode's trigger data.
//
//	GET /api/v1/executions/:id/trigger-context
//
// @Summary Get Trigger Context
// @Description Returns trigger context view for an execution.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} object "Trigger context"
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/trigger-context [get]
func (s *Server) handleGetTriggerContext(c *gin.Context) {
	execID := c.Param("id")
	view, err := s.workspaceSvc.GetTriggerContext(c.Request.Context(), execID)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrExecutionNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		if errors.Is(err, appWorkspace.ErrExecutionGet) {
			writeError(c, http.StatusInternalServerError, "trigger_context_error", "failed to get execution", err.Error())
			return
		}
		if errors.Is(err, appWorkspace.ErrEpisodeList) {
			writeError(c, http.StatusInternalServerError, "trigger_context_error", "failed to list episodes", err.Error())
			return
		}
		writeError(c, http.StatusInternalServerError, "trigger_context_error", "failed to get trigger context", err.Error())
		return
	}
	c.JSON(http.StatusOK, view)
}

// handleGetReviewState returns the aggregate human-review state for an execution.
//
//	GET /api/v1/executions/:id/review-state
//
// @Summary Get Review State
// @Description Returns aggregate human-review state for an execution.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} object "Review state"
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/review-state [get]
func (s *Server) handleGetReviewState(c *gin.Context) {
	execID := c.Param("id")
	state, err := s.workspaceSvc.GetReviewState(c.Request.Context(), execID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "review_state_error", "failed to get review state", err.Error())
		return
	}
	c.JSON(http.StatusOK, state)
}

// handlePostReviewAction records a human review decision on an execution.
//
//	POST /api/v1/executions/:id/review-actions
//
// @Summary Post Review Action
// @Description Records a human review decision on an execution.
// @Tags Workspace
// @Accept json
// @Produce json
// @Param id path string true "Execution ID"
// @Param request body object true "Review action request"
// @Success 200 {object} reviewActionResponse
// @Failure 400 {object} apiError
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/review-actions [post]
func (s *Server) handlePostReviewAction(c *gin.Context) {
	execID := c.Param("id")
	var req dto.ReviewActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid request body", err.Error())
		return
	}
	if err := s.workspaceSvc.PostReviewAction(c.Request.Context(), execID, req.EpisodeID, req.Status, req.Actor, req.Note); err != nil {
		if errors.Is(err, appWorkspace.ErrExecutionNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "review_action_error", "failed to write review state", err.Error())
		return
	}
	c.JSON(http.StatusOK, reviewActionResponse{OK: true})
}

// handleGetEpisodeReplay returns a replay slice view for a single episode.
//
//	GET /api/v1/executions/:id/episodes/:episode_id/replay?percent=N
//
// @Summary Get Episode Replay
// @Description Returns replay slice view for a single episode.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Param episode_id path string true "Episode ID"
// @Param percent query int false "Replay percentage (0-100)"
// @Success 200 {object} object "Replay slice view"
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/episodes/{episode_id}/replay [get]
func (s *Server) handleGetEpisodeReplay(c *gin.Context) {
	episodeID := c.Param("episode_id")
	percent := 100
	fmt.Sscanf(c.DefaultQuery("percent", "100"), "%d", &percent)
	view, err := s.workspaceSvc.GetEpisodeReplay(c.Request.Context(), episodeID, percent)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrEpisodeNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "episode not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "replay_error", "failed to get episode", err.Error())
		return
	}
	c.JSON(http.StatusOK, view)
}

// handleGetEpisodeDossier returns the full dossier for a single episode.
//
//	GET /api/v1/executions/:id/episodes/:episode_id/dossier
//
// @Summary Get Episode Dossier
// @Description Returns full dossier for a single episode.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Param episode_id path string true "Episode ID"
// @Success 200 {object} object "Episode dossier"
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/episodes/{episode_id}/dossier [get]
func (s *Server) handleGetEpisodeDossier(c *gin.Context) {
	episodeID := c.Param("episode_id")
	dossier, err := s.workspaceSvc.GetEpisodeDossier(c.Request.Context(), episodeID)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrEpisodeNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "episode not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "dossier_error", "failed to get episode", err.Error())
		return
	}
	c.JSON(http.StatusOK, dossier)
}

// handleGetEpisodeMemoryRecalls returns memory recall items for a single episode.
// CR-010: uses s.memory.Search() to perform real Experience retrieval instead
// of the former stub which always returned an empty slice.
//
//	GET /api/v1/executions/:id/episodes/:episode_id/memory-recalls
//
// @Summary Get Episode Memory Recalls
// @Description Returns memory recall items for a single episode.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Param episode_id path string true "Episode ID"
// @Success 200 {object} object "Memory recall list"
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/episodes/{episode_id}/memory-recalls [get]
func (s *Server) handleGetEpisodeMemoryRecalls(c *gin.Context) {
	episodeID := c.Param("episode_id")
	list, err := s.workspaceSvc.GetEpisodeMemoryRecalls(c.Request.Context(), episodeID)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrEpisodeNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "episode not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "memory_recall_error", "failed to search memory recalls", err.Error())
		return
	}
	c.JSON(http.StatusOK, list)
}

// buildMemoryRecallsForEpisode searches the Experience store for historical
// entries relevant to the given episode and maps them to MemoryRecallView
// items.  It is the single source of truth for both the /memory-recalls
// endpoint (CR-010) and the dossier's memory_recalls field (CR-011), ensuring
// both surfaces return consistent data.
//
// Returns (nil-safe empty slice, nil) when expStore is absent or when the
// search yields no results.  Returns a non-nil error only when the store is
// present but Search itself fails, allowing callers to distinguish "no match"
// from "store fault".
func buildMemoryRecallsForEpisode(ctx context.Context, ep *models.Episode, expStore memory.ExperienceStore) ([]workspaceView.MemoryRecallView, error) {
	if expStore == nil || ep == nil {
		return []workspaceView.MemoryRecallView{}, nil
	}

	// Build free-text search corpus from trigger payload, evidence labels, and
	// the verdict's conclusion / causal chain.
	var parts []string
	alertType := ""
	serviceName := ""

	if ep.Trigger != nil {
		for _, key := range []string{"alert_text", "alert_summary", "alert_type", "service_name", "symptom", "input"} {
			if v, ok := ep.Trigger.Payload[key]; ok {
				if s := fmt.Sprintf("%v", v); s != "" {
					parts = append(parts, s)
				}
			}
		}
		if v, ok := ep.Trigger.Payload["alert_type"]; ok {
			alertType = fmt.Sprintf("%v", v)
		}
		if v, ok := ep.Trigger.Payload["service_name"]; ok {
			serviceName = fmt.Sprintf("%v", v)
		}
	}

	for _, ev := range ep.Evidence {
		if ev.Label != "" {
			parts = append(parts, ev.Label)
		}
	}

	if ep.Verdict != nil {
		if ep.Verdict.Conclusion != "" {
			parts = append(parts, ep.Verdict.Conclusion)
		}
		parts = append(parts, ep.Verdict.CausalChain...)
	}

	query := store.SearchQuery{
		Text:        strings.TrimSpace(strings.Join(parts, "\n")),
		AlertType:   alertType,
		ServiceName: serviceName,
		TopK:        5,
	}

	experiences, err := expStore.Search(ctx, query)
	if err != nil {
		return []workspaceView.MemoryRecallView{}, err
	}
	if len(experiences) == 0 {
		return []workspaceView.MemoryRecallView{}, nil
	}

	recalls := make([]workspaceView.MemoryRecallView, 0, len(experiences))
	for _, exp := range experiences {
		// Map float score → confidence label consistent with EpisodeConfidence
		// vocabulary used elsewhere in the codebase.
		confidence := "low"
		if exp.Score >= 0.7 {
			confidence = "high"
		} else if exp.Score >= 0.4 {
			confidence = "medium"
		}

		title := exp.Summary
		if title == "" {
			title = exp.AlertType
		}
		if title == "" {
			title = exp.ID
		}

		matchedPattern := strings.Join(exp.Tags, ", ")
		if matchedPattern == "" {
			matchedPattern = exp.AlertType
		}

		recalls = append(recalls, workspaceView.MemoryRecallView{
			ID:                exp.ID,
			Title:             title,
			Summary:           exp.Summary,
			MatchedPattern:    matchedPattern,
			Confidence:        confidence,
			SourceExecutionID: exp.ExecutionID,
			Recommendation:    exp.ActionTaken,
		})
	}
	return recalls, nil
}

// handleGetComparisonTarget compares two executions and returns a summary.
//
//	GET /api/v1/executions/:id/comparison-targets/:historical_id
//
// @Summary Get Comparison Target
// @Description Compares two executions and returns a summary.
// @Tags Workspace
// @Produce json
// @Param id path string true "Current execution ID"
// @Param historical_id path string true "Historical execution ID"
// @Success 200 {object} object "Comparison summary"
// @Failure 404 {object} apiError
// @Failure 500 {object} apiError
// @Router /api/v1/executions/{id}/comparison-targets/{historical_id} [get]
func (s *Server) handleGetComparisonTarget(c *gin.Context) {
	execID := c.Param("id")
	historicalID := c.Param("historical_id")
	summary, err := s.workspaceSvc.GetComparisonTarget(c.Request.Context(), execID, historicalID)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrExecutionNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		if errors.Is(err, appWorkspace.ErrHistoricalNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "historical execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "comparison_error", "failed to build comparison", err.Error())
		return
	}
	c.JSON(http.StatusOK, summary)
}

// ---------------------------------------------------------------------------
// Execution Workspace view-building helpers
// ---------------------------------------------------------------------------

// buildTriggerContextView constructs a TriggerContextView from execution + episode data.
func buildTriggerContextView(exec *models.Execution, episodes []*models.Episode) workspaceView.TriggerContextView {
	view := workspaceView.TriggerContextView{
		Title:   fmt.Sprintf("Trigger — %s", exec.DAGName),
		Summary: fmt.Sprintf("Execution %s triggered on %s", exec.ID[:8], exec.StartedAt.Format(time.RFC3339)),
	}
	// Build sections from the first episode with trigger data.
	for _, ep := range episodes {
		if ep.Trigger == nil {
			continue
		}
		t := ep.Trigger
		// Helper to safely read string values from the Payload map.
		payloadStr := func(key string) string {
			if t.Payload == nil {
				return ""
			}
			if v, ok := t.Payload[key]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
			return ""
		}
		section := workspaceView.TriggerContextSectionView{
			Title: "Alert",
			Fields: []workspaceView.TriggerContextFieldView{
				{Label: "Trigger Type", Value: string(t.Type), Range: [2]int{0, 0}},
				{Label: "Alert Type", Value: payloadStr("alert_type"), Range: [2]int{0, 0}},
				{Label: "Service", Value: payloadStr("service_name"), Range: [2]int{0, 0}},
				{Label: "Severity", Value: payloadStr("severity"), Range: [2]int{0, 0}},
			},
		}
		// Remove empty fields.
		nonEmpty := section.Fields[:0]
		for _, f := range section.Fields {
			if f.Value != "" {
				nonEmpty = append(nonEmpty, f)
			}
		}
		if len(nonEmpty) > 0 {
			section.Fields = nonEmpty
			view.Sections = append(view.Sections, section)
		}
		// Investigation context section.
		if ep.InvestigationContext != nil {
			ic := ep.InvestigationContext
			icSection := workspaceView.TriggerContextSectionView{
				Title: "Investigation",
				Fields: []workspaceView.TriggerContextFieldView{
					{Label: "Hypothesis", Value: ic.Hypothesis},
				},
			}
			if len(ic.KnownSignals) > 0 {
				icSection.Fields = append(icSection.Fields, workspaceView.TriggerContextFieldView{
					Label: "Known Signals",
					Value: strings.Join(ic.KnownSignals, ", "),
				})
			}
			view.Sections = append(view.Sections, icSection)
		}
		// Only use the first episode with a trigger.
		break
	}
	return view
}

// buildReplaySliceView computes what is visible in the replay at the given percent.
func buildReplaySliceView(ep *models.Episode, trace []models.ProcessTraceEntryView, percent int) workspaceView.ReplaySliceView {
	visible := make([]models.ProcessTraceEntryView, 0, len(trace))
	visibleFactIDs := make([]string, 0)
	for _, entry := range trace {
		if entry.Range[0] <= percent {
			visible = append(visible, entry)
			visibleFactIDs = append(visibleFactIDs, entry.ID)
		}
	}
	// Derive checkpoint narrative from the last visible trace entry.
	checkpoint := workspaceView.ReplayCheckpointView{
		Label:    fmt.Sprintf("%d%%", percent),
		Headline: "Execution in progress",
	}
	if len(visible) > 0 {
		last := visible[len(visible)-1]
		checkpoint.Headline = last.Title
		checkpoint.Detail = last.Detail
	}
	return workspaceView.ReplaySliceView{
		EpisodeID:             ep.ID,
		Percent:               percent,
		Checkpoint:            checkpoint,
		VisibleProcessTrace:   visible,
		VisibleHandles:        []interface{}{},
		VisibleStateFields:    []interface{}{},
		VisibleRuntimeFactIDs: visibleFactIDs,
	}
}

// buildEpisodeDossier constructs an EpisodeDossierView from an episode and its facts.
func buildEpisodeDossier(ep *models.Episode, facts []models.RuntimeFactView, recalls []workspaceView.MemoryRecallView) workspaceView.EpisodeDossierView {
	display := models.DossierDisplayView{}
	if ep.Verdict != nil {
		display.Verdict = string(ep.Verdict.Result)
		display.VerdictLabel = domainEpisode.VerdictLabelFromResult(ep.Verdict.Result)
		display.Summary = ep.Verdict.Conclusion
	}
	domainEpisode.ApplyHumanReviewDisplay(ep, &display)

	// Derive a common focus_key for cross-column linkage when the episode has a
	// single unambiguous handle (or all handles share the same type:value).
	// Format: "<handle_type>:<handle_value>" e.g. "order_id:ORD-001".
	// When ambiguous (multiple distinct handles) we leave focus_key empty to
	// avoid incorrect cross-column linkage.
	commonFocusKey := ""
	if len(ep.Handles) == 1 {
		commonFocusKey = string(ep.Handles[0].Type) + ":" + ep.Handles[0].Value
	} else if len(ep.Handles) > 1 {
		first := string(ep.Handles[0].Type) + ":" + ep.Handles[0].Value
		allSame := true
		for _, h := range ep.Handles[1:] {
			if string(h.Type)+":"+h.Value != first {
				allSame = false
				break
			}
		}
		if allSame {
			commonFocusKey = first
		}
	}

	// Expected behaviors derived from Verdict's causal chain.
	var expectedBehaviors []workspaceView.ExpectedBehaviorView
	if ep.Verdict != nil {
		for i, link := range ep.Verdict.CausalChain {
			expectedBehaviors = append(expectedBehaviors, workspaceView.ExpectedBehaviorView{
				ID:          fmt.Sprintf("causal_%d", i),
				Title:       fmt.Sprintf("Causal Factor %d", i+1),
				Body:        link,
				FocusKey:    commonFocusKey,
				SourceType:  "ai",
				SourceLabel: "AI Hypothesized",
			})
		}
	}
	// Verdict bridge derived from recommendations.
	var verdictBridge []workspaceView.VerdictBridgeItemView
	if ep.Verdict != nil {
		for i, rec := range ep.Verdict.Recommendations {
			verdictBridge = append(verdictBridge, workspaceView.VerdictBridgeItemView{
				ID:       fmt.Sprintf("rec_%d", i),
				Title:    fmt.Sprintf("Recommendation %d", i+1),
				Body:     rec,
				FocusKey: commonFocusKey,
			})
		}
	}
	return workspaceView.EpisodeDossierView{
		Episode: workspaceView.DossierEpisodeRefView{
			EpisodeID: ep.ID,
			Label:     string(ep.EpisodeType),
		},
		Display: workspaceView.DossierDisplayView{
			Verdict:      display.Verdict,
			VerdictLabel: display.VerdictLabel,
			Summary:      display.Summary,
			Banner:       display.Banner,
		},
		ExpectedBehavior: expectedBehaviors,
		VerdictBridge:    verdictBridge,
		RuntimeFacts:     facts,
		Handles:          ep.Handles,
		MemoryRecalls:    recalls,
		HumanAuditTrail:  ep.HumanInterventions,
	}
}

// buildComparisonSummary compares two executions and returns a summary view.
func buildComparisonSummary(current, historical *models.Execution) appWorkspace.ComparisonSummaryView {
	summary := appWorkspace.ComparisonSummaryView{
		ExecutionID:     current.ID,
		Title:           fmt.Sprintf("%s vs %s", current.ID[:8], historical.ID[:8]),
		ComparedAgainst: historical.ID,
	}
	// CR-014: Outcome must be a semantic discriminator ("match"/"divergent"), not
	// the raw status string, so the frontend colour branches resolve correctly.
	if current.Status == historical.Status {
		summary.Summary = fmt.Sprintf("Both executions completed with status: %s", current.Status)
		summary.Outcome = "match"
	} else {
		summary.Summary = fmt.Sprintf("Current: %s — Historical: %s", current.Status, historical.Status)
		summary.Outcome = "divergent"
		summary.Caution = "Execution outcomes differ."
	}
	// Duration comparison highlight.
	if current.Duration > 0 && historical.Duration > 0 {
		diff := current.Duration - historical.Duration
		if diff > 0 {
			summary.Highlights = append(summary.Highlights,
				fmt.Sprintf("Current run was %s slower than historical", diff.Round(time.Millisecond)))
		} else if diff < 0 {
			summary.Highlights = append(summary.Highlights,
				fmt.Sprintf("Current run was %s faster than historical", (-diff).Round(time.Millisecond)))
		}
	}
	return summary
}

// execToSummaryView projects a raw Execution into an ExecutionSummaryView.
// Used by handleListExecutions when ?view=summary is requested (CR-015).
// Mirrors the logic in store.projectExecutionToSummary; kept inline to avoid
// coupling the API layer to the store package.
func execToSummaryView(exec *models.Execution) models.ExecutionSummaryView {
	label := exec.DAGName
	if len(exec.ID) >= 8 {
		label = fmt.Sprintf("%s #%s", exec.DAGName, exec.ID[:8])
	}
	return models.ExecutionSummaryView{
		ExecutionID:  exec.ID,
		DAGID:        exec.DAGID,
		DAGName:      exec.DAGName,
		Status:       exec.Status,
		StartedAt:    exec.StartedAt,
		EndedAt:      exec.EndedAt,
		DurationMs:   exec.Duration.Milliseconds(),
		Mode:         "execution",
		WorkflowKind: "investigation",
		Display: models.ExecutionDisplayView{
			RunLabel:   label,
			TraceTitle: exec.DAGName,
		},
	}
}

// projectExecutionList converts a slice of raw Executions to ExecutionSummaryView.
func projectExecutionList(execs []*models.Execution) []models.ExecutionSummaryView {
	out := make([]models.ExecutionSummaryView, len(execs))
	for i, e := range execs {
		out[i] = execToSummaryView(e)
	}
	return out
}
