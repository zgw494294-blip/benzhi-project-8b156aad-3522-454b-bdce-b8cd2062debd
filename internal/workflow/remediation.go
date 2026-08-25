package workflow

import (
	"context"
	"stone-restoration-trial/internal/domain"
	"strings"
)

type RemediateCommand struct {
	CommandMeta
	DeviationID         string              `json:"deviationID"`
	Cause               string              `json:"cause"`
	Disposition         string              `json:"disposition"`
	Ingredients         []domain.Ingredient `json:"ingredients"`
	ApplicationMethod   string              `json:"applicationMethod"`
	SubstrateConditions string              `json:"substrateConditions"`
	ChangeReason        string              `json:"changeReason"`
	RetestPatchCode     string              `json:"retestPatchCode"`
}

func (s *Service) Remediate(ctx context.Context, caseID string, command RemediateCommand) (*domain.RestorationCase, bool, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.repository.UpdateCase(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, requestHash(command), func(c *domain.RestorationCase) ([]domain.AuditEvent, error) {
		if err := c.EnsureMutable(); err != nil {
			return nil, err
		}
		if c.Status != domain.StatusRemediation {
			return nil, domain.ErrInvalidState
		}
		deviation, err := c.Deviation(command.DeviationID)
		if err != nil {
			return nil, err
		}
		if deviation.Status != domain.DeviationOpen {
			return nil, domain.ErrInvalidState
		}
		rootPatchID, retestRound, err := domain.ValidateNextRetest(c, deviation.PatchID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(command.Cause) == "" || strings.TrimSpace(command.Disposition) == "" {
			return nil, domain.Invalid("cause", "偏差原因和处置说明不能为空")
		}
		code := strings.TrimSpace(command.RetestPatchCode)
		if code == "" || c.HasPatchCode(code) {
			return nil, domain.Invalid("retestPatchCode", "复验块编号不能为空且不得重复")
		}
		formula := domain.FormulaRevision{FormulaID: newID("formula"), CaseID: caseID, RevisionNumber: c.NextFormulaRevision(), Ingredients: command.Ingredients, ApplicationMethod: strings.TrimSpace(command.ApplicationMethod), SubstrateConditions: strings.TrimSpace(command.SubstrateConditions), ChangeReason: strings.TrimSpace(command.ChangeReason), CreatedBy: command.Actor, CreatedAt: now}
		if err := domain.ValidateFormula(formula); err != nil {
			return nil, err
		}
		patch := domain.TrialPatch{PatchID: newID("patch"), CaseID: caseID, FormulaID: formula.FormulaID, PatchCode: code, ParentPatchID: deviation.PatchID, RootPatchID: rootPatchID, RetestRound: retestRound, Observations: []domain.Observation{}, Trends: []domain.StageTrend{}, CreatedAt: now}
		c.Formulas = append(c.Formulas, formula)
		c.Patches = append(c.Patches, patch)
		// 同一原试验块的所有开放偏差共享本次原因、处置和复验链路，避免多次建立重复复验块。
		for i := range c.Deviations {
			d := &c.Deviations[i]
			if d.PatchID == deviation.PatchID && d.Status == domain.DeviationOpen {
				d.Cause = strings.TrimSpace(command.Cause)
				d.Disposition = strings.TrimSpace(command.Disposition)
				d.ReplacementFormulaID = formula.FormulaID
				d.RetestPatchID = patch.PatchID
				d.Status = domain.DeviationRemediated
			}
		}
		c.Touch(now)
		return []domain.AuditEvent{
			event(caseID, "deviation.remediated", "登记偏差原因、处置与替代配方", command.Actor, now, map[string]any{"deviationID": command.DeviationID, "formulaID": formula.FormulaID, "rootPatchID": rootPatchID, "retestRound": retestRound}),
			event(caseID, "retest.created", "创建可追溯的下一轮复验试验块", command.Actor, now, map[string]any{"patchID": patch.PatchID, "parentPatchID": patch.ParentPatchID, "rootPatchID": rootPatchID, "retestRound": retestRound}),
		}, nil
	})
}
