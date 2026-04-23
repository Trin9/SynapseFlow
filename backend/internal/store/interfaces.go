package store

import (
	"context"

	"github.com/Trin9/SynapseFlow/backend/internal/audit"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
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

// EpisodeStore persists Episode product objects (Sprint 7).
type EpisodeStore interface {
	// Create inserts a new Episode record.
	Create(ctx context.Context, ep *models.Episode) error
	// Update replaces the mutable fields of an existing Episode.
	Update(ctx context.Context, ep *models.Episode) error
	// Get returns a single Episode by ID including all evidence and verdict.
	Get(ctx context.Context, id string) (*models.Episode, error)
	// ListByExecution returns all Episodes for a given execution, ordered by
	// created_at ascending.
	ListByExecution(ctx context.Context, execID string) ([]*models.Episode, error)
	// SaveArtifact upserts a large-payload artifact linked to an Episode.
	SaveArtifact(ctx context.Context, artifact *models.EpisodeArtifact) error
	// ListArtifacts returns all artifacts for an Episode.
	ListArtifacts(ctx context.Context, episodeID string) ([]*models.EpisodeArtifact, error)
}

type SearchQuery struct {
	Text        string
	AlertType   string
	ServiceName string
	TopK        int
}
