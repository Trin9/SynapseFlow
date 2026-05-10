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

// EpisodeResult is the domain enum for episode verdict result.
type EpisodeResult string

const (
	EpisodeResultPass         EpisodeResult = "pass"
	EpisodeResultFail         EpisodeResult = "fail"
	EpisodeResultInconclusive EpisodeResult = "inconclusive"
)

func (r EpisodeResult) IsValid() bool {
	switch r {
	case EpisodeResultPass, EpisodeResultFail, EpisodeResultInconclusive:
		return true
	default:
		return false
	}
}

func (r EpisodeResult) ToModel() models.EpisodeResult {
	return models.EpisodeResult(r)
}

// EpisodeConfidence is the domain enum for verdict confidence.
type EpisodeConfidence string

const (
	EpisodeConfidenceHigh   EpisodeConfidence = "high"
	EpisodeConfidenceMedium EpisodeConfidence = "medium"
	EpisodeConfidenceLow    EpisodeConfidence = "low"
)

func (c EpisodeConfidence) IsValid() bool {
	switch c {
	case EpisodeConfidenceHigh, EpisodeConfidenceMedium, EpisodeConfidenceLow:
		return true
	default:
		return false
	}
}

func (c EpisodeConfidence) ToModel() models.EpisodeConfidence {
	return models.EpisodeConfidence(c)
}
