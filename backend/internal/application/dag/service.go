package dag

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// Service orchestrates DAG CRUD and alert matching use-cases.
type Service struct {
	DAGs store.DAGStore
}

var (
	ErrDAGNotFound = errors.New("dag not found")
	ErrDAGGet      = errors.New("failed to get dag")
	ErrDAGList     = errors.New("failed to list dags")
	ErrDAGCreate   = errors.New("failed to create dag")
	ErrDAGUpdate   = errors.New("failed to update dag")
	ErrDAGDelete   = errors.New("failed to delete dag")
)

// CreateDAG creates a DAG and applies timestamps.
func (s *Service) CreateDAG(ctx context.Context, dag *models.DAGConfig) error {
	now := time.Now()
	if dag.CreatedAt.IsZero() {
		dag.CreatedAt = now
	}
	dag.UpdatedAt = now
	if err := s.DAGs.Create(ctx, dag); err != nil {
		return fmt.Errorf("%w: %v", ErrDAGCreate, err)
	}
	return nil
}

// ListDAGs returns all DAGs.
func (s *Service) ListDAGs(ctx context.Context) ([]*models.DAGConfig, error) {
	list, err := s.DAGs.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDAGList, err)
	}
	return list, nil
}

// GetDAG returns a DAG by ID.
func (s *Service) GetDAG(ctx context.Context, dagID string) (*models.DAGConfig, error) {
	dag, err := s.DAGs.Get(ctx, dagID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrDAGNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDAGGet, err)
	}
	return dag, nil
}

// UpdateDAG updates an existing DAG by ID.
func (s *Service) UpdateDAG(ctx context.Context, dagID string, dag *models.DAGConfig) error {
	if _, err := s.GetDAG(ctx, dagID); err != nil {
		if errors.Is(err, ErrDAGNotFound) {
			return ErrDAGNotFound
		}
		return err
	}
	dag.ID = dagID
	dag.UpdatedAt = time.Now()
	if err := s.DAGs.Update(ctx, dag); err != nil {
		return fmt.Errorf("%w: %v", ErrDAGUpdate, err)
	}
	return nil
}

// DeleteDAG deletes a DAG by ID.
func (s *Service) DeleteDAG(ctx context.Context, dagID string) error {
	if err := s.DAGs.Delete(ctx, dagID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrDAGNotFound
		}
		return fmt.Errorf("%w: %v", ErrDAGDelete, err)
	}
	return nil
}

// MatchDAGForAlert returns a matching DAG for alert labels.
func (s *Service) MatchDAGForAlert(ctx context.Context, labels map[string]string) (*models.DAGConfig, bool) {
	if dagID := strings.TrimSpace(labels["dag_id"]); dagID != "" {
		dag, err := s.DAGs.Get(ctx, dagID)
		return dag, err == nil
	}

	dags, err := s.DAGs.List(ctx)
	if err != nil {
		return nil, false
	}
	service := strings.TrimSpace(labels["service"])
	alertName := strings.TrimSpace(labels["alertname"])
	for _, dag := range dags {
		if dag == nil {
			continue
		}
		if dag.Metadata["service"] == service && dag.Metadata["alertname"] == alertName {
			return dag, true
		}
	}
	return nil, false
}
