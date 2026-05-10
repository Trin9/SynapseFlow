package system

import (
	"context"

	"github.com/Trin9/SynapseFlow/backend/internal/mcp"
)

// ProbeStatus represents one dependency probe result.
type ProbeStatus struct {
	MCP string
	DB  string
}

// Service orchestrates system-level probe use-cases.
type Service struct {
	MCP interface {
		ListTools(ctx context.Context) ([]mcp.ToolInfo, error)
	}
	DB interface {
		PingContext(ctx context.Context) error
	}
}

// ProbeDependencies returns current MCP/DB health status.
func (s *Service) ProbeDependencies(ctx context.Context) ProbeStatus {
	status := ProbeStatus{MCP: "ok", DB: "disabled"}
	if s.MCP != nil {
		if _, err := s.MCP.ListTools(ctx); err != nil {
			status.MCP = "degraded: " + err.Error()
		}
	}
	if s.DB != nil {
		if err := s.DB.PingContext(ctx); err != nil {
			status.DB = "degraded: " + err.Error()
		} else {
			status.DB = "ok"
		}
	}
	return status
}
