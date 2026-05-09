package store

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/Trin9/SynapseFlow/backend/internal/audit"
	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
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
func (s *MemoryExecutionStore) GetExecutionSummary(ctx context.Context, execID string) (*models.ExecutionSummaryView, error) {
	exec, err := s.Get(ctx, execID)
	if err != nil {
		return nil, err
	}
	return projectExecutionToSummary(exec), nil
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

func (s *MemoryEpisodeStore) ListEpisodeSummariesByExecution(ctx context.Context, execID string) ([]models.EpisodeSummaryView, error) {
	eps, err := s.ListByExecution(ctx, execID)
	if err != nil {
		return nil, err
	}
	out := make([]models.EpisodeSummaryView, len(eps))
	for i, ep := range eps {
		out[i] = projectEpisodeToSummary(ep)
	}
	return out, nil
}

func (s *MemoryEpisodeStore) ListProcessTraceByEpisode(ctx context.Context, episodeID string) ([]models.ProcessTraceEntryView, error) {
	ep, err := s.Get(ctx, episodeID)
	if err != nil {
		return nil, err
	}
	return projectEpisodeToProcessTrace(ep), nil
}

func (s *MemoryEpisodeStore) ListRuntimeFactsByEpisode(ctx context.Context, episodeID string) ([]models.RuntimeFactView, error) {
	ep, err := s.Get(ctx, episodeID)
	if err != nil {
		return nil, err
	}
	return projectEpisodeToRuntimeFacts(ep), nil
}

func (s *MemoryEpisodeStore) GetReviewStateByExecution(ctx context.Context, execID string) (*models.ReviewStateView, error) {
	eps, err := s.ListByExecution(ctx, execID)
	if err != nil {
		return nil, err
	}
	return projectEpisodesToReviewState(eps), nil
}

// ---------------------------------------------------------------------------
// Shared projection helpers (package-level, used by both memory and postgres)
// ---------------------------------------------------------------------------

// projectExecutionToSummary converts an Execution to an ExecutionSummaryView.
func projectExecutionToSummary(exec *models.Execution) *models.ExecutionSummaryView {
	label := exec.DAGName
	if len(exec.ID) >= 8 {
		label = fmt.Sprintf("%s #%s", exec.DAGName, exec.ID[:8])
	}
	return &models.ExecutionSummaryView{
		ExecutionID:  exec.ID,
		DAGID:        exec.DAGID,
		DAGName:      exec.DAGName,
		Status:       exec.Status,
		StartedAt:    exec.StartedAt,
		EndedAt:      exec.EndedAt,
		DurationMs:   exec.Duration.Milliseconds(),
		Mode:         "execution",
		WorkflowKind: "investigation",
		Display: models.ExecutionDisplayView{
			RunLabel:   label,
			TraceTitle: exec.DAGName,
		},
	}
}

// projectEpisodeToSummary converts an Episode to an EpisodeSummaryView.
func projectEpisodeToSummary(ep *models.Episode) models.EpisodeSummaryView {
	sv := models.EpisodeSummaryView{
		EpisodeID:     ep.ID,
		Label:         string(ep.EpisodeType),
		Status:        ep.Status,
		EvidenceCount: len(ep.Evidence),
		HandleCount:   len(ep.Handles),
	}
	switch ep.Status {
	case models.EpisodeStatusConverged, models.EpisodeStatusEscalated, models.EpisodeStatusFailed:
		sv.DefaultReplayPercent = 100
	case models.EpisodeStatusInProgress:
		sv.DefaultReplayPercent = 50
	}
	if ep.Verdict != nil {
		sv.Confidence = ep.Verdict.Confidence
		sv.Display = models.EpisodeDisplayView{
			Verdict:      string(ep.Verdict.Result),
			VerdictLabel: domainEpisode.VerdictLabelFromResult(ep.Verdict.Result),
			Summary:      truncateStr(ep.Verdict.Conclusion, 120),
		}
	}
	tmpDisplay := models.DossierDisplayView{Banner: sv.Display.Banner, VerdictLabel: sv.Display.VerdictLabel}
	domainEpisode.ApplyHumanReviewDisplay(ep, &tmpDisplay)
	sv.Display.Banner = tmpDisplay.Banner
	sv.Display.VerdictLabel = tmpDisplay.VerdictLabel
	return sv
}

// projectEpisodeToProcessTrace derives a process-trace timeline from an Episode.
func projectEpisodeToProcessTrace(ep *models.Episode) []models.ProcessTraceEntryView {
	total := len(ep.Evidence) + len(ep.HumanInterventions)
	if ep.Verdict != nil {
		total++
	}
	entries := make([]models.ProcessTraceEntryView, 0, total)
	roundN := 0
	for i, ev := range ep.Evidence {
		roundN++
		stage := fmt.Sprintf("Round %d", roundN)
		if i == 0 && ep.EpisodeType == models.EpisodeTypeActionVerification {
			stage = "Action"
		}
		startPct, endPct := rangeForIndex(i, len(ep.Evidence))
		title := ev.Label
		if title == "" {
			title = fmt.Sprintf("Evidence #%d", i+1)
		}
		entry := models.ProcessTraceEntryView{
			ID:     ev.ID,
			Stage:  stage,
			Title:  title,
			Detail: truncateStr(ev.Content, 200),
			Status: "success",
			Range:  [2]int{startPct, endPct},
		}
		if string(ev.NodeType) != "" {
			entry.Chips = []string{string(ev.NodeType)}
		}
		entries = append(entries, entry)
	}
	// Append human intervention entries.
	for _, hi := range ep.HumanInterventions {
		entries = append(entries, models.ProcessTraceEntryView{
			ID:     hi.NodeID + "_human",
			Stage:  "Human Review",
			Title:  domainEpisode.HumanActionLabel(hi.Action),
			Detail: truncateStr(hi.Detail, 200),
			Status: "success",
			Range:  [2]int{100, 100},
		})
	}
	// Append verdict entry.
	if ep.Verdict != nil {
		status := string(ep.Verdict.Result)
		if status == "" {
			status = "pending"
		}
		entries = append(entries, models.ProcessTraceEntryView{
			ID:     ep.ID + "_verdict",
			Stage:  "Verdict",
			Title:  domainEpisode.VerdictLabelFromResult(ep.Verdict.Result),
			Detail: truncateStr(ep.Verdict.Conclusion, 200),
			Status: status,
			Range:  [2]int{100, 100},
		})
	}
	// Append circuit-breaker entry if loop guard was maxed out.
	if ep.LoopGuard.MaxIterations > 0 && ep.LoopGuard.CurrentIteration >= ep.LoopGuard.MaxIterations {
		entries = append(entries, models.ProcessTraceEntryView{
			ID:     ep.ID + "_circuit_breaker",
			Stage:  "Circuit Breaker",
			Title:  "Max iterations reached",
			Status: "failed",
			Range:  [2]int{100, 100},
		})
	}
	return entries
}

// projectEpisodeToRuntimeFacts projects Evidence entries to RuntimeFactView.
func projectEpisodeToRuntimeFacts(ep *models.Episode) []models.RuntimeFactView {
	// Build a nodeID → focus_key map from the episode's handles.
	// Each handle records the Source (node ID) that extracted it; we use the
	// first matching handle per node to avoid ambiguous linkage.
	handleBySource := make(map[string]string, len(ep.Handles))
	for _, h := range ep.Handles {
		if _, exists := handleBySource[h.Source]; !exists {
			handleBySource[h.Source] = string(h.Type) + ":" + h.Value
		}
	}

	out := make([]models.RuntimeFactView, 0, len(ep.Evidence))
	for _, ev := range ep.Evidence {
		title := ev.Label
		if title == "" {
			title = fmt.Sprintf("Evidence (%s)", ev.Type)
		}
		fact := models.RuntimeFactView{
			ID:         ev.ID,
			Title:      title,
			Summary:    truncateStr(ev.Content, 200),
			FocusKey:   handleBySource[ev.NodeID], // empty string → omitempty → not sent
			SourceType: nodeTypeToSourceType(ev.NodeType),
			Collector:  string(ev.NodeType) + ":" + ev.NodeID,
			Content:    ev.Content,
			ContentRef: ev.ContentRef,
		}
		out = append(out, fact)
	}
	return out
}

// projectEpisodesToReviewState derives a ReviewStateView from HumanInterventions
// across all episodes. Returns {Status: "pending"} when there are none.
func projectEpisodesToReviewState(episodes []*models.Episode) *models.ReviewStateView {
	state := &models.ReviewStateView{Status: "pending"}
	for _, ep := range episodes {
		for _, hi := range ep.HumanInterventions {
			if hi.Action == models.HumanActionSuspended {
				// Suspension event: execution is now pending review.
				// Only update the action label and note; do NOT overwrite actor/acted_at
				// or change the status — the execution has not yet been reviewed.
				state.ActionLabel = domainEpisode.HumanActionLabel(hi.Action)
				state.Note = hi.Detail
				continue
			}
			t := hi.Timestamp
			state.Actor = hi.Actor
			state.ActedAt = &t
			state.ActionLabel = domainEpisode.HumanActionLabel(hi.Action)
			state.Note = hi.Detail
			state.Status = domainEpisode.ReviewStateFromAction(hi.Action)
		}
	}
	return state
}

// ---------------------------------------------------------------------------
// Projection helper utilities
// ---------------------------------------------------------------------------

// truncateStr truncates s to at most max bytes (UTF-8 safe enough for display).
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// Back off to a valid rune boundary.
	for max > 0 && !isRuneStart(s[max]) {
		max--
	}
	return s[:max] + "…"
}

func isRuneStart(b byte) bool {
	return b&0xC0 != 0x80
}

// rangeForIndex returns [start%, end%] for item i in a list of n items.
func rangeForIndex(i, n int) (int, int) {
	if n <= 0 {
		return 0, 100
	}
	return i * 100 / n, (i + 1) * 100 / n
}

// nodeTypeToSourceType maps a NodeType to the RuntimeFact source_type field.
func nodeTypeToSourceType(nt models.NodeType) string {
	switch nt {
	case models.NodeTypeScript:
		return "log"
	case models.NodeTypeLLM:
		return "text"
	case models.NodeTypeMCP:
		return "json"
	default:
		return "text"
	}
}
