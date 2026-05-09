package workspace

import (
	"fmt"

	workspaceView "github.com/Trin9/SynapseFlow/backend/internal/application/workspace/view"
	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// ExecutionToSummary converts an Execution to an ExecutionSummaryView.
func ExecutionToSummary(exec *models.Execution) *workspaceView.ExecutionSummaryView {
	label := exec.DAGName
	if len(exec.ID) >= 8 {
		label = fmt.Sprintf("%s #%s", exec.DAGName, exec.ID[:8])
	}
	return &workspaceView.ExecutionSummaryView{
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

// EpisodeToSummary converts an Episode to an EpisodeSummaryView.
func EpisodeToSummary(ep *models.Episode) workspaceView.EpisodeSummaryView {
	sv := workspaceView.EpisodeSummaryView{
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
		sv.Display = workspaceView.EpisodeDisplayView{
			Verdict:      string(ep.Verdict.Result),
			VerdictLabel: domainEpisode.VerdictLabelFromResult(ep.Verdict.Result),
			Summary:      truncateStr(ep.Verdict.Conclusion, 120),
		}
	}
	tmp := domainEpisode.HumanReviewDisplay{Banner: sv.Display.Banner, VerdictLabel: sv.Display.VerdictLabel}
	domainEpisode.ApplyHumanReviewDisplay(ep, &tmp)
	sv.Display.Banner = tmp.Banner
	sv.Display.VerdictLabel = tmp.VerdictLabel
	return sv
}

// EpisodeToProcessTrace derives a process-trace timeline from an Episode.
func EpisodeToProcessTrace(ep *models.Episode) []workspaceView.ProcessTraceEntryView {
	total := len(ep.Evidence) + len(ep.HumanInterventions)
	if ep.Verdict != nil {
		total++
	}
	entries := make([]workspaceView.ProcessTraceEntryView, 0, total)
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
		entry := workspaceView.ProcessTraceEntryView{ID: ev.ID, Stage: stage, Title: title, Detail: truncateStr(ev.Content, 200), Status: "success", Range: [2]int{startPct, endPct}}
		if string(ev.NodeType) != "" {
			entry.Chips = []string{string(ev.NodeType)}
		}
		entries = append(entries, entry)
	}
	for _, hi := range ep.HumanInterventions {
		entries = append(entries, workspaceView.ProcessTraceEntryView{ID: hi.NodeID + "_human", Stage: "Human Review", Title: domainEpisode.HumanActionLabel(hi.Action), Detail: truncateStr(hi.Detail, 200), Status: "success", Range: [2]int{100, 100}})
	}
	if ep.Verdict != nil {
		status := string(ep.Verdict.Result)
		if status == "" {
			status = "pending"
		}
		entries = append(entries, workspaceView.ProcessTraceEntryView{ID: ep.ID + "_verdict", Stage: "Verdict", Title: domainEpisode.VerdictLabelFromResult(ep.Verdict.Result), Detail: truncateStr(ep.Verdict.Conclusion, 200), Status: status, Range: [2]int{100, 100}})
	}
	if ep.LoopGuard.MaxIterations > 0 && ep.LoopGuard.CurrentIteration >= ep.LoopGuard.MaxIterations {
		entries = append(entries, workspaceView.ProcessTraceEntryView{ID: ep.ID + "_circuit_breaker", Stage: "Circuit Breaker", Title: "Max iterations reached", Status: "failed", Range: [2]int{100, 100}})
	}
	return entries
}

// EpisodeToRuntimeFacts projects evidence entries to RuntimeFactView.
func EpisodeToRuntimeFacts(ep *models.Episode) []workspaceView.RuntimeFactView {
	handleBySource := make(map[string]string, len(ep.Handles))
	for _, h := range ep.Handles {
		if _, exists := handleBySource[h.Source]; !exists {
			handleBySource[h.Source] = string(h.Type) + ":" + h.Value
		}
	}
	out := make([]workspaceView.RuntimeFactView, 0, len(ep.Evidence))
	for _, ev := range ep.Evidence {
		title := ev.Label
		if title == "" {
			title = fmt.Sprintf("Evidence (%s)", ev.Type)
		}
		out = append(out, workspaceView.RuntimeFactView{ID: ev.ID, Title: title, Summary: truncateStr(ev.Content, 200), FocusKey: handleBySource[ev.NodeID], SourceType: nodeTypeToSourceType(ev.NodeType), Collector: string(ev.NodeType) + ":" + ev.NodeID, Content: ev.Content, ContentRef: ev.ContentRef})
	}
	return out
}

// EpisodesToReviewState derives a ReviewStateView from human interventions.
func EpisodesToReviewState(episodes []*models.Episode) *workspaceView.ReviewStateView {
	state := &workspaceView.ReviewStateView{Status: "pending"}
	for _, ep := range episodes {
		for _, hi := range ep.HumanInterventions {
			if hi.Action == models.HumanActionSuspended {
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

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !isRuneStart(s[max]) {
		max--
	}
	return s[:max] + "…"
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

func rangeForIndex(i, n int) (int, int) {
	if n <= 0 {
		return 0, 100
	}
	return i * 100 / n, (i + 1) * 100 / n
}

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
