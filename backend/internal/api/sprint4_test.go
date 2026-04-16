package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xunchenzheng/synapse/internal/store"
)

// ---------------------------------------------------------------------------
// 4.1 Webhook tests
// ---------------------------------------------------------------------------

// TestWebhookAlert_MatchByDagID verifies that posting an Alertmanager-style
// payload with a dag_id label triggers a DAG run and returns 202.
func TestWebhookAlert_MatchByDagID(t *testing.T) {
	s := NewServer()

	// First create and save a DAG
	dag := map[string]interface{}{
		"name": "alert dag",
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo triggered"},
		},
		"edges": []map[string]interface{}{},
	}
	createRec := doJSON(t, s, http.MethodPost, "/api/v1/dags", dag)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create dag: expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created map[string]interface{}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	dagID, _ := created["id"].(string)

	// Post webhook alert matching by dag_id label
	payload := map[string]interface{}{
		"commonLabels": map[string]interface{}{
			"dag_id": dagID,
		},
		"alerts": []map[string]interface{}{
			{
				"labels": map[string]interface{}{
					"alertname": "HighLatency",
					"service":   "order-api",
				},
				"status": "firing",
			},
		},
	}
	webhookRec := doJSON(t, s, http.MethodPost, "/api/v1/webhook/alert", payload)
	if webhookRec.Code != http.StatusAccepted {
		t.Fatalf("webhook expected 202, got %d: %s", webhookRec.Code, webhookRec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(webhookRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal webhook resp: %v", err)
	}
	execID, _ := resp["execution_id"].(string)
	if execID == "" {
		t.Fatalf("expected execution_id in response, got %v", resp)
	}

	// Verify execution completes
	w := waitExecutionDone(t, s, execID)
	if w["status"] != "completed" {
		t.Fatalf("expected completed, got %v", w["status"])
	}
}

// TestWebhookAlert_MatchByServiceAlertname verifies service+alertname label matching.
func TestWebhookAlert_MatchByServiceAlertname(t *testing.T) {
	s := NewServer()

	dag := map[string]interface{}{
		"name": "service alert dag",
		"metadata": map[string]interface{}{
			"service":   "payment-api",
			"alertname": "OOMKill",
		},
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo oom"},
		},
		"edges": []map[string]interface{}{},
	}
	createRec := doJSON(t, s, http.MethodPost, "/api/v1/dags", dag)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", createRec.Code, createRec.Body.String())
	}

	payload := map[string]interface{}{
		"commonLabels": map[string]interface{}{
			"service":   "payment-api",
			"alertname": "OOMKill",
		},
		"alerts": []map[string]interface{}{},
	}
	rec := doJSON(t, s, http.MethodPost, "/api/v1/webhook/alert", payload)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("webhook expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestWebhookAlert_NoMatch verifies 404 when no DAG matches.
func TestWebhookAlert_NoMatch(t *testing.T) {
	s := NewServer()
	payload := map[string]interface{}{
		"commonLabels": map[string]interface{}{
			"service":   "nonexistent-service",
			"alertname": "FakeAlert",
		},
		"alerts": []map[string]interface{}{},
	}
	rec := doJSON(t, s, http.MethodPost, "/api/v1/webhook/alert", payload)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookAlert_AutoAnalyseDisabledIsIgnored(t *testing.T) {
	s := NewServer()
	payload := map[string]interface{}{
		"commonLabels": map[string]interface{}{
			"dag_id":       "missing",
			"auto_analyse": "false",
		},
	}
	rec := doJSON(t, s, http.MethodPost, "/api/v1/webhook/alert", payload)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal ignored response: %v", err)
	}
	if body["status"] != "ignored" {
		t.Fatalf("expected ignored status, got %v", body)
	}
	executions, err := s.execs.List(context.Background())
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(executions) != 0 {
		t.Fatalf("expected no execution to be created when auto_analyse is false")
	}
}

// ---------------------------------------------------------------------------
// 4.2 Auth / RBAC tests
// ---------------------------------------------------------------------------

// TestAuth_NoCredentials_OpenMode verifies that in dev/open mode (no API keys
// configured) all requests succeed without credentials.
func TestAuth_NoCredentials_OpenMode(t *testing.T) {
	s := NewServer() // no API keys configured → open mode
	rec := do(t, s, http.MethodGet, "/api/v1/dags", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("open mode: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAuth_MissingCredentials_Returns401 verifies 401 when API keys are configured.
func TestAuth_MissingCredentials_Returns401(t *testing.T) {
	s := NewServer()
	// Inject a key so that auth is enforced
	s.apiKeys = parseAPIKeys("supersecret:admin:sre-bot")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dags", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAuth_ValidAPIKey_Returns200 verifies that a valid API key passes.
func TestAuth_ValidAPIKey_Returns200(t *testing.T) {
	s := NewServer()
	s.apiKeys = parseAPIKeys("supersecret:admin:sre-bot")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dags", nil)
	req.Header.Set("X-API-Key", "supersecret")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid key, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAuth_ValidBearerToken_Returns200 verifies that a valid Bearer token passes.
func TestAuth_ValidBearerToken_Returns200(t *testing.T) {
	s := NewServer()
	s.apiKeys = parseAPIKeys("unused:admin:x") // enforce auth

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dags", nil)
	req.Header.Set("Authorization", "Bearer admin:alice")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAuth_ViewerCannotCreateDAG verifies 403 when viewer tries to create.
func TestAuth_ViewerCannotCreateDAG(t *testing.T) {
	s := NewServer()
	s.apiKeys = parseAPIKeys("viewerkey:viewer:dashboard")

	dag := map[string]interface{}{
		"name": "test",
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo hi"},
		},
		"edges": []map[string]interface{}{},
	}
	body, _ := json.Marshal(dag)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dags", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "viewerkey")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer should get 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAuth_OperatorCanRunDAG verifies operator role can trigger execution.
func TestAuth_OperatorCanRunDAG(t *testing.T) {
	s := NewServer()
	s.apiKeys = parseAPIKeys("opkey:operator:on-call-sre")

	// Create a DAG first (use the admin-only POST /dags via a direct open-mode server)
	sOpen := NewServer() // open mode for setup
	dag := map[string]interface{}{
		"name": "operator test dag",
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo hi"},
		},
		"edges": []map[string]interface{}{},
	}
	createRec := doJSON(t, sOpen, http.MethodPost, "/api/v1/dags", dag)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", createRec.Code, createRec.Body.String())
	}
	var created map[string]interface{}
	json.Unmarshal(createRec.Body.Bytes(), &created)
	dagID := created["id"].(string)

	// Copy the DAG into the auth-enforced server
	persisted, err := sOpen.dags.Get(context.Background(), dagID)
	if err != nil {
		t.Fatalf("get created dag from setup server: %v", err)
	}
	if err := s.dags.Create(context.Background(), persisted); err != nil && err != store.ErrNotFound {
		t.Fatalf("copy dag into auth server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dags/"+dagID+"/run", nil)
	req.Header.Set("X-API-Key", "opkey")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("operator run: expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAuditLog_RecordsEntries verifies audit log entries are written.
func TestAuditLog_RecordsEntries(t *testing.T) {
	s := NewServer() // open mode

	dag := map[string]interface{}{
		"name": "audit test",
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo hi"},
		},
		"edges": []map[string]interface{}{},
	}
	doJSON(t, s, http.MethodPost, "/api/v1/dags", dag)

	rec := do(t, s, http.MethodGet, "/api/v1/audit", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("audit expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var entries []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshal audit: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one audit entry after creating a DAG")
	}
	// Verify the entry has required fields
	first := entries[0]
	for _, field := range []string{"time", "action", "resource", "result"} {
		if first[field] == nil || first[field] == "" {
			t.Fatalf("audit entry missing field %q: %v", field, first)
		}
	}
}

// ---------------------------------------------------------------------------
// 4.3 Metrics tests
// ---------------------------------------------------------------------------

// TestMetrics_ReturnsPrometheusText verifies /metrics endpoint format.
func TestMetrics_ReturnsPrometheusText(t *testing.T) {
	s := NewServer()
	rec := do(t, s, http.MethodGet, "/metrics", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// Should contain at least the metric declarations (even with no data yet)
	for _, name := range []string{
		"synapse_executions_total",
		"synapse_execution_duration_seconds",
		"synapse_node_execution_duration_seconds",
		"synapse_mcp_calls_total",
		"synapse_llm_tokens_total",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("metrics missing %q in:\n%s", name, body)
		}
	}
}

// TestMetrics_RecordsAfterExecution verifies counter increments post-run.
func TestMetrics_RecordsAfterExecution(t *testing.T) {
	s := NewServer()

	dag := map[string]interface{}{
		"name": "metrics test dag",
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo metrics"},
		},
		"edges": []map[string]interface{}{},
	}
	runRec := doJSON(t, s, http.MethodPost, "/api/v1/run", dag)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("run: %d %s", runRec.Code, runRec.Body.String())
	}
	var runResp map[string]interface{}
	json.Unmarshal(runRec.Body.Bytes(), &runResp)
	execID := runResp["execution_id"].(string)

	waitExecutionDone(t, s, execID)

	// Allow metrics goroutine to complete
	time.Sleep(10 * time.Millisecond)

	rec := do(t, s, http.MethodGet, "/metrics", nil)
	body := rec.Body.String()
	if !strings.Contains(body, `synapse_executions_total{status="completed"}`) {
		t.Errorf("expected completed execution metric, got:\n%s", body)
	}
}

// TestHealth_Enhanced verifies /health returns deps section.
func TestHealth_Enhanced(t *testing.T) {
	s := NewServer()
	rec := do(t, s, http.MethodGet, "/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health expected 200, got %d", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal health: %v", err)
	}
	if body["deps"] == nil {
		t.Fatalf("expected deps field in health response, got %v", body)
	}
}

// ---------------------------------------------------------------------------
// 4.1 Notification (Slack) mock test
// ---------------------------------------------------------------------------

// TestSlackNotification_SentOnCompletion verifies notification fires after execution.
func TestSlackNotification_SentOnCompletion(t *testing.T) {
	var receivedMsg string
	mockSlack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedMsg = string(body)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer mockSlack.Close()

	s := NewServer()
	// Wire in the mock slack notifier
	s.notifier = &mockNotifier{}
	s.slackWebhookURL = mockSlack.URL

	dag := map[string]interface{}{
		"name": "notify test",
		"metadata": map[string]interface{}{
			"service":                    "order-api",
			"alertname":                  "HighLatency",
			"execution_details_base_url": "https://synapse.example",
		},
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo hello"},
			{"id": "b", "name": "Analyze", "type": "llm", "action": "Analyze {{a}}"},
		},
		"edges": []map[string]interface{}{{"from": "a", "to": "b"}},
	}
	runRec := doJSON(t, s, http.MethodPost, "/api/v1/run", dag)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("run: %d %s", runRec.Code, runRec.Body.String())
	}
	var runResp map[string]interface{}
	json.Unmarshal(runRec.Body.Bytes(), &runResp)
	execID := runResp["execution_id"].(string)
	waitExecutionDone(t, s, execID)

	// Allow notification goroutine to fire
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		if receivedMsg != "" {
			break
		}
	}
	if receivedMsg == "" {
		t.Fatal("expected Slack notification to be sent, but no request received")
	}
	if !strings.Contains(receivedMsg, "Synapse Execution") {
		t.Errorf("unexpected notification body: %s", receivedMsg)
	}
	if !strings.Contains(receivedMsg, "Conclusion:") {
		t.Errorf("expected conclusion in notification body: %s", receivedMsg)
	}
	if !strings.Contains(receivedMsg, "Details:") {
		t.Errorf("expected details url in notification body: %s", receivedMsg)
	}
}

// ---------------------------------------------------------------------------
// Helpers used only in Sprint 4 tests
// ---------------------------------------------------------------------------

// mockNotifier is a notify.Sender backed by a real HTTP call so tests can
// inspect what was received by the mock server.
type mockNotifier struct{}

func (m *mockNotifier) SendExecutionResult(ctx context.Context, webhookURL string, message string) error {
	body, _ := json.Marshal(map[string]string{"text": message})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
