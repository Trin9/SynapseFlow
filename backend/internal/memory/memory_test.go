package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

func TestRetrieverInjectsHistoricalContext(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Now().UTC()
	if err := store.Save(context.Background(), &models.Experience{
		ID:          "exp-1",
		AlertType:   "OOM",
		ServiceName: "order-api",
		Symptom:     "order-api pod restart with oomkill",
		RootCause:   "memory leak in cache warmup",
		ActionTaken: "reduce cache batch size and restart",
		Summary:     "OOM in order-api due to cache warmup leak",
		Document:    "OOM order-api cache warmup memory leak restart",
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("save experience: %v", err)
	}

	retriever := &Retriever{Store: store}
	state := models.NewGlobalState()
	state.Set("alert_type", "OOM")
	state.Set("service_name", "order-api")
	state.Set("alert_text", "order-api pod restart with oomkill observed")

	results, err := retriever.Inject(context.Background(), &models.DAGConfig{Name: "oom workflow"}, state)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 recalled experience, got %d", len(results))
	}

	historicalContext := state.GetString("historical_context")
	if !strings.Contains(historicalContext, "memory leak in cache warmup") {
		t.Fatalf("expected historical context to include root cause, got %q", historicalContext)
	}
	if _, ok := state.Get("historical_experiences"); !ok {
		t.Fatal("expected historical_experiences to be written to state")
	}
}

func TestExtractorCreatesExperience(t *testing.T) {
	store := NewInMemoryStore()
	extractor := &Extractor{Store: store}
	state := models.NewGlobalState()
	state.Set("alert_type", "latency")
	state.Set("service_name", "payment-api")
	state.Set("alert_text", "payment-api latency p99 > 2s")

	exec := &models.Execution{
		ID:     "exec-1",
		Status: models.StatusCompleted,
		State:  state,
		Results: []models.NodeResult{
			{
				NodeID: "analyze",
				Output: `{"root_cause":"database connection pool exhaustion","recommended_action":"add index and reduce pool contention"}`,
			},
		},
	}

	exp, err := extractor.Extract(context.Background(), &models.DAGConfig{Name: "latency workflow"}, exec)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if exp == nil {
		t.Fatal("expected extracted experience")
	}
	if exp.RootCause != "database connection pool exhaustion" {
		t.Fatalf("unexpected root cause: %q", exp.RootCause)
	}
	if !strings.Contains(exp.Document, "payment-api") {
		t.Fatalf("expected document to include service name, got %q", exp.Document)
	}

	listed, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list experiences: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 stored experience, got %d", len(listed))
	}
}
