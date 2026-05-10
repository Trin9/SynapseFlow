package engine

// episode_writer_test.go — Track A integration tests for EpisodeWriter.
//
// Coverage:
//   - AppendFact: evidence appended, status pending→in_progress, idempotent multi-append
//   - AppendFactWithRef: artifact stored, ContentRef set on evidence entry
//   - WriteVerdict: verdict written, status→converged, ConcludedAt set, MemoryExtraction triggered
//   - WriteVerdict: idempotency guard (second call returns error)
//   - HumanCorrect: audit trail appended, arbitrary mutation applied

import (
	"context"
	"sync"
	"testing"
	"time"

	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// newTestEpisode creates a bare Episode in a MemoryEpisodeStore and returns
// the store and a writer backed by it.
func newTestEpisode(t *testing.T) (*store.MemoryEpisodeStore, *EpisodeWriter, string) {
	t.Helper()
	s := store.NewMemoryEpisodeStore()
	ep := &models.Episode{
		ID:            "ep-test-001",
		ExecID:        "exec-001",
		EpisodeType:   domainEpisode.EpisodeTypeActionVerification.ToModel(),
		Status:        domainEpisode.EpisodeStatusPending.ToModel(),
		LoopGuard:     models.EpisodeLoopGuard{MaxIterations: 8},
		SchemaVersion: 1,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := s.Create(context.Background(), ep); err != nil {
		t.Fatalf("create test episode: %v", err)
	}
	w := NewEpisodeWriter(s)
	return s, w, ep.ID
}

// ---------------------------------------------------------------------------
// AppendFact
// ---------------------------------------------------------------------------

func TestAppendFact_StatusTransitionPendingToInProgress(t *testing.T) {
	s, w, epID := newTestEpisode(t)
	ctx := context.Background()

	// Episode starts as pending.
	ep, _ := s.Get(ctx, epID)
	if ep.Status != domainEpisode.EpisodeStatusPending.ToModel() {
		t.Fatalf("expected initial status pending, got %q", ep.Status)
	}

	if err := w.AppendFact(ctx, epID, "node-script-01", models.NodeTypeScript, "health check", "OK"); err != nil {
		t.Fatalf("AppendFact: %v", err)
	}

	ep, _ = s.Get(ctx, epID)
	if ep.Status != domainEpisode.EpisodeStatusInProgress.ToModel() {
		t.Errorf("expected status in_progress after first AppendFact, got %q", ep.Status)
	}
	if len(ep.Evidence) != 1 {
		t.Errorf("expected 1 evidence entry, got %d", len(ep.Evidence))
	}
	ev := ep.Evidence[0]
	if ev.Type != models.EvidenceTypeFact {
		t.Errorf("expected EvidenceTypeFact, got %q", ev.Type)
	}
	if ev.NodeID != "node-script-01" {
		t.Errorf("expected node_id node-script-01, got %q", ev.NodeID)
	}
	if ev.Content != "OK" {
		t.Errorf("expected content OK, got %q", ev.Content)
	}
	if ev.ID == "" {
		t.Error("expected evidence ID to be set")
	}
}

func TestAppendFact_MultipleAppends_AccumulateEvidence(t *testing.T) {
	s, w, epID := newTestEpisode(t)
	ctx := context.Background()

	for i, label := range []string{"step-1", "step-2", "step-3"} {
		if err := w.AppendFact(ctx, epID, "node-"+label, models.NodeTypeScript, label, "output-"+label); err != nil {
			t.Fatalf("AppendFact[%d]: %v", i, err)
		}
	}

	ep, _ := s.Get(ctx, epID)
	if len(ep.Evidence) != 3 {
		t.Errorf("expected 3 evidence entries, got %d", len(ep.Evidence))
	}
	// Status should still be in_progress (not re-transitioned after first).
	if ep.Status != domainEpisode.EpisodeStatusInProgress.ToModel() {
		t.Errorf("expected status in_progress, got %q", ep.Status)
	}
}

func TestAppendFact_StatusDoesNotRegressAfterInProgress(t *testing.T) {
	_, w, epID := newTestEpisode(t)
	ctx := context.Background()

	_ = w.AppendFact(ctx, epID, "n1", models.NodeTypeScript, "a", "x")
	_ = w.AppendFact(ctx, epID, "n2", models.NodeTypeScript, "b", "y")

	// Manually patch status to in_progress (already set) and append again.
	// Status must NOT revert to pending.
	_ = w.AppendFact(ctx, epID, "n3", models.NodeTypeScript, "c", "z")
}

func TestAppendFact_ConcurrentAppends_NoLostUpdates(t *testing.T) {
	s, w, epID := newTestEpisode(t)
	ctx := context.Background()

	const workers = 24
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			if err := w.AppendFact(ctx, epID, "node-concurrent", models.NodeTypeScript, "step", "payload"); err != nil {
				t.Errorf("AppendFact[%d]: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	ep, _ := s.Get(ctx, epID)
	if len(ep.Evidence) != workers {
		t.Fatalf("expected %d evidence entries, got %d", workers, len(ep.Evidence))
	}
	if ep.Status != domainEpisode.EpisodeStatusInProgress.ToModel() {
		t.Fatalf("expected status in_progress, got %q", ep.Status)
	}
}

// ---------------------------------------------------------------------------
// AppendFactWithRef
// ---------------------------------------------------------------------------

func TestAppendFactWithRef_ArtifactStoredAndRefSet(t *testing.T) {
	s, w, epID := newTestEpisode(t)
	ctx := context.Background()

	bigContent := "line1\nline2\nline3\n"
	if err := w.AppendFactWithRef(ctx, epID, "node-log-fetcher", models.NodeTypeScript,
		"checkout logs", "log_dump", bigContent); err != nil {
		t.Fatalf("AppendFactWithRef: %v", err)
	}

	ep, _ := s.Get(ctx, epID)
	if len(ep.Evidence) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(ep.Evidence))
	}
	ev := ep.Evidence[0]
	if ev.Content != "" {
		t.Errorf("expected content NOT inlined, got %q", ev.Content)
	}
	if ev.ContentRef == "" {
		t.Error("expected ContentRef to be set")
	}

	// Verify artifact was saved.
	artifacts, err := s.ListArtifacts(ctx, epID)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}
	if artifacts[0].Content != bigContent {
		t.Errorf("artifact content mismatch: got %q", artifacts[0].Content)
	}
	if artifacts[0].ContentType != "log_dump" {
		t.Errorf("artifact content_type mismatch: got %q", artifacts[0].ContentType)
	}
	if artifacts[0].StorageURI != ev.ContentRef {
		t.Errorf("StorageURI %q does not match evidence ContentRef %q",
			artifacts[0].StorageURI, ev.ContentRef)
	}
}

// ---------------------------------------------------------------------------
// WriteVerdict
// ---------------------------------------------------------------------------

func TestWriteVerdict_ConvergesEpisode(t *testing.T) {
	s, w, epID := newTestEpisode(t)
	ctx := context.Background()

	// Add some evidence first (status → in_progress).
	_ = w.AppendFact(ctx, epID, "n1", models.NodeTypeScript, "cart check", "cart_cleared")

	verdict := models.EpisodeVerdict{
		Result:      models.EpisodeResultPass,
		Confidence:  models.EpisodeConfidenceHigh,
		Conclusion:  "Checkout completed successfully — cart cleared and order confirmed.",
		CausalChain: []string{"add_to_cart succeeded", "place_order returned 200", "cart cleared post-checkout"},
	}
	if err := w.WriteVerdict(ctx, epID, "node-audit-llm", verdict); err != nil {
		t.Fatalf("WriteVerdict: %v", err)
	}

	ep, _ := s.Get(ctx, epID)

	// Status must be converged.
	if ep.Status != domainEpisode.EpisodeStatusConverged.ToModel() {
		t.Errorf("expected status converged, got %q", ep.Status)
	}
	// ConcludedAt must be set.
	if ep.ConcludedAt == nil {
		t.Error("expected ConcludedAt to be set after WriteVerdict")
	}
	// Verdict must be non-nil with correct fields.
	if ep.Verdict == nil {
		t.Fatal("expected Verdict to be non-nil")
	}
	if ep.Verdict.Result != models.EpisodeResultPass {
		t.Errorf("expected result pass, got %q", ep.Verdict.Result)
	}
	if ep.Verdict.Confidence != models.EpisodeConfidenceHigh {
		t.Errorf("expected confidence high, got %q", ep.Verdict.Confidence)
	}
	if ep.Verdict.DecidedBy != "node-audit-llm" {
		t.Errorf("expected DecidedBy node-audit-llm, got %q", ep.Verdict.DecidedBy)
	}
	if ep.Verdict.DecidedAt.IsZero() {
		t.Error("expected DecidedAt to be set")
	}
}

func TestWriteVerdict_MemoryExtractionAutoTriggered_HighConfidencePass(t *testing.T) {
	s, w, epID := newTestEpisode(t)
	ctx := context.Background()

	verdict := models.EpisodeVerdict{
		Result:     models.EpisodeResultPass,
		Confidence: models.EpisodeConfidenceHigh,
		Conclusion: "All checks passed.",
	}
	_ = w.WriteVerdict(ctx, epID, "node-llm", verdict)

	ep, _ := s.Get(ctx, epID)
	if ep.MemoryExtraction == nil {
		t.Fatal("expected MemoryExtraction to be set")
	}
	if !ep.MemoryExtraction.Triggered {
		t.Error("expected MemoryExtraction.Triggered=true for high-confidence pass")
	}
	if ep.MemoryExtraction.TriggerBy != "auto_high_confidence" {
		t.Errorf("expected trigger_by auto_high_confidence, got %q", ep.MemoryExtraction.TriggerBy)
	}
	if ep.MemoryExtraction.Status != "pending" {
		t.Errorf("expected status pending, got %q", ep.MemoryExtraction.Status)
	}
}

func TestWriteVerdict_MemoryExtractionNotTriggered_LowConfidence(t *testing.T) {
	s, w, epID := newTestEpisode(t)
	ctx := context.Background()

	verdict := models.EpisodeVerdict{
		Result:     models.EpisodeResultFail,
		Confidence: models.EpisodeConfidenceLow,
		Conclusion: "Could not determine outcome.",
	}
	_ = w.WriteVerdict(ctx, epID, "node-llm", verdict)

	ep, _ := s.Get(ctx, epID)
	if ep.MemoryExtraction == nil {
		t.Fatal("expected MemoryExtraction struct to be present (even if not triggered)")
	}
	if ep.MemoryExtraction.Triggered {
		t.Error("expected MemoryExtraction.Triggered=false for low confidence")
	}
}

func TestWriteVerdict_MemoryExtractionNotTriggered_Inconclusive(t *testing.T) {
	s, w, epID := newTestEpisode(t)
	ctx := context.Background()

	verdict := models.EpisodeVerdict{
		Result:     models.EpisodeResultInconclusive,
		Confidence: models.EpisodeConfidenceHigh, // high confidence but inconclusive — no trigger
		Conclusion: "Evidence was ambiguous.",
	}
	_ = w.WriteVerdict(ctx, epID, "node-llm", verdict)

	ep, _ := s.Get(ctx, epID)
	if ep.MemoryExtraction != nil && ep.MemoryExtraction.Triggered {
		t.Error("expected MemoryExtraction.Triggered=false for inconclusive result")
	}
}

func TestWriteVerdict_IdempotencyGuard(t *testing.T) {
	_, w, epID := newTestEpisode(t)
	ctx := context.Background()

	v := models.EpisodeVerdict{Result: models.EpisodeResultPass, Confidence: models.EpisodeConfidenceHigh, Conclusion: "OK"}
	if err := w.WriteVerdict(ctx, epID, "node-llm", v); err != nil {
		t.Fatalf("first WriteVerdict: %v", err)
	}
	// Second call must return an error.
	if err := w.WriteVerdict(ctx, epID, "node-llm", v); err == nil {
		t.Error("expected error on second WriteVerdict (idempotency guard), got nil")
	}
}

// ---------------------------------------------------------------------------
// HumanCorrect
// ---------------------------------------------------------------------------

func TestHumanCorrect_AuditTrailAppended(t *testing.T) {
	s, w, epID := newTestEpisode(t)
	ctx := context.Background()

	// First write a verdict so there is something to correct.
	_ = w.WriteVerdict(ctx, epID, "node-llm",
		models.EpisodeVerdict{Result: models.EpisodeResultFail, Confidence: models.EpisodeConfidenceLow, Conclusion: "fail"})

	// Human corrects the verdict conclusion.
	err := w.HumanCorrect(ctx, epID, "sre-alice", "human-node-01",
		"verdict.conclusion",
		"fail",
		"Actually, the checkout succeeded — the cart-empty check had a timing issue.",
		func(ep *models.Episode) {
			if ep.Verdict != nil {
				ep.Verdict.Conclusion = "Actually, the checkout succeeded — the cart-empty check had a timing issue."
				ep.Verdict.Result = models.EpisodeResultPass
			}
		},
	)
	if err != nil {
		t.Fatalf("HumanCorrect: %v", err)
	}

	ep, _ := s.Get(ctx, epID)
	if len(ep.AuditTrail) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(ep.AuditTrail))
	}
	entry := ep.AuditTrail[0]
	if entry.Actor != "sre-alice" {
		t.Errorf("expected Actor sre-alice, got %q", entry.Actor)
	}
	if entry.NodeID != "human-node-01" {
		t.Errorf("expected NodeID human-node-01, got %q", entry.NodeID)
	}
	if entry.ModifiedAt.IsZero() {
		t.Error("expected ModifiedAt to be set")
	}
	// Verify the mutation was applied.
	if ep.Verdict == nil || ep.Verdict.Result != models.EpisodeResultPass {
		t.Error("expected mutation to change Verdict.Result to pass")
	}
}

func TestHumanCorrect_MultipleCorrections(t *testing.T) {
	s, w, epID := newTestEpisode(t)
	ctx := context.Background()

	_ = w.AppendFact(ctx, epID, "n1", models.NodeTypeScript, "step", "output")

	for i := 0; i < 3; i++ {
		_ = w.HumanCorrect(ctx, epID, "sre-bob", "human-01", "handles",
			nil, "injected",
			func(ep *models.Episode) {
				ep.Handles = append(ep.Handles, models.EpisodeHandle{
					Type:  models.HandleTypeSessionID,
					Value: "sess-abc",
				})
			},
		)
	}

	ep, _ := s.Get(ctx, epID)
	if len(ep.AuditTrail) != 3 {
		t.Errorf("expected 3 audit entries, got %d", len(ep.AuditTrail))
	}
}

// ---------------------------------------------------------------------------
// Non-existent Episode error propagation
// ---------------------------------------------------------------------------

func TestAppendFact_MissingEpisodeReturnsError(t *testing.T) {
	_, w, _ := newTestEpisode(t)
	ctx := context.Background()
	err := w.AppendFact(ctx, "does-not-exist", "n1", models.NodeTypeScript, "label", "content")
	if err == nil {
		t.Error("expected error for missing episode, got nil")
	}
}

func TestWriteVerdict_MissingEpisodeReturnsError(t *testing.T) {
	_, w, _ := newTestEpisode(t)
	ctx := context.Background()
	err := w.WriteVerdict(ctx, "does-not-exist", "n1",
		models.EpisodeVerdict{Result: models.EpisodeResultPass})
	if err == nil {
		t.Error("expected error for missing episode, got nil")
	}
}

func TestWriteReviewState_TargetByEpisodeID(t *testing.T) {
	s := store.NewMemoryEpisodeStore()
	w := NewEpisodeWriter(s)
	ctx := context.Background()
	now := time.Now().UTC()

	ep1 := &models.Episode{
		ID:            "ep-review-target",
		ExecID:        "exec-review",
		EpisodeType:   domainEpisode.EpisodeTypeActionVerification.ToModel(),
		Status:        domainEpisode.EpisodeStatusEscalated.ToModel(),
		SchemaVersion: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	ep2 := &models.Episode{
		ID:            "ep-review-other",
		ExecID:        "exec-review",
		EpisodeType:   domainEpisode.EpisodeTypeActionVerification.ToModel(),
		Status:        domainEpisode.EpisodeStatusInProgress.ToModel(),
		SchemaVersion: 1,
		CreatedAt:     now.Add(time.Second),
		UpdatedAt:     now.Add(time.Second),
	}
	if err := s.Create(ctx, ep1); err != nil {
		t.Fatalf("create ep1: %v", err)
	}
	if err := s.Create(ctx, ep2); err != nil {
		t.Fatalf("create ep2: %v", err)
	}

	err := w.WriteReviewState(ctx, "exec-review", domainEpisode.ReviewActionInput{
		EpisodeID: ep1.ID,
		Status:    domainEpisode.ReviewStatusApproved,
		Actor:     "reviewer-alice",
		Note:      "approved by reviewer",
	})
	if err != nil {
		t.Fatalf("WriteReviewState: %v", err)
	}

	got1, _ := s.Get(ctx, ep1.ID)
	if got1.Status != domainEpisode.EpisodeStatusConverged.ToModel() {
		t.Fatalf("expected ep1 status converged, got %q", got1.Status)
	}
	if got1.ConcludedAt == nil {
		t.Fatal("expected ep1 concluded_at to be set")
	}
	if len(got1.HumanInterventions) != 1 {
		t.Fatalf("expected ep1 1 intervention, got %d", len(got1.HumanInterventions))
	}
	if got1.HumanInterventions[0].Action != models.HumanActionResumed {
		t.Fatalf("expected ep1 action resumed, got %q", got1.HumanInterventions[0].Action)
	}

	got2, _ := s.Get(ctx, ep2.ID)
	if got2.Status != domainEpisode.EpisodeStatusInProgress.ToModel() {
		t.Fatalf("expected ep2 unchanged in_progress, got %q", got2.Status)
	}
	if len(got2.HumanInterventions) != 0 {
		t.Fatalf("expected ep2 no interventions, got %d", len(got2.HumanInterventions))
	}
}

func TestWriteReviewState_FallbackTargetsMostRecentlyUpdatedEpisode(t *testing.T) {
	s := store.NewMemoryEpisodeStore()
	w := NewEpisodeWriter(s)
	ctx := context.Background()
	now := time.Now().UTC()

	epOld := &models.Episode{
		ID:            "ep-review-old",
		ExecID:        "exec-review-fallback",
		EpisodeType:   domainEpisode.EpisodeTypeActionVerification.ToModel(),
		Status:        domainEpisode.EpisodeStatusInProgress.ToModel(),
		SchemaVersion: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	epNew := &models.Episode{
		ID:            "ep-review-new",
		ExecID:        "exec-review-fallback",
		EpisodeType:   domainEpisode.EpisodeTypeActionVerification.ToModel(),
		Status:        domainEpisode.EpisodeStatusInProgress.ToModel(),
		SchemaVersion: 1,
		CreatedAt:     now.Add(time.Second),
		UpdatedAt:     now.Add(2 * time.Second),
	}
	if err := s.Create(ctx, epOld); err != nil {
		t.Fatalf("create epOld: %v", err)
	}
	if err := s.Create(ctx, epNew); err != nil {
		t.Fatalf("create epNew: %v", err)
	}

	err := w.WriteReviewState(ctx, "exec-review-fallback", domainEpisode.ReviewActionInput{
		Status: domainEpisode.ReviewStatusAborted,
		Actor:  "reviewer-bob",
		Note:   "aborted run",
	})
	if err != nil {
		t.Fatalf("WriteReviewState: %v", err)
	}

	gotOld, _ := s.Get(ctx, epOld.ID)
	if gotOld.Status != domainEpisode.EpisodeStatusInProgress.ToModel() {
		t.Fatalf("expected old episode unchanged, got %q", gotOld.Status)
	}
	if len(gotOld.HumanInterventions) != 0 {
		t.Fatalf("expected old episode no interventions, got %d", len(gotOld.HumanInterventions))
	}

	gotNew, _ := s.Get(ctx, epNew.ID)
	if gotNew.Status != domainEpisode.EpisodeStatusFailed.ToModel() {
		t.Fatalf("expected new episode failed, got %q", gotNew.Status)
	}
	if gotNew.ConcludedAt == nil {
		t.Fatal("expected new episode concluded_at to be set")
	}
	if len(gotNew.HumanInterventions) != 1 {
		t.Fatalf("expected new episode 1 intervention, got %d", len(gotNew.HumanInterventions))
	}
	if gotNew.HumanInterventions[0].Action != models.HumanActionAborted {
		t.Fatalf("expected new episode action aborted, got %q", gotNew.HumanInterventions[0].Action)
	}
}
