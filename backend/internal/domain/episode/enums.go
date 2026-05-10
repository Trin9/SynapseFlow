package episode

import "github.com/Trin9/SynapseFlow/backend/pkg/models"

// EpisodeType is the domain enum for episode type.
type EpisodeType string

const (
	EpisodeTypeActionVerification EpisodeType = "action_verification"
	EpisodeTypeInvestigationStep  EpisodeType = "investigation_step"
)

func (t EpisodeType) IsValid() bool {
	switch t {
	case EpisodeTypeActionVerification, EpisodeTypeInvestigationStep:
		return true
	default:
		return false
	}
}

func (t EpisodeType) ToModel() models.EpisodeType {
	return models.EpisodeType(t)
}
