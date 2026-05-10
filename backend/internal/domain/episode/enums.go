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

// EpisodeStatus is the domain enum for episode lifecycle status.
type EpisodeStatus string

const (
	EpisodeStatusPending    EpisodeStatus = "pending"
	EpisodeStatusInProgress EpisodeStatus = "in_progress"
	EpisodeStatusConverged  EpisodeStatus = "converged"
	EpisodeStatusEscalated  EpisodeStatus = "escalated"
	EpisodeStatusFailed     EpisodeStatus = "failed"
)

func (s EpisodeStatus) IsValid() bool {
	switch s {
	case EpisodeStatusPending, EpisodeStatusInProgress, EpisodeStatusConverged, EpisodeStatusEscalated, EpisodeStatusFailed:
		return true
	default:
		return false
	}
}

func (s EpisodeStatus) ToModel() models.EpisodeStatus {
	return models.EpisodeStatus(s)
}
