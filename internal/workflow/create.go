package workflow

import (
	"context"
	"stone-restoration-trial/internal/domain"
	"strings"
)

type CreateCaseCommand struct {
	IdempotencyKey       string            `json:"idempotencyKey"`
	Actor                string            `json:"actor"`
	SiteName             string            `json:"siteName"`
	BuildingArea         string            `json:"buildingArea"`
	StoneType            string            `json:"stoneType"`
	DeteriorationSummary string            `json:"deteriorationSummary"`
	TargetAppearance     string            `json:"targetAppearance"`
	AcceptanceThresholds domain.Thresholds `json:"acceptanceThresholds"`
}

func (s *Service) CreateCase(ctx context.Context, command CreateCaseCommand) (*domain.RestorationCase, bool, error) {
	if len(strings.TrimSpace(command.IdempotencyKey)) < 8 {
		return nil, false, domain.Invalid("idempotencyKey", "幂等键至少 8 个字符")
	}
	if strings.TrimSpace(command.Actor) == "" {
		return nil, false, domain.Invalid("actor", "操作人不能为空")
	}
	now := s.now().UTC()
	value := &domain.RestorationCase{
		CaseID: newID("case"), SiteName: strings.TrimSpace(command.SiteName), BuildingArea: strings.TrimSpace(command.BuildingArea), StoneType: strings.TrimSpace(command.StoneType),
		DeteriorationSummary: strings.TrimSpace(command.DeteriorationSummary), TargetAppearance: strings.TrimSpace(command.TargetAppearance), AcceptanceThresholds: command.AcceptanceThresholds,
		Status: domain.StatusDraft, Version: 1, Formulas: []domain.FormulaRevision{}, Patches: []domain.TrialPatch{}, Deviations: []domain.Deviation{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := domain.ValidateCaseText(*value); err != nil {
		return nil, false, err
	}
	audit := event(value.CaseID, "case.created", "创建修复试配任务", command.Actor, now, map[string]any{"status": value.Status, "version": value.Version})
	return s.repository.CreateCase(ctx, value, command.IdempotencyKey, requestHash(command), audit)
}
