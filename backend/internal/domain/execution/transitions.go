package execution

import "github.com/Trin9/SynapseFlow/backend/pkg/models"

// ResolveTerminalStatus maps scheduler result/error to persisted execution status.
func ResolveTerminalStatus(resultStatus models.ExecutionStatus, resultErr error) models.ExecutionStatus {
	if resultErr != nil || resultStatus == models.StatusFailed {
		return models.StatusFailed
	}
	if resultStatus == models.StatusSuspended {
		return models.StatusSuspended
	}
	return models.StatusCompleted
}
