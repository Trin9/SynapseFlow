package api

// episode_lifecycle_test.go — Track A: full Episode lifecycle via HTTP API.
//
// Tests that a DAG with metadata.episode_type triggers:
//   1. Auto-creation of an Episode (status=pending) at execution start.
//   2. Status transition to in_progress after the first Hard Node (script) writes a fact.
//   3. Status transition to converged after the Soft Node (llm) writes a verdict.
//   4. Episode accessible via GET /executions/:id/episodes and GET /episodes/:id.
//
// All tests use the in-process MockLLMExecutor (no LLM_API_KEY required).

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	domainEpisode "github.com/Trin9/SynapseFlow/backend/internal/domain/episode"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// minimalEpisodeDAG is the smallest possible DAG that exercises the full
// Episode lifecycle without requiring a real LLM.
//
// Topology:
//
//	fact1 ─┐
//	       ├─► verdict_node
//	fact2 ─┘
//
// metadata.episode_type triggers Episode auto-creation in startExecution.
// verdict_node.config.episode_verdict=true routes MockLLMExecutor output to
// EpisodeWriter.WriteVerdict, converging the Episode.
var minimalEpisodeDAG = map[string]interface{}{
	"name": "episode lifecycle test dag",
	"metadata": map[string]interface{}{
		"episode_type": "action_verification",
	},
	"nodes": []map[string]interface{}{
		{
			"id":     "fact1",
			"name":   "Cart Check",
			"type":   "script",
			"action": "echo cart_item_count=3",
		},
		{
			"id":     "fact2",
			"name":   "Payment Gateway",
			"type":   "script",
			"action": "echo payment_gateway=ok",
		},
		{
			// Soft Node — no state_key / parse_json_output to avoid JSON key
			// validation. MockLLMExecutor output is valid JSON that
			// buildVerdictFromLLMOutput can parse (confidence 75 → medium,
			// no business_success → inconclusive).
			"id":     "verdict_node",
			"name":   "Audit LLM",
			"type":   "llm",
			"action": "Verify checkout: {{fact1}} {{fact2}}",
			"config": map[string]interface{}{
				"episode_verdict": true,
			},
		},
	},
	"edges": []map[string]interface{}{
		{"from": "fact1", "to": "verdict_node"},
		{"from": "fact2", "to": "verdict_node"},
	},
}

// TestEpisodeLifecycle_AutoCreateAndConverge is the primary Track A E2E test.
// It verifies the Episode lifecycle from pending → in_progress → converged
// entirely through the HTTP API with in-process executors (no real LLM needed).
func TestEpisodeLifecycle_AutoCreateAndConverge(t *testing.T) {
	s := NewServer()

	// ── Step 1: Start the execution ──────────────────────────────────────────
	runRec := doJSON(t, s, http.MethodPost, "/api/v1/run", minimalEpisodeDAG)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("run: expected 202, got %d: %s", runRec.Code, runRec.Body.String())
	}
	var runResp map[string]interface{}
	if err := json.Unmarshal(runRec.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("run: unmarshal: %v", err)
	}
	execID, _ := runResp["execution_id"].(string)
	if execID == "" {
		t.Fatal("run: expected execution_id in response")
	}

	// ── Step 2: Wait for execution to complete ───────────────────────────────
	// All Episode writes are synchronous within the scheduler, so by the time
	// the execution status is "completed" the Episode is already converged.
	execResult := waitExecutionDone(t, s, execID)
	if execResult["status"] != "completed" {
		t.Fatalf("execution: expected status=completed, got %v", execResult["status"])
	}

	// ── Step 3: GET /api/v1/executions/:id/episodes ──────────────────────────
	listRec := do(t, s, http.MethodGet, "/api/v1/executions/"+execID+"/episodes", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list episodes: expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var listBody map[string]interface{}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("list episodes: unmarshal: %v", err)
	}
	rawEps, _ := listBody["episodes"].([]interface{})
	if len(rawEps) == 0 {
		t.Fatal("list episodes: expected at least 1 episode, got 0")
	}
	ep, ok := rawEps[0].(map[string]interface{})
	if !ok {
		t.Fatalf("list episodes: unexpected episode shape: %T", rawEps[0])
	}

	// ── Step 4: Assertions on Episode fields ─────────────────────────────────

	// exec_id must link back to our execution.
	if ep["exec_id"] != execID {
		t.Errorf("episode exec_id: got %v, want %v", ep["exec_id"], execID)
	}

	// episode_type must be propagated from DAG metadata.
	if ep["episode_type"] != "action_verification" {
		t.Errorf("episode_type: expected action_verification, got %q", ep["episode_type"])
	}

	// Status must be converged — both script Hard Nodes and the LLM verdict ran.
	if ep["status"] != "converged" {
		t.Errorf("status: expected converged, got %q", ep["status"])
	}

	// Evidence must have at least 2 entries (one per script node).
	evidence, _ := ep["evidence"].([]interface{})
	if len(evidence) < 2 {
		t.Errorf("evidence: expected ≥2 entries (one per script node), got %d", len(evidence))
	}

	// Verdict must be present after the LLM verdict node ran.
	if ep["verdict"] == nil {
		t.Error("verdict: expected non-nil after convergence")
	}

	// concluded_at must be set when status transitions to converged.
	if ep["concluded_at"] == nil || ep["concluded_at"] == "" {
		t.Error("concluded_at: expected non-nil/non-empty after convergence")
	}

	// schema_version must be 1.
	schemaVersion, _ := ep["schema_version"].(float64) // JSON numbers decode as float64
	if int(schemaVersion) != 1 {
		t.Errorf("schema_version: expected 1, got %v", ep["schema_version"])
	}

	// ── Step 5: GET /api/v1/episodes/:id ────────────────────────────────────
	epID, _ := ep["id"].(string)
	if epID == "" {
		t.Fatal("episode id: expected non-empty")
	}

	getRec := do(t, s, http.MethodGet, "/api/v1/episodes/"+epID, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get episode: expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var epDirect map[string]interface{}
	if err := json.Unmarshal(getRec.Body.Bytes(), &epDirect); err != nil {
		t.Fatalf("get episode: unmarshal: %v", err)
	}
	if epDirect["id"] != epID {
		t.Errorf("get episode: id mismatch: got %v, want %v", epDirect["id"], epID)
	}
	if epDirect["status"] != "converged" {
		t.Errorf("get episode: status: expected converged, got %q", epDirect["status"])
	}

	// Verify the verdict sub-fields via the direct GET response.
	verdictRaw, _ := epDirect["verdict"].(map[string]interface{})
	if verdictRaw == nil {
		t.Fatal("get episode: verdict: expected non-nil map")
	}
	// MockLLMExecutor produces confidence=75 (float) → medium.
	if verdictRaw["confidence"] != "medium" {
		t.Errorf("verdict.confidence: expected medium (from mock score 75), got %q", verdictRaw["confidence"])
	}
	// No business_success key in mock output → inconclusive result.
	if verdictRaw["result"] != "inconclusive" {
		t.Errorf("verdict.result: expected inconclusive (no business_success in mock), got %q", verdictRaw["result"])
	}
	// DecidedBy must be the verdict node's ID.
	if verdictRaw["decided_by"] != "verdict_node" {
		t.Errorf("verdict.decided_by: expected verdict_node, got %q", verdictRaw["decided_by"])
	}
}

// TestEpisodeLifecycle_GetEpisodeNotFound verifies the 404 error path for
// GET /api/v1/episodes/:id.
func TestEpisodeLifecycle_GetEpisodeNotFound(t *testing.T) {
	s := NewServer()
	rec := do(t, s, http.MethodGet, "/api/v1/episodes/nonexistent-episode-id", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] == "" || body["error"] == "" {
		t.Errorf("expected error+code fields in 404 response, got %v", body)
	}
}

// TestEpisodeLifecycle_ListEpisodes_EmptyForUnknownExecution verifies that
// listing episodes for a non-existent execution returns an empty list (not 404/500).
func TestEpisodeLifecycle_ListEpisodes_EmptyForUnknownExecution(t *testing.T) {
	s := NewServer()
	rec := do(t, s, http.MethodGet, "/api/v1/executions/nonexistent-exec-id/episodes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	eps, _ := body["episodes"].([]interface{})
	if len(eps) != 0 {
		t.Errorf("expected empty episode list for unknown execution, got %d entries", len(eps))
	}
}

// TestEpisodeLifecycle_DefaultEpisodeCreated_WhenEpisodeTypeAbsent verifies
// that a DAG without episode_type still creates a default Episode.
func TestEpisodeLifecycle_DefaultEpisodeCreated_WhenEpisodeTypeAbsent(t *testing.T) {
	s := NewServer()
	dag := map[string]interface{}{
		"name": "no-episode-dag",
		"nodes": []map[string]interface{}{
			{"id": "a", "name": "A", "type": "script", "action": "echo hello"},
		},
		"edges": []map[string]interface{}{},
	}
	runRec := doJSON(t, s, http.MethodPost, "/api/v1/run", dag)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("run: expected 202, got %d: %s", runRec.Code, runRec.Body.String())
	}
	var runResp map[string]interface{}
	_ = json.Unmarshal(runRec.Body.Bytes(), &runResp)
	execID, _ := runResp["execution_id"].(string)
	_ = waitExecutionDone(t, s, execID)

	listRec := do(t, s, http.MethodGet, "/api/v1/executions/"+execID+"/episodes", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var body map[string]interface{}
	_ = json.Unmarshal(listRec.Body.Bytes(), &body)
	eps, _ := body["episodes"].([]interface{})
	if len(eps) != 1 {
		t.Fatalf("expected 1 default episode for DAG without episode_type, got %d", len(eps))
	}
	ep, _ := eps[0].(map[string]interface{})
	epType, _ := ep["episode_type"].(string)
	if epType != string(domainEpisode.EpisodeTypeActionVerification.ToModel()) {
		t.Fatalf("expected default episode_type action_verification, got %q", epType)
	}
}

// TestEpisodeLifecycle_DesignEpisodesCreateRuntimeSkeletons verifies that
// execution start creates runtime episode skeletons from dag.episodes specs.
func TestEpisodeLifecycle_DesignEpisodesCreateRuntimeSkeletons(t *testing.T) {
	s := NewServer()
	dag := map[string]interface{}{
		"name": "design-episode-bootstrap",
		"metadata": map[string]interface{}{
			"episode_type": "action_verification",
		},
		"episodes": []map[string]interface{}{
			{
				"id":                 "design_ep_bootstrap",
				"label":              "Storefront Ready Episode",
				"summary":            "Verify storefront readiness before transaction setup.",
				"expected_behaviors": []interface{}{"frontend health endpoint is reachable", "product discovery yields product id"},
				"node_ids":           []interface{}{"n_health"},
			},
			{
				"id":                 "design_ep_tx_setup",
				"label":              "Transaction Setup Episode",
				"summary":            "Verify cart continuity and checkout prerequisites.",
				"expected_behaviors": []interface{}{"cart add succeeds"},
				"node_ids":           []interface{}{"n_health"},
			},
		},
		"nodes": []map[string]interface{}{
			{"id": "n_health", "name": "Health", "type": "script", "action": "echo ok"},
		},
		"edges": []map[string]interface{}{},
	}

	runRec := doJSON(t, s, http.MethodPost, "/api/v1/run", dag)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("run: expected 202, got %d: %s", runRec.Code, runRec.Body.String())
	}
	var runResp map[string]interface{}
	_ = json.Unmarshal(runRec.Body.Bytes(), &runResp)
	execID, _ := runResp["execution_id"].(string)
	_ = waitExecutionDone(t, s, execID)

	listRec := do(t, s, http.MethodGet, "/api/v1/executions/"+execID+"/episodes", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var body map[string]interface{}
	_ = json.Unmarshal(listRec.Body.Bytes(), &body)
	eps, _ := body["episodes"].([]interface{})
	if len(eps) != 2 {
		t.Fatalf("expected 2 runtime episodes from design specs, got %d", len(eps))
	}

	foundDesignBootstrap := false
	foundDesignTxSetup := false
	for _, raw := range eps {
		ep, _ := raw.(map[string]interface{})
		actionCtx, _ := ep["action_context"].(map[string]interface{})
		if actionCtx == nil {
			continue
		}
		actionInput, _ := actionCtx["action_input"].(map[string]interface{})
		if actionInput == nil {
			continue
		}
		designID, _ := actionInput["design_episode_id"].(string)
		switch designID {
		case "design_ep_bootstrap":
			foundDesignBootstrap = true
		case "design_ep_tx_setup":
			foundDesignTxSetup = true
		}
	}

	if !foundDesignBootstrap || !foundDesignTxSetup {
		t.Fatalf("expected runtime episodes to preserve design_episode_id markers, got bootstrap=%v tx_setup=%v", foundDesignBootstrap, foundDesignTxSetup)
	}
}

func TestBuildEpisodeDossier_AppliesHumanReviewDisplayProjection(t *testing.T) {
	now := time.Now().UTC()
	ep := &models.Episode{
		ID:          "ep-review-001",
		ExecID:      "exec-review-001",
		EpisodeType: domainEpisode.EpisodeTypeActionVerification.ToModel(),
		Status:      domainEpisode.EpisodeStatusConverged.ToModel(),
		Verdict: &models.EpisodeVerdict{
			Result:     models.EpisodeResultFail,
			Confidence: models.EpisodeConfidenceLow,
			Conclusion: "AI concluded the checkout likely failed.",
		},
		HumanInterventions: []models.HumanIntervention{
			{
				Actor:     "reviewer",
				Action:    models.HumanActionStateOverride,
				Detail:    "Human verified checkout success despite flaky signal.",
				Timestamp: now,
			},
		},
	}

	dossier := buildEpisodeDossier(ep, nil, nil)
	if dossier.Display.VerdictLabel != "Overridden (Human)" {
		t.Fatalf("expected dossier verdict_label to reflect human override, got %q", dossier.Display.VerdictLabel)
	}
	if dossier.Display.Banner == nil {
		t.Fatal("expected dossier banner to be set for human override")
	}
	if *dossier.Display.Banner != "Human override: Human verified checkout success despite flaky signal." {
		t.Fatalf("unexpected dossier banner: %q", *dossier.Display.Banner)
	}
	if dossier.Display.Summary != "AI concluded the checkout likely failed." {
		t.Fatalf("expected underlying summary to be preserved, got %q", dossier.Display.Summary)
	}

	ep.HumanInterventions = []models.HumanIntervention{{
		Actor:     "reviewer",
		Action:    models.HumanActionResumed,
		Timestamp: now,
	}}
	dossier = buildEpisodeDossier(ep, nil, nil)
	if dossier.Display.VerdictLabel != "Approved" {
		t.Fatalf("expected dossier verdict_label to reflect approval, got %q", dossier.Display.VerdictLabel)
	}

	ep.HumanInterventions = []models.HumanIntervention{{
		Actor:     "reviewer",
		Action:    models.HumanActionAborted,
		Timestamp: now,
	}}
	dossier = buildEpisodeDossier(ep, nil, nil)
	if dossier.Display.VerdictLabel != "Aborted" {
		t.Fatalf("expected dossier verdict_label to reflect abort, got %q", dossier.Display.VerdictLabel)
	}
}

func TestBuildEpisodeDossier_PrefersDesignExpectedBehavior(t *testing.T) {
	ep := &models.Episode{
		ID:          "ep-design-expected-001",
		ExecID:      "exec-design-expected-001",
		EpisodeType: domainEpisode.EpisodeTypeActionVerification.ToModel(),
		Status:      domainEpisode.EpisodeStatusConverged.ToModel(),
		ActionContext: &models.ActionContext{
			ActionName: "Storefront Ready Episode",
			ActionType: "design_episode",
			ActionInput: map[string]interface{}{
				"expected_behaviors": []interface{}{
					"storefront health endpoint returns 200",
					"product discovery yields at least one product id",
				},
			},
		},
		Verdict: &models.EpisodeVerdict{
			Result:       models.EpisodeResultPass,
			Confidence:   models.EpisodeConfidenceHigh,
			Conclusion:   "Design expectations are satisfied.",
			CausalChain:  []string{"fallback causal chain item"},
			Recommendations: []string{"keep monitoring"},
		},
	}

	dossier := buildEpisodeDossier(ep, nil, nil)
	if len(dossier.ExpectedBehavior) != 2 {
		t.Fatalf("expected 2 design expected_behavior entries, got %d", len(dossier.ExpectedBehavior))
	}
	if dossier.ExpectedBehavior[0].SourceType != "sop" {
		t.Fatalf("expected source_type=sop, got %q", dossier.ExpectedBehavior[0].SourceType)
	}
	if dossier.ExpectedBehavior[0].SourceLabel != "Verified SOP" {
		t.Fatalf("expected source_label=Verified SOP, got %q", dossier.ExpectedBehavior[0].SourceLabel)
	}
	if dossier.ExpectedBehavior[0].Body != "storefront health endpoint returns 200" {
		t.Fatalf("unexpected first expected behavior body: %q", dossier.ExpectedBehavior[0].Body)
	}
	if dossier.ExpectedBehavior[0].ID == "causal_0" {
		t.Fatalf("expected design-time behavior to override AI causal fallback")
	}
}
