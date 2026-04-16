package engine

import (
	"fmt"
	"strings"

	"github.com/xunchenzheng/synapse/pkg/models"
)

// EvaluateCondition parses and evaluates a condition string against the global state.
// Minimal implementation: "key == value" or "key != value" or just "key" (checks if non-empty).
// Variables are wrapped in {{}}.
func EvaluateCondition(condition string, state *models.GlobalState) (bool, error) {
	if condition == "" {
		return true, nil
	}

	// For now, let's support simple equality: {{key}} == value
	if strings.Contains(condition, "==") {
		parts := strings.Split(condition, "==")
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid condition format: %s", condition)
		}
		left := strings.TrimSpace(RenderTemplate(strings.TrimSpace(parts[0]), state))
		right := strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), "'\""))
		return left == right, nil
	}

	if strings.Contains(condition, "!=") {
		parts := strings.Split(condition, "!=")
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid condition format: %s", condition)
		}
		left := strings.TrimSpace(RenderTemplate(strings.TrimSpace(parts[0]), state))
		right := strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), "'\""))
		return left != right, nil
	}

	// Default: check if the rendered string is "true" or non-empty
	rendered := RenderTemplate(condition, state)
	return rendered == "true" || (rendered != "" && rendered != "false"), nil
}
