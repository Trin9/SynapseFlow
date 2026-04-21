package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// Retriever injects recalled historical context into the execution state.
type Retriever struct {
	Store ExperienceStore
}

func (r *Retriever) Inject(ctx context.Context, dag *models.DAGConfig, state *models.GlobalState) ([]models.Experience, error) {
	if r == nil || r.Store == nil || state == nil {
		return nil, nil
	}

	query := store.SearchQuery{
		Text:        buildRecallText(dag, state),
		AlertType:   state.GetString("alert_type"),
		ServiceName: state.GetString("service_name"),
		TopK:        3,
	}

	results, err := r.Store.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	state.Set("historical_context", formatHistoricalContext(results))
	state.Set("historical_experiences", results)
	return results, nil
}

func buildRecallText(dag *models.DAGConfig, state *models.GlobalState) string {
	parts := []string{}
	if dag != nil {
		parts = append(parts, dag.Name, dag.Description)
	}

	for _, key := range []string{"alert_text", "alert_summary", "service_name", "alert_type", "symptom", "input"} {
		if value := state.GetString(key); value != "" {
			parts = append(parts, value)
		}
	}

	if snap := state.Snapshot(); len(snap) > 0 {
		for _, key := range []string{"alert", "labels", "annotations"} {
			if value, ok := snap[key]; ok {
				parts = append(parts, fmt.Sprintf("%v", value))
			}
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func formatHistoricalContext(experiences []models.Experience) string {
	if len(experiences) == 0 {
		return ""
	}

	parts := make([]string, 0, len(experiences))
	for idx, exp := range experiences {
		parts = append(parts, fmt.Sprintf(
			"Historical Experience %d\nAlert Type: %s\nService: %s\nSymptom: %s\nRoot Cause: %s\nAction Taken: %s\nSummary: %s",
			idx+1,
			exp.AlertType,
			exp.ServiceName,
			exp.Symptom,
			exp.RootCause,
			exp.ActionTaken,
			exp.Summary,
		))
	}
	return strings.Join(parts, "\n\n")
}
