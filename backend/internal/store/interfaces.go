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
	// ListByDAGID returns executions for a specific DAG, newest first.
	// limit ≤ 0 means no limit; offset 0 means from the start.
	ListByDAGID(ctx context.Context, dagID string, limit, offset int) ([]*models.Execution, error)
	// ListByStatus returns executions matching the given status, newest first.
	ListByStatus(ctx context.Context, status models.ExecutionStatus) ([]*models.Execution, error)
	// GetExecutionSummary builds a high-level summary view of a single execution.
	GetExecutionSummary(ctx context.Context, execID string) (*models.ExecutionSummaryView, error)
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

	// --- View projection methods (M1.2) ---
	// Data sources are projections of existing Episode fields; no new tables.

	// ListEpisodeSummariesByExecution returns lightweight summary cards for all
	// episodes of a given execution, ordered by created_at ascending.
	ListEpisodeSummariesByExecution(ctx context.Context, execID string) ([]models.EpisodeSummaryView, error)
	// ListProcessTraceByEpisode returns the process-trace timeline for a single
	// episode, derived from its Evidence and HumanInterventions.
	ListProcessTraceByEpisode(ctx context.Context, episodeID string) ([]models.ProcessTraceEntryView, error)
	// ListRuntimeFactsByEpisode returns runtime facts (evidence projections) for
	// a single episode.
	ListRuntimeFactsByEpisode(ctx context.Context, episodeID string) ([]models.RuntimeFactView, error)
	// GetReviewStateByExecution returns the aggregate human-review state for an
	// execution, derived from HumanInterventions across its episodes.
	GetReviewStateByExecution(ctx context.Context, execID string) (*models.ReviewStateView, error)
}

type SearchQuery struct {
	Text        string
	AlertType   string
	ServiceName string
	TopK        int
}
