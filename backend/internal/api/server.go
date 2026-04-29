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
}

// apiError is the unified error response format.
// details is optional and should be JSON-serializable.
type apiError struct {
	Error   string      `json:"error"`
	Code    string      `json:"code"`
	Details interface{} `json:"details,omitempty"`
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
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "synapse",
		"version": "0.1.0",
		"deps": gin.H{
			"mcp": mcpStatus,
			"db":  dbStatus,
		},
	})
}

// handleLive is the Kubernetes liveness probe endpoint.
// It always returns 200 as long as the process is running.
func (s *Server) handleLive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleIssueToken exchanges a valid API key for a signed JWT.
// POST /api/v1/auth/token
// Body: {"api_key": "<key>"}
// Response: {"token": "<jwt>", "expires_in": 3600, "role": "...", "subject": "..."}
// Returns 501 when SYNAPSE_JWT_SECRET is not configured.
func (s *Server) handleIssueToken(c *gin.Context) {
	if len(s.jwtSecret) == 0 {
		writeError(c, http.StatusNotImplemented, "jwt_not_configured",
			"JWT signing is not configured (set SYNAPSE_JWT_SECRET)", nil)
		return
	}
	var req struct {
		APIKey string `json:"api_key" binding:"required"`
	}
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
	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"expires_in": int(ttl.Seconds()),
		"role":       string(id.Role),
		"subject":    id.Subject,
	})
}

func (s *Server) handleMetrics(c *gin.Context) {
	c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	c.String(http.StatusOK, s.metricsCollector.RenderPrometheus())
}

// --- DAG CRUD ---

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

func (s *Server) handleListDAGs(c *gin.Context) {
	list, err := s.dags.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "dag_list_failed", "failed to list DAGs", err.Error())
		return
	}
	c.JSON(http.StatusOK, list)
}

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

func (s *Server) handleDeleteDAG(c *gin.Context) {
	id := c.Param("id")
	if err := s.dags.Delete(c.Request.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
		return
	} else if err != nil {
		writeError(c, http.StatusInternalServerError, "dag_delete_failed", "failed to delete DAG", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "DAG deleted"})
}

// --- Execution ---

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
	// Safety: validate the DAG again for saved DAG runs.
	if _, err := engine.ParseDAG(dag); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_dag", "invalid DAG", err.Error())
		return
	}

	exec := s.startExecution(dag, nil, "api")
	c.JSON(http.StatusAccepted, gin.H{
		"execution_id": exec.ID,
		"status":       exec.Status,
	})
}

func (s *Server) startExecution(dag *models.DAGConfig, initialState *models.GlobalState, source string) *models.Execution {
	execID := generateID()
	exec := &models.Execution{
		ID:        execID,
		DAGID:     dag.ID,
		DAGName:   dag.Name,
		Status:    models.StatusRunning,
		StartedAt: time.Now(),
	}
	if err := s.execs.Create(context.Background(), exec); err != nil {
		logger.L().Errorw("failed to create execution", "execution_id", execID, "error", err)
	}

	// Auto-create Episode when the DAG metadata declares an episode_type.
	// The resulting episode_id is injected into GlobalState as "__episode_id__"
	// so every ScriptExecutor and LLMExecutor can pick it up automatically.
	if epType, ok := dag.Metadata["episode_type"]; ok && epType != "" {
		ep := &models.Episode{
			ID:            generateID(),
			ExecID:        execID,
			EpisodeType:   models.EpisodeType(epType),
			Status:        models.EpisodeStatusPending,
			Trigger:       &models.EpisodeTrigger{Type: models.EpisodeTriggerManual},
			LoopGuard:     models.EpisodeLoopGuard{MaxIterations: 10},
			SchemaVersion: 1,
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
		}
		if initialState == nil {
			initialState = models.NewGlobalState()
		}
		if err := s.episodes.Create(context.Background(), ep); err != nil {
			logger.L().Warnw("failed to auto-create episode", "exec_id", execID, "error", err)
		} else {
			initialState.Set("__episode_id__", ep.ID)
			logger.L().Infow("auto-created episode", "exec_id", execID, "episode_id", ep.ID, "type", epType)
		}
	}

	// Execute asynchronously. Frontend polls /executions/:id/nodes.
	// Do not inherit the request context for async execution, otherwise the
	// background run will be canceled as soon as the HTTP request completes.
	ctx := context.Background()
	go func(execID string, dag *models.DAGConfig) {
		result := s.scheduler.Execute(ctx, dag, initialState, nil)
		now := time.Now()

		exec, err := s.execs.Get(ctx, execID)
		if err != nil {
			logger.L().Errorw("failed to load execution for update", "execution_id", execID, "error", err)
			return
		}

		exec.Duration = result.Duration
		exec.Results = result.Results
		exec.State = result.State // Persist the state for resume

		if result.Err != nil || result.Status == models.StatusFailed {
			exec.Status = models.StatusFailed
			if result.Err != nil {
				exec.Error = result.Err.Error()
			}
			exec.EndedAt = &now
		} else if result.Status == models.StatusSuspended {
			exec.Status = models.StatusSuspended
			// Do not set EndedAt for suspended execution
		} else {
			exec.Status = models.StatusCompleted
			exec.EndedAt = &now
		}
		if err := s.execs.Update(ctx, exec); err != nil {
			logger.L().Errorw("failed to persist execution update", "execution_id", execID, "error", err)
		}
		if err := s.execs.SaveNodeResults(ctx, execID, result.Results); err != nil {
			logger.L().Errorw("failed to persist node results", "execution_id", execID, "error", err)
		}

		// Auto-close Episode when the DAG reaches a terminal state.
		// The episode_id was injected into GlobalState as "__episode_id__" at
		// startExecution time.  We only advance in_progress episodes — a verdict
		// written by an llm node may have already set the status to converged.
		// TODO: remove the in-memory-only note once Postgres Episode migration lands.
		if episodeID := result.State.GetString("__episode_id__"); episodeID != "" {
			if ep, epErr := s.episodes.Get(ctx, episodeID); epErr == nil {
				if ep.Status == models.EpisodeStatusInProgress {
					switch exec.Status {
					case models.StatusCompleted:
						ep.Status = models.EpisodeStatusConverged
					case models.StatusFailed:
						ep.Status = models.EpisodeStatusFailed
					}
					ep.UpdatedAt = time.Now().UTC()
					if err := s.episodes.Update(ctx, ep); err != nil {
						logger.L().Warnw("failed to auto-close episode", "episode_id", episodeID, "exec_status", exec.Status, "error", err)
					}
				}
			}
		}
		if exec.Status == models.StatusSuspended {
			checkpoint := &models.ExecutionCheckpoint{
				ExecutionID: exec.ID,
				DAGID:       exec.DAGID,
				State:       result.State.Snapshot(),
				LoopCounts:  result.State.LoopCountsSnapshot(),
				UpdatedAt:   now,
			}
			if err := s.execs.SaveCheckpoint(ctx, checkpoint); err != nil {
				logger.L().Errorw("failed to persist checkpoint", "execution_id", execID, "error", err)
			}
		}

		// Record metrics for this execution
		s.metricsCollector.RecordExecution(exec.Status, result.Duration)
		for _, r := range result.Results {
			s.metricsCollector.RecordNode(r.NodeType, r.Duration)
			if r.TokensIn+r.TokensOut > 0 {
				s.metricsCollector.RecordLLMTokens(string(r.NodeType), r.TokensIn+r.TokensOut)
			}
		}

		if exec.Status == models.StatusCompleted && s.extractor != nil {
			go func(execSnapshot *models.Execution, dagSnapshot *models.DAGConfig) {
				if _, err := s.extractor.Extract(context.Background(), dagSnapshot, execSnapshot); err != nil {
					logger.L().Warnw("Memory extraction failed", "execution_id", execSnapshot.ID, "error", err)
				}
			}(cloneExecution(exec), dag)
		}

		// Send Slack notification on completion or failure
		notifierURL := s.resolveSlackURL(dag)
		if notifierURL != "" && s.notifier != nil {
			msg := buildExecutionNotification(exec, dag, result.Duration)
			go func() {
				if err := s.notifier.SendExecutionResult(context.Background(), notifierURL, msg); err != nil {
					logger.L().Warnw("Slack notification failed", "execution_id", execID, "error", err)
				}
			}()
		}
	}(execID, dag)

	return exec
}

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
	c.JSON(http.StatusOK, gin.H{
		"execution_id": exec.ID,
		"status":       exec.Status,
		"results":      results,
		"error":        exec.Error,
		"started_at":   exec.StartedAt,
		"ended_at":     exec.EndedAt,
		"duration_ms":  exec.Duration.Milliseconds(),
	})
}

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

func cloneExecution(exec *models.Execution) *models.Execution {
	if exec == nil {
		return nil
	}
	clone := *exec
	if exec.Results != nil {
		clone.Results = append([]models.NodeResult(nil), exec.Results...)
	}
	clone.State = exec.State.Clone()
	return &clone
}

// handleResumeExecution resumes a suspended (human-in-the-loop) execution.
func (s *Server) handleResumeExecution(c *gin.Context) {
	id := c.Param("id")

	// Optional body: actor/action/detail for HumanIntervention recording.
	var resumeBody struct {
		Actor  string `json:"actor"`
		Action string `json:"action"`
		Detail string `json:"detail"`
	}
	// Ignore parse error — body is fully optional.
	_ = c.ShouldBindJSON(&resumeBody)

	exec, err := s.execs.Get(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "Execution not found", nil)
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "execution_get_failed", "failed to get execution", err.Error())
		return
	}
	if exec.Status != models.StatusSuspended {
		writeError(c, http.StatusConflict, "not_suspended", "execution is not suspended", exec.Status)
		return
	}
	checkpoint, err := s.execs.GetCheckpoint(c.Request.Context(), id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusInternalServerError, "checkpoint_get_failed", "failed to load checkpoint", err.Error())
		return
	}
	if checkpoint != nil {
		exec.State = models.NewGlobalStateFromSnapshot(checkpoint.State, checkpoint.LoopCounts)
	}

	// Record the human intervention in the Episode (if one is linked to this execution).
	if s.episodeWriter != nil {
		if episodeID := exec.State.GetString("__episode_id__"); episodeID != "" {
			actor := resumeBody.Actor
			if actor == "" {
				actor = "operator"
			}
			action := models.HumanInterventionAction(resumeBody.Action)
			if action == "" {
				action = models.HumanActionResumed
			}
			if err := s.episodeWriter.AppendHumanIntervention(
				c.Request.Context(), episodeID, "", actor, action, resumeBody.Detail,
			); err != nil {
				logger.L().Warnw("failed to record human intervention on resume",
					"episode_id", episodeID, "execution_id", id, "error", err)
			}
		}
	}

	// Build the set of completed node results so we can resume from where we stopped.
	completedNodes := make(map[string]models.NodeResult)
	for _, res := range exec.Results {
		if res.Status == "success" {
			completedNodes[res.NodeID] = res
		}
	}

	dag, err := s.dags.Get(c.Request.Context(), exec.DAGID)
	if errors.Is(err, store.ErrNotFound) {
		// DAG may have been created inline (not persisted); recreate from state metadata
		writeError(c, http.StatusUnprocessableEntity, "dag_not_found", "original DAG not available for resume", nil)
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "dag_get_failed", "failed to get DAG", err.Error())
		return
	}

	// Mark as running again and re-run remaining nodes
	exec.Status = models.StatusRunning
	exec.EndedAt = nil
	if err := s.execs.Update(c.Request.Context(), exec); err != nil {
		writeError(c, http.StatusInternalServerError, "execution_update_failed", "failed to update execution", err.Error())
		return
	}

	ctx := context.Background()
	go func() {
		result := s.scheduler.Execute(ctx, dag, exec.State, completedNodes)
		now := time.Now()

		exec, err := s.execs.Get(ctx, id)
		if err != nil {
			logger.L().Errorw("failed to load resumed execution", "execution_id", id, "error", err)
			return
		}
		exec.Duration = result.Duration
		// Merge resumed results
		for _, r := range result.Results {
			found := false
			for i, existing := range exec.Results {
				if existing.NodeID == r.NodeID {
					exec.Results[i] = r
					found = true
					break
				}
			}
			if !found {
				exec.Results = append(exec.Results, r)
			}
		}
		exec.State = result.State
		if result.Err != nil || result.Status == models.StatusFailed {
			exec.Status = models.StatusFailed
			if result.Err != nil {
				exec.Error = result.Err.Error()
			}
			exec.EndedAt = &now
		} else if result.Status == models.StatusSuspended {
			exec.Status = models.StatusSuspended
		} else {
			exec.Status = models.StatusCompleted
			exec.EndedAt = &now
		}
		if err := s.execs.Update(ctx, exec); err != nil {
			logger.L().Errorw("failed to update resumed execution", "execution_id", id, "error", err)
		}
		if err := s.execs.SaveNodeResults(ctx, id, exec.Results); err != nil {
			logger.L().Errorw("failed to save resumed node results", "execution_id", id, "error", err)
		}

		// Auto-close Episode on terminal state (mirrors startExecution logic).
		if episodeID := result.State.GetString("__episode_id__"); episodeID != "" {
			if ep, epErr := s.episodes.Get(ctx, episodeID); epErr == nil {
				if ep.Status == models.EpisodeStatusInProgress {
					switch exec.Status {
					case models.StatusCompleted:
						ep.Status = models.EpisodeStatusConverged
					case models.StatusFailed:
						ep.Status = models.EpisodeStatusFailed
					}
					ep.UpdatedAt = time.Now().UTC()
					if err := s.episodes.Update(ctx, ep); err != nil {
						logger.L().Warnw("failed to auto-close resumed episode", "episode_id", episodeID, "exec_status", exec.Status, "error", err)
					}
				}
			}
		}
		if exec.Status == models.StatusSuspended {
			checkpoint := &models.ExecutionCheckpoint{
				ExecutionID: exec.ID,
				DAGID:       exec.DAGID,
				State:       result.State.Snapshot(),
				LoopCounts:  result.State.LoopCountsSnapshot(),
				UpdatedAt:   now,
			}
			if err := s.execs.SaveCheckpoint(ctx, checkpoint); err != nil {
				logger.L().Errorw("failed to persist resumed checkpoint", "execution_id", id, "error", err)
			}
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"execution_id": id,
		"status":       models.StatusRunning,
	})
}

// handleListAudit returns the in-memory audit trail (admin only).
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
func (s *Server) handleListEpisodes(c *gin.Context) {
	execID := c.Param("id")
	ctx := c.Request.Context()
	if c.Query("view") == "summary" {
		summaries, err := s.episodes.ListEpisodeSummariesByExecution(ctx, execID)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "episode_list_error", "failed to list episode summaries", err.Error())
			return
		}
		if summaries == nil {
			summaries = []models.EpisodeSummaryView{}
		}
		c.JSON(http.StatusOK, gin.H{"episodes": summaries})
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
	c.JSON(http.StatusOK, gin.H{"episodes": episodes})
}

// handleGetEpisode returns a single episode by ID.
//
//	GET /api/v1/episodes/:id
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
func (s *Server) handleGetExecutionSummary(c *gin.Context) {
	id := c.Param("id")
	summary, err := s.execs.GetExecutionSummary(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
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
func (s *Server) handleGetTriggerContext(c *gin.Context) {
	execID := c.Param("id")
	ctx := c.Request.Context()
	exec, err := s.execs.Get(ctx, execID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "trigger_context_error", "failed to get execution", err.Error())
		return
	}
	episodes, err := s.episodes.ListByExecution(ctx, execID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "trigger_context_error", "failed to list episodes", err.Error())
		return
	}
	view := buildTriggerContextView(exec, episodes)
	c.JSON(http.StatusOK, view)
}

// handleGetReviewState returns the aggregate human-review state for an execution.
//
//	GET /api/v1/executions/:id/review-state
func (s *Server) handleGetReviewState(c *gin.Context) {
	execID := c.Param("id")
	state, err := s.episodes.GetReviewStateByExecution(c.Request.Context(), execID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "review_state_error", "failed to get review state", err.Error())
		return
	}
	c.JSON(http.StatusOK, state)
}

// handlePostReviewAction records a human review decision on an execution.
//
//	POST /api/v1/executions/:id/review-actions
func (s *Server) handlePostReviewAction(c *gin.Context) {
	execID := c.Param("id")
	var req models.ReviewActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid request body", err.Error())
		return
	}
	if err := s.episodeWriter.WriteReviewState(c.Request.Context(), execID, req); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "review_action_error", "failed to write review state", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleGetEpisodeReplay returns a replay slice view for a single episode.
//
//	GET /api/v1/executions/:id/episodes/:episode_id/replay?percent=N
func (s *Server) handleGetEpisodeReplay(c *gin.Context) {
	episodeID := c.Param("episode_id")
	ctx := c.Request.Context()
	percent := 100
	fmt.Sscanf(c.DefaultQuery("percent", "100"), "%d", &percent)
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	ep, err := s.episodes.Get(ctx, episodeID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "episode not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "replay_error", "failed to get episode", err.Error())
		return
	}
	trace, _ := s.episodes.ListProcessTraceByEpisode(ctx, episodeID)
	view := buildReplaySliceView(ep, trace, percent)
	c.JSON(http.StatusOK, view)
}

// handleGetEpisodeDossier returns the full dossier for a single episode.
//
//	GET /api/v1/executions/:id/episodes/:episode_id/dossier
func (s *Server) handleGetEpisodeDossier(c *gin.Context) {
	episodeID := c.Param("episode_id")
	ctx := c.Request.Context()
	ep, err := s.episodes.Get(ctx, episodeID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "episode not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "dossier_error", "failed to get episode", err.Error())
		return
	}
	facts, _ := s.episodes.ListRuntimeFactsByEpisode(ctx, episodeID)
	// CR-011: use the same Experience-backed helper as the /memory-recalls
	// endpoint so the dossier and the inset strip always show identical data.
	// Memory recall failure is treated as a soft error: the dossier is still
	// returned with an empty memory_recalls list so the rest of the workspace
	// remains functional; the error is logged for operator visibility.
	recalls, recallErr := buildMemoryRecallsForEpisode(ctx, ep, s.memory)
	if recallErr != nil {
		logger.L().Warnw("memory recall search failed; dossier served with empty recalls",
			"episode_id", episodeID, "error", recallErr)
	}
	dossier := buildEpisodeDossier(ep, facts, recalls)
	c.JSON(http.StatusOK, dossier)
}

// handleGetEpisodeMemoryRecalls returns memory recall items for a single episode.
// CR-010: uses s.memory.Search() to perform real Experience retrieval instead
// of the former stub which always returned an empty slice.
//
//	GET /api/v1/executions/:id/episodes/:episode_id/memory-recalls
func (s *Server) handleGetEpisodeMemoryRecalls(c *gin.Context) {
	episodeID := c.Param("episode_id")
	ctx := c.Request.Context()
	ep, err := s.episodes.Get(ctx, episodeID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "episode not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "memory_recall_error", "failed to get episode", err.Error())
		return
	}
	recalls, err := buildMemoryRecallsForEpisode(ctx, ep, s.memory)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "memory_recall_error", "failed to search memory recalls", err.Error())
		return
	}
	c.JSON(http.StatusOK, models.MemoryRecallListView{
		Items:              recalls,
		ImplementationNote: "keyword_overlap",
	})
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
func buildMemoryRecallsForEpisode(ctx context.Context, ep *models.Episode, expStore memory.ExperienceStore) ([]models.MemoryRecallView, error) {
	if expStore == nil || ep == nil {
		return []models.MemoryRecallView{}, nil
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
		return []models.MemoryRecallView{}, err
	}
	if len(experiences) == 0 {
		return []models.MemoryRecallView{}, nil
	}

	recalls := make([]models.MemoryRecallView, 0, len(experiences))
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

		recalls = append(recalls, models.MemoryRecallView{
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
func (s *Server) handleGetComparisonTarget(c *gin.Context) {
	execID := c.Param("id")
	historicalID := c.Param("historical_id")
	ctx := c.Request.Context()
	current, err := s.execs.Get(ctx, execID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "comparison_error", "failed to get current execution", err.Error())
		return
	}
	historical, err := s.execs.Get(ctx, historicalID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "historical execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "comparison_error", "failed to get historical execution", err.Error())
		return
	}
	summary := buildComparisonSummary(current, historical)
	c.JSON(http.StatusOK, summary)
}

// ---------------------------------------------------------------------------
// Execution Workspace view-building helpers
// ---------------------------------------------------------------------------

// buildTriggerContextView constructs a TriggerContextView from execution + episode data.
func buildTriggerContextView(exec *models.Execution, episodes []*models.Episode) models.TriggerContextView {
	view := models.TriggerContextView{
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
		section := models.TriggerContextSectionView{
			Title: "Alert",
			Fields: []models.TriggerContextFieldView{
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
			icSection := models.TriggerContextSectionView{
				Title: "Investigation",
				Fields: []models.TriggerContextFieldView{
					{Label: "Hypothesis", Value: ic.Hypothesis},
				},
			}
			if len(ic.KnownSignals) > 0 {
				icSection.Fields = append(icSection.Fields, models.TriggerContextFieldView{
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
func buildReplaySliceView(ep *models.Episode, trace []models.ProcessTraceEntryView, percent int) models.ReplaySliceView {
	visible := make([]models.ProcessTraceEntryView, 0, len(trace))
	visibleFactIDs := make([]string, 0)
	for _, entry := range trace {
		if entry.Range[0] <= percent {
			visible = append(visible, entry)
			visibleFactIDs = append(visibleFactIDs, entry.ID)
		}
	}
	// Derive checkpoint narrative from the last visible trace entry.
	checkpoint := models.ReplayCheckpointView{
		Label:    fmt.Sprintf("%d%%", percent),
		Headline: "Execution in progress",
	}
	if len(visible) > 0 {
		last := visible[len(visible)-1]
		checkpoint.Headline = last.Title
		checkpoint.Detail = last.Detail
	}
	return models.ReplaySliceView{
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
func buildEpisodeDossier(ep *models.Episode, facts []models.RuntimeFactView, recalls []models.MemoryRecallView) models.EpisodeDossierView {
	display := models.DossierDisplayView{}
	if ep.Verdict != nil {
		display.Verdict = string(ep.Verdict.Result)
		display.VerdictLabel = verdictLabelFromResult(ep.Verdict.Result)
		display.Summary = ep.Verdict.Conclusion
	}
	applyHumanReviewToDossierDisplay(ep, &display)

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
	var expectedBehaviors []models.ExpectedBehaviorView
	if ep.Verdict != nil {
		for i, link := range ep.Verdict.CausalChain {
			expectedBehaviors = append(expectedBehaviors, models.ExpectedBehaviorView{
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
	var verdictBridge []models.VerdictBridgeItemView
	if ep.Verdict != nil {
		for i, rec := range ep.Verdict.Recommendations {
			verdictBridge = append(verdictBridge, models.VerdictBridgeItemView{
				ID:       fmt.Sprintf("rec_%d", i),
				Title:    fmt.Sprintf("Recommendation %d", i+1),
				Body:     rec,
				FocusKey: commonFocusKey,
			})
		}
	}
	return models.EpisodeDossierView{
		Episode: models.DossierEpisodeRefView{
			EpisodeID: ep.ID,
			Label:     string(ep.EpisodeType),
		},
		Display:          display,
		ExpectedBehavior: expectedBehaviors,
		VerdictBridge:    verdictBridge,
		RuntimeFacts:     facts,
		Handles:          ep.Handles,
		MemoryRecalls:    recalls,
		HumanAuditTrail:  ep.HumanInterventions,
	}
}

// applyHumanReviewToDossierDisplay mirrors the human-review display projection
// used by episode summaries so the drawer and the summary list present the same
// post-review semantics.
func applyHumanReviewToDossierDisplay(ep *models.Episode, display *models.DossierDisplayView) {
	if ep == nil || display == nil {
		return
	}
	bannerSet := false
	for _, hi := range ep.HumanInterventions {
		switch hi.Action {
		case models.HumanActionStateOverride, models.HumanActionHypothesisCorrected:
			if !bannerSet {
				msg := fmt.Sprintf("Human override: %s", hi.Detail)
				display.Banner = &msg
				bannerSet = true
			}
			display.VerdictLabel = "Overridden (Human)"
		case models.HumanActionResumed:
			display.VerdictLabel = "Approved"
		case models.HumanActionAborted:
			display.VerdictLabel = "Aborted"
		}
	}
}

// buildComparisonSummary compares two executions and returns a summary view.
func buildComparisonSummary(current, historical *models.Execution) models.ComparisonSummaryView {
	summary := models.ComparisonSummaryView{
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

// verdictLabelFromResult maps an EpisodeResult to a human-readable display label.
// This is a thin wrapper that avoids importing the store package into API handlers.
func verdictLabelFromResult(r models.EpisodeResult) string {
	switch r {
	case models.EpisodeResultPass:
		return "Pass"
	case models.EpisodeResultFail:
		return "Fail"
	case models.EpisodeResultInconclusive:
		return "Inconclusive"
	default:
		return strings.Title(string(r))
	}
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
