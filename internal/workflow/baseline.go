package workflow

import (
	"context"
	"stone-restoration-trial/internal/domain"
	"strings"
)

type ReviseBaselineCommand struct {
	CommandMeta
	DeteriorationSummary string            `json:"deteriorationSummary"`
	TargetAppearance     string            `json:"targetAppearance"`
	AcceptanceThresholds domain.Thresholds `json:"acceptanceThresholds"`
	Reason               string            `json:"reason"`
}

func (s *Service) ReviseBaseline(ctx context.Context, caseID string, command ReviseBaselineCommand) (*domain.RestorationCase, bool, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return nil, false, err
	}
	deterioration := strings.TrimSpace(command.DeteriorationSummary)
	target := strings.TrimSpace(command.TargetAppearance)
	reason := strings.TrimSpace(command.Reason)
	if err := domain.ValidateBaselineRevision(deterioration, target, command.AcceptanceThresholds, reason); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.repository.UpdateCase(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, requestHash(command), func(c *domain.RestorationCase) ([]domain.AuditEvent, error) {
		if c.Status != domain.StatusDraft || len(c.Patches) != 0 {
			return nil, domain.ErrInvalidState
		}
		before := map[string]any{"deteriorationSummary": c.DeteriorationSummary, "targetAppearance": c.TargetAppearance, "acceptanceThresholds": c.AcceptanceThresholds}
		c.DeteriorationSummary = deterioration
		c.TargetAppearance = target
		c.AcceptanceThresholds = command.AcceptanceThresholds
		c.Touch(now)
		after := map[string]any{"deteriorationSummary": c.DeteriorationSummary, "targetAppearance": c.TargetAppearance, "acceptanceThresholds": c.AcceptanceThresholds}
		return []domain.AuditEvent{event(caseID, "baseline.revised", "修订草稿验收基线", command.Actor, now, map[string]any{"reason": reason, "before": before, "after": after, "version": c.Version})}, nil
	})
}
