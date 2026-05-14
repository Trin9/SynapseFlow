package store

import (
	"context"
	"sort"
	"sync"

	workspaceView "github.com/Trin9/SynapseFlow/backend/internal/application/workspace/view"
	"github.com/Trin9/SynapseFlow/backend/internal/audit"
	projectionWorkspace "github.com/Trin9/SynapseFlow/backend/internal/projection/workspace"
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

// ListByDAGID returns executions for a specific DAG, newest first.
// limit ≤ 0 means no limit; offset 0 means from the start.
func (s *MemoryExecutionStore) ListByDAGID(_ context.Context, dagID string, limit, offset int) ([]*models.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*models.Execution
	for id, exec := range s.executions {
		if exec.DAGID == dagID {
			clone := cloneExecution(exec)
			clone.Results = append([]models.NodeResult(nil), s.results[id]...)
			out = append(out, clone)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	if offset > 0 {
		if offset >= len(out) {
			return []*models.Execution{}, nil
		}
		out = out[offset:]
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ListByStatus returns executions matching the given status, newest first.
func (s *MemoryExecutionStore) ListByStatus(_ context.Context, status models.ExecutionStatus) ([]*models.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*models.Execution
	for id, exec := range s.executions {
		if exec.Status == status {
			clone := cloneExecution(exec)
			clone.Results = append([]models.NodeResult(nil), s.results[id]...)
			out = append(out, clone)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out, nil
}

// GetExecutionSummary builds a high-level summary view of a single execution.
func (s *MemoryExecutionStore) GetExecutionSummary(ctx context.Context, execID string) (*workspaceView.ExecutionSummaryView, error) {
	exec, err := s.Get(ctx, execID)
	if err != nil {
		return nil, err
	}
	return projectionWorkspace.ExecutionToSummary(exec), nil
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
	clone.Episodes = append([]models.DesignEpisode(nil), dag.Episodes...)
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

// ---------------------------------------------------------------------------
// MemoryEpisodeStore (Sprint 7 – used in tests and no-DB mode)
// ---------------------------------------------------------------------------

type MemoryEpisodeStore struct {
	mu        sync.RWMutex
	episodes  map[string]*models.Episode
	artifacts map[string][]*models.EpisodeArtifact // keyed by episodeID
}

func NewMemoryEpisodeStore() *MemoryEpisodeStore {
	return &MemoryEpisodeStore{
		episodes:  make(map[string]*models.Episode),
		artifacts: make(map[string][]*models.EpisodeArtifact),
	}
}

func (s *MemoryEpisodeStore) Create(_ context.Context, ep *models.Episode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := cloneEpisode(ep)
	s.episodes[ep.ID] = clone
	return nil
}

func (s *MemoryEpisodeStore) Update(_ context.Context, ep *models.Episode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.episodes[ep.ID]; !ok {
		return ErrNotFound
	}
	s.episodes[ep.ID] = cloneEpisode(ep)
	return nil
}

func (s *MemoryEpisodeStore) Get(_ context.Context, id string) (*models.Episode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ep, ok := s.episodes[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneEpisode(ep), nil
}

func (s *MemoryEpisodeStore) ListByExecution(_ context.Context, execID string) ([]*models.Episode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*models.Episode
	for _, ep := range s.episodes {
		if ep.ExecID == execID {
			out = append(out, cloneEpisode(ep))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *MemoryEpisodeStore) SaveArtifact(_ context.Context, artifact *models.EpisodeArtifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.artifacts[artifact.EpisodeID]
	for i, a := range list {
		if a.ID == artifact.ID {
			list[i] = cloneArtifact(artifact)
			s.artifacts[artifact.EpisodeID] = list
			return nil
		}
	}
	s.artifacts[artifact.EpisodeID] = append(list, cloneArtifact(artifact))
	return nil
}

func (s *MemoryEpisodeStore) ListArtifacts(_ context.Context, episodeID string) ([]*models.EpisodeArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := s.artifacts[episodeID]
	out := make([]*models.EpisodeArtifact, len(list))
	for i, a := range list {
		out[i] = cloneArtifact(a)
	}
	return out, nil
}

func cloneEpisode(ep *models.Episode) *models.Episode {
	if ep == nil {
		return nil
	}
	clone := *ep // shallow copy of scalar fields

	// Deep-copy Handles slice (was map[string]string in earlier versions).
	clone.Handles = append([]models.EpisodeHandle(nil), ep.Handles...)

	// Deep-copy Evidence slice.
	clone.Evidence = append([]models.EpisodeEvidence(nil), ep.Evidence...)

	// Deep-copy Verdict (and its own slices).
	if ep.Verdict != nil {
		v := *ep.Verdict
		v.CausalChain = append([]string(nil), ep.Verdict.CausalChain...)
		v.Gaps = append([]string(nil), ep.Verdict.Gaps...)
		v.Recommendations = append([]string(nil), ep.Verdict.Recommendations...)
		clone.Verdict = &v
	}

	// Deep-copy LoopGuard.
	loopGuard := ep.LoopGuard
	loopGuard.AttemptedActions = append([]string(nil), ep.LoopGuard.AttemptedActions...)
	clone.LoopGuard = loopGuard

	// Deep-copy audit / intervention slices.
	clone.AuditTrail = append([]models.EpisodeAuditEntry(nil), ep.AuditTrail...)
	clone.HumanInterventions = append([]models.HumanIntervention(nil), ep.HumanInterventions...)

	// Deep-copy pointer fields (Trigger, ActionContext, InvestigationContext,
	// MemoryExtraction, ConcludedAt) — these are small structs, so copying the
	// pointed-to value is sufficient.
	if ep.Trigger != nil {
		t := *ep.Trigger
		if ep.Trigger.Payload != nil {
			t.Payload = make(map[string]interface{}, len(ep.Trigger.Payload))
			for k, v := range ep.Trigger.Payload {
				t.Payload[k] = v
			}
		}
		clone.Trigger = &t
	}
	if ep.ActionContext != nil {
		ac := *ep.ActionContext
		clone.ActionContext = &ac
	}
	if ep.InvestigationContext != nil {
		ic := *ep.InvestigationContext
		ic.KnownSignals = append([]string(nil), ep.InvestigationContext.KnownSignals...)
		clone.InvestigationContext = &ic
	}
	if ep.MemoryExtraction != nil {
		me := *ep.MemoryExtraction
		clone.MemoryExtraction = &me
	}
	if ep.ConcludedAt != nil {
		t := *ep.ConcludedAt
		clone.ConcludedAt = &t
	}

	return &clone
}

func cloneArtifact(a *models.EpisodeArtifact) *models.EpisodeArtifact {
	if a == nil {
		return nil
	}
	clone := *a
	return &clone
}

// ---------------------------------------------------------------------------
// MemoryEpisodeStore – view projection methods (M1.2)
// ---------------------------------------------------------------------------

func (s *MemoryEpisodeStore) ListEpisodeSummariesByExecution(ctx context.Context, execID string) ([]workspaceView.EpisodeSummaryView, error) {
	eps, err := s.ListByExecution(ctx, execID)
	if err != nil {
		return nil, err
	}
	out := make([]workspaceView.EpisodeSummaryView, len(eps))
	for i, ep := range eps {
		out[i] = projectionWorkspace.EpisodeToSummary(ep)
	}
	return out, nil
}

func (s *MemoryEpisodeStore) ListProcessTraceByEpisode(ctx context.Context, episodeID string) ([]workspaceView.ProcessTraceEntryView, error) {
	ep, err := s.Get(ctx, episodeID)
	if err != nil {
		return nil, err
	}
	return projectionWorkspace.EpisodeToProcessTrace(ep), nil
}

func (s *MemoryEpisodeStore) ListRuntimeFactsByEpisode(ctx context.Context, episodeID string) ([]workspaceView.RuntimeFactView, error) {
	ep, err := s.Get(ctx, episodeID)
	if err != nil {
		return nil, err
	}
	return projectionWorkspace.EpisodeToRuntimeFacts(ep), nil
}
