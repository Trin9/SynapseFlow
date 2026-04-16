package store

import (
	"context"
	"testing"
	"time"

	"github.com/xunchenzheng/synapse/internal/audit"
	"github.com/xunchenzheng/synapse/pkg/models"
)

func TestMemoryDAGStoreCRUD(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDAGStore()
	dag := &models.DAGConfig{
		ID:        "dag-1",
		Name:      "test",
		Nodes:     []models.Node{{ID: "a", Name: "A", Type: models.NodeTypeScript, Action: "echo hi"}},
		Edges:     []models.Edge{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.Create(ctx, dag); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.Get(ctx, dag.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != dag.Name {
		t.Fatalf("expected name %q, got %q", dag.Name, got.Name)
	}
	got.Name = "updated"
	got.UpdatedAt = time.Now()
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "updated" {
		t.Fatalf("unexpected list: %+v", list)
	}
	if err := s.Delete(ctx, dag.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, dag.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMemoryExecutionStoreCheckpoint(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryExecutionStore()
	state := models.NewGlobalState()
	state.Set("alert_type", "oom")
	state.IncrementLoopCount("node-a")
	exec := &models.Execution{
		ID:        "exec-1",
		DAGID:     "dag-1",
		DAGName:   "test",
		Status:    models.StatusSuspended,
		State:     state,
		StartedAt: time.Now(),
	}
	if err := s.Create(ctx, exec); err != nil {
		t.Fatalf("create: %v", err)
	}
	results := []models.NodeResult{{NodeID: "node-a", NodeName: "A", NodeType: models.NodeTypeHuman, Status: string(models.StatusSuspended)}}
	if err := s.SaveNodeResults(ctx, exec.ID, results); err != nil {
		t.Fatalf("save node results: %v", err)
	}
	checkpoint := &models.ExecutionCheckpoint{
		ExecutionID: exec.ID,
		DAGID:       exec.DAGID,
		State:       state.Snapshot(),
		LoopCounts:  state.LoopCountsSnapshot(),
		UpdatedAt:   time.Now(),
	}
	if err := s.SaveCheckpoint(ctx, checkpoint); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	loaded, err := s.Get(ctx, exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if len(loaded.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(loaded.Results))
	}
	storedCheckpoint, err := s.GetCheckpoint(ctx, exec.ID)
	if err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if storedCheckpoint.State["alert_type"] != "oom" {
		t.Fatalf("unexpected checkpoint state: %+v", storedCheckpoint.State)
	}
	if storedCheckpoint.LoopCounts["node-a"] != 1 {
		t.Fatalf("unexpected loop count snapshot: %+v", storedCheckpoint.LoopCounts)
	}
}

func TestMemoryAuditStore(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryAuditStore()
	entry := audit.Entry{Actor: "alice", Action: "create_dag", Resource: "dag", Result: "success", Time: time.Now()}
	if err := s.Record(ctx, entry); err != nil {
		t.Fatalf("record: %v", err)
	}
	entries, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 || entries[0].Actor != "alice" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}
