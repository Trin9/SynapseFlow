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
	"time"

	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
	"github.com/google/uuid"
)

// EpisodeWriter wraps an EpisodeStore and enforces per-node-type write rules.
type EpisodeWriter struct {
	store store.EpisodeStore
}

// NewEpisodeWriter creates an EpisodeWriter backed by the given store.
func NewEpisodeWriter(s store.EpisodeStore) *EpisodeWriter {
	return &EpisodeWriter{store: s}
}

// ---------------------------------------------------------------------------
// Hard Node — append a fact evidence entry
// ---------------------------------------------------------------------------

// AppendFact appends a new EvidenceTypeFact entry to the Episode.
// Only Hard Nodes (script, mcp, web_interaction) should call this.
func (w *EpisodeWriter) AppendFact(
	ctx context.Context,
	episodeID string,
	nodeID string,
	nodeType models.NodeType,
	label string,
	content string,
) error {
	ep, err := w.store.Get(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode_writer.AppendFact get: %w", err)
	}
	ev := models.EpisodeEvidence{
		ID:          uuid.New().String(),
		Type:        models.EvidenceTypeFact,
		NodeID:      nodeID,
		NodeType:    nodeType,
		Label:       label,
		Content:     content,
		CollectedAt: time.Now().UTC(),
	}
	ep.Evidence = append(ep.Evidence, ev)
	ep.UpdatedAt = time.Now().UTC()
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.AppendFact update: %w", err)
	}
	return nil
}

// AppendFactWithRef is the large-payload variant: stores an artifact and
// records a ContentRef instead of inlining the content.
func (w *EpisodeWriter) AppendFactWithRef(
	ctx context.Context,
	episodeID string,
	nodeID string,
	nodeType models.NodeType,
	label string,
	contentType string,
	content string,
) error {
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
		Type:        models.EvidenceTypeFact,
		NodeID:      nodeID,
		NodeType:    nodeType,
		Label:       label,
		ContentRef:  storageURI,
		CollectedAt: time.Now().UTC(),
	}
	ep.Evidence = append(ep.Evidence, ev)
	ep.UpdatedAt = time.Now().UTC()
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.AppendFactWithRef update: %w", err)
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
func (w *EpisodeWriter) WriteVerdict(
	ctx context.Context,
	episodeID string,
	nodeID string,
	verdict models.EpisodeVerdict,
) error {
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
	ep.UpdatedAt = now
	if err := w.store.Update(ctx, ep); err != nil {
		return fmt.Errorf("episode_writer.WriteVerdict update: %w", err)
	}
	return nil
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
