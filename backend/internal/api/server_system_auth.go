package api

import (
	"net/http"
	"time"

	"github.com/Trin9/SynapseFlow/backend/internal/api/dto"
	"github.com/Trin9/SynapseFlow/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

// handleHealth returns service and dependency health status.
// @Summary Health Check
// @Description Returns service and dependency health status.
// @Tags System
// @Produce json
// @Success 200 {object} dto.HealthResponse
// @Router /health [get]
func (s *Server) handleHealth(c *gin.Context) {
	mcpStatus := "unknown"
	dbStatus := "unknown"
	if s.systemSvc != nil {
		status := s.systemSvc.ProbeDependencies(c.Request.Context())
		mcpStatus = status.MCP
		dbStatus = status.DB
	}
	c.JSON(http.StatusOK, dto.HealthResponse{
		Status:  "ok",
		Service: "synapse",
		Version: "0.1.0",
		Deps: dto.HealthDepsResponse{
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
// @Success 200 {object} dto.LiveResponse
// @Router /health/live [get]
func (s *Server) handleLive(c *gin.Context) {
	c.JSON(http.StatusOK, dto.LiveResponse{Status: "ok"})
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
// @Param request body dto.IssueTokenRequest true "Issue token request"
// @Success 200 {object} dto.IssueTokenResponse
// @Failure 400 {object} dto.APIError
// @Failure 401 {object} dto.APIError
// @Failure 500 {object} dto.APIError
// @Failure 501 {object} dto.APIError
// @Router /api/v1/auth/token [post]
func (s *Server) handleIssueToken(c *gin.Context) {
	if len(s.jwtSecret) == 0 {
		writeError(c, http.StatusNotImplemented, "jwt_not_configured",
			"JWT signing is not configured (set SYNAPSE_JWT_SECRET)", nil)
		return
	}
	var req dto.IssueTokenRequest
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
	c.JSON(http.StatusOK, dto.IssueTokenResponse{
		Token:     token,
		ExpiresIn: int(ttl.Seconds()),
		Role:      string(id.Role),
		Subject:   id.Subject,
	})
}

// handleMetrics returns Prometheus metrics output.
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
