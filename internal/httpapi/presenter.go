package httpapi

import (
	"stone-restoration-trial/internal/domain"
	"time"
)

type caseListItem struct {
	CaseID         string            `json:"caseID"`
	SiteName       string            `json:"siteName"`
	BuildingArea   string            `json:"buildingArea"`
	StoneType      string            `json:"stoneType"`
	Status         domain.CaseStatus `json:"status"`
	Version        int64             `json:"version"`
	FormulaCount   int               `json:"formulaCount"`
	PatchCount     int               `json:"patchCount"`
	DeviationCount int               `json:"deviationCount"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

type caseDetail struct {
	CaseID               string                   `json:"caseID"`
	SiteName             string                   `json:"siteName"`
	BuildingArea         string                   `json:"buildingArea"`
	StoneType            string                   `json:"stoneType"`
	DeteriorationSummary string                   `json:"deteriorationSummary"`
	TargetAppearance     string                   `json:"targetAppearance"`
	AcceptanceThresholds domain.Thresholds        `json:"acceptanceThresholds"`
	Status               domain.CaseStatus        `json:"status"`
	Version              int64                    `json:"version"`
	Formulas             []domain.FormulaRevision `json:"formulas"`
	Patches              []domain.TrialPatch      `json:"patches"`
	Deviations           []domain.Deviation       `json:"deviations"`
	Approval             *domain.ApprovalRecord   `json:"approval,omitempty"`
	Readiness            domain.ReadinessReport   `json:"readiness"`
	CreatedAt            time.Time                `json:"createdAt"`
	UpdatedAt            time.Time                `json:"updatedAt"`
}

type approvalView struct {
	ApprovalID         string                `json:"approvalID"`
	CaseID             string                `json:"caseID"`
	Decision           domain.Decision       `json:"decision"`
	ReviewComment      string                `json:"reviewComment"`
	ApprovedFormulaID  string                `json:"approvedFormulaID"`
	FrozenCaseVersion  int64                 `json:"frozenCaseVersion"`
	SnapshotDigest     string                `json:"snapshotDigest"`
	DecidedBy          string                `json:"decidedBy"`
	DecidedAt          time.Time             `json:"decidedAt"`
	KeyMetrics         *domain.LatestMetrics `json:"keyMetrics,omitempty"`
	VerificationStatus string                `json:"verificationStatus"`
	Evidence           domain.FrozenSnapshot `json:"evidence"`
}

func presentCaseList(values []domain.RestorationCase) []caseListItem {
	result := make([]caseListItem, 0, len(values))
	for _, value := range values {
		openDeviations := 0
		for _, deviation := range value.Deviations {
			if deviation.Status != domain.DeviationClosed {
				openDeviations++
			}
		}
		result = append(result, caseListItem{
			CaseID:         value.CaseID,
			SiteName:       value.SiteName,
			BuildingArea:   value.BuildingArea,
			StoneType:      value.StoneType,
			Status:         value.Status,
			Version:        value.Version,
			FormulaCount:   len(value.Formulas),
			PatchCount:     len(value.Patches),
			DeviationCount: openDeviations,
			UpdatedAt:      value.UpdatedAt,
		})
	}
	return result
}

func presentCase(value *domain.RestorationCase) caseDetail {
	return caseDetail{
		CaseID:               value.CaseID,
		SiteName:             value.SiteName,
		BuildingArea:         value.BuildingArea,
		StoneType:            value.StoneType,
		DeteriorationSummary: value.DeteriorationSummary,
		TargetAppearance:     value.TargetAppearance,
		AcceptanceThresholds: value.AcceptanceThresholds,
		Status:               value.Status,
		Version:              value.Version,
		Formulas:             value.Formulas,
		Patches:              value.Patches,
		Deviations:           value.Deviations,
		Approval:             value.Approval,
		Readiness:            domain.BuildReadiness(value),
		CreatedAt:            value.CreatedAt,
		UpdatedAt:            value.UpdatedAt,
	}
}

func presentApproval(value *domain.ApprovalEvidence) approvalView {
	record := value.Approval
	var metrics *domain.LatestMetrics
	for i := len(value.Evidence.Patches) - 1; i >= 0; i-- {
		patch := value.Evidence.Patches[i]
		if patch.EvaluationResult != nil && patch.EvaluationResult.Conclusion == "passed" && len(patch.Observations) > 0 {
			observation := patch.Observations[len(patch.Observations)-1]
			metrics = &domain.LatestMetrics{PatchID: patch.PatchID, PatchCode: patch.PatchCode, ColorDifference: observation.ColorDifference, WaterAbsorption: observation.WaterAbsorption, AdhesionStrength: observation.AdhesionStrength, SurfaceDefectCount: len(observation.SurfaceDefects)}
			break
		}
	}
	return approvalView{
		ApprovalID: record.ApprovalID, CaseID: record.CaseID, Decision: record.Decision,
		ReviewComment: record.ReviewComment, ApprovedFormulaID: record.ApprovedFormulaID,
		FrozenCaseVersion: record.FrozenCaseVersion, SnapshotDigest: record.SnapshotDigest,
		DecidedBy: record.DecidedBy, DecidedAt: record.DecidedAt, KeyMetrics: metrics,
		VerificationStatus: value.VerificationStatus, Evidence: value.Evidence,
	}
}
