package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Trin9/SynapseFlow/backend/internal/api/dto"
	appWorkspace "github.com/Trin9/SynapseFlow/backend/internal/application/workspace"
	workspaceView "github.com/Trin9/SynapseFlow/backend/internal/application/workspace/view"
	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
	"github.com/Trin9/SynapseFlow/backend/internal/memory"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
	"github.com/gin-gonic/gin"
)

// handleListEpisodes returns all episodes for a given execution.
//
//	GET /api/v1/executions/:id/episodes[?view=summary]
//
// When ?view=summary is set, returns EpisodeSummaryView list instead of raw Episodes.
// @Summary List Episodes
// @Description Returns all episodes for an execution; supports summary view.
// @Tags Episodes
// @Produce json
// @Param id path string true "Execution ID"
// @Param view query string false "Set to summary for summary view"
// @Success 200 {object} dto.EpisodesResponse
// @Failure 500 {object} dto.APIError
// @Router /api/v1/executions/{id}/episodes [get]
func (s *Server) handleListEpisodes(c *gin.Context) {
	execID := c.Param("id")
	ctx := c.Request.Context()
	if c.Query("view") == "summary" {
		summaries, err := s.workspaceSvc.ListEpisodeSummaries(ctx, execID)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "episode_list_error", "failed to list episode summaries", err.Error())
			return
		}
		c.JSON(http.StatusOK, dto.EpisodeSummariesResponse{Episodes: summaries})
		return
	}
	episodes, err := s.workspaceSvc.ListEpisodes(ctx, execID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "episode_list_error", "failed to list episodes", err.Error())
		return
	}
	c.JSON(http.StatusOK, dto.EpisodesResponse{Episodes: episodes})
}

// handleGetEpisode returns a single episode by ID.
//
//	GET /api/v1/episodes/:id
//
// @Summary Get Episode
// @Description Returns a single episode by ID.
// @Tags Episodes
// @Produce json
// @Param id path string true "Episode ID"
// @Success 200 {object} object "Episode"
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
// @Router /api/v1/episodes/{id} [get]
func (s *Server) handleGetEpisode(c *gin.Context) {
	id := c.Param("id")
	ep, err := s.workspaceSvc.GetEpisode(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrEpisodeNotFound) {
			writeError(c, http.StatusNotFound, "episode_not_found", "episode not found", id)
			return
		}
		writeError(c, http.StatusInternalServerError, "episode_get_error", "failed to get episode", err.Error())
		return
	}
	c.JSON(http.StatusOK, ep)
}

// handleGetExecutionSummary returns a high-level summary view of a single execution.
//
//	GET /api/v1/executions/:id/summary
//
// @Summary Get Execution Summary
// @Description Returns a high-level summary view of one execution.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} object "Execution summary"
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
// @Router /api/v1/executions/{id}/summary [get]
func (s *Server) handleGetExecutionSummary(c *gin.Context) {
	id := c.Param("id")
	summary, err := s.workspaceSvc.GetExecutionSummary(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrExecutionNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "summary_error", "failed to get execution summary", err.Error())
		return
	}
	c.JSON(http.StatusOK, summary)
}

// handleGetTriggerContext returns the trigger context view for an execution,
// built from the first episode's trigger data.
//
//	GET /api/v1/executions/:id/trigger-context
//
// @Summary Get Trigger Context
// @Description Returns trigger context view for an execution.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} object "Trigger context"
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
// @Router /api/v1/executions/{id}/trigger-context [get]
func (s *Server) handleGetTriggerContext(c *gin.Context) {
	execID := c.Param("id")
	view, err := s.workspaceSvc.GetTriggerContext(c.Request.Context(), execID)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrExecutionNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		if errors.Is(err, appWorkspace.ErrExecutionGet) {
			writeError(c, http.StatusInternalServerError, "trigger_context_error", "failed to get execution", err.Error())
			return
		}
		if errors.Is(err, appWorkspace.ErrEpisodeList) {
			writeError(c, http.StatusInternalServerError, "trigger_context_error", "failed to list episodes", err.Error())
			return
		}
		writeError(c, http.StatusInternalServerError, "trigger_context_error", "failed to get trigger context", err.Error())
		return
	}
	c.JSON(http.StatusOK, view)
}

// handleGetReviewState returns the aggregate human-review state for an execution.
//
//	GET /api/v1/executions/:id/review-state
//
// @Summary Get Review State
// @Description Returns aggregate human-review state for an execution.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} object "Review state"
// @Failure 500 {object} dto.APIError
// @Router /api/v1/executions/{id}/review-state [get]
func (s *Server) handleGetReviewState(c *gin.Context) {
	execID := c.Param("id")
	state, err := s.workspaceSvc.GetReviewState(c.Request.Context(), execID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "review_state_error", "failed to get review state", err.Error())
		return
	}
	c.JSON(http.StatusOK, state)
}

// handlePostReviewAction records a human review decision on an execution.
//
//	POST /api/v1/executions/:id/review-actions
//
// @Summary Post Review Action
// @Description Records a human review decision on an execution.
// @Tags Workspace
// @Accept json
// @Produce json
// @Param id path string true "Execution ID"
// @Param request body object true "Review action request"
// @Success 200 {object} dto.ReviewActionResponse
// @Failure 400 {object} dto.APIError
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
// @Router /api/v1/executions/{id}/review-actions [post]
func (s *Server) handlePostReviewAction(c *gin.Context) {
	execID := c.Param("id")
	var req dto.ReviewActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid request body", err.Error())
		return
	}
	if err := s.workspaceSvc.PostReviewAction(c.Request.Context(), execID, req.EpisodeID, req.Status, req.Actor, req.Note); err != nil {
		if errors.Is(err, appWorkspace.ErrInvalidReviewState) {
			writeError(c, http.StatusBadRequest, "invalid_request", "invalid review status", req.Status)
			return
		}
		if errors.Is(err, appWorkspace.ErrExecutionNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "review_action_error", "failed to write review state", err.Error())
		return
	}
	c.JSON(http.StatusOK, dto.ReviewActionResponse{OK: true})
}

// handleGetEpisodeReplay returns a replay slice view for a single episode.
//
//	GET /api/v1/executions/:id/episodes/:episode_id/replay?percent=N
//
// @Summary Get Episode Replay
// @Description Returns replay slice view for a single episode.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Param episode_id path string true "Episode ID"
// @Param percent query int false "Replay percentage (0-100)"
// @Success 200 {object} object "Replay slice view"
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
// @Router /api/v1/executions/{id}/episodes/{episode_id}/replay [get]
func (s *Server) handleGetEpisodeReplay(c *gin.Context) {
	episodeID := c.Param("episode_id")
	percentStr := c.DefaultQuery("percent", "100")
	percent, err := strconv.Atoi(percentStr)
	if err != nil || percent < 0 || percent > 100 {
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid replay percent", percentStr)
		return
	}
	view, err := s.workspaceSvc.GetEpisodeReplay(c.Request.Context(), episodeID, percent)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrEpisodeNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "episode not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "replay_error", "failed to get episode", err.Error())
		return
	}
	c.JSON(http.StatusOK, view)
}

// handleGetEpisodeDossier returns the full dossier for a single episode.
//
//	GET /api/v1/executions/:id/episodes/:episode_id/dossier
//
// @Summary Get Episode Dossier
// @Description Returns full dossier for a single episode.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Param episode_id path string true "Episode ID"
// @Success 200 {object} object "Episode dossier"
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
// @Router /api/v1/executions/{id}/episodes/{episode_id}/dossier [get]
func (s *Server) handleGetEpisodeDossier(c *gin.Context) {
	episodeID := c.Param("episode_id")
	dossier, err := s.workspaceSvc.GetEpisodeDossier(c.Request.Context(), episodeID)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrEpisodeNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "episode not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "dossier_error", "failed to get episode", err.Error())
		return
	}
	c.JSON(http.StatusOK, dossier)
}

// handleGetEpisodeMemoryRecalls returns memory recall items for a single episode.
// CR-010: uses s.memory.Search() to perform real Experience retrieval instead
// of the former stub which always returned an empty slice.
//
//	GET /api/v1/executions/:id/episodes/:episode_id/memory-recalls
//
// @Summary Get Episode Memory Recalls
// @Description Returns memory recall items for a single episode.
// @Tags Workspace
// @Produce json
// @Param id path string true "Execution ID"
// @Param episode_id path string true "Episode ID"
// @Success 200 {object} object "Memory recall list"
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
// @Router /api/v1/executions/{id}/episodes/{episode_id}/memory-recalls [get]
func (s *Server) handleGetEpisodeMemoryRecalls(c *gin.Context) {
	episodeID := c.Param("episode_id")
	list, err := s.workspaceSvc.GetEpisodeMemoryRecalls(c.Request.Context(), episodeID)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrEpisodeNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "episode not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "memory_recall_error", "failed to search memory recalls", err.Error())
		return
	}
	c.JSON(http.StatusOK, list)
}

func buildMemoryRecallsForEpisode(ctx context.Context, ep *models.Episode, expStore memory.ExperienceStore) ([]workspaceView.MemoryRecallView, error) {
	if expStore == nil || ep == nil {
		return []workspaceView.MemoryRecallView{}, nil
	}

	var parts []string
	alertType := ""
	serviceName := ""

	if ep.Trigger != nil {
		for _, key := range []string{"alert_text", "alert_summary", "alert_type", "service_name", "symptom", "input"} {
			if v, ok := ep.Trigger.Payload[key]; ok {
				if s := fmt.Sprintf("%v", v); s != "" {
					parts = append(parts, s)
				}
			}
		}
		if v, ok := ep.Trigger.Payload["alert_type"]; ok {
			alertType = fmt.Sprintf("%v", v)
		}
		if v, ok := ep.Trigger.Payload["service_name"]; ok {
			serviceName = fmt.Sprintf("%v", v)
		}
	}

	for _, ev := range ep.Evidence {
		if ev.Label != "" {
			parts = append(parts, ev.Label)
		}
	}

	if ep.Verdict != nil {
		if ep.Verdict.Conclusion != "" {
			parts = append(parts, ep.Verdict.Conclusion)
		}
		parts = append(parts, ep.Verdict.CausalChain...)
	}

	query := store.SearchQuery{
		Text:        strings.TrimSpace(strings.Join(parts, "\n")),
		AlertType:   alertType,
		ServiceName: serviceName,
		TopK:        5,
	}

	experiences, err := expStore.Search(ctx, query)
	if err != nil {
		return []workspaceView.MemoryRecallView{}, err
	}
	if len(experiences) == 0 {
		return []workspaceView.MemoryRecallView{}, nil
	}

	recalls := make([]workspaceView.MemoryRecallView, 0, len(experiences))
	for _, exp := range experiences {
		confidence := "low"
		if exp.Score >= 0.7 {
			confidence = "high"
		} else if exp.Score >= 0.4 {
			confidence = "medium"
		}

		title := exp.Summary
		if title == "" {
			title = exp.AlertType
		}
		if title == "" {
			title = exp.ID
		}

		matchedPattern := strings.Join(exp.Tags, ", ")
		if matchedPattern == "" {
			matchedPattern = exp.AlertType
		}

		recalls = append(recalls, workspaceView.MemoryRecallView{
			ID:                exp.ID,
			Title:             title,
			Summary:           exp.Summary,
			MatchedPattern:    matchedPattern,
			Confidence:        confidence,
			SourceExecutionID: exp.ExecutionID,
			Recommendation:    exp.ActionTaken,
		})
	}
	return recalls, nil
}

// handleGetComparisonTarget compares two executions and returns a summary.
//
//	GET /api/v1/executions/:id/comparison-targets/:historical_id
//
// @Summary Get Comparison Target
// @Description Compares two executions and returns a summary.
// @Tags Workspace
// @Produce json
// @Param id path string true "Current execution ID"
// @Param historical_id path string true "Historical execution ID"
// @Success 200 {object} object "Comparison summary"
// @Failure 404 {object} dto.APIError
// @Failure 500 {object} dto.APIError
// @Router /api/v1/executions/{id}/comparison-targets/{historical_id} [get]
func (s *Server) handleGetComparisonTarget(c *gin.Context) {
	execID := c.Param("id")
	historicalID := c.Param("historical_id")
	summary, err := s.workspaceSvc.GetComparisonTarget(c.Request.Context(), execID, historicalID)
	if err != nil {
		if errors.Is(err, appWorkspace.ErrExecutionNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "execution not found", nil)
			return
		}
		if errors.Is(err, appWorkspace.ErrHistoricalNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "historical execution not found", nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "comparison_error", "failed to build comparison", err.Error())
		return
	}
	c.JSON(http.StatusOK, summary)
}

// buildTriggerContextView constructs a TriggerContextView from execution + episode data.
func buildTriggerContextView(exec *models.Execution, episodes []*models.Episode) workspaceView.TriggerContextView {
	view := workspaceView.TriggerContextView{
		Title:   fmt.Sprintf("Trigger — %s", exec.DAGName),
		Summary: fmt.Sprintf("Execution %s triggered on %s", exec.ID[:8], exec.StartedAt.Format(time.RFC3339)),
	}
	for _, ep := range episodes {
		if ep.Trigger == nil {
			continue
		}
		t := ep.Trigger
		payloadStr := func(key string) string {
			if t.Payload == nil {
				return ""
			}
			if v, ok := t.Payload[key]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
			return ""
		}
		section := workspaceView.TriggerContextSectionView{
			Title: "Alert",
			Fields: []workspaceView.TriggerContextFieldView{
				{Label: "Trigger Type", Value: string(t.Type), Range: [2]int{0, 0}},
				{Label: "Alert Type", Value: payloadStr("alert_type"), Range: [2]int{0, 0}},
				{Label: "Service", Value: payloadStr("service_name"), Range: [2]int{0, 0}},
				{Label: "Severity", Value: payloadStr("severity"), Range: [2]int{0, 0}},
			},
		}
		nonEmpty := section.Fields[:0]
		for _, f := range section.Fields {
			if f.Value != "" {
				nonEmpty = append(nonEmpty, f)
			}
		}
		if len(nonEmpty) > 0 {
			section.Fields = nonEmpty
			view.Sections = append(view.Sections, section)
		}
		if ep.InvestigationContext != nil {
			ic := ep.InvestigationContext
			icSection := workspaceView.TriggerContextSectionView{
				Title: "Investigation",
				Fields: []workspaceView.TriggerContextFieldView{
					{Label: "Hypothesis", Value: ic.Hypothesis},
				},
			}
			if len(ic.KnownSignals) > 0 {
				icSection.Fields = append(icSection.Fields, workspaceView.TriggerContextFieldView{
					Label: "Known Signals",
					Value: strings.Join(ic.KnownSignals, ", "),
				})
			}
			view.Sections = append(view.Sections, icSection)
		}
		break
	}
	return view
}

// buildReplaySliceView computes what is visible in the replay at the given percent.
func buildReplaySliceView(ep *models.Episode, trace []workspaceView.ProcessTraceEntryView, percent int) workspaceView.ReplaySliceView {
	visible := make([]workspaceView.ProcessTraceEntryView, 0, len(trace))
	visibleFactIDs := make([]string, 0)
	for _, entry := range trace {
		if entry.Range[0] <= percent {
			visible = append(visible, entry)
			visibleFactIDs = append(visibleFactIDs, entry.ID)
		}
	}
	checkpoint := workspaceView.ReplayCheckpointView{
		Label:    fmt.Sprintf("%d%%", percent),
		Headline: "Execution in progress",
	}
	if len(visible) > 0 {
		last := visible[len(visible)-1]
		checkpoint.Headline = last.Title
		checkpoint.Detail = last.Detail
	}
	return workspaceView.ReplaySliceView{
		EpisodeID:             ep.ID,
		Percent:               percent,
		Checkpoint:            checkpoint,
		VisibleProcessTrace:   visible,
		VisibleHandles:        []interface{}{},
		VisibleStateFields:    []interface{}{},
		VisibleRuntimeFactIDs: visibleFactIDs,
	}
}

// buildEpisodeDossier constructs an EpisodeDossierView from an episode and its facts.
func buildEpisodeDossier(ep *models.Episode, facts []workspaceView.RuntimeFactView, recalls []workspaceView.MemoryRecallView) workspaceView.EpisodeDossierView {
	display := workspaceView.DossierDisplayView{}
	if ep.Verdict != nil {
		display.Verdict = string(ep.Verdict.Result)
		display.VerdictLabel = domainEpisode.VerdictLabelFromResult(ep.Verdict.Result)
		display.Summary = ep.Verdict.Conclusion
	}
	tmp := domainEpisode.HumanReviewDisplay{VerdictLabel: display.VerdictLabel, Banner: display.Banner}
	domainEpisode.ApplyHumanReviewDisplay(ep, &tmp)
	display.VerdictLabel = tmp.VerdictLabel
	display.Banner = tmp.Banner

	commonFocusKey := ""
	if len(ep.Handles) == 1 {
		commonFocusKey = string(ep.Handles[0].Type) + ":" + ep.Handles[0].Value
	} else if len(ep.Handles) > 1 {
		first := string(ep.Handles[0].Type) + ":" + ep.Handles[0].Value
		allSame := true
		for _, h := range ep.Handles[1:] {
			if string(h.Type)+":"+h.Value != first {
				allSame = false
				break
			}
		}
		if allSame {
			commonFocusKey = first
		}
	}

	expectedBehaviors := buildDesignExpectedBehaviors(ep, commonFocusKey)
	if len(expectedBehaviors) == 0 && ep.Verdict != nil {
		for i, link := range ep.Verdict.CausalChain {
			expectedBehaviors = append(expectedBehaviors, workspaceView.ExpectedBehaviorView{
				ID:          fmt.Sprintf("causal_%d", i),
				Title:       fmt.Sprintf("Causal Factor %d", i+1),
				Body:        link,
				FocusKey:    commonFocusKey,
				SourceType:  "ai",
				SourceLabel: "AI Hypothesized",
			})
		}
	}
	var verdictBridge []workspaceView.VerdictBridgeItemView
	if ep.Verdict != nil {
		for i, rec := range ep.Verdict.Recommendations {
			verdictBridge = append(verdictBridge, workspaceView.VerdictBridgeItemView{
				ID:       fmt.Sprintf("rec_%d", i),
				Title:    fmt.Sprintf("Recommendation %d", i+1),
				Body:     rec,
				FocusKey: commonFocusKey,
			})
		}
	}
	return workspaceView.EpisodeDossierView{
		Episode: workspaceView.DossierEpisodeRefView{
			EpisodeID: ep.ID,
			Label:     string(ep.EpisodeType),
		},
		Display: workspaceView.DossierDisplayView{
			Verdict:      display.Verdict,
			VerdictLabel: display.VerdictLabel,
			Summary:      display.Summary,
			Banner:       display.Banner,
		},
		ExpectedBehavior: expectedBehaviors,
		VerdictBridge:    verdictBridge,
		RuntimeFacts:     facts,
		Handles:          ep.Handles,
		MemoryRecalls:    recalls,
		HumanAuditTrail:  ep.HumanInterventions,
	}
}

func buildDesignExpectedBehaviors(ep *models.Episode, focusKey string) []workspaceView.ExpectedBehaviorView {
	if ep == nil {
		return nil
	}

	expected := make([]string, 0)
	if ep.ActionContext != nil {
		expected = append(expected, normalizeStringList(ep.ActionContext.ActionInput["expected_behaviors"])...)
	}
	if len(expected) == 0 && ep.InvestigationContext != nil {
		expected = append(expected, ep.InvestigationContext.KnownSignals...)
	}
	if len(expected) == 0 {
		return nil
	}

	out := make([]workspaceView.ExpectedBehaviorView, 0, len(expected))
	for i, item := range expected {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, workspaceView.ExpectedBehaviorView{
			ID:           fmt.Sprintf("design_expected_%d", i),
			Title:        fmt.Sprintf("Expected Behavior %d", i+1),
			Body:         trimmed,
			FocusKey:     focusKey,
			SourceType:   "sop",
			SourceLabel:  "Verified SOP",
			SourceDetail: "Defined in Design mode episode specification.",
		})
	}
	return out
}

func normalizeStringList(raw interface{}) []string {
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// buildComparisonSummary compares two executions and returns a summary view.
func buildComparisonSummary(current, historical *models.Execution) appWorkspace.ComparisonSummaryView {
	summary := appWorkspace.ComparisonSummaryView{
		ExecutionID:     current.ID,
		Title:           fmt.Sprintf("%s vs %s", current.ID[:8], historical.ID[:8]),
		ComparedAgainst: historical.ID,
	}
	if current.Status == historical.Status {
		summary.Summary = fmt.Sprintf("Both executions completed with status: %s", current.Status)
		summary.Outcome = "match"
	} else {
		summary.Summary = fmt.Sprintf("Current: %s — Historical: %s", current.Status, historical.Status)
		summary.Outcome = "divergent"
		summary.Caution = "Execution outcomes differ."
	}
	if current.Duration > 0 && historical.Duration > 0 {
		diff := current.Duration - historical.Duration
		if diff > 0 {
			summary.Highlights = append(summary.Highlights,
				fmt.Sprintf("Current run was %s slower than historical", diff.Round(time.Millisecond)))
		} else if diff < 0 {
			summary.Highlights = append(summary.Highlights,
				fmt.Sprintf("Current run was %s faster than historical", (-diff).Round(time.Millisecond)))
		}
	}
	return summary
}

// execToSummaryView projects a raw Execution into an ExecutionSummaryView.
func execToSummaryView(exec *models.Execution) workspaceView.ExecutionSummaryView {
	label := exec.DAGName
	if len(exec.ID) >= 8 {
		label = fmt.Sprintf("%s #%s", exec.DAGName, exec.ID[:8])
	}
	return workspaceView.ExecutionSummaryView{
		ExecutionID:  exec.ID,
		DAGID:        exec.DAGID,
		DAGName:      exec.DAGName,
		Status:       exec.Status,
		StartedAt:    exec.StartedAt,
		EndedAt:      exec.EndedAt,
		DurationMs:   exec.Duration.Milliseconds(),
		Mode:         "execution",
		WorkflowKind: "investigation",
		Display: workspaceView.ExecutionDisplayView{
			RunLabel:   label,
			TraceTitle: exec.DAGName,
		},
	}
}

// projectExecutionList converts a slice of raw Executions to ExecutionSummaryView.
func projectExecutionList(execs []*models.Execution) []workspaceView.ExecutionSummaryView {
	out := make([]workspaceView.ExecutionSummaryView, len(execs))
	for i, e := range execs {
		out[i] = execToSummaryView(e)
	}
	return out
}
