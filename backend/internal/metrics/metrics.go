package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

type Collector struct {
	mu                   sync.RWMutex
	executionsByStatus   map[string]int
	executionDurations   []float64
	nodeDurationsByType  map[string][]float64
	mcpCallsByToolStatus map[string]int
	llmTokensByModel     map[string]int
}

func NewCollector() *Collector {
	return &Collector{
		executionsByStatus:   make(map[string]int),
		executionDurations:   make([]float64, 0, 32),
		nodeDurationsByType:  make(map[string][]float64),
		mcpCallsByToolStatus: make(map[string]int),
		llmTokensByModel:     make(map[string]int),
	}
}

func (c *Collector) RecordExecution(status models.ExecutionStatus, duration time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.executionsByStatus[string(status)]++
	c.executionDurations = append(c.executionDurations, duration.Seconds())
}

func (c *Collector) RecordNode(nodeType models.NodeType, duration time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := string(nodeType)
	c.nodeDurationsByType[key] = append(c.nodeDurationsByType[key], duration.Seconds())
}

func (c *Collector) RecordMCPCall(tool, status string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mcpCallsByToolStatus[tool+"|"+status]++
}

func (c *Collector) RecordLLMTokens(model string, tokens int) {
	if c == nil || tokens <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if model == "" {
		model = "unknown"
	}
	c.llmTokensByModel[model] += tokens
}

func (c *Collector) RenderPrometheus() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	var b strings.Builder
	b.WriteString("# HELP synapse_executions_total Total workflow executions by status\n")
	b.WriteString("# TYPE synapse_executions_total counter\n")
	statuses := sortedKeys(c.executionsByStatus)
	for _, status := range statuses {
		b.WriteString(fmt.Sprintf("synapse_executions_total{status=%q} %d\n", status, c.executionsByStatus[status]))
	}

	b.WriteString("# HELP synapse_execution_duration_seconds Workflow execution duration summary\n")
	b.WriteString("# TYPE synapse_execution_duration_seconds gauge\n")
	b.WriteString(renderSeries("synapse_execution_duration_seconds", c.executionDurations, nil))

	b.WriteString("# HELP synapse_node_execution_duration_seconds Node execution duration summary\n")
	b.WriteString("# TYPE synapse_node_execution_duration_seconds gauge\n")
	nodeTypes := sortedSliceKeys(c.nodeDurationsByType)
	for _, nodeType := range nodeTypes {
		b.WriteString(renderSeries("synapse_node_execution_duration_seconds", c.nodeDurationsByType[nodeType], map[string]string{"node_type": nodeType}))
	}

	b.WriteString("# HELP synapse_mcp_calls_total MCP calls by tool and status\n")
	b.WriteString("# TYPE synapse_mcp_calls_total counter\n")
	mcpKeys := sortedKeys(c.mcpCallsByToolStatus)
	for _, key := range mcpKeys {
		parts := strings.SplitN(key, "|", 2)
		tool := parts[0]
		status := "unknown"
		if len(parts) == 2 {
			status = parts[1]
		}
		b.WriteString(fmt.Sprintf("synapse_mcp_calls_total{tool=%q,status=%q} %d\n", tool, status, c.mcpCallsByToolStatus[key]))
	}

	b.WriteString("# HELP synapse_llm_tokens_total LLM tokens by model\n")
	b.WriteString("# TYPE synapse_llm_tokens_total counter\n")
	models := sortedKeys(c.llmTokensByModel)
	for _, model := range models {
		b.WriteString(fmt.Sprintf("synapse_llm_tokens_total{model=%q} %d\n", model, c.llmTokensByModel[model]))
	}

	return b.String()
}

func renderSeries(name string, values []float64, labels map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	min, max, sum := values[0], values[0], 0.0
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg := sum / float64(len(values))
	labelText := ""
	if len(labels) > 0 {
		keys := sortedMapKeys(labels)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%q", key, labels[key]))
		}
		labelText = "{" + strings.Join(parts, ",") + "}"
	}
	return fmt.Sprintf("%s_min%s %.6f\n%s_max%s %.6f\n%s_avg%s %.6f\n", name, labelText, min, name, labelText, max, name, labelText, avg)
}

func sortedKeys[T int](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSliceKeys(m map[string][]float64) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
