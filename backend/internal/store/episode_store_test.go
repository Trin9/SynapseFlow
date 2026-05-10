package store

// episode_store_test.go — Track A unit tests for MemoryEpisodeStore.
//
// Coverage:
//   - Create / Get / Update / ListByExecution
//   - Cloning isolation (mutations after store.Get do NOT affect stored state)
//   - SaveArtifact / ListArtifacts
//   - ErrNotFound on missing IDs

import (
	"context"
	"testing"
	"time"

	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

func baseEpisode(id, execID string) *models.Episode {
	return &models.Episode{
		ID:            id,
		ExecID:        execID,
		EpisodeType:   domainEpisode.EpisodeTypeActionVerification.ToModel(),
		Status:        domainEpisode.EpisodeStatusPending.ToModel(),
		LoopGuard:     models.EpisodeLoopGuard{MaxIterations: 8},
		SchemaVersion: 1,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
}

// ---------------------------------------------------------------------------
// Basic CRUD
// ---------------------------------------------------------------------------

func TestMemoryEpisodeStore_CreateGet(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()

	ep := baseEpisode("ep-001", "exec-001")
	if err := s.Create(ctx, ep); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, ep.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != ep.ID {
		t.Errorf("ID mismatch: got %q want %q", got.ID, ep.ID)
	}
	if got.Status != domainEpisode.EpisodeStatusPending.ToModel() {
		t.Errorf("Status mismatch: got %q", got.Status)
	}
}

func TestMemoryEpisodeStore_GetNotFound(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()
	_, err := s.Get(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryEpisodeStore_Update(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()

	ep := baseEpisode("ep-002", "exec-001")
	_ = s.Create(ctx, ep)

	// Transition to in_progress.
	ep.Status = domainEpisode.EpisodeStatusInProgress.ToModel()
	ep.Evidence = append(ep.Evidence, models.EpisodeEvidence{
		ID: "ev-01", Type: models.EvidenceTypeFact, NodeID: "n1",
		NodeType: models.NodeTypeScript, Label: "check", Content: "ok",
		CollectedAt: time.Now().UTC(),
	})
	ep.UpdatedAt = time.Now().UTC()

	if err := s.Update(ctx, ep); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := s.Get(ctx, ep.ID)
	if got.Status != domainEpisode.EpisodeStatusInProgress.ToModel() {
		t.Errorf("expected in_progress, got %q", got.Status)
	}
	if len(got.Evidence) != 1 {
		t.Errorf("expected 1 evidence, got %d", len(got.Evidence))
	}
}

func TestMemoryEpisodeStore_UpdateNotFound(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()
	ep := baseEpisode("nonexistent", "exec-001")
	if err := s.Update(ctx, ep); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListByExecution
// ---------------------------------------------------------------------------

func TestMemoryEpisodeStore_ListByExecution(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()

	ep1 := baseEpisode("ep-a", "exec-X")
	ep1.CreatedAt = time.Now().UTC()
	ep2 := baseEpisode("ep-b", "exec-X")
	ep2.CreatedAt = ep1.CreatedAt.Add(time.Second)
	ep3 := baseEpisode("ep-c", "exec-Y") // different execution

	_ = s.Create(ctx, ep1)
	_ = s.Create(ctx, ep2)
	_ = s.Create(ctx, ep3)

	list, err := s.ListByExecution(ctx, "exec-X")
	if err != nil {
		t.Fatalf("ListByExecution: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 episodes for exec-X, got %d", len(list))
	}
	// Results must be sorted by CreatedAt ascending.
	if list[0].ID != "ep-a" || list[1].ID != "ep-b" {
		t.Errorf("unexpected order: %q %q", list[0].ID, list[1].ID)
	}

	// exec-Y should have exactly 1.
	listY, _ := s.ListByExecution(ctx, "exec-Y")
	if len(listY) != 1 {
		t.Errorf("expected 1 episode for exec-Y, got %d", len(listY))
	}

	// Unknown exec returns empty slice (not error).
	listZ, err := s.ListByExecution(ctx, "exec-Z")
	if err != nil {
		t.Errorf("unexpected error for unknown exec: %v", err)
	}
	if len(listZ) != 0 {
		t.Errorf("expected 0 episodes for exec-Z, got %d", len(listZ))
	}
}

// ---------------------------------------------------------------------------
// Isolation — mutations after Get must not affect stored state
// ---------------------------------------------------------------------------

func TestMemoryEpisodeStore_IsolationAfterGet(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()

	ep := baseEpisode("ep-iso", "exec-001")
	_ = s.Create(ctx, ep)

	got, _ := s.Get(ctx, ep.ID)
	// Mutate the returned copy.
	got.Status = domainEpisode.EpisodeStatusConverged.ToModel()
	got.Evidence = append(got.Evidence, models.EpisodeEvidence{ID: "injected"})

	// Re-fetch; mutations must not have propagated.
	got2, _ := s.Get(ctx, ep.ID)
	if got2.Status != domainEpisode.EpisodeStatusPending.ToModel() {
		t.Errorf("mutation leaked into store: status=%q", got2.Status)
	}
	if len(got2.Evidence) != 0 {
		t.Errorf("mutation leaked into store: evidence count=%d", len(got2.Evidence))
	}
}

// ---------------------------------------------------------------------------
// Verdict round-trip including new fields (Result, Confidence string, Recommendations)
// ---------------------------------------------------------------------------

func TestMemoryEpisodeStore_VerdictRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()

	ep := baseEpisode("ep-v", "exec-001")
	_ = s.Create(ctx, ep)

	now := time.Now().UTC()
	ep.Verdict = &models.EpisodeVerdict{
		Result:          models.EpisodeResultPass,
		Confidence:      models.EpisodeConfidenceHigh,
		Conclusion:      "All checks passed.",
		CausalChain:     []string{"add_to_cart ok", "place_order ok"},
		Gaps:            []string{},
		Recommendations: []string{"monitor cart-clear latency"},
		DecidedBy:       "node-llm",
		DecidedAt:       now,
	}
	ep.Status = domainEpisode.EpisodeStatusConverged.ToModel()
	ep.ConcludedAt = &now
	ep.UpdatedAt = now
	_ = s.Update(ctx, ep)

	got, _ := s.Get(ctx, ep.ID)
	if got.Verdict == nil {
		t.Fatal("verdict nil after round-trip")
	}
	if got.Verdict.Result != models.EpisodeResultPass {
		t.Errorf("Result: got %q", got.Verdict.Result)
	}
	if got.Verdict.Confidence != models.EpisodeConfidenceHigh {
		t.Errorf("Confidence: got %q", got.Verdict.Confidence)
	}
	if len(got.Verdict.Recommendations) != 1 {
		t.Errorf("Recommendations count: got %d", len(got.Verdict.Recommendations))
	}
	if got.Verdict.Recommendations[0] != "monitor cart-clear latency" {
		t.Errorf("Recommendation value: got %q", got.Verdict.Recommendations[0])
	}
	if got.ConcludedAt == nil {
		t.Error("ConcludedAt nil after round-trip")
	}
}

// ---------------------------------------------------------------------------
// Handles ([]EpisodeHandle)
// ---------------------------------------------------------------------------

func TestMemoryEpisodeStore_HandlesRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()

	ep := baseEpisode("ep-h", "exec-001")
	ep.Handles = []models.EpisodeHandle{
		{Type: models.HandleTypeSessionID, Value: "sess-xyz", Source: "n1", ExtractedAt: time.Now().UTC()},
		{Type: models.HandleTypeOrderID, Value: "order-123", Source: "n2", ExtractedAt: time.Now().UTC()},
	}
	_ = s.Create(ctx, ep)

	got, _ := s.Get(ctx, ep.ID)
	if len(got.Handles) != 2 {
		t.Fatalf("expected 2 handles, got %d", len(got.Handles))
	}
	if got.Handles[0].Type != models.HandleTypeSessionID {
		t.Errorf("handle[0] type: got %q", got.Handles[0].Type)
	}
	if got.Handles[1].Value != "order-123" {
		t.Errorf("handle[1] value: got %q", got.Handles[1].Value)
	}

	// Mutation isolation: mutating the returned slice must not affect stored handles.
	got.Handles[0].Value = "TAMPERED"
	got2, _ := s.Get(ctx, ep.ID)
	if got2.Handles[0].Value == "TAMPERED" {
		t.Error("handle mutation leaked into store")
	}
}

// ---------------------------------------------------------------------------
// HumanInterventions round-trip
// ---------------------------------------------------------------------------

func TestMemoryEpisodeStore_HumanInterventionsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()

	ep := baseEpisode("ep-hi", "exec-001")
	ep.HumanInterventions = []models.HumanIntervention{
		{
			NodeID:    "human-01",
			Actor:     "sre-alice",
			Action:    models.HumanActionStateOverride,
			Detail:    "Overrode cart-check result based on direct DB inspection.",
			Timestamp: time.Now().UTC(),
		},
	}
	_ = s.Create(ctx, ep)

	got, _ := s.Get(ctx, ep.ID)
	if len(got.HumanInterventions) != 1 {
		t.Fatalf("expected 1 HumanIntervention, got %d", len(got.HumanInterventions))
	}
	if got.HumanInterventions[0].Actor != "sre-alice" {
		t.Errorf("Actor: got %q", got.HumanInterventions[0].Actor)
	}
	if got.HumanInterventions[0].Action != models.HumanActionStateOverride {
		t.Errorf("Action: got %q", got.HumanInterventions[0].Action)
	}
}

// ---------------------------------------------------------------------------
// Artifacts
// ---------------------------------------------------------------------------

func TestMemoryEpisodeStore_ArtifactsCRUD(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()

	ep := baseEpisode("ep-art", "exec-001")
	_ = s.Create(ctx, ep)

	a1 := &models.EpisodeArtifact{
		ID:          "art-01",
		EpisodeID:   ep.ID,
		EvidenceID:  "ev-01",
		ContentType: "log_dump",
		SizeBytes:   42,
		StorageURI:  "artifact://exec-001/ev-01",
		Content:     "some log line",
		CreatedAt:   time.Now().UTC(),
	}
	a2 := &models.EpisodeArtifact{
		ID:          "art-02",
		EpisodeID:   ep.ID,
		EvidenceID:  "ev-02",
		ContentType: "api_response",
		SizeBytes:   100,
		StorageURI:  "artifact://exec-001/ev-02",
		Content:     `{"status": "ok"}`,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.SaveArtifact(ctx, a1); err != nil {
		t.Fatalf("SaveArtifact a1: %v", err)
	}
	if err := s.SaveArtifact(ctx, a2); err != nil {
		t.Fatalf("SaveArtifact a2: %v", err)
	}

	list, err := s.ListArtifacts(ctx, ep.ID)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(list))
	}

	// Upsert: save a1 again with updated content.
	a1.Content = "updated log content"
	_ = s.SaveArtifact(ctx, a1)
	list2, _ := s.ListArtifacts(ctx, ep.ID)
	if len(list2) != 2 {
		t.Errorf("upsert should not add a duplicate: got %d artifacts", len(list2))
	}
	for _, a := range list2 {
		if a.ID == "art-01" && a.Content != "updated log content" {
			t.Errorf("upsert did not update content: got %q", a.Content)
		}
	}
}

func TestMemoryEpisodeStore_ListArtifacts_EmptyWhenNone(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()
	ep := baseEpisode("ep-noart", "exec-001")
	_ = s.Create(ctx, ep)

	list, err := s.ListArtifacts(ctx, ep.ID)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 artifacts, got %d", len(list))
	}
}

// ---------------------------------------------------------------------------
// MemoryExtraction round-trip
// ---------------------------------------------------------------------------

func TestMemoryEpisodeStore_MemoryExtractionRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEpisodeStore()

	ep := baseEpisode("ep-mem", "exec-001")
	ep.MemoryExtraction = &models.EpisodeMemoryExtraction{
		Triggered: true,
		TriggerBy: "auto_high_confidence",
		Status:    "pending",
	}
	_ = s.Create(ctx, ep)

	got, _ := s.Get(ctx, ep.ID)
	if got.MemoryExtraction == nil {
		t.Fatal("MemoryExtraction nil after round-trip")
	}
	if !got.MemoryExtraction.Triggered {
		t.Error("MemoryExtraction.Triggered should be true")
	}
	if got.MemoryExtraction.TriggerBy != "auto_high_confidence" {
		t.Errorf("TriggerBy: got %q", got.MemoryExtraction.TriggerBy)
	}
}
