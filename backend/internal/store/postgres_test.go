package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Trin9/SynapseFlow/backend/internal/audit"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// postgresDB opens a real PostgreSQL connection for integration tests.
// Tests that call this function are skipped when SYNAPSE_TEST_DATABASE_URL
// is not set, so the suite still passes in environments without a database.
func postgresDB(t *testing.T) *store.PostgresStores {
	t.Helper()
	dsn := os.Getenv("SYNAPSE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SYNAPSE_TEST_DATABASE_URL not set – skipping postgres integration test")
	}
	db, err := store.OpenPostgres(context.Background(), dsn, 5, 2, 5*time.Minute, 1*time.Hour)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Apply migrations so the schema is up to date.
	migrationsDir := migrationDir(t)
	if err := store.RunMigrations(context.Background(), db, migrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return store.NewPostgresStores(db)
}

// migrationDir locates the migrations directory relative to this test file.
func migrationDir(t *testing.T) string {
	t.Helper()
	// Walk up from the store package to backend root, then into migrations/.
	dir := "../../../../migrations"
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("cannot locate migrations dir at %s: %v", dir, err)
	}
	return dir
}

// ─── DAGStore ──────────────────────────────────────────────────────────────

func TestPostgresDAGStore(t *testing.T) {
	s := postgresDB(t)
	ctx := context.Background()

	dag := &models.DAGConfig{
		ID:          "test-dag-" + uniqueSuffix(),
		Name:        "Integration Test DAG",
		Description: "created by TestPostgresDAGStore",
		Nodes: []models.Node{
			{ID: "n1", Name: "Node 1", Type: models.NodeTypeScript, Action: "echo hello"},
		},
		Edges:     []models.Edge{{From: "n1", To: "n1"}},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	t.Cleanup(func() { _ = s.DAGs.Delete(ctx, dag.ID) })

	// Create
	if err := s.DAGs.Create(ctx, dag); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Get
	got, err := s.DAGs.Get(ctx, dag.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != dag.Name {
		t.Errorf("Name mismatch: got %q want %q", got.Name, dag.Name)
	}

	// Update
	dag.Name = "Updated DAG"
	dag.UpdatedAt = time.Now().UTC()
	if err := s.DAGs.Update(ctx, dag); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = s.DAGs.Get(ctx, dag.ID)
	if got.Name != "Updated DAG" {
		t.Errorf("updated Name mismatch: got %q", got.Name)
	}

	// List
	list, err := s.DAGs.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, d := range list {
		if d.ID == dag.ID {
			found = true
		}
	}
	if !found {
		t.Error("created DAG not found in List")
	}

	// Delete
	if err := s.DAGs.Delete(ctx, dag.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.DAGs.Get(ctx, dag.ID); err == nil {
		t.Error("expected error after Delete, got nil")
	}
}

// ─── ExecutionStore ────────────────────────────────────────────────────────

func TestPostgresExecutionStore(t *testing.T) {
	s := postgresDB(t)
	ctx := context.Background()

	// We need a parent DAG row because executions reference dag_id (soft ref).
	dag := &models.DAGConfig{
		ID: "exec-test-dag-" + uniqueSuffix(), Name: "Exec Test DAG",
		Nodes: []models.Node{{ID: "n1", Name: "N1", Type: models.NodeTypeScript}},
		Edges: []models.Edge{}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	_ = s.DAGs.Create(ctx, dag)
	t.Cleanup(func() { _ = s.DAGs.Delete(ctx, dag.ID) })

	exec := &models.Execution{
		ID:        "exec-" + uniqueSuffix(),
		DAGID:     dag.ID,
		DAGName:   dag.Name,
		Status:    models.StatusRunning,
		StartedAt: time.Now().UTC(),
		State:     models.NewGlobalState(),
	}
	t.Cleanup(func() {
		// node_executions cascade on delete; no explicit cleanup needed beyond exec.
		_ = s.Executions.Update(ctx, func() *models.Execution {
			exec.Status = models.StatusFailed
			return exec
		}())
	})

	// Create
	if err := s.Executions.Create(ctx, exec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// SaveNodeResults
	results := []models.NodeResult{
		{NodeID: "n1", NodeName: "N1", NodeType: models.NodeTypeScript, Status: "success", Output: "hello"},
	}
	if err := s.Executions.SaveNodeResults(ctx, exec.ID, results); err != nil {
		t.Fatalf("SaveNodeResults: %v", err)
	}

	// ListNodeResults
	got, err := s.Executions.ListNodeResults(ctx, exec.ID)
	if err != nil {
		t.Fatalf("ListNodeResults: %v", err)
	}
	if len(got) != 1 || got[0].Output != "hello" {
		t.Errorf("unexpected node results: %+v", got)
	}

	// Get (includes Results)
	full, err := s.Executions.Get(ctx, exec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if full.Status != models.StatusRunning {
		t.Errorf("Status mismatch: got %q", full.Status)
	}

	// Update status
	exec.Status = models.StatusCompleted
	now := time.Now().UTC()
	exec.EndedAt = &now
	if err := s.Executions.Update(ctx, exec); err != nil {
		t.Fatalf("Update: %v", err)
	}
	full, _ = s.Executions.Get(ctx, exec.ID)
	if full.Status != models.StatusCompleted {
		t.Errorf("updated Status mismatch: got %q", full.Status)
	}
}

// ─── Checkpoint (cross-restart simulation) ─────────────────────────────────

func TestPostgresCheckpointRoundtrip(t *testing.T) {
	s := postgresDB(t)
	ctx := context.Background()

	dag := &models.DAGConfig{
		ID: "cp-dag-" + uniqueSuffix(), Name: "CP Test DAG",
		Nodes: []models.Node{{ID: "n1", Name: "N1", Type: models.NodeTypeHuman}},
		Edges: []models.Edge{}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	_ = s.DAGs.Create(ctx, dag)
	t.Cleanup(func() { _ = s.DAGs.Delete(ctx, dag.ID) })

	exec := &models.Execution{
		ID: "cp-exec-" + uniqueSuffix(), DAGID: dag.ID, DAGName: dag.Name,
		Status: models.StatusSuspended, StartedAt: time.Now().UTC(),
		State: models.NewGlobalState(),
	}
	exec.State.Set("phase", "waiting_approval")
	_ = s.Executions.Create(ctx, exec)

	cp := &models.ExecutionCheckpoint{
		ExecutionID: exec.ID,
		DAGID:       dag.ID,
		State:       exec.State.Snapshot(),
		LoopCounts:  exec.State.LoopCountsSnapshot(),
		UpdatedAt:   time.Now().UTC(),
	}

	// Save checkpoint
	if err := s.Executions.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	// Simulate restart: create a fresh store pointing at the same DB.
	s2 := store.NewPostgresStores(s.DB)

	// GetCheckpoint on new store instance (simulates post-restart read).
	restored, err := s2.Executions.GetCheckpoint(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetCheckpoint after restart simulation: %v", err)
	}
	state := models.NewGlobalStateFromSnapshot(restored.State, restored.LoopCounts)
	if state.GetString("phase") != "waiting_approval" {
		t.Errorf("state not restored: got %q", state.GetString("phase"))
	}
}

// ─── AuditStore ────────────────────────────────────────────────────────────

func TestPostgresAuditStore(t *testing.T) {
	s := postgresDB(t)
	ctx := context.Background()

	entry := audit.Entry{
		Time:       time.Now().UTC(),
		Actor:      "test-actor",
		Role:       "admin",
		Action:     "test_action",
		Resource:   "test_resource",
		ResourceID: "res-" + uniqueSuffix(),
		Result:     "success",
		Details:    "integration test",
	}
	if err := s.Audits.Record(ctx, entry); err != nil {
		t.Fatalf("Record: %v", err)
	}
	list, err := s.Audits.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, e := range list {
		if e.ResourceID == entry.ResourceID {
			found = true
		}
	}
	if !found {
		t.Error("recorded audit entry not found in List")
	}
}

// ─── Migrations idempotency ─────────────────────────────────────────────────

func TestMigrationIdempotency(t *testing.T) {
	s := postgresDB(t)
	ctx := context.Background()
	migrationsDir := migrationDir(t)

	// Running migrations a second time should be a no-op.
	if err := store.RunMigrations(ctx, s.DB, migrationsDir); err != nil {
		t.Fatalf("second RunMigrations call: %v", err)
	}
}

// ─── helpers ───────────────────────────────────────────────────────────────

func uniqueSuffix() string {
	return time.Now().Format("20060102150405.000000000")
}
