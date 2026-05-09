package workspace

import (
	"context"
	"errors"
	"fmt"

	workspaceView "github.com/Trin9/SynapseFlow/backend/internal/application/workspace/view"
	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
	"github.com/Trin9/SynapseFlow/backend/internal/memory"
	projectionWorkspace "github.com/Trin9/SynapseFlow/backend/internal/projection/workspace"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// Service orchestrates workspace read use-cases.
type Service struct {
	Executions    store.ExecutionStore
	Episodes      store.EpisodeStore
	MemoryStore   memory.ExperienceStore
	EpisodeWriter interface {
		WriteReviewState(ctx context.Context, execID string, req domainEpisode.ReviewActionInput) error
	}

	BuildTriggerContextView func(exec *models.Execution, episodes []*models.Episode) workspaceView.TriggerContextView
	BuildReplaySliceView    func(ep *models.Episode, trace []models.ProcessTraceEntryView, percent int) workspaceView.ReplaySliceView
	BuildComparisonSummary  func(current, historical *models.Execution) ComparisonSummaryView
	BuildEpisodeDossier     func(ep *models.Episode, facts []models.RuntimeFactView, recalls []workspaceView.MemoryRecallView) workspaceView.EpisodeDossierView
	BuildMemoryRecalls      func(ctx context.Context, ep *models.Episode, expStore memory.ExperienceStore) ([]workspaceView.MemoryRecallView, error)
	LogMemoryRecallWarning  func(episodeID string, err error)
}

var (
	ErrExecutionNotFound  = errors.New("execution not found")
	ErrHistoricalNotFound = errors.New("historical execution not found")
	ErrSummaryGet         = errors.New("failed to get execution summary")
	ErrExecutionGet       = errors.New("failed to get execution")
	ErrEpisodeList        = errors.New("failed to list episodes")
	ErrReviewStateGet     = errors.New("failed to get review state")
	ErrReviewActionWrite  = errors.New("failed to write review state")
	ErrEpisodeNotFound    = errors.New("episode not found")
	ErrReplayGetEpisode   = errors.New("failed to get episode")
	ErrComparisonBuild    = errors.New("failed to build comparison")
	ErrDossierBuild       = errors.New("failed to build dossier")
	ErrMemoryRecallSearch = errors.New("failed to search memory recalls")
)

// GetExecutionSummary returns summary projection for one execution.
func (s *Service) GetExecutionSummary(ctx context.Context, executionID string) (*models.ExecutionSummaryView, error) {
	summary, err := s.Executions.GetExecutionSummary(ctx, executionID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrExecutionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSummaryGet, err)
	}
	return summary, nil
}

// GetTriggerContext returns trigger context projection for one execution.
func (s *Service) GetTriggerContext(ctx context.Context, executionID string) (workspaceView.TriggerContextView, error) {
	exec, err := s.Executions.Get(ctx, executionID)
	if errors.Is(err, store.ErrNotFound) {
		return workspaceView.TriggerContextView{}, ErrExecutionNotFound
	}
	if err != nil {
		return workspaceView.TriggerContextView{}, fmt.Errorf("%w: %v", ErrExecutionGet, err)
	}
	episodes, err := s.Episodes.ListByExecution(ctx, executionID)
	if err != nil {
		return workspaceView.TriggerContextView{}, fmt.Errorf("%w: %v", ErrEpisodeList, err)
	}
	if s.BuildTriggerContextView == nil {
		return workspaceView.TriggerContextView{}, fmt.Errorf("%w: trigger context builder unavailable", ErrExecutionGet)
	}
	return s.BuildTriggerContextView(exec, episodes), nil
}

// GetReviewState returns aggregate review-state projection for one execution.
func (s *Service) GetReviewState(ctx context.Context, executionID string) (*workspaceView.ReviewStateView, error) {
	episodes, err := s.Episodes.ListByExecution(ctx, executionID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReviewStateGet, err)
	}
	return projectionWorkspace.EpisodesToReviewState(episodes), nil
}

// PostReviewAction writes a review action for one execution.
func (s *Service) PostReviewAction(ctx context.Context, executionID, episodeID, status, actor, note string) error {
	if s.EpisodeWriter == nil {
		return fmt.Errorf("%w: episode writer unavailable", ErrReviewActionWrite)
	}
	if err := s.EpisodeWriter.WriteReviewState(ctx, executionID, domainEpisode.ReviewActionInput{
		EpisodeID: episodeID,
		Status:    status,
		Actor:     actor,
		Note:      note,
	}); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrExecutionNotFound
		}
		return fmt.Errorf("%w: %v", ErrReviewActionWrite, err)
	}
	return nil
}

// GetEpisodeReplay returns replay slice for one episode at requested percent.
func (s *Service) GetEpisodeReplay(ctx context.Context, episodeID string, percent int) (workspaceView.ReplaySliceView, error) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	ep, err := s.Episodes.Get(ctx, episodeID)
	if errors.Is(err, store.ErrNotFound) {
		return workspaceView.ReplaySliceView{}, ErrEpisodeNotFound
	}
	if err != nil {
		return workspaceView.ReplaySliceView{}, fmt.Errorf("%w: %v", ErrReplayGetEpisode, err)
	}
	trace := projectionWorkspace.EpisodeToProcessTrace(ep)
	if s.BuildReplaySliceView == nil {
		return workspaceView.ReplaySliceView{}, fmt.Errorf("%w: replay builder unavailable", ErrReplayGetEpisode)
	}
	return s.BuildReplaySliceView(ep, trace, percent), nil
}

// GetComparisonTarget compares current and historical executions.
func (s *Service) GetComparisonTarget(ctx context.Context, executionID, historicalID string) (ComparisonSummaryView, error) {
	current, err := s.Executions.Get(ctx, executionID)
	if errors.Is(err, store.ErrNotFound) {
		return ComparisonSummaryView{}, ErrExecutionNotFound
	}
	if err != nil {
		return ComparisonSummaryView{}, fmt.Errorf("%w: %v", ErrComparisonBuild, err)
	}
	historical, err := s.Executions.Get(ctx, historicalID)
	if errors.Is(err, store.ErrNotFound) {
		return ComparisonSummaryView{}, ErrHistoricalNotFound
	}
	if err != nil {
		return ComparisonSummaryView{}, fmt.Errorf("%w: %v", ErrComparisonBuild, err)
	}
	if s.BuildComparisonSummary == nil {
		return ComparisonSummaryView{}, fmt.Errorf("%w: comparison builder unavailable", ErrComparisonBuild)
	}
	return s.BuildComparisonSummary(current, historical), nil
}

// GetEpisodeDossier returns dossier view for one episode.
func (s *Service) GetEpisodeDossier(ctx context.Context, episodeID string) (workspaceView.EpisodeDossierView, error) {
	ep, err := s.Episodes.Get(ctx, episodeID)
	if errors.Is(err, store.ErrNotFound) {
		return workspaceView.EpisodeDossierView{}, ErrEpisodeNotFound
	}
	if err != nil {
		return workspaceView.EpisodeDossierView{}, fmt.Errorf("%w: %v", ErrDossierBuild, err)
	}
	facts := projectionWorkspace.EpisodeToRuntimeFacts(ep)
	recalls := []workspaceView.MemoryRecallView{}
	if s.BuildMemoryRecalls != nil {
		r, recallErr := s.BuildMemoryRecalls(ctx, ep, s.MemoryStore)
		if recallErr != nil {
			if s.LogMemoryRecallWarning != nil {
				s.LogMemoryRecallWarning(episodeID, recallErr)
			}
		} else {
			recalls = r
		}
	}
	if s.BuildEpisodeDossier == nil {
		return workspaceView.EpisodeDossierView{}, fmt.Errorf("%w: dossier builder unavailable", ErrDossierBuild)
	}
	return s.BuildEpisodeDossier(ep, facts, recalls), nil
}

// GetEpisodeMemoryRecalls returns recall list for one episode.
func (s *Service) GetEpisodeMemoryRecalls(ctx context.Context, episodeID string) (workspaceView.MemoryRecallListView, error) {
	ep, err := s.Episodes.Get(ctx, episodeID)
	if errors.Is(err, store.ErrNotFound) {
		return workspaceView.MemoryRecallListView{}, ErrEpisodeNotFound
	}
	if err != nil {
		return workspaceView.MemoryRecallListView{}, fmt.Errorf("%w: %v", ErrMemoryRecallSearch, err)
	}
	if s.BuildMemoryRecalls == nil {
		return workspaceView.MemoryRecallListView{}, fmt.Errorf("%w: memory recall builder unavailable", ErrMemoryRecallSearch)
	}
	recalls, err := s.BuildMemoryRecalls(ctx, ep, s.MemoryStore)
	if err != nil {
		return workspaceView.MemoryRecallListView{}, fmt.Errorf("%w: %v", ErrMemoryRecallSearch, err)
	}
	return workspaceView.MemoryRecallListView{
		Items:              recalls,
		ImplementationNote: "keyword_overlap",
	}, nil
}
