package api

import (
	"errors"
	"net/http"

	"github.com/Trin9/SynapseFlow/backend/internal/api/dto"
	appDAG "github.com/Trin9/SynapseFlow/backend/internal/application/dag"
	"github.com/gin-gonic/gin"
)

// handleCreateDAG creates a new DAG configuration.
// @Summary Create DAG
// @Description Creates a new DAG configuration.
// @Tags DAG
// @Accept json
// @Produce json
// @Param request body models.DAGConfig true "DAG configuration"
// @Success 201 {object} object "Created DAG"
// @Failure 400 {object} dto.APIError
// @Failure 500 {object} dto.APIError
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

// handleListDAGs returns all DAG configurations.
// @Summary List DAGs
// @Description Returns all DAG configurations.
// @Tags DAG
// @Produce json
// @Success 200 {array} object "DAG list"
// @Failure 500 {object} dto.APIError
// @Router /api/v1/dags [get]
func (s *Server) handleListDAGs(c *gin.Context) {
	list, err := s.dagService.ListDAGs(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "dag_list_failed", "failed to list DAGs", err.Error())
		return
	}
	c.JSON(http.StatusOK, list)
}

// handleGetDAG returns a DAG configuration by ID.
// @Summary Get DAG
// @Description Returns a DAG configuration by ID.
// @Tags DAG
// @Produce json
// @Param id path string true "DAG ID"
// @Success 200 {object} object "DAG"
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
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

// handleUpdateDAG updates an existing DAG configuration.
// @Summary Update DAG
// @Description Updates an existing DAG configuration.
// @Tags DAG
// @Accept json
// @Produce json
// @Param id path string true "DAG ID"
// @Param request body models.DAGConfig true "DAG configuration"
// @Success 200 {object} object "Updated DAG"
// @Failure 400 {object} dto.APIError
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
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

// handleDeleteDAG deletes a DAG configuration by ID.
// @Summary Delete DAG
// @Description Deletes a DAG configuration by ID.
// @Tags DAG
// @Produce json
// @Param id path string true "DAG ID"
// @Success 200 {object} dto.DeleteDAGResponse
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
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
	c.JSON(http.StatusOK, dto.DeleteDAGResponse{Message: "DAG deleted"})
}

// handleRunDAG starts execution for a saved DAG.
// @Summary Run DAG
// @Description Starts execution for a saved DAG.
// @Tags Execution
// @Produce json
// @Param id path string true "DAG ID"
// @Success 202 {object} dto.RunExecutionResponse
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
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
