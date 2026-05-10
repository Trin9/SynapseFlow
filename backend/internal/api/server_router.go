package api

import (
	"net/http"

	"github.com/Trin9/SynapseFlow/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

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

	// Token issuance (public - authentication happens inside the handler)
	r.POST("/api/v1/auth/token", s.handleIssueToken)

	// Webhook endpoint: authenticated via API key (machine-to-machine)
	r.POST("/api/v1/webhook/alert", s.authMiddleware(), s.handleWebhookAlert)

	// API v1 - all routes require authentication
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

		// Episodes
		v1.GET("/executions/:id/episodes", s.handleListEpisodes)
		v1.GET("/episodes/:id", s.handleGetEpisode)

		// Execution workspace
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
