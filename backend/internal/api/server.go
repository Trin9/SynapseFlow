package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xunchenzheng/synapse/internal/engine"
	"github.com/xunchenzheng/synapse/internal/llm"
	"github.com/xunchenzheng/synapse/internal/mcp"
	"github.com/xunchenzheng/synapse/pkg/logger"
	"github.com/xunchenzheng/synapse/pkg/models"
)

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

// Server holds the HTTP server state including in-memory stores.
type Server struct {
	router    *gin.Engine
	httpSrv   *http.Server
	scheduler *engine.Scheduler
	mcpMgr    mcp.ToolCaller

	// In-memory stores (will be replaced with PostgreSQL in M2)
	dagsMu sync.RWMutex
	dags   map[string]*models.DAGConfig

	execsMu    sync.RWMutex
	executions map[string]*models.Execution
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

	s := &Server{
		dags:       make(map[string]*models.DAGConfig),
		executions: make(map[string]*models.Execution),
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

	s.scheduler = engine.NewScheduler(executors)

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
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Health check
	r.GET("/health", s.handleHealth)

	// API v1
	v1 := r.Group("/api/v1")
	{
		// Tool discovery
		v1.GET("/tools", s.handleListTools)

		// DAG management
		v1.POST("/dags", s.validateDAGMiddleware(), s.handleCreateDAG)
		v1.GET("/dags", s.handleListDAGs)
		v1.GET("/dags/:id", s.handleGetDAG)
		v1.PUT("/dags/:id", s.validateDAGMiddleware(), s.handleUpdateDAG)
		v1.DELETE("/dags/:id", s.handleDeleteDAG)

		// Execution
		v1.POST("/dags/:id/run", s.handleRunDAG)
		v1.POST("/run", s.validateDAGMiddleware(), s.handleRunInline) // run a DAG without saving it first
		v1.GET("/executions/:id", s.handleGetExecution)
		v1.GET("/executions/:id/nodes", s.handleGetExecutionNodes)
		v1.GET("/executions", s.handleListExecutions)
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
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "synapse",
		"version": "0.1.0",
	})
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

	s.dagsMu.Lock()
	s.dags[dag.ID] = dag
	s.dagsMu.Unlock()

	c.JSON(http.StatusCreated, dag)
}

func (s *Server) handleListDAGs(c *gin.Context) {
	s.dagsMu.RLock()
	defer s.dagsMu.RUnlock()

	list := make([]*models.DAGConfig, 0, len(s.dags))
	for _, d := range s.dags {
		list = append(list, d)
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) handleGetDAG(c *gin.Context) {
	id := c.Param("id")

	s.dagsMu.RLock()
	dag, ok := s.dags[id]
	s.dagsMu.RUnlock()

	if !ok {
		writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
		return
	}
	c.JSON(http.StatusOK, dag)
}

func (s *Server) handleUpdateDAG(c *gin.Context) {
	id := c.Param("id")

	s.dagsMu.RLock()
	_, ok := s.dags[id]
	s.dagsMu.RUnlock()

	if !ok {
		writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
		return
	}

	dag, ok := getValidatedDAG(c)
	if !ok {
		return
	}

	dag.ID = id
	dag.UpdatedAt = time.Now()

	s.dagsMu.Lock()
	s.dags[id] = dag
	s.dagsMu.Unlock()

	c.JSON(http.StatusOK, dag)
}

func (s *Server) handleDeleteDAG(c *gin.Context) {
	id := c.Param("id")

	s.dagsMu.Lock()
	defer s.dagsMu.Unlock()

	if _, ok := s.dags[id]; !ok {
		writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
		return
	}

	delete(s.dags, id)
	c.JSON(http.StatusOK, gin.H{"message": "DAG deleted"})
}

// --- Execution ---

func (s *Server) handleRunDAG(c *gin.Context) {
	id := c.Param("id")

	s.dagsMu.RLock()
	dag, ok := s.dags[id]
	s.dagsMu.RUnlock()

	if !ok {
		writeError(c, http.StatusNotFound, "not_found", "DAG not found", nil)
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

	execID := generateID()
	exec := &models.Execution{
		ID:        execID,
		DAGID:     dag.ID,
		DAGName:   dag.Name,
		Status:    models.StatusRunning,
		StartedAt: time.Now(),
	}

	s.execsMu.Lock()
	s.executions[execID] = exec
	s.execsMu.Unlock()

	// Execute asynchronously. Frontend polls /executions/:id/nodes.
	// Do not inherit the request context for async execution, otherwise the
	// background run will be canceled as soon as the HTTP request completes.
	ctx := context.Background()
	go func(execID string, dag *models.DAGConfig) {
		result := s.scheduler.Execute(ctx, dag)
		now := time.Now()

		s.execsMu.Lock()
		defer s.execsMu.Unlock()
		exec, ok := s.executions[execID]
		if !ok {
			return
		}
		exec.EndedAt = &now
		exec.Duration = result.Duration
		exec.Results = result.Results
		if result.Err != nil {
			exec.Status = models.StatusFailed
			exec.Error = result.Err.Error()
		} else {
			exec.Status = models.StatusCompleted
		}
		s.executions[execID] = exec
	}(execID, dag)

	c.JSON(http.StatusAccepted, gin.H{
		"execution_id": execID,
		"status":       exec.Status,
	})
}

func (s *Server) handleGetExecution(c *gin.Context) {
	id := c.Param("id")

	s.execsMu.RLock()
	exec, ok := s.executions[id]
	s.execsMu.RUnlock()

	if !ok {
		writeError(c, http.StatusNotFound, "not_found", "Execution not found", nil)
		return
	}

	c.JSON(http.StatusOK, exec)
}

func (s *Server) handleListExecutions(c *gin.Context) {
	s.execsMu.RLock()
	defer s.execsMu.RUnlock()

	list := make([]*models.Execution, 0, len(s.executions))
	for _, e := range s.executions {
		list = append(list, e)
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) handleGetExecutionNodes(c *gin.Context) {
	id := c.Param("id")

	s.execsMu.RLock()
	exec, ok := s.executions[id]
	s.execsMu.RUnlock()

	if !ok {
		writeError(c, http.StatusNotFound, "not_found", "Execution not found", nil)
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
