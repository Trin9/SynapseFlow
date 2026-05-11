package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleListTools returns all available MCP tools.
// @Summary List Tools
// @Description Returns all available MCP tools.
// @Tags Tools
// @Produce json
// @Success 200 {array} object "MCP tool list"
// @Failure 500 {object} dto.APIError
// @Router /api/v1/tools [get]
func (s *Server) handleListTools(c *gin.Context) {
	tools, err := s.mcpMgr.ListTools(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "tools_error", "failed to list tools", err.Error())
		return
	}
	c.JSON(http.StatusOK, tools)
}
