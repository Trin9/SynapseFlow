package engine

import (
	"testing"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

func TestRenderTemplate_DottedPath(t *testing.T) {
	state := models.NewGlobalState()
	state.Set("plan", map[string]interface{}{
		"scope": "src/checkout",
		"query": "failed to charge card",
	})

	rendered := RenderTemplate("search {{plan.scope}} for {{plan.query}}", state)
	if rendered != "search src/checkout for failed to charge card" {
		t.Fatalf("unexpected rendered template: %q", rendered)
	}
}

func TestEvaluateCondition_DottedPathAndBoolean(t *testing.T) {
	state := models.NewGlobalState()
	state.Set("plan", map[string]interface{}{
		"next_action": "search_code",
		"done":        false,
		"attempt":     2,
	})

	tests := []struct {
		name      string
		condition string
		want      bool
	}{
		{name: "string equality", condition: "{{plan.next_action}} == search_code", want: true},
		{name: "boolean equality", condition: "{{plan.done}} == false", want: true},
		{name: "boolean truthy default", condition: "{{plan.done}}", want: false},
		{name: "numeric equality", condition: "{{plan.attempt}} == 2", want: true},
		{name: "string inequality", condition: "{{plan.next_action}} != finish_report", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EvaluateCondition(tc.condition, state)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("condition %q: expected %v, got %v", tc.condition, tc.want, got)
			}
		})
	}
}
