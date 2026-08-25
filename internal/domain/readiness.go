package domain

import (
	"fmt"
	"sort"
)

type ReadinessLevel string

const (
	ReadinessBlocked  ReadinessLevel = "blocked"
	ReadinessProgress ReadinessLevel = "in_progress"
	ReadinessReady    ReadinessLevel = "ready"
	ReadinessFrozen   ReadinessLevel = "frozen"
)

type ReadinessIssue struct {
	Code            string        `json:"code"`
	Message         string        `json:"message"`
	EntityType      string        `json:"entityType,omitempty"`
	EntityID        string        `json:"entityID,omitempty"`
	PatchID         string        `json:"patchID,omitempty"`
	PatchCode       string        `json:"patchCode,omitempty"`
	Stage           CuringStage   `json:"stage,omitempty"`
	MissingStages   []CuringStage `json:"missingStages,omitempty"`
	DeviationID     string        `json:"deviationID,omitempty"`
	Metric          string        `json:"metric,omitempty"`
	SuggestedAction string        `json:"suggestedAction"`
}

type ReadinessError struct{ Issues []ReadinessIssue }

func (e *ReadinessError) Error() string {
	return fmt.Sprintf("送审仍有 %d 项阻断", len(e.Issues))
}

func (e *ReadinessError) Unwrap() error { return ErrValidation }

type PatchProgress struct {
	PatchID            string      `json:"patchID"`
	PatchCode          string      `json:"patchCode"`
	FormulaID          string      `json:"formulaID"`
	ParentPatchID      string      `json:"parentPatchID,omitempty"`
	RootPatchID        string      `json:"rootPatchID"`
	RetestRound        int         `json:"retestRound"`
	CompletedStages    int         `json:"completedStages"`
	NextStage          CuringStage `json:"nextStage,omitempty"`
	Conclusion         string      `json:"conclusion,omitempty"`
	OpenDeviationCount int         `json:"openDeviationCount"`
}

type LatestMetrics struct {
	PatchID            string  `json:"patchID,omitempty"`
	PatchCode          string  `json:"patchCode,omitempty"`
	ColorDifference    float64 `json:"colorDifference,omitempty"`
	WaterAbsorption    float64 `json:"waterAbsorption,omitempty"`
	AdhesionStrength   float64 `json:"adhesionStrength,omitempty"`
	SurfaceDefectCount int     `json:"surfaceDefectCount,omitempty"`
}

type ReadinessReport struct {
	Level                ReadinessLevel   `json:"level"`
	CanSubmitReview      bool             `json:"canSubmitReview"`
	TotalFormulaCount    int              `json:"totalFormulaCount"`
	TotalPatchCount      int              `json:"totalPatchCount"`
	CompletedPatchCount  int              `json:"completedPatchCount"`
	OpenDeviationCount   int              `json:"openDeviationCount"`
	PatchProgress        []PatchProgress  `json:"patchProgress"`
	Issues               []ReadinessIssue `json:"issues"`
	LatestPassingMetrics *LatestMetrics   `json:"latestPassingMetrics,omitempty"`
}

func BuildReadiness(c *RestorationCase) ReadinessReport {
	_ = PopulateDerivedEvidence(c)
	report := ReadinessReport{Level: ReadinessProgress, TotalFormulaCount: len(c.Formulas), TotalPatchCount: len(c.Patches), PatchProgress: []PatchProgress{}, Issues: buildReadinessIssues(c)}
	if len(c.Formulas) == 0 {
		report.Issues = append(report.Issues, ReadinessIssue{Code: "formula_missing", Message: "尚未编制材料配方", EntityType: "case", EntityID: c.CaseID, SuggestedAction: "create_formula"})
	}
	if len(c.Patches) == 0 {
		report.Issues = append(report.Issues, ReadinessIssue{Code: "patch_missing", Message: "尚未登记试验块", EntityType: "case", EntityID: c.CaseID, SuggestedAction: "register_patch"})
	}
	for _, patch := range c.Patches {
		progress := PatchProgress{PatchID: patch.PatchID, PatchCode: patch.PatchCode, FormulaID: patch.FormulaID, ParentPatchID: patch.ParentPatchID, RootPatchID: patch.RootPatchID, RetestRound: patch.RetestRound, CompletedStages: len(patch.Observations)}
		if len(patch.Observations) < len(stages) {
			progress.NextStage = stages[len(patch.Observations)]
		}
		if patch.EvaluationResult != nil {
			progress.Conclusion = patch.EvaluationResult.Conclusion
			report.CompletedPatchCount++
			if patch.EvaluationResult.Conclusion == "passed" {
				report.LatestPassingMetrics = metricsFromPatch(patch)
			}
		}
		for _, deviation := range c.Deviations {
			if deviation.PatchID == patch.PatchID && deviation.Status != DeviationClosed {
				progress.OpenDeviationCount++
			}
		}
		report.PatchProgress = append(report.PatchProgress, progress)
	}
	for _, deviation := range c.Deviations {
		if deviation.Status != DeviationClosed {
			report.OpenDeviationCount++
		}
	}
	sortReadiness(report.Issues)
	sort.SliceStable(report.PatchProgress, func(i, j int) bool {
		if report.PatchProgress[i].RootPatchID != report.PatchProgress[j].RootPatchID {
			return report.PatchProgress[i].PatchCode < report.PatchProgress[j].PatchCode
		}
		if report.PatchProgress[i].RetestRound != report.PatchProgress[j].RetestRound {
			return report.PatchProgress[i].RetestRound < report.PatchProgress[j].RetestRound
		}
		return report.PatchProgress[i].PatchCode < report.PatchProgress[j].PatchCode
	})
	report.CanSubmitReview = (c.Status == StatusTesting || c.Status == StatusRemediation) && len(report.Issues) == 0
	if c.Status == StatusApproved {
		report.Level = ReadinessFrozen
	} else if report.CanSubmitReview {
		report.Level = ReadinessReady
	} else if len(report.Issues) > 0 {
		report.Level = ReadinessBlocked
	}
	return report
}

func buildReadinessIssues(c *RestorationCase) []ReadinessIssue {
	issues := []ReadinessIssue{}
	for _, patch := range c.Patches {
		if len(patch.Observations) < len(stages) {
			missing := append([]CuringStage(nil), stages[len(patch.Observations):]...)
			issues = append(issues, ReadinessIssue{Code: "missing_observation_stage", Message: "试验块缺少养护阶段观测", EntityType: "patch", EntityID: patch.PatchID, PatchID: patch.PatchID, PatchCode: patch.PatchCode, Stage: missing[0], MissingStages: missing, SuggestedAction: "record_observation"})
		} else if patch.EvaluationResult == nil {
			issues = append(issues, ReadinessIssue{Code: "final_evaluation_incomplete", Message: "试验块尚未完成终期评价", EntityType: "patch", EntityID: patch.PatchID, PatchID: patch.PatchID, PatchCode: patch.PatchCode, Stage: StageFinal, SuggestedAction: "record_final_evaluation"})
		}
		if patch.EvaluationResult != nil && patch.EvaluationResult.Conclusion == "failed" && !HasPassingDescendant(c, patch.PatchID) {
			issues = append(issues, ReadinessIssue{Code: "passing_retest_missing", Message: "不合格试验块尚无最终合格后代复验", EntityType: "patch", EntityID: patch.PatchID, PatchID: patch.PatchID, PatchCode: patch.PatchCode, SuggestedAction: "remediate_deviation"})
		}
	}
	for _, deviation := range c.Deviations {
		if deviation.Status == DeviationClosed {
			continue
		}
		patchCode := ""
		if patch, err := c.Patch(deviation.PatchID); err == nil {
			patchCode = patch.PatchCode
		}
		issues = append(issues, ReadinessIssue{Code: "deviation_open", Message: "偏差尚未通过复验关闭", EntityType: "deviation", EntityID: deviation.DeviationID, PatchID: deviation.PatchID, PatchCode: patchCode, DeviationID: deviation.DeviationID, Metric: deviation.Metric, SuggestedAction: "remediate_deviation"})
	}
	return deduplicateIssues(issues)
}

func deduplicateIssues(issues []ReadinessIssue) []ReadinessIssue {
	result := make([]ReadinessIssue, 0, len(issues))
	seen := map[string]bool{}
	for _, issue := range issues {
		key := issue.Code + "|" + issue.EntityType + "|" + issue.EntityID + "|" + string(issue.Stage) + "|" + issue.Metric
		if !seen[key] {
			seen[key] = true
			result = append(result, issue)
		}
	}
	return result
}

func sortReadiness(issues []ReadinessIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].PatchCode != issues[j].PatchCode {
			return issues[i].PatchCode < issues[j].PatchCode
		}
		if StageIndex(issues[i].Stage) != StageIndex(issues[j].Stage) {
			return StageIndex(issues[i].Stage) < StageIndex(issues[j].Stage)
		}
		if issues[i].Metric != issues[j].Metric {
			return issues[i].Metric < issues[j].Metric
		}
		return issues[i].Code < issues[j].Code
	})
}

func CanSubmitForReview(c *RestorationCase) error {
	if c.Status != StatusTesting && c.Status != StatusRemediation {
		return ErrInvalidState
	}
	issues := buildReadinessIssues(c)
	if len(c.Patches) == 0 {
		issues = append(issues, ReadinessIssue{Code: "patch_missing", Message: "尚未登记试验块", EntityType: "case", EntityID: c.CaseID, SuggestedAction: "register_patch"})
	}
	if len(issues) > 0 {
		sortReadiness(issues)
		return &ReadinessError{Issues: issues}
	}
	return nil
}

func metricsFromPatch(patch TrialPatch) *LatestMetrics {
	if len(patch.Observations) == 0 {
		return nil
	}
	observation := patch.Observations[len(patch.Observations)-1]
	return &LatestMetrics{PatchID: patch.PatchID, PatchCode: patch.PatchCode, ColorDifference: observation.ColorDifference, WaterAbsorption: observation.WaterAbsorption, AdhesionStrength: observation.AdhesionStrength, SurfaceDefectCount: len(observation.SurfaceDefects)}
}

func (c *RestorationCase) ApprovedFormula() *FormulaRevision {
	if c.Approval == nil || c.Approval.ApprovedFormulaID == "" {
		return nil
	}
	formula, err := c.Formula(c.Approval.ApprovedFormulaID)
	if err != nil {
		return nil
	}
	copy := *formula
	return &copy
}
