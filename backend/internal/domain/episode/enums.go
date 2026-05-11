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

// EpisodeTriggerType is the domain enum for episode trigger source.
type EpisodeTriggerType string

const (
	EpisodeTriggerAlert     EpisodeTriggerType = "alert"
	EpisodeTriggerWebhook   EpisodeTriggerType = "webhook"
	EpisodeTriggerManual    EpisodeTriggerType = "manual"
	EpisodeTriggerScheduled EpisodeTriggerType = "scheduled"
)

func (t EpisodeTriggerType) IsValid() bool {
	switch t {
	case EpisodeTriggerAlert, EpisodeTriggerWebhook, EpisodeTriggerManual, EpisodeTriggerScheduled:
		return true
	default:
		return false
	}
}

func (t EpisodeTriggerType) ToModel() models.EpisodeTriggerType {
	return models.EpisodeTriggerType(t)
}

// EpisodeEvidenceType is the domain enum for episode evidence entry type.
type EpisodeEvidenceType string

const (
	EpisodeEvidenceTypeFact            EpisodeEvidenceType = "fact"
	EpisodeEvidenceTypeInference       EpisodeEvidenceType = "inference"
	EpisodeEvidenceTypeHumanCorrection EpisodeEvidenceType = "human_correction"
)

func (t EpisodeEvidenceType) IsValid() bool {
	switch t {
	case EpisodeEvidenceTypeFact, EpisodeEvidenceTypeInference, EpisodeEvidenceTypeHumanCorrection:
		return true
	default:
		return false
	}
}

func (t EpisodeEvidenceType) ToModel() models.EpisodeEvidenceType {
	return models.EpisodeEvidenceType(t)
}

// EpisodeHandleType is the domain enum for episode handle kind.
type EpisodeHandleType string

const (
	EpisodeHandleTypeRequestID    EpisodeHandleType = "request_id"
	EpisodeHandleTypeTraceID      EpisodeHandleType = "trace_id"
	EpisodeHandleTypeOrderID      EpisodeHandleType = "order_id"
	EpisodeHandleTypeSessionID    EpisodeHandleType = "session_id"
	EpisodeHandleTypeGitRef       EpisodeHandleType = "git_ref"
	EpisodeHandleTypeDeployRev    EpisodeHandleType = "deploy_revision"
	EpisodeHandleTypeFileLocation EpisodeHandleType = "file_location"
	EpisodeHandleTypePodName      EpisodeHandleType = "pod_name"
	EpisodeHandleTypeCustom       EpisodeHandleType = "custom"
)

func (t EpisodeHandleType) IsValid() bool {
	switch t {
	case EpisodeHandleTypeRequestID,
		EpisodeHandleTypeTraceID,
		EpisodeHandleTypeOrderID,
		EpisodeHandleTypeSessionID,
		EpisodeHandleTypeGitRef,
		EpisodeHandleTypeDeployRev,
		EpisodeHandleTypeFileLocation,
		EpisodeHandleTypePodName,
		EpisodeHandleTypeCustom:
		return true
	default:
		return false
	}
}

func (t EpisodeHandleType) ToModel() models.EpisodeHandleType {
	return models.EpisodeHandleType(t)
}

// HumanInterventionAction is the domain enum for human intervention actions.
type HumanInterventionAction string

const (
	HumanActionStateOverride         HumanInterventionAction = "state_override"
	HumanActionEvidenceMarkedInvalid HumanInterventionAction = "evidence_marked_invalid"
	HumanActionHandleInjected        HumanInterventionAction = "handle_injected"
	HumanActionHypothesisCorrected   HumanInterventionAction = "hypothesis_corrected"
	HumanActionSuspended             HumanInterventionAction = "review_requested"
	HumanActionResumed               HumanInterventionAction = "resumed"
	HumanActionAborted               HumanInterventionAction = "aborted"
)

func (a HumanInterventionAction) IsValid() bool {
	switch a {
	case HumanActionStateOverride,
		HumanActionEvidenceMarkedInvalid,
		HumanActionHandleInjected,
		HumanActionHypothesisCorrected,
		HumanActionSuspended,
		HumanActionResumed,
		HumanActionAborted:
		return true
	default:
		return false
	}
}

func (a HumanInterventionAction) IsResumeAction() bool {
	switch a {
	case HumanActionResumed, HumanActionAborted, HumanActionStateOverride:
		return true
	default:
		return false
	}
}

func (a HumanInterventionAction) ToModel() models.HumanInterventionAction {
	return models.HumanInterventionAction(a)
}
