package domain

import (
	"errors"
	"testing"
	"time"
)

func TestValidateFormulaAndStages(t *testing.T) {
	formula := FormulaRevision{
		Ingredients:       []Ingredient{{Name: "石灰", Percentage: 60}, {Name: "石英砂", Percentage: 40}},
		ApplicationMethod: "薄层刮涂", SubstrateConditions: "清洁湿润", CreatedBy: "试验员",
	}
	if err := ValidateFormula(formula); err != nil {
		t.Fatalf("合法配方被拒绝: %v", err)
	}
	formula.Ingredients[1].Percentage = 30
	if err := ValidateFormula(formula); !errors.Is(err, ErrValidation) {
		t.Fatalf("比例错误未识别: %v", err)
	}
	patch := &TrialPatch{}
	if err := ValidateNextStage(patch, StageStable); !errors.Is(err, ErrValidation) {
		t.Fatalf("阶段乱序未识别: %v", err)
	}
	if err := ValidateNextStage(patch, StageInitial); err != nil {
		t.Fatalf("初期阶段被拒绝: %v", err)
	}
}

func TestEvaluationIsDeterministic(t *testing.T) {
	observation := Observation{Stage: StageFinal, ColorDifference: 4, WaterAbsorption: 5, AdhesionStrength: .8, SurfaceDefects: []string{"起砂"}}
	thresholds := Thresholds{MaxColorDifference: 3, MaxWaterAbsorption: 6, MinAdhesionStrength: 1}
	deviations := EvaluateObservation(observation, thresholds)
	if len(deviations) != 3 {
		t.Fatalf("偏差数量 = %d, 期望 3", len(deviations))
	}
	if deviations[0].Metric != "color_difference" || deviations[1].Metric != "adhesion_strength" || deviations[2].Metric != "surface_defect" {
		t.Fatalf("偏差顺序或类型不稳定: %#v", deviations)
	}
}

func TestReadinessReportsMissingEvidence(t *testing.T) {
	now := time.Now()
	c := &RestorationCase{Status: StatusTesting, Formulas: []FormulaRevision{{FormulaID: "f1"}}, Patches: []TrialPatch{{PatchID: "p1", PatchCode: "T-1", CreatedAt: now}}}
	report := BuildReadiness(c)
	if report.CanSubmitReview {
		t.Fatal("缺少终期观测时不应可送审")
	}
	if report.Level != ReadinessBlocked || len(report.Issues) == 0 {
		t.Fatalf("就绪报告不完整: %#v", report)
	}
}

func TestApprovalSnapshotVerification(t *testing.T) {
	now := time.Now().UTC()
	formula := FormulaRevision{FormulaID: "f1", CaseID: "c1", RevisionNumber: 1, Ingredients: []Ingredient{{Name: "石灰", Percentage: 100}}, ApplicationMethod: "刮涂", SubstrateConditions: "清洁", ChangeReason: "初始", CreatedBy: "甲", CreatedAt: now}
	observation := Observation{ObservationID: "o1", PatchID: "p1", Stage: StageFinal, ColorDifference: 1, WaterAbsorption: 2, AdhesionStrength: 1.5, SurfaceDefects: []string{}, ObservedAt: now.Add(time.Minute), EvidenceSummary: "证据", RecordedBy: "甲"}
	patch := TrialPatch{PatchID: "p1", CaseID: "c1", FormulaID: "f1", PatchCode: "P-1", RootPatchID: "p1", Observations: []Observation{observation}, CreatedAt: now, EvaluationResult: &EvaluationResult{Conclusion: "passed", EvaluatedAt: now.Add(time.Minute)}}
	c := &RestorationCase{CaseID: "c1", Version: 5, AcceptanceThresholds: Thresholds{MaxColorDifference: 3, MaxWaterAbsorption: 5, MinAdhesionStrength: 1}, Formulas: []FormulaRevision{formula}, Patches: []TrialPatch{patch}, Deviations: []Deviation{}}
	snapshot, digest, err := BuildSnapshot(c, "f1")
	if err != nil {
		t.Fatal(err)
	}
	record := &ApprovalRecord{ApprovalID: "a1", CaseID: "c1", ApprovedFormulaID: "f1", FrozenCaseVersion: 6, SnapshotJSON: snapshot, SnapshotDigest: digest}
	evidence, err := VerifyApprovalSnapshot(record)
	if err != nil || evidence.VerificationStatus != VerificationVerified || evidence.Evidence.Thresholds.MaxColorDifference != 3 {
		t.Fatalf("正常快照未通过核验: %#v err=%v", evidence, err)
	}
	record.SnapshotDigest = "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = VerifyApprovalSnapshot(record)
	var integrity *IntegrityError
	if !errors.Is(err, ErrIntegrity) || !errors.As(err, &integrity) || integrity.VerificationStatus != VerificationDigestMismatch {
		t.Fatalf("摘要不匹配状态错误: %v", err)
	}
	record.SnapshotJSON = "{broken"
	_, err = VerifyApprovalSnapshot(record)
	if !errors.As(err, &integrity) || integrity.VerificationStatus != VerificationUnparseable {
		t.Fatalf("损坏快照状态错误: %v", err)
	}
}
