package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Data Structures
// ---------------------------------------------------------------------------

type WorkflowConfig struct {
	Nodes []NodeConfig `json:"nodes"`
	Edges []EdgeConfig `json:"edges"`
}

type NodeConfig struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`   // "script" or "llm"
	Action string `json:"action"` // bash command or LLM prompt template with {{var}} placeholders
}

type EdgeConfig struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type GlobalState struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewGlobalState() *GlobalState {
	return &GlobalState{data: make(map[string]string)}
}

func (s *GlobalState) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

func (s *GlobalState) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *GlobalState) Snapshot() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := make(map[string]string, len(s.data))
	for k, v := range s.data {
		snap[k] = v
	}
	return snap
}

// ---------------------------------------------------------------------------
// Node Execution Results (for audit report)
// ---------------------------------------------------------------------------

type NodeExecResult struct {
	NodeID    string
	NodeName  string
	NodeType  string
	Duration  time.Duration
	Output    string
	TokensIn  int
	TokensOut int
	Err       error
}

// ---------------------------------------------------------------------------
// LLM API (OpenAI-compatible)
// ---------------------------------------------------------------------------

type llmConfig struct {
	apiURL string
	apiKey string
	model  string
}

func loadLLMConfig() llmConfig {
	cfg := llmConfig{
		apiURL: os.Getenv("LLM_API_URL"),
		apiKey: os.Getenv("LLM_API_KEY"),
		model:  os.Getenv("LLM_MODEL"),
	}
	if cfg.apiURL == "" {
		cfg.apiURL = "https://api.openai.com/v1/chat/completions"
	}
	if cfg.model == "" {
		cfg.model = "gpt-4o-mini"
	}
	return cfg
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func callLLM(ctx context.Context, cfg llmConfig, systemPrompt, userPrompt string) (response string, tokensIn, tokensOut int, err error) {
	reqBody := chatRequest{
		Model: cfg.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, 0, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", 0, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, fmt.Errorf("LLM API returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return "", 0, 0, fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return "", 0, 0, fmt.Errorf("LLM API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("LLM API returned no choices")
	}

	return chatResp.Choices[0].Message.Content,
		chatResp.Usage.PromptTokens,
		chatResp.Usage.CompletionTokens,
		nil
}

// ---------------------------------------------------------------------------
// Template Engine: replace {{var}} with GlobalState values
// ---------------------------------------------------------------------------

var templateRegex = regexp.MustCompile(`\{\{(\w+)\}\}`)

func renderTemplate(tmpl string, state *GlobalState) string {
	return templateRegex.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := match[2 : len(match)-2]
		if val, ok := state.Get(key); ok {
			return val
		}
		return match
	})
}

// ---------------------------------------------------------------------------
// DAG Engine: Topological Sort + Concurrent Execution
// ---------------------------------------------------------------------------

func buildDependencyGraph(cfg *WorkflowConfig) (nodeMap map[string]*NodeConfig, deps map[string][]string, reverseDeps map[string][]string) {
	nodeMap = make(map[string]*NodeConfig, len(cfg.Nodes))
	for i := range cfg.Nodes {
		nodeMap[cfg.Nodes[i].ID] = &cfg.Nodes[i]
	}

	deps = make(map[string][]string)
	reverseDeps = make(map[string][]string)

	for _, n := range cfg.Nodes {
		deps[n.ID] = nil
	}

	for _, e := range cfg.Edges {
		deps[e.To] = append(deps[e.To], e.From)
		reverseDeps[e.From] = append(reverseDeps[e.From], e.To)
	}

	return nodeMap, deps, reverseDeps
}

func executeWorkflow(cfg *WorkflowConfig, llmCfg llmConfig) ([]NodeExecResult, time.Duration) {
	engineStart := time.Now()

	_, deps, _ := buildDependencyGraph(cfg)
	state := NewGlobalState()

	results := make([]NodeExecResult, 0, len(cfg.Nodes))
	var resultsMu sync.Mutex

	completedCh := make(map[string]chan struct{})
	for _, n := range cfg.Nodes {
		completedCh[n.ID] = make(chan struct{})
	}

	var wg sync.WaitGroup

	for _, node := range cfg.Nodes {
		wg.Add(1)
		go func(n NodeConfig) {
			defer wg.Done()

			for _, depID := range deps[n.ID] {
				<-completedCh[depID]
			}

			result := executeNode(n, state, llmCfg)

			resultsMu.Lock()
			results = append(results, result)
			resultsMu.Unlock()

			close(completedCh[n.ID])
		}(node)
	}

	wg.Wait()

	return results, time.Since(engineStart)
}

func executeNode(n NodeConfig, state *GlobalState, llmCfg llmConfig) NodeExecResult {
	start := time.Now()
	result := NodeExecResult{
		NodeID:   n.ID,
		NodeName: n.Name,
		NodeType: n.Type,
	}

	switch n.Type {
	case "script":
		output, err := executeScript(n.Action)
		result.Duration = time.Since(start)
		result.Output = output
		result.Err = err
		if err == nil {
			state.Set(n.ID, output)
		}

	case "llm":
		systemPrompt := "You are a senior SRE root-cause analysis engine. " +
			"Based ONLY on the provided factual data (logs, metrics, DB status), perform logical reasoning and output a structured JSON conclusion. " +
			"You must NOT suggest calling any external tools, APIs, or commands. " +
			"You must NOT hallucinate information not present in the provided data. " +
			"Output ONLY valid JSON with keys: root_cause, severity (critical/high/medium/low), confidence (0-100), explanation, recommended_action."

		userPrompt := renderTemplate(n.Action, state)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		output, tokensIn, tokensOut, err := callLLM(ctx, llmCfg, systemPrompt, userPrompt)
		result.Duration = time.Since(start)
		result.Output = output
		result.TokensIn = tokensIn
		result.TokensOut = tokensOut
		result.Err = err
		if err == nil {
			state.Set(n.ID, output)
		}

	default:
		result.Err = fmt.Errorf("unknown node type: %s", n.Type)
		result.Duration = time.Since(start)
	}

	return result
}

func executeScript(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("script failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ---------------------------------------------------------------------------
// Performance Audit Report
// ---------------------------------------------------------------------------

func printAuditReport(results []NodeExecResult, totalDuration time.Duration) {
	separator := strings.Repeat("═", 70)
	thinSep := strings.Repeat("─", 70)

	fmt.Println()
	fmt.Println(separator)
	fmt.Println("  SYNAPSE PoC — PERFORMANCE AUDIT REPORT")
	fmt.Println(separator)

	// Section 1: Execution Timeline
	fmt.Println()
	fmt.Println("  [1] EXECUTION TIMELINE")
	fmt.Println(thinSep)

	var maxHardDuration time.Duration
	var totalLLMDuration time.Duration
	hardCount := 0

	for _, r := range results {
		icon := "⚙️"
		if r.NodeType == "llm" {
			icon = "🧠"
		}
		status := "✅"
		if r.Err != nil {
			status = "❌"
		}

		fmt.Printf("  %s %s [%s] %-25s %10s  %s\n",
			status, icon, r.NodeType, r.NodeName, r.Duration.Round(time.Millisecond), r.NodeID)

		if r.NodeType == "script" {
			hardCount++
			if r.Duration > maxHardDuration {
				maxHardDuration = r.Duration
			}
		} else if r.NodeType == "llm" {
			totalLLMDuration += r.Duration
		}
	}

	fmt.Println(thinSep)
	fmt.Printf("  Hard Nodes (concurrent, max):  %s  (%d nodes ran in parallel)\n",
		maxHardDuration.Round(time.Millisecond), hardCount)
	fmt.Printf("  Soft Nodes (LLM inference):    %s\n", totalLLMDuration.Round(time.Millisecond))
	fmt.Printf("  Total Engine Time:             %s\n", totalDuration.Round(time.Millisecond))

	// Section 2: Token Consumption
	fmt.Println()
	fmt.Println("  [2] TOKEN CONSUMPTION")
	fmt.Println(thinSep)

	totalIn, totalOut := 0, 0
	for _, r := range results {
		if r.NodeType == "llm" {
			totalIn += r.TokensIn
			totalOut += r.TokensOut
			fmt.Printf("  🧠 %-25s  Input: %5d tokens  |  Output: %5d tokens\n",
				r.NodeName, r.TokensIn, r.TokensOut)
		}
	}
	fmt.Println(thinSep)
	fmt.Printf("  Total:  Input = %d tokens  |  Output = %d tokens  |  Sum = %d tokens\n",
		totalIn, totalOut, totalIn+totalOut)

	// Section 3: Determinism Verification
	fmt.Println()
	fmt.Println("  [3] DETERMINISM VERIFICATION — LLM OUTPUT")
	fmt.Println(thinSep)

	for _, r := range results {
		if r.NodeType == "llm" {
			fmt.Printf("  Node: %s (%s)\n", r.NodeName, r.NodeID)
			if r.Err != nil {
				fmt.Printf("  ERROR: %v\n", r.Err)
			} else {
				var prettyJSON bytes.Buffer
				if err := json.Indent(&prettyJSON, []byte(r.Output), "  ", "  "); err != nil {
					fmt.Printf("  (raw output, not valid JSON):\n  %s\n", r.Output)
				} else {
					fmt.Printf("  %s\n", prettyJSON.String())
				}

				output := strings.ToLower(r.Output)
				hasToolCall := strings.Contains(output, "call") && strings.Contains(output, "tool")
				hasHallucination := strings.Contains(output, "i need to") || strings.Contains(output, "let me check") || strings.Contains(output, "i should run")

				if hasToolCall || hasHallucination {
					fmt.Println("  ⚠️  WARNING: LLM output may contain unauthorized tool-calling or hallucination patterns!")
				} else {
					fmt.Println("  ✅ PASS: LLM acted as a pure referee — no unauthorized tool calls detected.")
				}
			}
		}
	}

	// Section 4: Hard Node Outputs (for reference)
	fmt.Println()
	fmt.Println("  [4] HARD NODE OUTPUTS (Collected Facts)")
	fmt.Println(thinSep)

	for _, r := range results {
		if r.NodeType == "script" {
			fmt.Printf("  ⚙️  %s (%s):\n", r.NodeName, r.NodeID)
			for _, line := range strings.Split(r.Output, "\n") {
				fmt.Printf("     %s\n", line)
			}
			fmt.Println()
		}
	}

	fmt.Println(separator)
	fmt.Println("  END OF AUDIT REPORT")
	fmt.Println(separator)
	fmt.Println()
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	llmCfg := loadLLMConfig()

	workflowPath := filepath.Join("cmd", "poc", "test_workflow.json")
	if len(os.Args) > 1 {
		workflowPath = os.Args[1]
	}

	data, err := os.ReadFile(workflowPath)
	if err != nil {
		fmt.Printf("ERROR: Cannot read workflow file %s: %v\n", workflowPath, err)
		os.Exit(1)
	}

	var config WorkflowConfig
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Printf("ERROR: Invalid workflow JSON: %v\n", err)
		os.Exit(1)
	}

	hasLLMNodes := countNodesByType(config.Nodes, "llm") > 0
	if hasLLMNodes && llmCfg.apiKey == "" {
		fmt.Println("ERROR: LLM_API_KEY environment variable is required (workflow contains LLM nodes).")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  LLM_API_KEY=sk-xxx go run cmd/poc/main.go [workflow.json]")
		fmt.Println()
		fmt.Println("Optional environment variables:")
		fmt.Println("  LLM_API_URL  — OpenAI-compatible endpoint (default: https://api.openai.com/v1/chat/completions)")
		fmt.Println("  LLM_MODEL    — Model name (default: gpt-4o-mini)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  LLM_API_KEY=sk-xxx go run cmd/poc/main.go")
		fmt.Println("  LLM_API_KEY=sk-xxx go run cmd/poc/main.go cmd/poc/test_workflow.json")
		fmt.Println("  LLM_API_KEY=sk-xxx LLM_API_URL=https://api.deepseek.com/v1/chat/completions LLM_MODEL=deepseek-chat go run cmd/poc/main.go")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("🚀 Synapse PoC Engine starting...\n")
	fmt.Printf("   Workflow: %s\n", workflowPath)
	fmt.Printf("   Nodes:    %d (%d hard, %d soft)\n",
		len(config.Nodes), countNodesByType(config.Nodes, "script"), countNodesByType(config.Nodes, "llm"))
	if hasLLMNodes {
		fmt.Printf("   LLM:      %s @ %s\n", llmCfg.model, llmCfg.apiURL)
	}
	fmt.Println()

	results, totalDuration := executeWorkflow(&config, llmCfg)

	printAuditReport(results, totalDuration)
}

func countNodesByType(nodes []NodeConfig, nodeType string) int {
	count := 0
	for _, n := range nodes {
		if n.Type == nodeType {
			count++
		}
	}
	return count
}
