package engine

// episode_writer.go — write-permission layer for Episode objects (Sprint 7).
//
// Design contract (from AGENTS.md):
//   - Hard Node  → may only append EvidenceTypeFact entries.
//   - Soft Node  → may only write the EpisodeVerdict (once).
//   - Human Node → may modify any field but MUST append an AuditEntry.
//
// All mutations go through the three exported helpers below; nothing else
// should touch Episode fields directly inside node executors.

import (
	"context"
	"fmt"
	"sync"
	"time"

	workspaceView "github.com/Trin9/SynapseFlow/backend/internal/application/workspace/view"
	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
	"github.com/google/uuid"
)

// EpisodeWriter wraps an EpisodeStore and enforces per-node-type write rules.
type EpisodeWriter struct {
	store store.EpisodeStore
	locks sync.Map // map[episodeID]*sync.Mutex
}

// NewEpisodeWriter creates an EpisodeWriter backed by the given store.
func NewEpisodeWriter(s store.EpisodeStore) *EpisodeWriter {
	return &EpisodeWriter{store: s}
}

func (w *EpisodeWriter) lockEpisode(episodeID string) func() {
	v, _ := w.locks.LoadOrStore(episodeID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// ---------------------------------------------------------------------------
// Hard Node — append a fact evidence entry
// ---------------------------------------------------------------------------

// AppendFact appends a new EvidenceTypeFact entry to the Episode.
// Only Hard Nodes (script, mcp, web_interaction) should call this.
// Side-effect: transitions Episode status from pending → in_progress on first call.
func (w *EpisodeWriter) AppendFact(
	ctx context.Context,
	episodeID string,
	nodeID string,
	nodeType models.NodeType,
	label string,
	content string,
) error {
	unlock := w.lockEpisode(episodeID)
	defer unlock()

	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.AppendFact get: %w", err)
	}
	ev := models.EpisodeEvidence{
		ID:          uuid.New().String(),
		Type:        domainEpisode.EpisodeEvidenceTypeFact.ToModel(),
		NodeID:      nodeID,
		NodeType:    nodeType,
		Label:       label,
		Content:     content,
		CollectedAt: time.Now().UTC(),
	}
	ep.Evidence = append(ep.Evidence, ev)
	// Status lifecycle: pending → in_progress on first evidence write.
	if ep.Status == domainEpisode.EpisodeStatusPending.ToModel() {
		ep.Status = domainEpisode.EpisodeStatusInProgress.ToModel()
	}
	ep.UpdatedAt = time.Now().UTC()
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.AppendFact update: %w", err)
	}
	return nil
}

// AppendFactWithSpec is like AppendFact but also records how the evidence was
// collected (the query parameters, resolved command, etc.) via CollectorSpec.
// Hard Nodes that know their collection method (script, log_query, db_query…)
// should prefer this over AppendFact for full evidence traceability.
// Side-effect: transitions Episode status from pending → in_progress on first call.
func (w *EpisodeWriter) AppendFactWithSpec(
	ctx context.Context,
	episodeID string,
	nodeID string,
	nodeType models.NodeType,
	label string,
	content string,
	spec *models.EvidenceCollectorSpec,
) error {
	unlock := w.lockEpisode(episodeID)
	defer unlock()

	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.AppendFactWithSpec get: %w", err)
	}
	ev := models.EpisodeEvidence{
		ID:            uuid.New().String(),
		Type:          domainEpisode.EpisodeEvidenceTypeFact.ToModel(),
		NodeID:        nodeID,
		NodeType:      nodeType,
		Label:         label,
		Content:       content,
		CollectorSpec: spec,
		CollectedAt:   time.Now().UTC(),
	}
	ep.Evidence = append(ep.Evidence, ev)
	if ep.Status == domainEpisode.EpisodeStatusPending.ToModel() {
		ep.Status = domainEpisode.EpisodeStatusInProgress.ToModel()
	}
	ep.UpdatedAt = time.Now().UTC()
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.AppendFactWithSpec update: %w", err)
	}
	return nil
}

// AppendFactWithRef is the large-payload variant: stores an artifact and
// records a ContentRef instead of inlining the content.
// Side-effect: transitions Episode status from pending → in_progress on first call.
func (w *EpisodeWriter) AppendFactWithRef(
	ctx context.Context,
	episodeID string,
	nodeID string,
	nodeType models.NodeType,
	label string,
	contentType string,
	content string,
) error {
	unlock := w.lockEpisode(episodeID)
	defer unlock()

	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.AppendFactWithRef get: %w", err)
	}
	evID := uuid.New().String()
	storageURI := fmt.Sprintf("artifact://%s/%s", ep.ExecID, evID)

	artifact := &models.EpisodeArtifact{
		ID:          uuid.New().String(),
		EpisodeID:   episodeID,
		EvidenceID:  evID,
		ContentType: contentType,
		SizeBytes:   int64(len(content)),
		StorageURI:  storageURI,
		Content:     content,
		CreatedAt:   time.Now().UTC(),
	}
	if err := w.store.SaveArtifact(ctx, artifact); err != nil {
		return fmt.Errorf("episode_writer.AppendFactWithRef save_artifact: %w", err)
	}

	ev := models.EpisodeEvidence{
		ID:          evID,
		Type:        domainEpisode.EpisodeEvidenceTypeFact.ToModel(),
		NodeID:      nodeID,
		NodeType:    nodeType,
		Label:       label,
		ContentRef:  storageURI,
		CollectedAt: time.Now().UTC(),
	}
	ep.Evidence = append(ep.Evidence, ev)
	// Status lifecycle: pending → in_progress on first evidence write.
	if ep.Status == domainEpisode.EpisodeStatusPending.ToModel() {
		ep.Status = domainEpisode.EpisodeStatusInProgress.ToModel()
	}
	ep.UpdatedAt = time.Now().UTC()
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.AppendFactWithRef update: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Hard Node — append a handle (structured tracking identifier)
// ---------------------------------------------------------------------------

// AppendHandle appends an EpisodeHandle to the Episode's Handles slice.
// Only Hard Nodes that extract structured identifiers (session IDs, order IDs,
// trace IDs, etc.) from script output should call this.
// The handleType is the raw config value (e.g. "session_id", "product_id")
// and maps directly to EpisodeHandleType — predefined constants are preferred
// but any non-empty string is accepted.
func (w *EpisodeWriter) AppendHandle(
	ctx context.Context,
	episodeID string,
	handleType models.EpisodeHandleType,
	value string,
	sourceNodeID string,
) error {
	unlock := w.lockEpisode(episodeID)
	defer unlock()

	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.AppendHandle get: %w", err)
	}
	h := models.EpisodeHandle{
		Type:        handleType,
		Value:       value,
		Source:      sourceNodeID,
		ExtractedAt: time.Now().UTC(),
	}
	ep.Handles = append(ep.Handles, h)
	ep.UpdatedAt = time.Now().UTC()
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.AppendHandle update: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Soft Node — write the verdict
// ---------------------------------------------------------------------------

// WriteVerdict sets the EpisodeVerdict on an Episode.
// Only Soft Nodes (llm, router) should call this.
// Returns an error if a verdict already exists (idempotent update must go
// through HumanCorrect instead).
// Side-effects:
//   - Transitions Episode status to converged.
//   - Sets ConcludedAt timestamp.
//   - Populates MemoryExtraction trigger state based on confidence level.
func (w *EpisodeWriter) WriteVerdict(
	ctx context.Context,
	episodeID string,
	nodeID string,
	verdict models.EpisodeVerdict,
) error {
	unlock := w.lockEpisode(episodeID)
	defer unlock()

	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.WriteVerdict get: %w", err)
	}
	if ep.Verdict != nil {
		return fmt.Errorf("episode_writer.WriteVerdict: verdict already set for episode %s; use HumanCorrect to override", episodeID)
	}
	now := time.Now().UTC()
	verdict.DecidedBy = nodeID
	verdict.DecidedAt = now
	ep.Verdict = &verdict
	ep.Status = domainEpisode.EpisodeStatusConverged.ToModel()
	ep.ConcludedAt = &now
	ep.MemoryExtraction = computeMemoryExtraction(&verdict)
	ep.UpdatedAt = now
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.WriteVerdict update: %w", err)
	}
	return nil
}

// computeMemoryExtraction derives the memory extraction trigger state from the
// verdict.  Auto-trigger fires when confidence=="high" and result!="inconclusive".
func computeMemoryExtraction(v *models.EpisodeVerdict) *models.EpisodeMemoryExtraction {
	if v == nil {
		return nil
	}
	if v.Confidence == domainEpisode.EpisodeConfidenceHigh.ToModel() && v.Result != domainEpisode.EpisodeResultInconclusive.ToModel() {
		return &models.EpisodeMemoryExtraction{
			Triggered: true,
			TriggerBy: "auto_high_confidence",
			Status:    "pending",
		}
	}
	return &models.EpisodeMemoryExtraction{Triggered: false}
}

// ---------------------------------------------------------------------------
// Human Node — correct any field with mandatory audit trail
// ---------------------------------------------------------------------------

// HumanCorrect applies an actor-driven correction to an Episode field and
// records it in AuditTrail.  fieldPath is a human-readable description of
// what changed (e.g. "verdict.conclusion").
// Only Human Nodes should call this; it is the ONLY path allowed to mutate
// previously-written fields.
func (w *EpisodeWriter) HumanCorrect(
	ctx context.Context,
	episodeID string,
	actorID string,
	nodeID string,
	fieldPath string,
	oldValue interface{},
	newValue interface{},
	applyFn func(ep *models.Episode),
) error {
	unlock := w.lockEpisode(episodeID)
	defer unlock()

	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.HumanCorrect get: %w", err)
	}
	entry := models.EpisodeAuditEntry{
		Actor:         actorID,
		NodeID:        nodeID,
		FieldModified: fieldPath,
		OldValue:      oldValue,
		NewValue:      newValue,
		ModifiedAt:    time.Now().UTC(),
	}
	applyFn(ep)
	ep.AuditTrail = append(ep.AuditTrail, entry)
	ep.UpdatedAt = time.Now().UTC()
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.HumanCorrect update: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Human Node — record a structured intervention event
// ---------------------------------------------------------------------------

// AppendHumanIntervention appends a HumanIntervention record to the Episode.
// This should be called when a human takes an action on a suspended episode
// (resume, abort, state override, etc.).  nodeID may be empty for API-level
// interventions that happen outside the graph (e.g. direct resume via REST).
func (w *EpisodeWriter) AppendHumanIntervention(
	ctx context.Context,
	episodeID string,
	nodeID string,
	actor string,
	action models.HumanInterventionAction,
	detail string,
) error {
	unlock := w.lockEpisode(episodeID)
	defer unlock()

	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.AppendHumanIntervention get: %w", err)
	}
	intervention := models.HumanIntervention{
		NodeID:    nodeID,
		Actor:     actor,
		Action:    action,
		Detail:    detail,
		Timestamp: time.Now().UTC(),
	}
	ep.HumanInterventions = append(ep.HumanInterventions, intervention)
	ep.UpdatedAt = time.Now().UTC()
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.AppendHumanIntervention update: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Display / workspace write helpers (M1.3)
// ---------------------------------------------------------------------------

// AppendProcessTraceEntry converts a ProcessTraceEntryView into an Evidence
// entry and appends it to the episode.  Callers use this when an engine step
// is better described by an explicit display entry than by a raw fact (e.g.
// a circuit-breaker firing, or a router decision with no artifact output).
// If entry.ID is non-empty it is used as the evidence ID to enable idempotent
// re-runs; otherwise a new UUID is generated.
func (w *EpisodeWriter) AppendProcessTraceEntry(
	ctx context.Context,
	episodeID string,
	entry workspaceView.ProcessTraceEntryView,
) error {
	unlock := w.lockEpisode(episodeID)
	defer unlock()

	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.AppendProcessTraceEntry get: %w", err)
	}
	evID := entry.ID
	if evID == "" {
		evID = uuid.New().String()
	}
	// Store range hint in CollectorSpec params so replay logic can use it.
	spec := &models.EvidenceCollectorSpec{
		CollectorType: "process_trace",
		Params: map[string]interface{}{
			"_stage":        entry.Stage,
			"_status":       entry.Status,
			"_replay_start": entry.Range[0],
			"_replay_end":   entry.Range[1],
		},
	}
	label := fmt.Sprintf("[%s] %s", entry.Stage, entry.Title)
	ev := models.EpisodeEvidence{
		ID:            evID,
		Type:          domainEpisode.EpisodeEvidenceTypeFact.ToModel(),
		NodeType:      models.NodeType(""),
		Label:         label,
		Content:       entry.Detail,
		CollectorSpec: spec,
		CollectedAt:   time.Now().UTC(),
	}
	ep.Evidence = append(ep.Evidence, ev)
	if ep.Status == domainEpisode.EpisodeStatusPending.ToModel() {
		ep.Status = domainEpisode.EpisodeStatusInProgress.ToModel()
	}
	ep.UpdatedAt = time.Now().UTC()
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.AppendProcessTraceEntry update: %w", err)
	}
	return nil
}

// UpdateDisplaySummary updates the episode verdict's Conclusion field to the
// provided summary string.  This is the canonical source for the one-line
// display summary shown in EpisodeSummaryView.Display.Summary.
// If no verdict exists yet this call is a no-op; the summary will be populated
// automatically when WriteVerdict is called.
func (w *EpisodeWriter) UpdateDisplaySummary(
	ctx context.Context,
	episodeID string,
	summary string,
) error {
	unlock := w.lockEpisode(episodeID)
	defer unlock()

	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.UpdateDisplaySummary get: %w", err)
	}
	if ep.Verdict == nil {
		// No verdict yet — nothing to update; summary will come from WriteVerdict.
		return nil
	}
	ep.Verdict.Conclusion = summary
	ep.UpdatedAt = time.Now().UTC()
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.UpdateDisplaySummary update: %w", err)
	}
	return nil
}

// WriteReviewState records a human review decision at execution level.
// If req.EpisodeID is non-empty the intervention is written to that specific
// episode; otherwise the most recently updated episode for the execution is
// used as the target (legacy fallback, suitable for single-episode executions).
// The status field is mapped to a HumanInterventionAction as follows:
//
//	"approved"   → HumanActionResumed
//	"aborted"    → HumanActionAborted
//	"overridden" → HumanActionStateOverride
//	other        → HumanActionResumed (safe default)
func (w *EpisodeWriter) WriteReviewState(
	ctx context.Context,
	execID string,
	req domainEpisode.ReviewActionInput,
) error {
	eps, err := w.store.ListByExecution(ctx, execID)
	if err != nil {
		return fmt.Errorf("episode_writer.WriteReviewState list: %w", err)
	}
	if len(eps) == 0 {
		return fmt.Errorf("episode_writer.WriteReviewState: no episodes found for exec %s", execID)
	}

	// Select target episode: precise by ID, or most recently updated as fallback.
	var target *models.Episode
	if req.EpisodeID != "" {
		for _, ep := range eps {
			if ep.ID == req.EpisodeID {
				target = ep
				break
			}
		}
		if target == nil {
			return fmt.Errorf("episode_writer.WriteReviewState: episode %s not found in exec %s", req.EpisodeID, execID)
		}
	} else {
		target = eps[0]
		for _, ep := range eps[1:] {
			if ep.UpdatedAt.After(target.UpdatedAt) {
				target = ep
			}
		}
	}

	unlock := w.lockEpisode(target.ID)
	defer unlock()

	target, err = w.store.Get(ctx, target.ID)
	if err != nil {
		return fmt.Errorf("episode_writer.WriteReviewState get: %w", err)
	}

	now := time.Now().UTC()
	action := domainEpisode.ReviewStatusToAction(req.Status)
	intervention := models.HumanIntervention{
		NodeID:    "",
		Actor:     req.Actor,
		Action:    action,
		Detail:    req.Note,
		Timestamp: now,
	}
	target.HumanInterventions = append(target.HumanInterventions, intervention)

	// CR-013: update Episode.Status and ConcludedAt to reflect the review decision
	// so that the display layer can show the correct state without re-deriving it.
	mutation := domainEpisode.ReviewMutationFromStatus(req.Status, target.Status)
	if mutation.NewStatus != "" {
		target.Status = mutation.NewStatus
	}
	if mutation.SetConcluded {
		target.ConcludedAt = &now
	}
	if mutation.ApplyConclusion {
		// Surface the reviewer note as the verdict conclusion when none exists yet.
		if req.Note != "" && target.Verdict != nil && target.Verdict.Conclusion == "" {
			target.Verdict.Conclusion = req.Note
		}
	}

	target.UpdatedAt = now
	if err := w.store.Update(ctx, target); err != nil {
		return fmt.Errorf("episode_writer.WriteReviewState update: %w", err)
	}
	return nil
}

// AppendReplayTimelineRange annotates an existing evidence entry (identified by
// factID) with an explicit replay range [start, end].  The range is stored in
// the evidence's CollectorSpec params under "_replay_start" / "_replay_end".
// If the evidence entry does not exist the call returns an error.
func (w *EpisodeWriter) AppendReplayTimelineRange(
	ctx context.Context,
	episodeID string,
	factID string,
	start, end int,
) error {
	unlock := w.lockEpisode(episodeID)
	defer unlock()

	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.AppendReplayTimelineRange get: %w", err)
	}
	for i, ev := range ep.Evidence {
		if ev.ID != factID {
			continue
		}
		if ep.Evidence[i].CollectorSpec == nil {
			ep.Evidence[i].CollectorSpec = &models.EvidenceCollectorSpec{
				CollectorType: "replay_annotation",
				Params:        make(map[string]interface{}),
			}
		}
		if ep.Evidence[i].CollectorSpec.Params == nil {
			ep.Evidence[i].CollectorSpec.Params = make(map[string]interface{})
		}
		ep.Evidence[i].CollectorSpec.Params["_replay_start"] = start
		ep.Evidence[i].CollectorSpec.Params["_replay_end"] = end
		ep.UpdatedAt = time.Now().UTC()
		if err := w.store.Update(ctx, ep); err != nil {
			return fmt.Errorf("episode_writer.AppendReplayTimelineRange update: %w", err)
		}
		return nil
	}
	return fmt.Errorf("episode_writer.AppendReplayTimelineRange: fact %s not found in episode %s", factID, episodeID)
}
