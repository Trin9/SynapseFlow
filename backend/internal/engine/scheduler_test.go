package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// allExecutors returns a standard set of executors for testing.
func allExecutors() map[models.NodeType]NodeExecutor {
	return map[models.NodeType]NodeExecutor{
		models.NodeTypeScript: &ScriptExecutor{},
		models.NodeTypeLLM:    &MockLLMExecutor{},
		models.NodeTypeHuman:  &HumanExecutor{},
		models.NodeTypeRouter: &RouterExecutor{},
	}
}

type sequentialPlannerExecutor struct {
	mu        sync.Mutex
	responses []string
	callCount int
}

func (s *sequentialPlannerExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.callCount >= len(s.responses) {
		return models.NodeResult{
			NodeID:   node.ID,
			NodeName: node.Name,
			NodeType: node.Type,
			Status:   "error",
			Error:    "planner called more times than configured",
			Duration: time.Since(start),
		}
	}

	response := s.responses[s.callCount]
	s.callCount++
	state.Set(node.ID, response)
	if err := writeStructuredLLMState(node, state, response); err != nil {
		return models.NodeResult{
			NodeID:   node.ID,
			NodeName: node.Name,
			NodeType: node.Type,
			Status:   "error",
			Error:    err.Error(),
			Duration: time.Since(start),
		}
	}

	return models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
		Status:   "success",
		Output:   response,
		Duration: time.Since(start),
	}
}

func TestScheduler_ScriptExecution(t *testing.T) {
	scheduler := NewScheduler(allExecutors(), nil)

	dag := &models.DAGConfig{
		ID:   "test-1",
		Name: "basic-script-test",
		Nodes: []models.Node{
			{ID: "s1", Name: "Echo", Type: models.NodeTypeScript, Action: "echo hello"},
			{ID: "s2", Name: "Use Output", Type: models.NodeTypeScript, Action: "echo got: {{s1}}"},
		},
		Edges: []models.Edge{
			{From: "s1", To: "s2"},
		},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	if len(result.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.Results))
	}

	for _, r := range result.Results {
		if r.Status != "success" {
			t.Errorf("node %s failed: %s", r.NodeID, r.Error)
		}
	}

	s1Out := result.State.GetString("s1")
	if s1Out != "hello" {
		t.Errorf("expected s1 output 'hello', got '%s'", s1Out)
	}

	s2Out := result.State.GetString("s2")
	if s2Out != "got: hello" {
		t.Errorf("expected s2 output 'got: hello', got '%s'", s2Out)
	}
}

func TestScheduler_ConfigCommand(t *testing.T) {
	scheduler := NewScheduler(allExecutors(), nil)

	dag := &models.DAGConfig{
		ID:   "test-2",
		Name: "config-command-test",
		Nodes: []models.Node{
			{
				ID:   "s1",
				Name: "Echo via config",
				Type: models.NodeTypeScript,
				Config: map[string]interface{}{
					"command": "echo config-hello",
				},
			},
		},
		Edges: []models.Edge{},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	if result.Results[0].Status != "success" {
		t.Fatalf("node failed: %s", result.Results[0].Error)
	}

	out := result.State.GetString("s1")
	if out != "config-hello" {
		t.Errorf("expected output 'config-hello', got '%s'", out)
	}
}

func TestScheduler_ConcurrentExecution(t *testing.T) {
	scheduler := NewScheduler(allExecutors(), nil)

	dag := &models.DAGConfig{
		ID:   "test-3",
		Name: "concurrent-test",
		Nodes: []models.Node{
			{ID: "s1", Name: "Start", Type: models.NodeTypeScript, Action: "echo start"},
			{ID: "s2", Name: "Branch A", Type: models.NodeTypeScript, Action: "echo branch-a from {{s1}}"},
			{ID: "s3", Name: "Branch B", Type: models.NodeTypeScript, Action: "echo branch-b from {{s1}}"},
			{ID: "s4", Name: "Join", Type: models.NodeTypeScript, Action: "echo joined: {{s2}} and {{s3}}"},
		},
		Edges: []models.Edge{
			{From: "s1", To: "s2"},
			{From: "s1", To: "s3"},
			{From: "s2", To: "s4"},
			{From: "s3", To: "s4"},
		},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	if len(result.Results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(result.Results))
	}

	for _, r := range result.Results {
		if r.Status != "success" {
			t.Errorf("node %s failed: %s", r.NodeID, r.Error)
		}
	}

	s4Out := result.State.GetString("s4")
	if s4Out == "" {
		t.Error("expected s4 to have output")
	}
	t.Logf("s4 output: %s", s4Out)
}

func TestScheduler_MockLLMExecution(t *testing.T) {
	scheduler := NewScheduler(allExecutors(), nil)

	dag := &models.DAGConfig{
		ID:   "test-4",
		Name: "llm-test",
		Nodes: []models.Node{
			{ID: "data", Name: "Get Data", Type: models.NodeTypeScript, Action: "echo 'error: connection timeout at 10:32 UTC'"},
			{ID: "analyze", Name: "Analyze", Type: models.NodeTypeLLM, Action: "Analyze this error: {{data}}"},
		},
		Edges: []models.Edge{
			{From: "data", To: "analyze"},
		},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	for _, r := range result.Results {
		if r.Status != "success" {
			t.Errorf("node %s failed: %s", r.NodeID, r.Error)
		}
	}

	llmOut := result.State.GetString("analyze")
	if llmOut == "" {
		t.Error("expected LLM node to produce output")
	}
	t.Logf("LLM output: %s", llmOut)
}

func TestScheduler_PlannerStateDrivesConditionalRouting(t *testing.T) {
	executors := allExecutors()
	executors[models.NodeTypeLLM] = &LLMExecutor{Client: &fakeLLMClient{response: `{"next_action":"search_code","scope":"src/checkout","query":"failed to charge card","done":false}`}}

	scheduler := NewScheduler(executors, nil)
	dag := &models.DAGConfig{
		ID:   "test-planner-loop",
		Name: "planner-loop",
		Nodes: []models.Node{
			{ID: "start", Name: "Start", Type: models.NodeTypeScript, Action: "echo incident"},
			{
				ID:     "planner",
				Name:   "Planner",
				Type:   models.NodeTypeLLM,
				Action: "plan next step from {{start}}",
				Config: map[string]interface{}{
					"json_mode":         true,
					"state_key":         "plan",
					"parse_json_output": true,
				},
			},
			{ID: "route", Name: "Route", Type: models.NodeTypeRouter, Action: "{{plan.next_action}}"},
			{ID: "search", Name: "Search", Type: models.NodeTypeScript, Action: "echo scope={{plan.scope}} query={{plan.query}}"},
		},
		Edges: []models.Edge{
			{From: "start", To: "planner"},
			{From: "planner", To: "route"},
			{From: "route", To: "search", Condition: "{{plan.next_action}} == search_code"},
		},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)
	if result.Err != nil {
		t.Fatalf("unexpected execution error: %v", result.Err)
	}

	if got := result.State.GetPathString("plan.next_action"); got != "search_code" {
		t.Fatalf("expected structured planner state, got %q", got)
	}

	if got := result.State.GetString("route"); got != "search_code" {
		t.Fatalf("expected router output search_code, got %q", got)
	}

	if got := result.State.GetString("search"); got != "scope=src/checkout query=failed to charge card" {
		t.Fatalf("unexpected search output: %q", got)
	}

	if len(result.Results) != 4 {
		t.Fatalf("expected 4 node results, got %d", len(result.Results))
	}
}

func TestScheduler_MixedDependenciesSupportPlannerLoop(t *testing.T) {
	planner := &sequentialPlannerExecutor{
		responses: []string{
			`{"next_action":"search_logs","service":"checkout","query":"lookup badAddress","done":false}`,
			`{"next_action":"finish_report","report":"checkout fails because payment resolves badAddress","done":true}`,
		},
	}

	executors := allExecutors()
	executors[models.NodeTypeLLM] = planner

	scheduler := NewScheduler(executors, nil)
	dag := &models.DAGConfig{
		ID:   "test-planner-mixed-deps-loop",
		Name: "planner-mixed-deps-loop",
		Nodes: []models.Node{
			{ID: "start", Name: "Start", Type: models.NodeTypeScript, Action: "echo incident"},
			{
				ID:     "planner",
				Name:   "Planner",
				Type:   models.NodeTypeLLM,
				Action: "plan next step",
				Config: map[string]interface{}{
					"json_mode":         true,
					"state_key":         "plan",
					"parse_json_output": true,
				},
			},
			{ID: "route", Name: "Route", Type: models.NodeTypeRouter, Action: "{{plan.next_action}}"},
			{ID: "search_logs", Name: "Search Logs", Type: models.NodeTypeScript, Action: "echo {{plan.service}}:{{plan.query}}"},
			{ID: "final_report", Name: "Final Report", Type: models.NodeTypeScript, Action: "echo {{plan.report}}"},
		},
		Edges: []models.Edge{
			{From: "start", To: "planner"},
			{From: "planner", To: "route", Condition: "{{planner}}"},
			{From: "route", To: "search_logs", Condition: "{{plan.next_action}} == search_logs"},
			{From: "route", To: "final_report", Condition: "{{plan.done}} == true"},
			{From: "search_logs", To: "planner", Condition: "{{plan.done}} == false"},
		},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)
	if result.Err != nil {
		t.Fatalf("unexpected execution error: %v", result.Err)
	}

	if planner.callCount != 2 {
		t.Fatalf("expected planner to run twice, got %d", planner.callCount)
	}

	if got := result.State.GetString("search_logs"); got != "checkout:lookup badAddress" {
		t.Fatalf("unexpected log search output: %q", got)
	}

	if got := result.State.GetPathString("plan.report"); got != "checkout fails because payment resolves badAddress" {
		t.Fatalf("unexpected final report in state: %q", got)
	}

	if got := result.State.GetString("final_report"); got != "checkout fails because payment resolves badAddress" {
		t.Fatalf("unexpected final report output: %q", got)
	}

	if len(result.Results) != 7 {
		t.Fatalf("expected 7 node results for two planner cycles, got %d", len(result.Results))
	}

	if result.Status != models.StatusCompleted {
		t.Fatalf("expected completed status, got %s", result.Status)
	}
}

func TestScheduler_NoCommandError(t *testing.T) {
	scheduler := NewScheduler(allExecutors(), nil)

	dag := &models.DAGConfig{
		ID:   "test-5",
		Name: "no-command-test",
		Nodes: []models.Node{
			{ID: "empty", Name: "Empty Node", Type: models.NodeTypeScript},
		},
		Edges: []models.Edge{},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)

	if result.Err != nil {
		t.Fatalf("unexpected execution-level error: %v", result.Err)
	}

	if result.Results[0].Status != "error" {
		t.Errorf("expected error status for empty command node, got '%s'", result.Results[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Sprint 2: New tests
// ---------------------------------------------------------------------------

func TestScheduler_DownstreamSkipOnFailure(t *testing.T) {
	scheduler := NewScheduler(allExecutors(), nil)

	// Linear: s1 (fails) -> s2 -> s3
	// s2 and s3 should be skipped
	dag := &models.DAGConfig{
		ID:   "test-skip",
		Name: "downstream-skip-test",
		Nodes: []models.Node{
			{ID: "s1", Name: "Failing Node", Type: models.NodeTypeScript, Action: "exit 1"},
			{ID: "s2", Name: "Should Skip", Type: models.NodeTypeScript, Action: "echo should-not-run"},
			{ID: "s3", Name: "Also Skip", Type: models.NodeTypeScript, Action: "echo also-should-not-run"},
		},
		Edges: []models.Edge{
			{From: "s1", To: "s2"},
			{From: "s2", To: "s3"},
		},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)

	if result.Err != nil {
		t.Fatalf("unexpected execution-level error: %v", result.Err)
	}

	if len(result.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.Results))
	}

	// Build result map
	resultMap := make(map[string]models.NodeResult)
	for _, r := range result.Results {
		resultMap[r.NodeID] = r
	}

	if resultMap["s1"].Status != "error" {
		t.Errorf("expected s1 to be 'error', got '%s'", resultMap["s1"].Status)
	}
	if resultMap["s2"].Status != "skipped" {
		t.Errorf("expected s2 to be 'skipped', got '%s'", resultMap["s2"].Status)
	}
	if resultMap["s3"].Status != "skipped" {
		t.Errorf("expected s3 to be 'skipped', got '%s'", resultMap["s3"].Status)
	}

	// Verify skipped nodes did not write to state
	if result.State.GetString("s2") != "" {
		t.Error("expected s2 to have no state output")
	}
}

func TestScheduler_PartialFailure_DiamondGraph(t *testing.T) {
	scheduler := NewScheduler(allExecutors(), nil)

	// Diamond: s1 -> s2 (fails), s3 (succeeds) -> s4
	// s2 fails but s3 succeeds. s4 depends on both, so s4 should be skipped.
	dag := &models.DAGConfig{
		ID:   "test-partial",
		Name: "partial-failure-diamond",
		Nodes: []models.Node{
			{ID: "s1", Name: "Start", Type: models.NodeTypeScript, Action: "echo start"},
			{ID: "s2", Name: "Branch Fail", Type: models.NodeTypeScript, Action: "exit 1"},
			{ID: "s3", Name: "Branch OK", Type: models.NodeTypeScript, Action: "echo ok-branch"},
			{ID: "s4", Name: "Join", Type: models.NodeTypeScript, Action: "echo joined"},
		},
		Edges: []models.Edge{
			{From: "s1", To: "s2"},
			{From: "s1", To: "s3"},
			{From: "s2", To: "s4"},
			{From: "s3", To: "s4"},
		},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	resultMap := make(map[string]models.NodeResult)
	for _, r := range result.Results {
		resultMap[r.NodeID] = r
	}

	if resultMap["s1"].Status != "success" {
		t.Errorf("expected s1 success, got %s", resultMap["s1"].Status)
	}
	if resultMap["s2"].Status != "error" {
		t.Errorf("expected s2 error, got %s", resultMap["s2"].Status)
	}
	if resultMap["s3"].Status != "success" {
		t.Errorf("expected s3 success, got %s", resultMap["s3"].Status)
	}
	// s4 depends on both s2 and s3; s2 failed, so s4 should be skipped
	if resultMap["s4"].Status != "skipped" {
		t.Errorf("expected s4 skipped (upstream s2 failed), got %s", resultMap["s4"].Status)
	}
}

func TestScheduler_HumanAndRouterNodes(t *testing.T) {
	scheduler := NewScheduler(allExecutors(), nil)

	dag := &models.DAGConfig{
		ID:   "test-all-types",
		Name: "all-node-types",
		Nodes: []models.Node{
			{ID: "data", Name: "Get Data", Type: models.NodeTypeScript, Action: "echo data-collected"},
			{ID: "route", Name: "Decide", Type: models.NodeTypeRouter, Action: "{{data}}"},
			{ID: "review", Name: "Review", Type: models.NodeTypeHuman, Action: "Please review the data"},
			{ID: "analyze", Name: "Analyze", Type: models.NodeTypeLLM, Action: "Analyze: {{data}}"},
		},
		Edges: []models.Edge{
			{From: "data", To: "route"},
			{From: "route", To: "review"},
			{From: "review", To: "analyze"},
		},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	for _, r := range result.Results {
		if r.NodeType == models.NodeTypeHuman {
			if r.Status != string(models.StatusSuspended) {
				t.Errorf("human node %s should suspend, got %s", r.NodeID, r.Status)
			}
			continue
		}
		if r.Status != "success" {
			t.Errorf("node %s (%s) failed: %s", r.NodeID, r.NodeType, r.Error)
		}
	}
	if result.Status != models.StatusSuspended {
		t.Fatalf("expected workflow suspended, got %s", result.Status)
	}
}

func TestScheduler_ConditionalTargetsDoNotRunPrematurely(t *testing.T) {
	scheduler := NewScheduler(allExecutors(), nil)

	dag := &models.DAGConfig{
		ID:   "test-conditional-routing",
		Name: "conditional-routing",
		Nodes: []models.Node{
			{ID: "data", Name: "Get Flag", Type: models.NodeTypeScript, Action: "echo false"},
			{ID: "route", Name: "Route", Type: models.NodeTypeRouter, Action: "{{data}}"},
			{ID: "report", Name: "Report", Type: models.NodeTypeScript, Action: "echo routed-report"},
			{ID: "review", Name: "Review", Type: models.NodeTypeHuman, Action: "review me"},
		},
		Edges: []models.Edge{
			{From: "data", To: "route"},
			{From: "route", To: "report", Condition: "{{route}} == false"},
			{From: "route", To: "review", Condition: "{{route}} == true"},
		},
	}

	result := scheduler.Execute(context.Background(), dag, nil, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	resultMap := make(map[string]models.NodeResult)
	for _, r := range result.Results {
		resultMap[r.NodeID] = r
	}

	if resultMap["data"].Status != "success" {
		t.Fatalf("expected data success, got %s", resultMap["data"].Status)
	}
	if resultMap["route"].Status != "success" {
		t.Fatalf("expected route success, got %s", resultMap["route"].Status)
	}
	if resultMap["report"].Status != "success" {
		t.Fatalf("expected report success, got %s", resultMap["report"].Status)
	}
	if _, ok := resultMap["review"]; ok {
		t.Fatalf("review should not run when condition is false")
	}
	if result.Status != models.StatusCompleted {
		t.Fatalf("expected workflow completed, got %s", result.Status)
	}
}

func TestScheduler_StressConcurrent(t *testing.T) {
	scheduler := NewScheduler(allExecutors(), nil)

	// Run 50 graph executions concurrently to verify no data races.
	// Each graph: A -> B, C -> D
	const numGraphs = 50
	var wg sync.WaitGroup
	errors := make([]error, numGraphs)

	for i := 0; i < numGraphs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			dag := &models.DAGConfig{
				ID:   fmt.Sprintf("stress-%d", idx),
				Name: fmt.Sprintf("stress-test-%d", idx),
				Nodes: []models.Node{
					{ID: "a", Name: "A", Type: models.NodeTypeScript, Action: fmt.Sprintf("echo graph-%d-a", idx)},
					{ID: "b", Name: "B", Type: models.NodeTypeScript, Action: fmt.Sprintf("echo graph-%d-b from {{a}}", idx)},
					{ID: "c", Name: "C", Type: models.NodeTypeScript, Action: fmt.Sprintf("echo graph-%d-c from {{a}}", idx)},
					{ID: "d", Name: "D", Type: models.NodeTypeScript, Action: "echo joined: {{b}} and {{c}}"},
				},
				Edges: []models.Edge{
					{From: "a", To: "b"},
					{From: "a", To: "c"},
					{From: "b", To: "d"},
					{From: "c", To: "d"},
				},
			}

			result := scheduler.Execute(context.Background(), dag, nil, nil)
			if result.Err != nil {
				errors[idx] = result.Err
				return
			}

			for _, r := range result.Results {
				if r.Status != "success" {
					errors[idx] = fmt.Errorf("node %s failed: %s", r.NodeID, r.Error)
					return
				}
			}

			// Verify state correctness
			aOut := result.State.GetString("a")
			if aOut != fmt.Sprintf("graph-%d-a", idx) {
				errors[idx] = fmt.Errorf("graph %d: expected a output 'graph-%d-a', got '%s'", idx, idx, aOut)
			}
		}(i)
	}

	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Errorf("graph %d failed: %v", i, err)
		}
	}
}
