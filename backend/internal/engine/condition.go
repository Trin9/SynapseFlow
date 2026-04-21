package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
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
		return compareConditionValues(left, right), nil
	}

	if strings.Contains(condition, "!=") {
		parts := strings.Split(condition, "!=")
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid condition format: %s", condition)
		}
		left := strings.TrimSpace(RenderTemplate(strings.TrimSpace(parts[0]), state))
		right := strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), "'\""))
		return !compareConditionValues(left, right), nil
	}

	// Default: check if the rendered string is "true" or non-empty
	rendered := RenderTemplate(condition, state)
	return truthyConditionValue(strings.TrimSpace(rendered)), nil
}

func compareConditionValues(left, right string) bool {
	leftBool, leftIsBool := parseBoolConditionValue(left)
	rightBool, rightIsBool := parseBoolConditionValue(right)
	if leftIsBool && rightIsBool {
		return leftBool == rightBool
	}

	leftInt, leftIsInt := parseIntConditionValue(left)
	rightInt, rightIsInt := parseIntConditionValue(right)
	if leftIsInt && rightIsInt {
		return leftInt == rightInt
	}

	return left == right
}

func truthyConditionValue(value string) bool {
	if value == "" {
		return false
	}
	if parsed, ok := parseBoolConditionValue(value); ok {
		return parsed
	}
	return true
}

func parseBoolConditionValue(value string) (bool, bool) {
	parsed, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(value)))
	if err != nil {
		return false, false
	}
	return parsed, true
}

func parseIntConditionValue(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
