package approval_snapshot_byte_integrity_test

import (
	"errors"
	"stone-restoration-trial/internal/domain"
	"testing"
	"time"
)

func TestApprovalDigestCoversExactStoredSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	formula := domain.FormulaRevision{
		FormulaID: "formula-1", CaseID: "case-1", RevisionNumber: 1,
		Ingredients:       []domain.Ingredient{{Name: "石灰", Percentage: 100}},
		ApplicationMethod: "薄层刮涂", SubstrateConditions: "清洁并预湿",
		ChangeReason: "初始试配", CreatedBy: "试验员", CreatedAt: now,
	}
	value := &domain.RestorationCase{
		CaseID: "case-1", Version: 7,
		AcceptanceThresholds: domain.Thresholds{MaxColorDifference: 3, MaxWaterAbsorption: 5, MinAdhesionStrength: 1},
		Formulas:             []domain.FormulaRevision{formula},
		Patches: []domain.TrialPatch{{
			PatchID: "patch-1", CaseID: "case-1", FormulaID: formula.FormulaID, PatchCode: "P-01",
			Observations:     []domain.Observation{{ObservationID: "obs-1", PatchID: "patch-1", Stage: domain.StageFinal, ColorDifference: 2, WaterAbsorption: 4, AdhesionStrength: 1.2, ObservedAt: now}},
			EvaluationResult: &domain.EvaluationResult{Conclusion: "passed", EvaluatedAt: now}, CreatedAt: now,
		}},
	}
	snapshot, digest, err := domain.BuildSnapshot(value, formula.FormulaID)
	if err != nil {
		t.Fatal(err)
	}
	record := &domain.ApprovalRecord{
		CaseID: value.CaseID, ApprovedFormulaID: formula.FormulaID,
		FrozenCaseVersion: value.Version + 1, SnapshotDigest: digest,
		SnapshotJSON: "\n" + snapshot,
	}

	_, err = domain.VerifyApprovalSnapshot(record)
	if !errors.Is(err, domain.ErrIntegrity) {
		t.Fatalf("原始冻结快照字节已改变但摘要校验仍通过: %v", err)
	}
	var integrity *domain.IntegrityError
	if !errors.As(err, &integrity) || integrity.VerificationStatus != domain.VerificationDigestMismatch {
		t.Fatalf("应传播 digest_mismatch 完整性错误，实际为: %v", err)
	}
}
