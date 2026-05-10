package episode

import (
	"fmt"
	"strings"
	"time"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// ApplyExecutionTerminalStatus updates in-progress episode status based on execution terminal status.
// Returns true when episode was changed.
func ApplyExecutionTerminalStatus(ep *models.Episode, executionStatus models.ExecutionStatus, now time.Time) bool {
	if ep == nil || ep.Status != EpisodeStatusInProgress.ToModel() {
		return false
	}
	switch executionStatus {
	case models.StatusCompleted:
		ep.Status = EpisodeStatusConverged.ToModel()
	case models.StatusFailed:
		ep.Status = EpisodeStatusFailed.ToModel()
	default:
		return false
	}
	ep.UpdatedAt = now.UTC()
	return true
}

// ReviewStateMutation defines episode status mutations from review actions.
type ReviewStateMutation struct {
	NewStatus       models.EpisodeStatus
	SetConcluded    bool
	ApplyConclusion bool
}

// HumanReviewDisplay carries only the display fields mutated by human review actions.
type HumanReviewDisplay struct {
	VerdictLabel string
	Banner       *string
}

// ReviewMutationFromStatus maps review action status to episode mutation policy.
func ReviewMutationFromStatus(status ReviewStatus, current models.EpisodeStatus) ReviewStateMutation {
	switch status {
	case ReviewStatusApproved:
		if current == EpisodeStatusEscalated.ToModel() {
			return ReviewStateMutation{NewStatus: EpisodeStatusConverged.ToModel(), SetConcluded: true}
		}
		return ReviewStateMutation{}
	case ReviewStatusAborted:
		return ReviewStateMutation{NewStatus: EpisodeStatusFailed.ToModel(), SetConcluded: true}
	case ReviewStatusOverridden:
		return ReviewStateMutation{NewStatus: EpisodeStatusConverged.ToModel(), SetConcluded: true, ApplyConclusion: true}
	default:
		return ReviewStateMutation{}
	}
}

// ReviewStatusToAction maps review request status to intervention action.
func ReviewStatusToAction(status ReviewStatus) models.HumanInterventionAction {
	return status.ToHumanAction()
}

// HumanActionLabel maps intervention action to user-facing label.
func HumanActionLabel(a models.HumanInterventionAction) string {
	switch a {
	case models.HumanActionStateOverride:
		return "State Override"
	case models.HumanActionEvidenceMarkedInvalid:
		return "Evidence Marked Invalid"
	case models.HumanActionHandleInjected:
		return "Handle Injected"
	case models.HumanActionHypothesisCorrected:
		return "Hypothesis Corrected"
	case models.HumanActionSuspended:
		return "Review Requested"
	case models.HumanActionResumed:
		return "Resumed"
	case models.HumanActionAborted:
		return "Aborted"
	default:
		return string(a)
	}
}

// VerdictLabelFromResult maps EpisodeResult to display label.
func VerdictLabelFromResult(r models.EpisodeResult) string {
	switch r {
	case models.EpisodeResultPass:
		return "Pass"
	case models.EpisodeResultFail:
		return "Fail"
	case models.EpisodeResultInconclusive:
		return "Inconclusive"
	default:
		return strings.Title(string(r))
	}
}

// ApplyHumanReviewDisplay updates dossier display fields from human interventions.
func ApplyHumanReviewDisplay(ep *models.Episode, display *HumanReviewDisplay) {
	if ep == nil || display == nil {
		return
	}
	bannerSet := false
	for _, hi := range ep.HumanInterventions {
		switch hi.Action {
		case models.HumanActionStateOverride, models.HumanActionHypothesisCorrected:
			if !bannerSet {
				msg := fmt.Sprintf("Human override: %s", hi.Detail)
				display.Banner = &msg
				bannerSet = true
			}
			display.VerdictLabel = "Overridden (Human)"
		case models.HumanActionResumed:
			display.VerdictLabel = "Approved"
		case models.HumanActionAborted:
			display.VerdictLabel = "Aborted"
		}
	}
}

// ReviewStateFromAction maps intervention action to aggregate review-state status.
func ReviewStateFromAction(a models.HumanInterventionAction) string {
	switch a {
	case models.HumanActionAborted:
		return "aborted"
	case models.HumanActionResumed:
		return "approved"
	default:
		return "overridden"
	}
}
