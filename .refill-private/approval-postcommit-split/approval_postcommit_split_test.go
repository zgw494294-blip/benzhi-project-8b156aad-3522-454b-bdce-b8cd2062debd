package approval_postcommit_split_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"stone-restoration-trial/internal/domain"
	"stone-restoration-trial/internal/store"
	"stone-restoration-trial/internal/workflow"
)

func TestApprovalFailureRollsBackFrozenCase(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "approval-atomicity.db")
	repository, err := store.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("打开测试数据库: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })

	now := time.Date(2026, time.August, 25, 9, 30, 0, 0, time.UTC)
	caseValue := &domain.RestorationCase{
		CaseID:               "case-approval-atomicity",
		SiteName:             "城墙遗址",
		BuildingArea:         "北立面基座",
		StoneType:            "砂岩",
		DeteriorationSummary: "表层粉化",
		TargetAppearance:     "色泽与原石协调",
		AcceptanceThresholds: domain.Thresholds{MaxColorDifference: 3, MaxWaterAbsorption: 5, MinAdhesionStrength: 1},
		Status:               domain.StatusDraft,
		Version:              1,
		Formulas:             []domain.FormulaRevision{},
		Patches:              []domain.TrialPatch{},
		Deviations:           []domain.Deviation{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	createdEvent := domain.AuditEvent{
		EventID: "event-create-atomicity", CaseID: caseValue.CaseID, EventType: "case.created",
		Summary: "创建原子性测试任务", Actor: "测试员", Data: map[string]any{"version": 1}, OccurredAt: now,
	}
	if _, _, err := repository.CreateCase(ctx, caseValue, "atomic-create-001", "create-hash", createdEvent); err != nil {
		t.Fatalf("创建测试任务: %v", err)
	}

	prepared, _, err := repository.UpdateCase(ctx, caseValue.CaseID, 1, "atomic-prepare-001", "prepare-hash", func(current *domain.RestorationCase) ([]domain.AuditEvent, error) {
		formula := domain.FormulaRevision{
			FormulaID: "formula-approved", CaseID: current.CaseID, RevisionNumber: 1,
			Ingredients:       []domain.Ingredient{{Name: "石灰", Percentage: 100}},
			ApplicationMethod: "薄层刮涂", SubstrateConditions: "清洁干燥", ChangeReason: "初始试配",
			CreatedBy: "测试员", CreatedAt: now.Add(time.Minute),
		}
		completedAt := now.Add(2 * time.Minute)
		patch := domain.TrialPatch{
			PatchID: "patch-passed", CaseID: current.CaseID, FormulaID: formula.FormulaID, PatchCode: "A-01",
			CuringStage: domain.StageFinal, EvaluationResult: &domain.EvaluationResult{Conclusion: "passed", EvaluatedAt: completedAt},
			CreatedAt: now.Add(time.Minute), CompletedAt: &completedAt,
		}
		current.Formulas = []domain.FormulaRevision{formula}
		current.Patches = []domain.TrialPatch{patch}
		current.Status = domain.StatusPending
		current.Touch(now.Add(3 * time.Minute))
		return nil, nil
	})
	if err != nil {
		t.Fatalf("准备待审任务: %v", err)
	}

	admin, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatalf("打开故障注入连接: %v", err)
	}
	_, err = admin.ExecContext(ctx, `CREATE TRIGGER force_approval_failure BEFORE INSERT ON approval_records BEGIN SELECT RAISE(ABORT, 'forced approval persistence failure'); END`)
	if err != nil {
		_ = admin.Close()
		t.Fatalf("安装批准写入故障: %v", err)
	}
	if err := admin.Close(); err != nil {
		t.Fatalf("关闭故障注入连接: %v", err)
	}

	service := workflow.NewWithClock(repository, func() time.Time { return now.Add(4 * time.Minute) })
	decision := workflow.ReviewDecisionCommand{
		CommandMeta: workflow.CommandMeta{ExpectedVersion: prepared.Version, IdempotencyKey: "atomic-approve-001", Actor: "批准负责人"},
		Decision:    domain.DecisionApprove, ReviewComment: "证据完整，同意批准", ApprovedFormulaID: "formula-approved",
	}
	_, _, err = service.DecideReview(ctx, prepared.CaseID, decision)
	if err == nil || !strings.Contains(err.Error(), "forced approval persistence failure") {
		t.Fatalf("应返回确定性的批准记录写入错误，实际为 %v", err)
	}

	persisted, err := repository.GetCase(ctx, prepared.CaseID)
	if err != nil {
		t.Fatalf("重新读取任务: %v", err)
	}
	if persisted.Status != domain.StatusPending || persisted.Version != prepared.Version || persisted.Approval != nil {
		t.Errorf("批准记录写入失败后事务未整体回滚: status=%s version=%d approval=%v", persisted.Status, persisted.Version, persisted.Approval)
	}

	admin, err = sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatalf("重新打开故障注入连接: %v", err)
	}
	if _, err := admin.ExecContext(ctx, `DROP TRIGGER force_approval_failure`); err != nil {
		_ = admin.Close()
		t.Fatalf("移除批准写入故障: %v", err)
	}
	if err := admin.Close(); err != nil {
		t.Fatalf("关闭故障注入连接: %v", err)
	}

	_, replayed, err := service.DecideReview(ctx, prepared.CaseID, decision)
	if err != nil {
		t.Fatalf("移除故障后重试批准: %v", err)
	}
	if replayed {
		t.Errorf("失败的批准不应留下可回放的幂等结果")
	}
	persisted, err = repository.GetCase(ctx, prepared.CaseID)
	if err != nil {
		t.Fatalf("读取重试后的任务: %v", err)
	}
	if persisted.Status != domain.StatusApproved || persisted.Approval == nil {
		t.Errorf("重试后应原子保存批准状态和记录: status=%s approval=%v", persisted.Status, persisted.Approval)
	}
}
