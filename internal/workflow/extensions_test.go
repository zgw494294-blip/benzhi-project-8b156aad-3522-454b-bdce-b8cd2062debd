package workflow

import (
	"context"
	"errors"
	"stone-restoration-trial/internal/domain"
	"stone-restoration-trial/internal/store"
	"testing"
	"time"
)

func TestBaselineRevisionAndBatchRegistrationAreAtomic(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	service := NewWithClock(repository, func() time.Time { return now })
	c := createExtensionCase(t, service, now)
	revision := ReviseBaselineCommand{CommandMeta: meta(c.Version, "baseline-revise-01"), DeteriorationSummary: "片状剥落并伴随粉化", TargetAppearance: "纹理连续且色泽协调", AcceptanceThresholds: domain.Thresholds{MaxColorDifference: 2.5, MaxWaterAbsorption: 5, MinAdhesionStrength: 1}, Reason: "建档时颜色差上限录入有误"}
	c, replayed, err := service.ReviseBaseline(ctx, c.CaseID, revision)
	if err != nil || replayed || c.Version != 2 || c.AcceptanceThresholds.MaxColorDifference != 2.5 {
		t.Fatalf("基线修订失败: %#v replay=%v err=%v", c, replayed, err)
	}
	c, _, err = service.AddFormula(ctx, c.CaseID, AddFormulaCommand{CommandMeta: meta(c.Version, "extension-formula-01"), Ingredients: ingredients(60, 40), ApplicationMethod: "薄层刮涂", SubstrateConditions: "清洁并预湿", ChangeReason: "初始试配"})
	if err != nil {
		t.Fatal(err)
	}
	formulaID := c.Formulas[0].FormulaID
	batch := AddPatchCommand{CommandMeta: meta(c.Version, "batch-patches-001"), FormulaID: formulaID, PatchCodes: []string{" B-01 ", "B-02", "B-03"}}
	c, _, err = service.AddPatch(ctx, c.CaseID, batch)
	if err != nil || c.Version != 4 || c.Status != domain.StatusTesting || len(c.Patches) != 3 {
		t.Fatalf("成组登记失败: %#v err=%v", c, err)
	}
	if c.Patches[0].PatchCode != "B-01" {
		t.Fatalf("编号未规范化: %q", c.Patches[0].PatchCode)
	}
	beforeVersion, beforeCount := c.Version, len(c.Patches)
	_, _, err = service.AddPatch(ctx, c.CaseID, AddPatchCommand{CommandMeta: meta(c.Version, "batch-patches-002"), FormulaID: formulaID, PatchCodes: []string{"B-04", "B-02"}})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("占用编号未拒绝: %v", err)
	}
	c, err = service.GetCase(ctx, c.CaseID)
	if err != nil || c.Version != beforeVersion || len(c.Patches) != beforeCount {
		t.Fatalf("失败批次改变聚合: %#v err=%v", c, err)
	}
	replayedCase, replayed, err := service.ReviseBaseline(ctx, c.CaseID, revision)
	if err != nil || !replayed || replayedCase.Version != 2 {
		t.Fatalf("启动试验后基线幂等重放失败: %#v replay=%v err=%v", replayedCase, replayed, err)
	}
	timeline, _ := service.Timeline(ctx, c.CaseID)
	baselineEvents := 0
	for _, event := range timeline {
		if event.EventType == "baseline.revised" {
			baselineEvents++
		}
	}
	if baselineEvents != 1 {
		t.Fatalf("基线事件重复: %d", baselineEvents)
	}
}

func TestTrendWarningsStrictTimeAndMultiRoundCascade(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	now := time.Date(2026, 8, 25, 11, 0, 0, 0, time.UTC)
	service := NewWithClock(repository, func() time.Time { return now })
	c := createExtensionCase(t, service, now)
	c, _, err = service.AddFormula(ctx, c.CaseID, AddFormulaCommand{CommandMeta: meta(c.Version, "cascade-formula-01"), Ingredients: ingredients(60, 40), ApplicationMethod: "薄层刮涂", SubstrateConditions: "清洁并预湿", ChangeReason: "初始试配"})
	if err != nil {
		t.Fatal(err)
	}
	c, _, err = service.AddPatch(ctx, c.CaseID, AddPatchCommand{CommandMeta: meta(c.Version, "cascade-patch-001"), FormulaID: c.Formulas[0].FormulaID, PatchCode: "C-ROOT"})
	if err != nil {
		t.Fatal(err)
	}
	rootID := c.Patches[0].PatchID
	c = observeExtension(t, service, c, rootID, domain.StageInitial, now.Add(time.Minute), 1, 2, 1.5, false, "cascade-observe-01")
	version := c.Version
	_, _, err = service.RecordObservation(ctx, c.CaseID, rootID, RecordObservationCommand{CommandMeta: meta(c.Version, "cascade-time-bad"), Stage: domain.StageStable, ColorDifference: 2, WaterAbsorption: 3, AdhesionStrength: 1.2, ObservedAt: now.Add(30 * time.Second), EvidenceSummary: "时间倒序证据"})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("倒序时间未拒绝: %v", err)
	}
	c, _ = service.GetCase(ctx, c.CaseID)
	if c.Version != version {
		t.Fatalf("倒序观测改变版本: %d", c.Version)
	}
	c = observeExtension(t, service, c, rootID, domain.StageStable, now.Add(2*time.Minute), 2, 3, 1.2, false, "cascade-observe-02")
	if len(c.Patches[0].Trends[1].Warnings) != 3 || len(c.Deviations) != 0 || c.Status != domain.StatusTesting {
		t.Fatalf("稳定期趋势预警语义错误: %#v", c.Patches[0].Trends[1])
	}
	c = observeExtension(t, service, c, rootID, domain.StageFinal, now.Add(3*time.Minute), 4, 6, .7, true, "cascade-observe-03")
	rootDeviationID := c.Deviations[0].DeviationID
	c, _, err = service.Remediate(ctx, c.CaseID, remediateCommand(c.Version, rootDeviationID, "C-R1", "cascade-remediate-1"))
	if err != nil {
		t.Fatal(err)
	}
	roundOneID := c.Patches[1].PatchID
	_, _, err = service.Remediate(ctx, c.CaseID, remediateCommand(c.Version, c.Deviations[1].DeviationID, "C-R1-PARALLEL", "cascade-parallel-1"))
	if !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("平行复验未拒绝: %v", err)
	}
	c = observeThreeExtension(t, service, c, roundOneID, now, true, "r1")
	var roundOneDeviationID string
	for _, deviation := range c.Deviations {
		if deviation.PatchID == roundOneID {
			roundOneDeviationID = deviation.DeviationID
			break
		}
	}
	if roundOneDeviationID == "" {
		t.Fatal("第一轮失败未生成新偏差")
	}
	c, _, err = service.Remediate(ctx, c.CaseID, remediateCommand(c.Version, roundOneDeviationID, "C-R2", "cascade-remediate-2"))
	if err != nil {
		t.Fatal(err)
	}
	roundTwoID := c.Patches[2].PatchID
	c = observeThreeExtension(t, service, c, roundTwoID, now, false, "r2")
	for _, deviation := range c.Deviations {
		if deviation.Status != domain.DeviationClosed || deviation.ClosedAt == nil {
			t.Fatalf("祖先偏差未级联关闭: %#v", deviation)
		}
	}
	if c.Patches[0].RetestRound != 0 || c.Patches[1].RetestRound != 1 || c.Patches[2].RetestRound != 2 || c.Patches[2].RootPatchID != rootID {
		t.Fatalf("复验链派生信息错误: %#v", c.Patches)
	}
	report := domain.BuildReadiness(c)
	if !report.CanSubmitReview || len(report.Issues) != 0 {
		t.Fatalf("多轮闭环后仍阻断送审: %#v", report.Issues)
	}
}

func createExtensionCase(t *testing.T, service *Service, now time.Time) *domain.RestorationCase {
	t.Helper()
	c, _, err := service.CreateCase(context.Background(), CreateCaseCommand{IdempotencyKey: "extension-create-01", Actor: "试验员甲", SiteName: "古建", BuildingArea: "南立面", StoneType: "砂岩", DeteriorationSummary: "片状剥落", TargetAppearance: "色泽协调", AcceptanceThresholds: domain.Thresholds{MaxColorDifference: 3, MaxWaterAbsorption: 5, MinAdhesionStrength: 1}})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func observeExtension(t *testing.T, service *Service, c *domain.RestorationCase, patchID string, stage domain.CuringStage, observedAt time.Time, color, water, adhesion float64, defect bool, key string) *domain.RestorationCase {
	t.Helper()
	defects := []string{}
	if defect {
		defects = []string{"细裂纹"}
	}
	value, _, err := service.RecordObservation(context.Background(), c.CaseID, patchID, RecordObservationCommand{CommandMeta: meta(c.Version, key), Stage: stage, ColorDifference: color, WaterAbsorption: water, AdhesionStrength: adhesion, SurfaceDefects: defects, ObservedAt: observedAt, EvidenceSummary: "阶段照片和检测表"})
	if err != nil {
		t.Fatalf("记录 %s 失败: %v", stage, err)
	}
	return value
}

func observeThreeExtension(t *testing.T, service *Service, c *domain.RestorationCase, patchID string, now time.Time, fail bool, prefix string) *domain.RestorationCase {
	t.Helper()
	c = observeExtension(t, service, c, patchID, domain.StageInitial, now.Add(time.Minute), 1, 2, 1.5, false, prefix+"-observe-01")
	c = observeExtension(t, service, c, patchID, domain.StageStable, now.Add(2*time.Minute), 2, 3, 1.2, false, prefix+"-observe-02")
	color, water, adhesion, defect := 2.5, 4.0, 1.1, false
	if fail {
		color, water, adhesion, defect = 4, 6, .7, true
	}
	return observeExtension(t, service, c, patchID, domain.StageFinal, now.Add(3*time.Minute), color, water, adhesion, defect, prefix+"-observe-03")
}

func remediateCommand(version int64, deviationID, code, key string) RemediateCommand {
	return RemediateCommand{CommandMeta: meta(version, key), DeviationID: deviationID, Cause: "养护与级配需调整", Disposition: "调整配方并进入下一轮复验", Ingredients: ingredients(50, 50), ApplicationMethod: "分层压实", SubstrateConditions: "清洁并预湿", ChangeReason: "复验调整", RetestPatchCode: code}
}
