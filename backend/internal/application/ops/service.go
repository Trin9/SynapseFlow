package ops

import (
	"context"
	"errors"
	"fmt"

	"github.com/Trin9/SynapseFlow/backend/internal/audit"
	"github.com/Trin9/SynapseFlow/backend/internal/memory"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// Service orchestrates operational read endpoints (audit/experiences).
type Service struct {
	Audits store.AuditStore
	Memory memory.ExperienceStore
}

var (
	ErrAuditList      = errors.New("failed to list audit entries")
	ErrExperienceList = errors.New("failed to list experiences")
)

// ListAuditEntries returns audit trail entries.
func (s *Service) ListAuditEntries(ctx context.Context) ([]audit.Entry, error) {
	entries, err := s.Audits.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuditList, err)
	}
	return entries, nil
}

// ListExperiences returns memory experiences.
func (s *Service) ListExperiences(ctx context.Context) ([]models.Experience, error) {
	if s.Memory == nil {
		return []models.Experience{}, nil
	}
	experiences, err := s.Memory.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExperienceList, err)
	}
	if experiences == nil {
		experiences = []models.Experience{}
	}
	return experiences, nil
}
