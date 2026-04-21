package store

import (
	"context"
	"sort"
	"sync"

	"github.com/Trin9/SynapseFlow/backend/internal/audit"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

type MemoryDAGStore struct {
	mu   sync.RWMutex
	dags map[string]*models.DAGConfig
}

func NewMemoryDAGStore() *MemoryDAGStore {
	return &MemoryDAGStore{dags: make(map[string]*models.DAGConfig)}
}

func (s *MemoryDAGStore) Create(_ context.Context, dag *models.DAGConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dags[dag.ID] = cloneDAG(dag)
	return nil
}

func (s *MemoryDAGStore) Update(_ context.Context, dag *models.DAGConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.dags[dag.ID]; !ok {
		return ErrNotFound
	}
	s.dags[dag.ID] = cloneDAG(dag)
	return nil
}

func (s *MemoryDAGStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.dags[id]; !ok {
		return ErrNotFound
	}
	delete(s.dags, id)
	return nil
}

func (s *MemoryDAGStore) Get(_ context.Context, id string) (*models.DAGConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dag, ok := s.dags[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneDAG(dag), nil
}

func (s *MemoryDAGStore) List(_ context.Context) ([]*models.DAGConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.DAGConfig, 0, len(s.dags))
	for _, dag := range s.dags {
		out = append(out, cloneDAG(dag))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

type MemoryExecutionStore struct {
	mu          sync.RWMutex
	executions  map[string]*models.Execution
	results     map[string][]models.NodeResult
	checkpoints map[string]*models.ExecutionCheckpoint
}

func NewMemoryExecutionStore() *MemoryExecutionStore {
	return &MemoryExecutionStore{
		executions:  make(map[string]*models.Execution),
		results:     make(map[string][]models.NodeResult),
		checkpoints: make(map[string]*models.ExecutionCheckpoint),
	}
}

func (s *MemoryExecutionStore) Create(_ context.Context, exec *models.Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executions[exec.ID] = cloneExecution(exec)
	return nil
}

func (s *MemoryExecutionStore) Update(_ context.Context, exec *models.Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.executions[exec.ID]; !ok {
		return ErrNotFound
	}
	s.executions[exec.ID] = cloneExecution(exec)
	return nil
}

func (s *MemoryExecutionStore) Get(_ context.Context, id string) (*models.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	exec, ok := s.executions[id]
	if !ok {
		return nil, ErrNotFound
	}
	clone := cloneExecution(exec)
	clone.Results = append([]models.NodeResult(nil), s.results[id]...)
	return clone, nil
}

func (s *MemoryExecutionStore) List(_ context.Context) ([]*models.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.Execution, 0, len(s.executions))
	for id, exec := range s.executions {
		clone := cloneExecution(exec)
		clone.Results = append([]models.NodeResult(nil), s.results[id]...)
		out = append(out, clone)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out, nil
}

func (s *MemoryExecutionStore) SaveNodeResults(_ context.Context, executionID string, results []models.NodeResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[executionID] = append([]models.NodeResult(nil), results...)
	if exec, ok := s.executions[executionID]; ok {
		exec.Results = append([]models.NodeResult(nil), results...)
	}
	return nil
}

func (s *MemoryExecutionStore) ListNodeResults(_ context.Context, executionID string) ([]models.NodeResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	results, ok := s.results[executionID]
	if !ok {
		if _, exists := s.executions[executionID]; !exists {
			return nil, ErrNotFound
		}
		return []models.NodeResult{}, nil
	}
	return append([]models.NodeResult(nil), results...), nil
}

func (s *MemoryExecutionStore) SaveCheckpoint(_ context.Context, checkpoint *models.ExecutionCheckpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *checkpoint
	clone.State = cloneMap(checkpoint.State)
	clone.LoopCounts = cloneIntMap(checkpoint.LoopCounts)
	s.checkpoints[checkpoint.ExecutionID] = &clone
	return nil
}

func (s *MemoryExecutionStore) GetCheckpoint(_ context.Context, executionID string) (*models.ExecutionCheckpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	checkpoint, ok := s.checkpoints[executionID]
	if !ok {
		return nil, ErrNotFound
	}
	clone := *checkpoint
	clone.State = cloneMap(checkpoint.State)
	clone.LoopCounts = cloneIntMap(checkpoint.LoopCounts)
	return &clone, nil
}

type MemoryAuditStore struct {
	mu      sync.RWMutex
	entries []audit.Entry
}

func NewMemoryAuditStore() *MemoryAuditStore {
	return &MemoryAuditStore{entries: make([]audit.Entry, 0, 32)}
}

func (s *MemoryAuditStore) Record(_ context.Context, entry audit.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	return nil
}

func (s *MemoryAuditStore) List(_ context.Context) ([]audit.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]audit.Entry, len(s.entries))
	copy(out, s.entries)
	return out, nil
}

func cloneDAG(dag *models.DAGConfig) *models.DAGConfig {
	if dag == nil {
		return nil
	}
	clone := *dag
	if dag.Metadata != nil {
		clone.Metadata = make(map[string]string, len(dag.Metadata))
		for k, v := range dag.Metadata {
			clone.Metadata[k] = v
		}
	}
	clone.Nodes = append([]models.Node(nil), dag.Nodes...)
	clone.Edges = append([]models.Edge(nil), dag.Edges...)
	return &clone
}

func cloneExecution(exec *models.Execution) *models.Execution {
	if exec == nil {
		return nil
	}
	clone := *exec
	clone.Results = append([]models.NodeResult(nil), exec.Results...)
	clone.State = exec.State.Clone()
	return &clone
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
