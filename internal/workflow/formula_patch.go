package workflow

import (
	"context"
	"stone-restoration-trial/internal/domain"
	"strings"
)

type AddFormulaCommand struct {
	CommandMeta
	Ingredients         []domain.Ingredient `json:"ingredients"`
	ApplicationMethod   string              `json:"applicationMethod"`
	SubstrateConditions string              `json:"substrateConditions"`
	ChangeReason        string              `json:"changeReason"`
}

func (s *Service) AddFormula(ctx context.Context, caseID string, command AddFormulaCommand) (*domain.RestorationCase, bool, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.updateCase(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, requestHash(command), func(c *domain.RestorationCase) ([]domain.AuditEvent, error) {
		if err := c.EnsureMutable(); err != nil {
			return nil, err
		}
		if c.Status != domain.StatusDraft && c.Status != domain.StatusRemediation {
			return nil, domain.ErrInvalidState
		}
		formula := domain.FormulaRevision{FormulaID: newID("formula"), CaseID: caseID, RevisionNumber: c.NextFormulaRevision(), Ingredients: command.Ingredients, ApplicationMethod: strings.TrimSpace(command.ApplicationMethod), SubstrateConditions: strings.TrimSpace(command.SubstrateConditions), ChangeReason: strings.TrimSpace(command.ChangeReason), CreatedBy: command.Actor, CreatedAt: now}
		if err := domain.ValidateFormula(formula); err != nil {
			return nil, err
		}
		c.Formulas = append(c.Formulas, formula)
		c.Touch(now)
		return []domain.AuditEvent{event(caseID, "formula.created", "新增配方修订", command.Actor, now, map[string]any{"formulaID": formula.FormulaID, "revisionNumber": formula.RevisionNumber})}, nil
	})
}

type AddPatchCommand struct {
	CommandMeta
	FormulaID     string   `json:"formulaID"`
	PatchCode     string   `json:"patchCode"`
	PatchCodes    []string `json:"patchCodes,omitempty"`
	ParentPatchID string   `json:"parentPatchID,omitempty"`
}

func (s *Service) AddPatch(ctx context.Context, caseID string, command AddPatchCommand) (*domain.RestorationCase, bool, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.updateCase(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, requestHash(command), func(c *domain.RestorationCase) ([]domain.AuditEvent, error) {
		if err := c.EnsureMutable(); err != nil {
			return nil, err
		}
		if c.Status != domain.StatusDraft && c.Status != domain.StatusTesting && c.Status != domain.StatusRemediation {
			return nil, domain.ErrInvalidState
		}
		formula, err := c.Formula(command.FormulaID)
		if err != nil {
			return nil, domain.Invalid("formulaID", "配方不存在")
		}
		if command.ParentPatchID != "" {
			return nil, domain.Invalid("parentPatchID", "复验块必须通过偏差整改入口登记")
		}
		var codes []string
		if command.PatchCodes != nil {
			if strings.TrimSpace(command.PatchCode) != "" {
				return nil, domain.Invalid("patchCode", "单块编号与成组编号不能同时提交")
			}
			codes, err = domain.ValidatePatchCodes(c, command.PatchCodes)
			if err != nil {
				return nil, err
			}
		} else {
			code := strings.TrimSpace(command.PatchCode)
			if code == "" || len([]rune(code)) > 80 {
				return nil, domain.Invalid("patchCode", "试验块编号不能为空且不能超过 80 个字符")
			}
			if c.HasPatchCode(code) {
				return nil, domain.Invalid("patchCode", "试验块编号已被占用: "+code)
			}
			codes = []string{code}
		}
		patchIDs := make([]string, 0, len(codes))
		for _, code := range codes {
			patchID := newID("patch")
			patch := domain.TrialPatch{PatchID: patchID, CaseID: caseID, FormulaID: command.FormulaID, PatchCode: code, RootPatchID: patchID, Observations: []domain.Observation{}, Trends: []domain.StageTrend{}, CreatedAt: now}
			c.Patches = append(c.Patches, patch)
			patchIDs = append(patchIDs, patchID)
		}
		if c.Status == domain.StatusDraft {
			if err := domain.Transition(c, domain.StatusTesting); err != nil {
				return nil, err
			}
		}
		c.Touch(now)
		eventType, summary := "patch.created", "登记试验块并固化试配输入"
		if len(codes) > 1 {
			eventType, summary = "patch.batch_created", "成组登记同配方试验块并固化试配输入"
		}
		return []domain.AuditEvent{event(caseID, eventType, summary, command.Actor, now, map[string]any{"patchIDs": patchIDs, "patchCodes": codes, "formulaID": command.FormulaID, "formulaRevisionNumber": formula.RevisionNumber})}, nil
	})
}
