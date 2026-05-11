package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	appExecution "github.com/Trin9/SynapseFlow/backend/internal/application/execution"
	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
	"github.com/Trin9/SynapseFlow/backend/pkg/logger"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
	"github.com/gin-gonic/gin"
)

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
	exec, err := s.execService.GetExecution(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, appExecution.ErrExecutionNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "Execution not found", nil)
			return
		}
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
	input := appExecution.ListInput{}

	if dagID := c.Query("dag_id"); dagID != "" {
		input.DAGID = dagID
		limitStr := c.DefaultQuery("limit", "0")
		offsetStr := c.DefaultQuery("offset", "0")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			writeError(c, http.StatusBadRequest, "invalid_request", "invalid limit", limitStr)
			return
		}
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			writeError(c, http.StatusBadRequest, "invalid_request", "invalid offset", offsetStr)
			return
		}
		input.Limit = limit
		input.Offset = offset
	}
	if statusStr := c.Query("status"); statusStr != "" {
		status := models.ExecutionStatus(statusStr)
		if !status.IsValid() {
			writeError(c, http.StatusBadRequest, "invalid_request", "invalid execution status", statusStr)
			return
		}
		input.Status = status
	}
	list, err := s.execService.ListExecutions(ctx, input)
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
	exec, err := s.execService.GetExecution(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, appExecution.ErrExecutionNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "Execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "execution_get_failed", "failed to get execution", err.Error())
		return
	}

	results := exec.Results
	if results == nil {
		results = make([]models.NodeResult, 0)
	}

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
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&resumeBody); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "invalid request body", err.Error())
			return
		}
	}
	action := domainEpisode.HumanInterventionAction(resumeBody.Action)
	if action != "" && !action.IsResumeAction() {
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid resume action", resumeBody.Action)
		return
	}

	exec, err := s.execService.ResumeExecution(c.Request.Context(), appExecution.ResumeInput{
		ExecutionID: id,
		Actor:       resumeBody.Actor,
		Action:      action.ToModel(),
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
