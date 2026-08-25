package workflow

import (
	"context"
	"errors"
	"stone-restoration-trial/internal/domain"
	"stone-restoration-trial/internal/store"
	"testing"
	"time"
)

func TestCompleteRestorationApprovalFlow(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	service := NewWithClock(repository, func() time.Time { return now })
	created, replayed, err := service.CreateCase(ctx, CreateCaseCommand{
		IdempotencyKey: "create-case-001", Actor: "试验员甲", SiteName: "钟楼", BuildingArea: "南立面勒脚",
		StoneType: "砂岩", DeteriorationSummary: "片状剥落", TargetAppearance: "色泽与原石协调",
		AcceptanceThresholds: domain.Thresholds{MaxColorDifference: 3, MaxWaterAbsorption: 5, MinAdhesionStrength: 1},
	})
	if err != nil || replayed {
		t.Fatalf("创建失败: replay=%v err=%v", replayed, err)
	}
	caseID := created.CaseID
	formulaCommand := AddFormulaCommand{CommandMeta: meta(1, "add-formula-001"), Ingredients: ingredients(60, 40), ApplicationMethod: "薄层刮涂", SubstrateConditions: "基材清洁并预湿", ChangeReason: "初始配方"}
	value, _, err := service.AddFormula(ctx, caseID, formulaCommand)
	if err != nil {
		t.Fatal(err)
	}
	formulaID := value.Formulas[0].FormulaID
	replayedValue, replayed, err := service.AddFormula(ctx, caseID, formulaCommand)
	if err != nil || !replayed || replayedValue.Version != 2 || len(replayedValue.Formulas) != 1 {
		t.Fatalf("幂等重放失败: %#v %v", replayedValue, err)
	}
	value, _, err = service.AddPatch(ctx, caseID, AddPatchCommand{CommandMeta: meta(2, "add-patch-0001"), FormulaID: formulaID, PatchCode: "SP-01"})
	if err != nil {
		t.Fatal(err)
	}
	patchID := value.Patches[0].PatchID
	value = recordThreeStages(t, service, caseID, patchID, 3, now, true)
	if value.Status != domain.StatusRemediation || len(value.Deviations) != 3 {
		t.Fatalf("终期偏差未生成: status=%s deviations=%d", value.Status, len(value.Deviations))
	}
	_, _, err = service.SubmitReview(ctx, caseID, SubmitReviewCommand{CommandMeta: meta(value.Version, "submit-too-early")})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("未闭环偏差应阻止送审: %v", err)
	}
	value, _, err = service.Remediate(ctx, caseID, RemediateCommand{
		CommandMeta: meta(value.Version, "remediate-00001"), DeviationID: value.Deviations[0].DeviationID,
		Cause: "骨料级配偏细", Disposition: "调整骨料比例并重新制作", Ingredients: ingredients(50, 50),
		ApplicationMethod: "分两层压实", SubstrateConditions: "基材清洁并预湿", ChangeReason: "降低收缩与吸水", RetestPatchCode: "SP-01-R1",
	})
	if err != nil {
		t.Fatal(err)
	}
	retestID := value.Patches[1].PatchID
	for _, deviation := range value.Deviations {
		if deviation.Status != domain.DeviationRemediated || deviation.RetestPatchID != retestID {
			t.Fatalf("偏差整改关联错误: %#v", deviation)
		}
	}
	value = recordThreeStages(t, service, caseID, retestID, value.Version, now, false)
	for _, deviation := range value.Deviations {
		if deviation.Status != domain.DeviationClosed {
			t.Fatalf("合格复验未关闭偏差: %#v", deviation)
		}
	}
	value, _, err = service.SubmitReview(ctx, caseID, SubmitReviewCommand{CommandMeta: meta(value.Version, "submit-review-01")})
	if err != nil || value.Status != domain.StatusPending {
		t.Fatalf("送审失败: %#v %v", value, err)
	}
	approvedFormula := value.Formulas[1].FormulaID
	value, _, err = service.DecideReview(ctx, caseID, ReviewDecisionCommand{CommandMeta: meta(value.Version, "approve-review01"), Decision: domain.DecisionApprove, ReviewComment: "证据完整，同意用于限定部位", ApprovedFormulaID: approvedFormula})
	if err != nil {
		t.Fatal(err)
	}
	if value.Status != domain.StatusApproved || value.Approval == nil || len(value.Approval.SnapshotDigest) != 64 {
		t.Fatalf("批准冻结不完整: %#v", value.Approval)
	}
	evidence, err := service.ApprovalEvidence(ctx, caseID)
	if err != nil || evidence.VerificationStatus != domain.VerificationVerified || evidence.Evidence.ApprovedFormula.FormulaID != approvedFormula {
		t.Fatalf("冻结批准证据核验失败: %#v %v", evidence, err)
	}
	_, _, err = service.AddFormula(ctx, caseID, AddFormulaCommand{CommandMeta: meta(value.Version, "mutate-frozen-01"), Ingredients: ingredients(50, 50), ApplicationMethod: "改写", SubstrateConditions: "改写"})
	if !errors.Is(err, domain.ErrFrozen) {
		t.Fatalf("批准后仍可变更: %v", err)
	}
	timeline, err := service.Timeline(ctx, caseID)
	if err != nil || len(timeline) < 14 {
		t.Fatalf("审计时间线不完整: %d %v", len(timeline), err)
	}
}

func TestExpectedVersionConflictAndIdempotencyReuse(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := New(repository)
	c, _, err := service.CreateCase(ctx, CreateCaseCommand{IdempotencyKey: "create-conflict1", Actor: "甲", SiteName: "城墙", BuildingArea: "墙身", StoneType: "砖石", DeteriorationSummary: "酥碱", TargetAppearance: "协调", AcceptanceThresholds: domain.Thresholds{MaxColorDifference: 3, MaxWaterAbsorption: 5, MinAdhesionStrength: 1}})
	if err != nil {
		t.Fatal(err)
	}
	command := AddFormulaCommand{CommandMeta: meta(c.Version+1, "wrong-version-01"), Ingredients: ingredients(60, 40), ApplicationMethod: "刮涂", SubstrateConditions: "干燥"}
	if _, _, err := service.AddFormula(ctx, c.CaseID, command); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("预期版本冲突: %v", err)
	}
	command.ExpectedVersion = c.Version
	command.IdempotencyKey = "same-key-request"
	if _, _, err := service.AddFormula(ctx, c.CaseID, command); err != nil {
		t.Fatal(err)
	}
	command.ApplicationMethod = "不同请求"
	if _, _, err := service.AddFormula(ctx, c.CaseID, command); !errors.Is(err, domain.ErrIdempotencyReuse) {
		t.Fatalf("幂等键复用未拒绝: %v", err)
	}
}

func recordThreeStages(t *testing.T, service *Service, caseID, patchID string, version int64, now time.Time, fail bool) *domain.RestorationCase {
	t.Helper()
	stages := []domain.CuringStage{domain.StageInitial, domain.StageStable, domain.StageFinal}
	var value *domain.RestorationCase
	for index, stage := range stages {
		color, water, adhesion := 2.0, 4.0, 1.2
		defects := []string{}
		if fail && stage == domain.StageFinal {
			color, adhesion, defects = 4.2, .7, []string{"细裂纹"}
		}
		var err error
		value, _, err = service.RecordObservation(context.Background(), caseID, patchID, RecordObservationCommand{CommandMeta: meta(version+int64(index), "observe-"+patchID+string(rune('a'+index))), Stage: stage, ColorDifference: color, WaterAbsorption: water, AdhesionStrength: adhesion, SurfaceDefects: defects, ObservedAt: now.Add(time.Duration(index+1) * time.Minute), EvidenceSummary: "照片与检测记录齐全"})
		if err != nil {
			t.Fatalf("记录 %s 失败: %v", stage, err)
		}
	}
	return value
}

func meta(version int64, key string) CommandMeta {
	return CommandMeta{ExpectedVersion: version, IdempotencyKey: key, Actor: "试验员甲"}
}
func ingredients(lime, sand float64) []domain.Ingredient {
	return []domain.Ingredient{{Name: "石灰", Percentage: lime}, {Name: "石英砂", Percentage: sand}}
}
