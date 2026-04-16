package store

import (
	"context"

	"github.com/xunchenzheng/synapse/internal/audit"
	"github.com/xunchenzheng/synapse/pkg/models"
)

type DAGStore interface {
	Create(context.Context, *models.DAGConfig) error
	Update(context.Context, *models.DAGConfig) error
	Delete(context.Context, string) error
	Get(context.Context, string) (*models.DAGConfig, error)
	List(context.Context) ([]*models.DAGConfig, error)
}

type ExecutionStore interface {
	Create(context.Context, *models.Execution) error
	Update(context.Context, *models.Execution) error
	Get(context.Context, string) (*models.Execution, error)
	List(context.Context) ([]*models.Execution, error)
	SaveNodeResults(context.Context, string, []models.NodeResult) error
	ListNodeResults(context.Context, string) ([]models.NodeResult, error)
	SaveCheckpoint(context.Context, *models.ExecutionCheckpoint) error
	GetCheckpoint(context.Context, string) (*models.ExecutionCheckpoint, error)
}

type AuditStore interface {
	Record(context.Context, audit.Entry) error
	List(context.Context) ([]audit.Entry, error)
}

type ExperienceStore interface {
	Save(context.Context, *models.Experience) error
	List(context.Context) ([]models.Experience, error)
	Search(context.Context, SearchQuery) ([]models.Experience, error)
}

type SearchQuery struct {
	Text        string
	AlertType   string
	ServiceName string
	TopK        int
}
