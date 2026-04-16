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

	"github.com/gin-gonic/gin"
	"github.com/xunchenzheng/synapse/internal/auth"
	"github.com/xunchenzheng/synapse/internal/config"
	"github.com/xunchenzheng/synapse/internal/engine"
	"github.com/xunchenzheng/synapse/internal/llm"
	"github.com/xunchenzheng/synapse/internal/mcp"
	"github.com/xunchenzheng/synapse/internal/memory"
	"github.com/xunchenzheng/synapse/internal/metrics"
	"github.com/xunchenzheng/synapse/internal/notify"
	"github.com/xunchenzheng/synapse/internal/store"
	"github.com/xunchenzheng/synapse/pkg/logger"
	"github.com/xunchenzheng/synapse/pkg/models"
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
	dags    store.DAGStore
	execs   store.ExecutionStore
	audits  store.AuditStore
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
	executors := map[models.NodeType]engine.NodeExecutor{
		models.NodeTypeScript: &engine.ScriptExecutor{},
		models.NodeTypeHuman:  &engine.HumanExecutor{},
		models.NodeTypeRouter: &engine.RouterExecutor{},
		models.NodeTypeMCP:    &engine.MCPExecutor{MCP: s.mcpMgr},
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

		executors[models.NodeTypeLLM] = &engine.LLMExecutor{Client: client}
	} else {
		executors[models.NodeTypeLLM] = &engine.MockLLMExecutor{}
		log.Infow("Using mock LLM executor (set LLM_API_KEY for real LLM)")
	}

	s.extractor = &memory.Extractor{Store: s.memory}
	retriever := &memory.Retriever{Store: s.memory}
	s.scheduler = engine.NewScheduler(executors, retriever)

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
	r.GET("/metrics", s.handleMetrics)

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

		if result.Err != nil {
			exec.Status = models.StatusFailed
			exec.Error = result.Err.Error()
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
	list, err := s.execs.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "execution_list_failed", "failed to list executions", err.Error())
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
	return fmt.Sprintf("%d", time.Now().UnixNano())
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
		if result.Err != nil {
			exec.Status = models.StatusFailed
			exec.Error = result.Err.Error()
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
