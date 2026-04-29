package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Trin9/SynapseFlow/backend/internal/llm"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// ---------------------------------------------------------------------------
// Default system prompt for SRE analysis nodes
// ---------------------------------------------------------------------------

const defaultSRESystemPrompt = "You are a senior SRE root-cause analysis engine. " +
	"Based ONLY on the provided factual data, perform logical reasoning and output a structured JSON conclusion. " +
	"You must NOT suggest calling any external tools, APIs, or commands. " +
	"You must NOT hallucinate information not present in the provided data. " +
	"Output ONLY valid JSON with keys: root_cause, severity (critical/high/medium/low), confidence (0-100), explanation, recommended_action."

// ---------------------------------------------------------------------------
// LLMExecutor: Soft Node executor (uses llm.LLMClient abstraction)
// ---------------------------------------------------------------------------

// LLMExecutor calls an LLM API for reasoning tasks via the llm.LLMClient interface.
// Per AGENTS.md: LLM is a Node, not a Scheduler. It receives collected facts
// from GlobalState and outputs structured JSON conclusions.
//
// If Writer is non-nil and an episode_id is available, the executor writes:
//   - A verdict (WriteVerdict) when node.Config["episode_verdict"] == true
//   - A fact-evidence entry (AppendFact) otherwise
type LLMExecutor struct {
	Client llm.LLMClient
	Writer *EpisodeWriter // optional; nil = no Episode tracking
}

func (e *LLMExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()
	result := models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
	}

	systemPrompt := defaultSRESystemPrompt

	// Check if node has custom system prompt in config
	if node.Config != nil {
		if sp, ok := node.Config["system_prompt"].(string); ok && sp != "" {
			systemPrompt = sp
		}
	}

	// Resolve prompt: prefer node.Action, fall back to config["prompt"]
	promptTemplate := node.Action
	if promptTemplate == "" {
		if node.Config != nil {
			if p, ok := node.Config["prompt"].(string); ok {
				promptTemplate = p
			}
		}
	}

	// Render user prompt with state values
	userPrompt := RenderTemplate(promptTemplate, state)
	if historicalContext := strings.TrimSpace(state.GetString("historical_context")); historicalContext != "" && !strings.Contains(userPrompt, historicalContext) {
		userPrompt = strings.TrimSpace(userPrompt + "\n\nHistorical context:\n" + historicalContext)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := llm.ChatOptions{
		JSONMode: true, // SRE analysis nodes always output JSON
	}

	// Allow node config to override options
	if node.Config != nil {
		if temp, ok := node.Config["temperature"].(float64); ok {
			opts.Temperature = temp
		}
		if model, ok := node.Config["model"].(string); ok {
			opts.Model = model
		}
		if maxTokens, ok := node.Config["max_tokens"].(float64); ok {
			opts.MaxTokens = int(maxTokens)
		}
		// Allow disabling JSON mode for free-form analysis
		if jsonMode, ok := node.Config["json_mode"].(bool); ok {
			opts.JSONMode = jsonMode
		}
	}

	resp, err := e.Client.Chat(ctx, messages, opts)
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("LLM call failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Status = "success"
	result.Output = resp.Content
	result.TokensIn = resp.Usage.PromptTokens
	result.TokensOut = resp.Usage.CompletionTokens
	result.Duration = time.Since(start)

	state.Set(node.ID, resp.Content)

	// Episode tracking: Soft Node writes verdict or appends fact evidence.
	if e.Writer != nil {
		episodeID := configString(node.Config, "episode_id")
		if episodeID == "" {
			episodeID = state.GetString("__episode_id__")
		}
		if episodeID != "" {
			isVerdict, _ := node.Config["episode_verdict"].(bool)
			if isVerdict {
				verdict := buildVerdictFromLLMOutput(resp.Content)
				_ = e.Writer.WriteVerdict(ctx, episodeID, node.ID, verdict)
			} else {
				rawCmd := userPrompt
				if len(rawCmd) > 300 {
					rawCmd = rawCmd[:300] + "…"
				}
				spec := &models.EvidenceCollectorSpec{
					CollectorType: "llm_query",
					RawCommand:    rawCmd,
				}
				_ = e.Writer.AppendFactWithSpec(ctx, episodeID, node.ID, node.Type, node.Name, resp.Content, spec)
			}
		}
	}

	if err := writeStructuredLLMState(node, state, resp.Content); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}

	return result
}

// ---------------------------------------------------------------------------
// MockLLMExecutor: For testing without real LLM API
// ---------------------------------------------------------------------------

// MockLLMExecutor simulates an LLM response for testing/development.
// Set Responses[nodeID] to override the generic mock for a specific node.
type MockLLMExecutor struct {
	Writer    *EpisodeWriter    // optional; nil = no Episode tracking
	Responses map[string]string // optional per-node response overrides (keyed by node.ID)
}

func (e *MockLLMExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()

	// Resolve prompt: prefer node.Action, fall back to config["prompt"]
	promptTemplate := node.Action
	if promptTemplate == "" {
		if node.Config != nil {
			if p, ok := node.Config["prompt"].(string); ok {
				promptTemplate = p
			}
		}
	}

	userPrompt := RenderTemplate(promptTemplate, state)
	if historicalContext := strings.TrimSpace(state.GetString("historical_context")); historicalContext != "" && !strings.Contains(userPrompt, historicalContext) {
		userPrompt = strings.TrimSpace(userPrompt + "\n\nHistorical context:\n" + historicalContext)
	}

	// Use a per-node override if provided, otherwise use the generic mock.
	mockResponse := ""
	if e.Responses != nil {
		mockResponse = e.Responses[node.ID]
	}
	if mockResponse == "" {
		mockResponse = fmt.Sprintf(`{
  "root_cause": "Mock analysis based on provided data",
  "severity": "medium",
  "confidence": 75,
  "explanation": "This is a mock LLM response. In production, the LLM would analyze the collected facts and provide real root cause analysis. Input data length: %d chars",
  "recommended_action": "Configure LLM_API_KEY to enable real LLM analysis"
}`, len(userPrompt))
	}

	result := models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
		Status:   "success",
		Output:   mockResponse,
		Duration: time.Since(start),
	}

	state.Set(node.ID, mockResponse)

	// Episode tracking (mock path — same rules as real LLMExecutor).
	if e.Writer != nil {
		episodeID := configString(node.Config, "episode_id")
		if episodeID == "" {
			episodeID = state.GetString("__episode_id__")
		}
		if episodeID != "" {
			isVerdict, _ := node.Config["episode_verdict"].(bool)
			if isVerdict {
				verdict := buildVerdictFromLLMOutput(mockResponse)
				_ = e.Writer.WriteVerdict(ctx, episodeID, node.ID, verdict)
			} else {
				rawCmd := userPrompt
				if len(rawCmd) > 300 {
					rawCmd = rawCmd[:300] + "…"
				}
				spec := &models.EvidenceCollectorSpec{
					CollectorType: "llm_query",
					RawCommand:    rawCmd,
				}
				_ = e.Writer.AppendFactWithSpec(ctx, episodeID, node.ID, node.Type, node.Name, mockResponse, spec)
			}
		}
	}

	if err := writeStructuredLLMState(node, state, mockResponse); err != nil {
		result.Status = "error"
		result.Error = err.Error()
	}

	return result
}

func writeStructuredLLMState(node models.Node, state *models.GlobalState, content string) error {
	if node.Config == nil {
		return nil
	}

	stateKey, _ := node.Config["state_key"].(string)
	if stateKey == "" {
		return nil
	}

	parseJSON, _ := node.Config["parse_json_output"].(bool)
	if !parseJSON {
		state.Set(stateKey, content)
		return nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return fmt.Errorf("failed to parse LLM output for state_key %q: %w", stateKey, err)
	}
	applyStructuredDefaults(node.Config, parsed)
	if err := validateStructuredKeys(node.Config, parsed); err != nil {
		return err
	}
	state.Set(stateKey, parsed)
	return nil
}

func applyStructuredDefaults(config map[string]interface{}, parsed map[string]interface{}) {
	defaults, ok := config["json_defaults"].(map[string]interface{})
	if !ok {
		return
	}

	for key, value := range defaults {
		existing, exists := parsed[key]
		if exists && existing != nil {
			if stringValue, ok := existing.(string); ok && strings.TrimSpace(stringValue) == "" {
				parsed[key] = value
				continue
			}
			continue
		}
		parsed[key] = value
	}
}

func validateStructuredKeys(config map[string]interface{}, parsed map[string]interface{}) error {
	rawKeys, ok := config["required_json_keys"]
	if !ok {
		return nil
	}

	keys := make([]string, 0)
	switch typed := rawKeys.(type) {
	case []string:
		keys = append(keys, typed...)
	case []interface{}:
		for _, key := range typed {
			keyStr, ok := key.(string)
			if !ok || keyStr == "" {
				continue
			}
			keys = append(keys, keyStr)
		}
	}

	for _, key := range keys {
		value, exists := parsed[key]
		if !exists {
			return fmt.Errorf("missing required LLM output key %q", key)
		}
		if value == nil {
			return fmt.Errorf("missing required LLM output key %q", key)
		}
		if stringValue, ok := value.(string); ok && strings.TrimSpace(stringValue) == "" {
			return fmt.Errorf("missing required LLM output key %q", key)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Episode helpers
// ---------------------------------------------------------------------------

// buildVerdictFromLLMOutput parses a JSON LLM response and extracts the fields
// required to populate an EpisodeVerdict.  Best-effort: unknown shapes fall
// back to a plain-text conclusion.
//
// Confidence handling:
//   - String values "high"/"medium"/"low" are used directly.
//   - Float values (0–100 scale) are mapped: ≥80→high, ≥50→medium, else→low.
//
// Result derivation:
//   - business_success == true  → pass
//   - business_success == false → fail
//   - key absent                → inconclusive
func buildVerdictFromLLMOutput(content string) models.EpisodeVerdict {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return models.EpisodeVerdict{
			Conclusion: content,
			Result:     models.EpisodeResultInconclusive,
			Confidence: models.EpisodeConfidenceLow,
		}
	}

	verdict := models.EpisodeVerdict{}

	// Conclusion: prefer summary → root_cause → conclusion → explanation
	for _, key := range []string{"summary", "root_cause", "conclusion", "explanation"} {
		if v, ok := parsed[key].(string); ok && strings.TrimSpace(v) != "" {
			verdict.Conclusion = v
			break
		}
	}

	// Confidence: accept string label or float.
	switch cv := parsed["confidence"].(type) {
	case string:
		switch strings.ToLower(cv) {
		case "high":
			verdict.Confidence = models.EpisodeConfidenceHigh
		case "medium":
			verdict.Confidence = models.EpisodeConfidenceMedium
		default:
			verdict.Confidence = models.EpisodeConfidenceLow
		}
	case float64:
		switch {
		case cv >= 80:
			verdict.Confidence = models.EpisodeConfidenceHigh
		case cv >= 50:
			verdict.Confidence = models.EpisodeConfidenceMedium
		default:
			verdict.Confidence = models.EpisodeConfidenceLow
		}
	default:
		verdict.Confidence = models.EpisodeConfidenceLow
	}

	// Result: derived from business_success flag when present.
	if bs, ok := parsed["business_success"]; ok {
		switch v := bs.(type) {
		case bool:
			if v {
				verdict.Result = models.EpisodeResultPass
			} else {
				verdict.Result = models.EpisodeResultFail
			}
		}
	} else {
		verdict.Result = models.EpisodeResultInconclusive
	}

	// CausalChain: root_cause as first entry when it differs from conclusion.
	if rc, ok := parsed["root_cause"].(string); ok && rc != "" && rc != verdict.Conclusion {
		verdict.CausalChain = []string{rc}
	}

	// Gaps from missing_info array.
	if mi, ok := parsed["missing_info"].([]interface{}); ok {
		for _, item := range mi {
			if s, ok := item.(string); ok && s != "" {
				verdict.Gaps = append(verdict.Gaps, s)
			}
		}
	}

	// Recommendations from recommended_action string or recommendations array.
	if ra, ok := parsed["recommended_action"].(string); ok && strings.TrimSpace(ra) != "" {
		verdict.Recommendations = []string{ra}
	}
	if recs, ok := parsed["recommendations"].([]interface{}); ok {
		for _, item := range recs {
			if s, ok := item.(string); ok && s != "" {
				verdict.Recommendations = append(verdict.Recommendations, s)
			}
		}
	}

	return verdict
}
