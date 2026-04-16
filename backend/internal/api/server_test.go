package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealth(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListTools_Empty(t *testing.T) {
	s := NewServer()
	rec := do(t, s, http.MethodGet, "/api/v1/tools", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var tools []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &tools); err != nil {
		t.Fatalf("unmarshal tools: %v", err)
	}
	// Default config has no servers, so empty list is expected.
	if len(tools) != 0 {
		t.Fatalf("expected empty tool list, got %v", tools)
	}
}

func TestDAG_CRUD_And_Run(t *testing.T) {
	s := NewServer()

	dag := map[string]interface{}{
		"name": "test dag",
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo hi"},
		},
		"edges": []map[string]interface{}{},
	}

	// Create
	createRec := doJSON(t, s, http.MethodPost, "/api/v1/dags", dag)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created map[string]interface{}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("expected id")
	}

	// List
	listRec := do(t, s, http.MethodGet, "/api/v1/dags", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	// Get
	getRec := do(t, s, http.MethodGet, "/api/v1/dags/"+id, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	// Update
	update := map[string]interface{}{
		"name": "test dag v2",
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo hi"},
			{"id": "b", "name": "B", "type": "llm", "action": "{\"ok\":true}"},
		},
		"edges": []map[string]interface{}{
			{"from": "a", "to": "b"},
		},
	}
	updRec := doJSON(t, s, http.MethodPut, "/api/v1/dags/"+id, update)
	if updRec.Code != http.StatusOK {
		t.Fatalf("update expected 200, got %d: %s", updRec.Code, updRec.Body.String())
	}

	// Run saved DAG
	runRec := do(t, s, http.MethodPost, "/api/v1/dags/"+id+"/run", nil)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("run expected 202, got %d: %s", runRec.Code, runRec.Body.String())
	}
	var runResp map[string]interface{}
	if err := json.Unmarshal(runRec.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("unmarshal run: %v", err)
	}
	execID, _ := runResp["execution_id"].(string)
	if execID == "" {
		t.Fatalf("expected execution_id")
	}

	// Poll nodes until completed
	w := waitExecutionDone(t, s, execID)
	if w["status"] != "completed" {
		// Mock LLM is deterministic; should succeed.
		t.Fatalf("expected completed, got %v", w["status"])
	}

	// Get execution
	exRec := do(t, s, http.MethodGet, "/api/v1/executions/"+execID, nil)
	if exRec.Code != http.StatusOK {
		t.Fatalf("get execution expected 200, got %d: %s", exRec.Code, exRec.Body.String())
	}

	// Get execution nodes
	nodesRec := do(t, s, http.MethodGet, "/api/v1/executions/"+execID+"/nodes", nil)
	if nodesRec.Code != http.StatusOK {
		t.Fatalf("get execution nodes expected 200, got %d: %s", nodesRec.Code, nodesRec.Body.String())
	}

	// List executions
	lexRec := do(t, s, http.MethodGet, "/api/v1/executions", nil)
	if lexRec.Code != http.StatusOK {
		t.Fatalf("list executions expected 200, got %d: %s", lexRec.Code, lexRec.Body.String())
	}

	// Delete
	delRec := do(t, s, http.MethodDelete, "/api/v1/dags/"+id, nil)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete expected 200, got %d: %s", delRec.Code, delRec.Body.String())
	}
}

func TestExecutionStoresExperienceAndExposesListAPI(t *testing.T) {
	s := NewServer()

	dag := map[string]interface{}{
		"name":        "memory dag",
		"description": "latency analysis",
		"metadata": map[string]interface{}{
			"service_name": "payment-api",
			"alert_type":   "latency",
			"alert_text":   "payment-api latency p99 high",
		},
		"nodes": []map[string]interface{}{
			{"id": "facts", "name": "Facts", "type": "script", "action": "echo connection pool exhausted"},
			{"id": "analyze", "name": "Analyze", "type": "llm", "action": "Analyze {{facts}} for {{service_name}} {{alert_type}}"},
		},
		"edges": []map[string]interface{}{
			{"from": "facts", "to": "analyze"},
		},
	}

	runRec := doJSON(t, s, http.MethodPost, "/api/v1/run", dag)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("run expected 202, got %d: %s", runRec.Code, runRec.Body.String())
	}
	var runResp map[string]interface{}
	if err := json.Unmarshal(runRec.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("unmarshal run: %v", err)
	}
	execID, _ := runResp["execution_id"].(string)
	if execID == "" {
		t.Fatal("expected execution_id")
	}

	_ = waitExecutionDone(t, s, execID)

	var experiences []map[string]interface{}
	for i := 0; i < 100; i++ {
		time.Sleep(5 * time.Millisecond)
		rec := do(t, s, http.MethodGet, "/api/v1/experiences", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("experiences expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &experiences); err != nil {
			t.Fatalf("unmarshal experiences: %v", err)
		}
		if len(experiences) > 0 {
			break
		}
	}

	if len(experiences) == 0 {
		t.Fatal("expected at least one extracted experience")
	}
	document, _ := experiences[0]["document"].(string)
	if !strings.Contains(document, "Execution State Snapshot") {
		t.Fatalf("expected stored experience document, got %q", document)
	}
}

func waitExecutionDone(t *testing.T, s *Server, execID string) map[string]interface{} {
	t.Helper()
	// Tight loop is fine; in-memory scheduler finishes quickly in tests.
	for i := 0; i < 200; i++ {
		time.Sleep(5 * time.Millisecond)
		rec := do(t, s, http.MethodGet, "/api/v1/executions/"+execID+"/nodes", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("poll expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal poll: %v", err)
		}
		status, _ := body["status"].(string)
		if status == "completed" || status == "failed" {
			return body
		}
	}
	t.Fatalf("execution did not complete in time")
	return nil
}

func TestRunInline_InvalidJSON(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateDAG_InvalidDAG(t *testing.T) {
	s := NewServer()

	// Edge to missing node
	dag := map[string]interface{}{
		"name": "bad",
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo hi"},
		},
		"edges": []map[string]interface{}{
			{"from": "a", "to": "missing"},
		},
	}

	rec := doJSON(t, s, http.MethodPost, "/api/v1/dags", dag)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNotFound_ErrorFormat(t *testing.T) {
	s := NewServer()
	rec := do(t, s, http.MethodGet, "/api/v1/dags/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["error"] == "" || body["code"] == "" {
		t.Fatalf("expected error and code fields, got %v", body)
	}
}

func do(t *testing.T, s *Server, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
	}
	if method == http.MethodPost || method == http.MethodPut {
		r.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, r)
	return rec
}

func doJSON(t *testing.T, s *Server, method, path string, v interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return do(t, s, method, path, b)
}
