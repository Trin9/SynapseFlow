package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// Extractor distills a completed execution into a reusable troubleshooting memory.
type Extractor struct {
	Store ExperienceStore
}

func (e *Extractor) Extract(ctx context.Context, dag *models.DAGConfig, exec *models.Execution) (*models.Experience, error) {
	if e == nil || e.Store == nil || exec == nil || exec.State == nil {
		return nil, nil
	}

	snapshot := exec.State.Snapshot()
	rootCause, actionTaken := pickConclusionFields(exec.Results)
	alertType := firstNonEmpty(
		stringValue(snapshot["alert_type"]),
		stringValue(snapshot["alert_name"]),
	)
	serviceName := firstNonEmpty(
		stringValue(snapshot["service_name"]),
		stringValue(snapshot["service"]),
	)
	symptom := firstNonEmpty(
		stringValue(snapshot["alert_text"]),
		stringValue(snapshot["alert_summary"]),
		stringValue(snapshot["symptom"]),
	)

	createdAt := nowUTC()
	exp := &models.Experience{
		ID:          fmt.Sprintf("exp-%d", createdAt.UnixNano()),
		AlertType:   alertType,
		ServiceName: serviceName,
		Tags:        collectTags(alertType, serviceName, rootCause),
		Symptom:     symptom,
		RootCause:   rootCause,
		ActionTaken: actionTaken,
		Summary:     buildSummary(alertType, serviceName, rootCause),
		Document:    buildDocument(dag, exec, snapshot, alertType, serviceName, symptom, rootCause, actionTaken, createdAt),
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}

	if err := e.Store.Save(ctx, exp); err != nil {
		return nil, err
	}
	return exp, nil
}

func pickConclusionFields(results []models.NodeResult) (string, string) {
	for i := len(results) - 1; i >= 0; i-- {
		result := results[i]
		if result.Output == "" {
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &payload); err == nil {
			rootCause := stringValue(payload["root_cause"])
			actionTaken := firstNonEmpty(
				stringValue(payload["recommended_action"]),
				stringValue(payload["action_taken"]),
			)
			if rootCause != "" || actionTaken != "" {
				return rootCause, actionTaken
			}
		}
	}

	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Output != "" {
			return summarizeText(results[i].Output, 240), "Review node output for remediation steps"
		}
	}
	return "", ""
}

func buildSummary(alertType, serviceName, rootCause string) string {
	parts := []string{}
	if alertType != "" {
		parts = append(parts, alertType)
	}
	if serviceName != "" {
		parts = append(parts, serviceName)
	}
	if rootCause != "" {
		parts = append(parts, rootCause)
	}
	return strings.Join(parts, " | ")
}

func buildDocument(dag *models.DAGConfig, exec *models.Execution, snapshot map[string]interface{}, alertType, serviceName, symptom, rootCause, actionTaken string, createdAt time.Time) string {
	stateJSON, _ := json.MarshalIndent(snapshot, "", "  ")
	dagName := ""
	if dag != nil {
		dagName = dag.Name
	}

	return fmt.Sprintf(`---
alert_type: %s
service_name: %s
created_at: %s
tags: [%s]
---

# Experience Summary

- DAG: %s
- Execution ID: %s
- Symptom: %s
- Root Cause: %s
- Action Taken: %s

## Execution State Snapshot

%s
`, alertType, serviceName, createdAt.Format(time.RFC3339), strings.Join(collectTags(alertType, serviceName, rootCause), ", "), dagName, exec.ID, symptom, rootCause, actionTaken, string(stateJSON))
}

func collectTags(values ...string) []string {
	seen := make(map[string]struct{})
	tags := make([]string, 0, len(values))
	for _, value := range values {
		for _, token := range strings.Fields(strings.ToLower(strings.ReplaceAll(value, "|", " "))) {
			clean := strings.Trim(token, ",.:;[]{}()\"'")
			if clean == "" {
				continue
			}
			if _, ok := seen[clean]; ok {
				continue
			}
			seen[clean] = struct{}{}
			tags = append(tags, clean)
		}
	}
	return tags
}

func summarizeText(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return strings.TrimSpace(text[:max]) + "..."
}

func stringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		if value == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
