package episode

import "github.com/Trin9/SynapseFlow/backend/pkg/models"

// ReviewStatus is the allowed review-action status enum from API/application.
type ReviewStatus string

const (
	ReviewStatusApproved   ReviewStatus = "approved"
	ReviewStatusAborted    ReviewStatus = "aborted"
	ReviewStatusOverridden ReviewStatus = "overridden"
)

// IsValid reports whether status is a known review status.
func (s ReviewStatus) IsValid() bool {
	switch s {
	case ReviewStatusApproved, ReviewStatusAborted, ReviewStatusOverridden:
		return true
	default:
		return false
	}
}

// ToHumanAction maps review status to human intervention action.
func (s ReviewStatus) ToHumanAction() models.HumanInterventionAction {
	switch s {
	case ReviewStatusApproved:
		return models.HumanActionResumed
	case ReviewStatusAborted:
		return models.HumanActionAborted
	case ReviewStatusOverridden:
		return models.HumanActionStateOverride
	default:
		return models.HumanActionResumed
	}
}

// ReviewActionInput is the domain-layer input for human review decisions.
type ReviewActionInput struct {
	EpisodeID string
	Status    ReviewStatus
	Actor     string
	Note      string
}
