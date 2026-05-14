package execution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
	domainExecution "github.com/Trin9/SynapseFlow/backend/internal/domain/execution"
	"github.com/Trin9/SynapseFlow/backend/internal/engine"
	"github.com/Trin9/SynapseFlow/backend/internal/memory"
	"github.com/Trin9/SynapseFlow/backend/internal/metrics"
	"github.com/Trin9/SynapseFlow/backend/internal/notify"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/logger"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
	"github.com/google/uuid"
)

// Service orchestrates execution lifecycle use-cases.
type Service struct {
	Scheduler *engine.Scheduler

	DAGs          store.DAGStore
	Executions    store.ExecutionStore
	Episodes      store.EpisodeStore
	EpisodeWriter *engine.EpisodeWriter

	MetricsCollector *metrics.Collector
	Notifier         notify.Sender
	Extractor        *memory.Extractor
	GetNotifier      func() notify.Sender

	ResolveSlackURL            func(*models.DAGConfig) string
	BuildExecutionNotification func(*models.Execution, *models.DAGConfig, time.Duration) string
}

var (
	ErrExecutionNotFound    = errors.New("execution not found")
	ErrExecutionGet         = errors.New("failed to get execution")
	ErrExecutionList        = errors.New("failed to list executions")
	ErrCheckpointGet        = errors.New("failed to load checkpoint")
	ErrDAGNotFoundForResume = errors.New("original DAG not available for resume")
	ErrDAGGet               = errors.New("failed to get DAG")
	ErrExecutionUpdate      = errors.New("failed to update execution")
)

// NotSuspendedError indicates resume was requested for a non-suspended execution.
type NotSuspendedError struct {
	Status models.ExecutionStatus
}

func (e NotSuspendedError) Error() string {
	return "execution is not suspended"
}

// ResumeInput is the optional request context for resume.
type ResumeInput struct {
	ExecutionID string
	Actor       string
	Action      models.HumanInterventionAction
	Detail      string
}

// ListInput controls optional execution listing filters.
type ListInput struct {
	DAGID  string
	Status models.ExecutionStatus
	Limit  int
	Offset int
}

// GetExecution returns one execution by ID.
func (s *Service) GetExecution(ctx context.Context, executionID string) (*models.Execution, error) {
	exec, err := s.Executions.Get(ctx, executionID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrExecutionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExecutionGet, err)
	}
	return exec, nil
}

// ListExecutions returns execution list with optional dag/status filters.
func (s *Service) ListExecutions(ctx context.Context, input ListInput) ([]*models.Execution, error) {
	var (
		list []*models.Execution
		err  error
	)
	if input.DAGID != "" {
		list, err = s.Executions.ListByDAGID(ctx, input.DAGID, input.Limit, input.Offset)
	} else if input.Status != "" {
		list, err = s.Executions.ListByStatus(ctx, input.Status)
	} else {
		list, err = s.Executions.List(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExecutionList, err)
	}
	if list == nil {
		list = []*models.Execution{}
	}
	return list, nil
}

// RunWorkflow validates and starts one execution run.
func (s *Service) RunWorkflow(dag *models.DAGConfig, initialState *models.GlobalState, source string) (*models.Execution, error) {
	if _, err := engine.ParseDAG(dag); err != nil {
		return nil, err
	}
	return s.StartExecution(dag, initialState, source), nil
}

// StartExecution creates and asynchronously advances an execution.
func (s *Service) StartExecution(dag *models.DAGConfig, initialState *models.GlobalState, source string) *models.Execution {
	execID := generateID()
	if initialState == nil {
		initialState = models.NewGlobalState()
	}
	exec := &models.Execution{
		ID:        execID,
		DAGID:     dag.ID,
		DAGName:   dag.Name,
		Status:    models.StatusRunning,
		StartedAt: time.Now(),
	}
	if err := s.Executions.Create(context.Background(), exec); err != nil {
		logger.L().Errorw("failed to create execution", "execution_id", execID, "source", source, "error", err)
	}

	if s.Episodes != nil {
		defaultType := inferEpisodeType(dag)
		createdEpisodeIDs := make([]string, 0, len(dag.Episodes)+1)
		primaryEpisodeID := ""

		for _, spec := range dag.Episodes {
			ep := newEpisodeFromDesignSpec(execID, spec, defaultType)
			if err := s.Episodes.Create(context.Background(), ep); err != nil {
				logger.L().Warnw("failed to auto-create design episode", "exec_id", execID, "design_episode_id", spec.ID, "error", err)
				continue
			}
			createdEpisodeIDs = append(createdEpisodeIDs, ep.ID)
			if primaryEpisodeID == "" {
				primaryEpisodeID = ep.ID
			}
		}

		if primaryEpisodeID == "" {
			ep := newDefaultEpisode(execID, defaultType)
			if err := s.Episodes.Create(context.Background(), ep); err != nil {
				logger.L().Warnw("failed to auto-create default episode", "exec_id", execID, "error", err)
			} else {
				createdEpisodeIDs = append(createdEpisodeIDs, ep.ID)
				primaryEpisodeID = ep.ID
			}
		}

		if primaryEpisodeID != "" {
			initialState.Set("__episode_id__", primaryEpisodeID)
		}
		if len(createdEpisodeIDs) > 0 {
			initialState.Set("__episode_ids__", createdEpisodeIDs)
		}
		logger.L().Infow("auto-created episodes", "exec_id", execID, "count", len(createdEpisodeIDs), "primary_episode_id", primaryEpisodeID)
	}

	ctx := context.Background()
	go func(execID string, dag *models.DAGConfig) {
		result := s.Scheduler.Execute(ctx, dag, initialState, nil)
		now := time.Now()

		exec, err := s.Executions.Get(ctx, execID)
		if err != nil {
			logger.L().Errorw("failed to load execution for update", "execution_id", execID, "error", err)
			return
		}

		exec.Duration = result.Duration
		exec.Results = result.Results
		exec.State = result.State

		exec.Status = domainExecution.ResolveTerminalStatus(result.Status, result.Err)
		if exec.Status == models.StatusFailed {
			if result.Err != nil {
				exec.Error = result.Err.Error()
			}
			exec.EndedAt = &now
		} else if exec.Status == models.StatusCompleted {
			exec.EndedAt = &now
		}
		if err := s.Executions.Update(ctx, exec); err != nil {
			logger.L().Errorw("failed to persist execution update", "execution_id", execID, "error", err)
		}
		if err := s.Executions.SaveNodeResults(ctx, execID, result.Results); err != nil {
			logger.L().Errorw("failed to persist node results", "execution_id", execID, "error", err)
		}

		if s.Episodes != nil {
			if episodeID := result.State.GetString("__episode_id__"); episodeID != "" {
				if ep, epErr := s.Episodes.Get(ctx, episodeID); epErr == nil {
					if domainEpisode.ApplyExecutionTerminalStatus(ep, exec.Status, time.Now()) {
						if err := s.Episodes.Update(ctx, ep); err != nil {
							logger.L().Warnw("failed to auto-close episode", "episode_id", episodeID, "exec_status", exec.Status, "error", err)
						}
					}
				}
			}
		}

		if exec.Status == models.StatusSuspended {
			checkpoint := &models.ExecutionCheckpoint{
				ExecutionID: exec.ID,
				DAGID:       exec.DAGID,
				State:       result.State.Snapshot(),
				LoopCounts:  result.State.LoopCountsSnapshot(),
				UpdatedAt:   now,
			}
			if err := s.Executions.SaveCheckpoint(ctx, checkpoint); err != nil {
				logger.L().Errorw("failed to persist checkpoint", "execution_id", execID, "error", err)
			}
		}

		if s.MetricsCollector != nil {
			s.MetricsCollector.RecordExecution(exec.Status, result.Duration)
			for _, r := range result.Results {
				s.MetricsCollector.RecordNode(r.NodeType, r.Duration)
				if r.TokensIn+r.TokensOut > 0 {
					s.MetricsCollector.RecordLLMTokens(string(r.NodeType), r.TokensIn+r.TokensOut)
				}
			}
		}

		if exec.Status == models.StatusCompleted && s.Extractor != nil {
			go func(execSnapshot *models.Execution, dagSnapshot *models.DAGConfig) {
				if _, err := s.Extractor.Extract(context.Background(), dagSnapshot, execSnapshot); err != nil {
					logger.L().Warnw("Memory extraction failed", "execution_id", execSnapshot.ID, "error", err)
				}
			}(cloneExecution(exec), dag)
		}

		notifier := s.Notifier
		if s.GetNotifier != nil {
			notifier = s.GetNotifier()
		}
		if notifier != nil && s.ResolveSlackURL != nil && s.BuildExecutionNotification != nil {
			notifierURL := s.ResolveSlackURL(dag)
			if notifierURL != "" {
				msg := s.BuildExecutionNotification(exec, dag, result.Duration)
				go func() {
					if err := notifier.SendExecutionResult(context.Background(), notifierURL, msg); err != nil {
						logger.L().Warnw("Slack notification failed", "execution_id", execID, "error", err)
					}
				}()
			}
		}
	}(execID, dag)

	return exec
}

func inferEpisodeType(dag *models.DAGConfig) domainEpisode.EpisodeType {
	if dag != nil && dag.Metadata != nil {
		if epType := dag.Metadata["episode_type"]; epType != "" {
			domainType := domainEpisode.EpisodeType(epType)
			if domainType.IsValid() {
				return domainType
			}
			logger.L().Warnw("invalid episode_type metadata; fallback to inferred type", "episode_type", epType, "dag_id", dag.ID)
		}
		if workflowKind := strings.TrimSpace(strings.ToLower(dag.Metadata["workflow_kind"])); workflowKind == "investigation" {
			return domainEpisode.EpisodeTypeInvestigationStep
		}
	}

	haystack := ""
	if dag != nil {
		haystack = strings.ToLower(dag.ID + " " + dag.Name)
	}
	if strings.Contains(haystack, "investigation") || strings.Contains(haystack, "incident") || strings.Contains(haystack, "unreachable") {
		return domainEpisode.EpisodeTypeInvestigationStep
	}

	return domainEpisode.EpisodeTypeActionVerification
}

func inferEpisodeTypeFromDesignSpec(spec models.DesignEpisode, fallback domainEpisode.EpisodeType) domainEpisode.EpisodeType {
	if spec.Config != nil {
		if raw, ok := spec.Config["episode_type"]; ok {
			if s, ok := raw.(string); ok {
				candidate := domainEpisode.EpisodeType(strings.TrimSpace(s))
				if candidate.IsValid() {
					return candidate
				}
			}
		}
	}
	return fallback
}

func newDefaultEpisode(execID string, episodeType domainEpisode.EpisodeType) *models.Episode {
	now := time.Now().UTC()
	return &models.Episode{
		ID:            generateID(),
		ExecID:        execID,
		EpisodeType:   episodeType.ToModel(),
		Status:        domainEpisode.EpisodeStatusPending.ToModel(),
		Trigger:       &models.EpisodeTrigger{Type: domainEpisode.EpisodeTriggerManual.ToModel()},
		LoopGuard:     models.EpisodeLoopGuard{MaxIterations: 10},
		SchemaVersion: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func newEpisodeFromDesignSpec(execID string, spec models.DesignEpisode, fallbackType domainEpisode.EpisodeType) *models.Episode {
	episodeType := inferEpisodeTypeFromDesignSpec(spec, fallbackType)
	ep := newDefaultEpisode(execID, episodeType)

	summary := strings.TrimSpace(spec.Summary)
	label := strings.TrimSpace(spec.Label)
	if episodeType == domainEpisode.EpisodeTypeInvestigationStep {
		ep.InvestigationContext = &models.InvestigationContext{
			Hypothesis:    firstNonEmpty(summary, label),
			KnownSignals:  append([]string(nil), spec.ExpectedBehaviors...),
			RetrievalPlan: "",
		}
	} else {
		ep.ActionContext = &models.ActionContext{
			ActionName: firstNonEmpty(label, spec.ID, string(episodeType)),
			ActionType: "design_episode",
			ActionInput: map[string]interface{}{
				"design_episode_id":  spec.ID,
				"summary":            summary,
				"expected_behaviors": append([]string(nil), spec.ExpectedBehaviors...),
				"node_ids":           append([]string(nil), spec.NodeIDs...),
			},
		}
	}

	return ep
}

func firstNonEmpty(candidates ...string) string {
	for _, c := range candidates {
		if strings.TrimSpace(c) != "" {
			return c
		}
	}
	return ""
}

// ResumeExecution resumes a suspended execution and returns the updated running execution.
func (s *Service) ResumeExecution(ctx context.Context, input ResumeInput) (*models.Execution, error) {
	exec, err := s.Executions.Get(ctx, input.ExecutionID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrExecutionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExecutionGet, err)
	}
	if exec.Status != models.StatusSuspended {
		return nil, NotSuspendedError{Status: exec.Status}
	}

	checkpoint, err := s.Executions.GetCheckpoint(ctx, input.ExecutionID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("%w: %v", ErrCheckpointGet, err)
	}
	if checkpoint != nil {
		exec.State = models.NewGlobalStateFromSnapshot(checkpoint.State, checkpoint.LoopCounts)
	}

	if s.EpisodeWriter != nil && exec.State != nil {
		if episodeID := exec.State.GetString("__episode_id__"); episodeID != "" {
			actor := input.Actor
			if actor == "" {
				actor = "operator"
			}
			action := input.Action
			if action == "" {
				action = domainEpisode.HumanActionResumed.ToModel()
			}
			if err := s.EpisodeWriter.AppendHumanIntervention(
				ctx, episodeID, "", actor, action, input.Detail,
			); err != nil {
				logger.L().Warnw("failed to record human intervention on resume",
					"episode_id", episodeID, "execution_id", input.ExecutionID, "error", err)
			}
		}
	}

	completedNodes := make(map[string]models.NodeResult)
	for _, res := range exec.Results {
		if models.NodeResultStatus(res.Status) == models.NodeResultSuccess {
			completedNodes[res.NodeID] = res
		}
	}

	dag, err := s.DAGs.Get(ctx, exec.DAGID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrDAGNotFoundForResume
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDAGGet, err)
	}

	exec.Status = models.StatusRunning
	exec.EndedAt = nil
	if err := s.Executions.Update(ctx, exec); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExecutionUpdate, err)
	}

	go s.runResumedExecution(context.Background(), input.ExecutionID, dag, exec.State, completedNodes)
	return exec, nil
}

func (s *Service) runResumedExecution(ctx context.Context, executionID string, dag *models.DAGConfig, state *models.GlobalState, completedNodes map[string]models.NodeResult) {
	result := s.Scheduler.Execute(ctx, dag, state, completedNodes)
	now := time.Now()

	exec, err := s.Executions.Get(ctx, executionID)
	if err != nil {
		logger.L().Errorw("failed to load resumed execution", "execution_id", executionID, "error", err)
		return
	}
	exec.Duration = result.Duration
	for _, r := range result.Results {
		found := false
		for i, existing := range exec.Results {
			if existing.NodeID == r.NodeID {
				exec.Results[i] = r
				found = true
				break
			}
		}
		if !found {
			exec.Results = append(exec.Results, r)
		}
	}
	exec.State = result.State
	exec.Status = domainExecution.ResolveTerminalStatus(result.Status, result.Err)
	if exec.Status == models.StatusFailed {
		if result.Err != nil {
			exec.Error = result.Err.Error()
		}
		exec.EndedAt = &now
	} else if exec.Status == models.StatusCompleted {
		exec.EndedAt = &now
	}
	if err := s.Executions.Update(ctx, exec); err != nil {
		logger.L().Errorw("failed to update resumed execution", "execution_id", executionID, "error", err)
	}
	if err := s.Executions.SaveNodeResults(ctx, executionID, exec.Results); err != nil {
		logger.L().Errorw("failed to save resumed node results", "execution_id", executionID, "error", err)
	}

	if s.Episodes != nil {
		if episodeID := result.State.GetString("__episode_id__"); episodeID != "" {
			if ep, epErr := s.Episodes.Get(ctx, episodeID); epErr == nil {
				if domainEpisode.ApplyExecutionTerminalStatus(ep, exec.Status, time.Now()) {
					if err := s.Episodes.Update(ctx, ep); err != nil {
						logger.L().Warnw("failed to auto-close resumed episode", "episode_id", episodeID, "exec_status", exec.Status, "error", err)
					}
				}
			}
		}
	}
	if exec.Status == models.StatusSuspended {
		checkpoint := &models.ExecutionCheckpoint{
			ExecutionID: exec.ID,
			DAGID:       exec.DAGID,
			State:       result.State.Snapshot(),
			LoopCounts:  result.State.LoopCountsSnapshot(),
			UpdatedAt:   now,
		}
		if err := s.Executions.SaveCheckpoint(ctx, checkpoint); err != nil {
			logger.L().Errorw("failed to persist resumed checkpoint", "execution_id", executionID, "error", err)
		}
	}
}

func generateID() string {
	return uuid.New().String()
}

func cloneExecution(exec *models.Execution) *models.Execution {
	if exec == nil {
		return nil
	}
	clone := *exec
	if exec.Results != nil {
		clone.Results = append([]models.NodeResult(nil), exec.Results...)
	}
	if exec.State != nil {
		clone.State = exec.State.Clone()
	}
	return &clone
}
