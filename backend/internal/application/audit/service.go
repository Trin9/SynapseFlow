package audit

import (
	"context"
	"time"

	coreAudit "github.com/Trin9/SynapseFlow/backend/internal/audit"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
)

// RecordInput describes one audit write operation.
type RecordInput struct {
	Actor      string
	Role       string
	Action     string
	Resource   string
	ResourceID string
	Result     string
	Details    string
}

// Service orchestrates audit write use-cases.
type Service struct {
	Audits store.AuditStore
}

// Record writes one audit entry.
func (s *Service) Record(ctx context.Context, input RecordInput) error {
	if s == nil || s.Audits == nil {
		return nil
	}
	return s.Audits.Record(ctx, coreAudit.Entry{
		Time:       time.Now(),
		Actor:      input.Actor,
		Role:       input.Role,
		Action:     input.Action,
		Resource:   input.Resource,
		ResourceID: input.ResourceID,
		Result:     input.Result,
		Details:    input.Details,
	})
}
