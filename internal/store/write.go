package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"stone-restoration-trial/internal/domain"
	"time"
)

func (s *SQLiteStore) CreateCase(ctx context.Context, value *domain.RestorationCase, key, requestHash string, event domain.AuditEvent) (*domain.RestorationCase, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	if previous, found, err := readIdempotent(ctx, tx, "create", key, requestHash); err != nil {
		return nil, false, err
	} else if found {
		return previous, true, nil
	}
	if err := insertCase(ctx, tx, value); err != nil {
		if isConstraint(err) {
			return nil, false, domain.ErrDuplicate
		}
		return nil, false, err
	}
	if err := insertEvent(ctx, tx, event); err != nil {
		return nil, false, err
	}
	if err := writeIdempotent(ctx, tx, "create", key, requestHash, value); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return value, false, nil
}

func (s *SQLiteStore) UpdateCase(ctx context.Context, caseID string, expectedVersion int64, key, requestHash string, mutate func(*domain.RestorationCase) ([]domain.AuditEvent, error)) (*domain.RestorationCase, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	if previous, found, err := readIdempotent(ctx, tx, caseID, key, requestHash); err != nil {
		return nil, false, err
	} else if found {
		return previous, true, nil
	}
	current, err := loadCase(ctx, tx, caseID)
	if err != nil {
		return nil, false, err
	}
	if current.Version != expectedVersion {
		return nil, false, domain.ErrConflict
	}
	events, err := mutate(current)
	if err != nil {
		return nil, false, err
	}
	if err := domain.PopulateDerivedEvidence(current); err != nil {
		return nil, false, err
	}
	if current.Version != expectedVersion+1 {
		return nil, false, fmt.Errorf("工作流必须将版本递增一次")
	}
	result, err := tx.ExecContext(ctx, `UPDATE restoration_cases SET site_name=?, building_area=?, stone_type=?, deterioration_summary=?, target_appearance=?, max_color_difference=?, max_water_absorption=?, min_adhesion_strength=?, status=?, version=?, updated_at=? WHERE case_id=? AND version=?`,
		current.SiteName, current.BuildingArea, current.StoneType, current.DeteriorationSummary, current.TargetAppearance,
		current.AcceptanceThresholds.MaxColorDifference, current.AcceptanceThresholds.MaxWaterAbsorption,
		current.AcceptanceThresholds.MinAdhesionStrength, current.Status, current.Version, timestamp(current.UpdatedAt), current.CaseID, expectedVersion)
	if err != nil {
		return nil, false, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, false, domain.ErrConflict
	}
	if err := replaceChildren(ctx, tx, current); err != nil {
		return nil, false, err
	}
	for _, event := range events {
		if err := insertEvent(ctx, tx, event); err != nil {
			return nil, false, err
		}
	}
	if err := writeIdempotent(ctx, tx, caseID, key, requestHash, current); err != nil {
		return nil, false, err
	}
	// 批准记录与任务变更、审计事件和幂等记录必须同事务原子写入：
	// 触发器、约束或存储资源拒绝批准记录时，整个操作回滚到调用前状态，
	// 既不递增版本、不变更为 approved，也不消耗幂等键，故障解除后同一业务请求可重试完成。
	if current.Approval != nil {
		if err := insertApproval(ctx, tx, *current.Approval); err != nil {
			return nil, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return current, false, nil
}

func insertCase(ctx context.Context, tx *sql.Tx, c *domain.RestorationCase) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO restoration_cases(case_id,site_name,building_area,stone_type,deterioration_summary,target_appearance,max_color_difference,max_water_absorption,min_adhesion_strength,status,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.CaseID, c.SiteName, c.BuildingArea, c.StoneType, c.DeteriorationSummary, c.TargetAppearance,
		c.AcceptanceThresholds.MaxColorDifference, c.AcceptanceThresholds.MaxWaterAbsorption,
		c.AcceptanceThresholds.MinAdhesionStrength, c.Status, c.Version, timestamp(c.CreatedAt), timestamp(c.UpdatedAt))
	return err
}

func replaceChildren(ctx context.Context, tx *sql.Tx, c *domain.RestorationCase) error {
	// 子记录不可原地覆盖业务含义；事务重建仅是聚合持久化策略，ID 和创建时间保持不变。
	// 先解除同一聚合内试验块的自引用，避免 SQLite 在批量重建时按逐行外键检查父子删除顺序。
	if _, err := tx.ExecContext(ctx, `UPDATE trial_patches SET parent_patch_id=NULL WHERE case_id=?`, c.CaseID); err != nil {
		return err
	}
	for _, table := range []string{"observations", "deviations", "trial_patches", "formula_revisions"} {
		query := map[string]string{
			"observations": `DELETE FROM observations WHERE patch_id IN (SELECT patch_id FROM trial_patches WHERE case_id=?)`,
			"deviations":   `DELETE FROM deviations WHERE case_id=?`, "trial_patches": `DELETE FROM trial_patches WHERE case_id=?`,
			"formula_revisions": `DELETE FROM formula_revisions WHERE case_id=?`,
		}[table]
		if _, err := tx.ExecContext(ctx, query, c.CaseID); err != nil {
			return fmt.Errorf("重建聚合时清理 %s: %w", table, err)
		}
	}
	for _, formula := range c.Formulas {
		if err := insertFormula(ctx, tx, formula); err != nil {
			return fmt.Errorf("重建配方 %s: %w", formula.FormulaID, err)
		}
	}
	if err := insertPatchesInTraceOrder(ctx, tx, c.Patches); err != nil {
		return err
	}
	for _, patch := range c.Patches {
		for _, observation := range patch.Observations {
			if err := insertObservation(ctx, tx, observation); err != nil {
				return fmt.Errorf("重建观测 %s: %w", observation.ObservationID, err)
			}
		}
	}
	for _, deviation := range c.Deviations {
		if err := insertDeviation(ctx, tx, deviation); err != nil {
			return fmt.Errorf("重建偏差 %s: %w", deviation.DeviationID, err)
		}
	}
	return nil
}

func insertPatchesInTraceOrder(ctx context.Context, tx *sql.Tx, patches []domain.TrialPatch) error {
	pending := make(map[string]domain.TrialPatch, len(patches))
	inserted := make(map[string]bool, len(patches))
	for _, patch := range patches {
		pending[patch.PatchID] = patch
	}
	for len(pending) > 0 {
		progress := false
		for patchID, patch := range pending {
			if patch.ParentPatchID != "" && !inserted[patch.ParentPatchID] {
				continue
			}
			if err := insertPatch(ctx, tx, patch); err != nil {
				return fmt.Errorf("重建试验块 %s: %w", patch.PatchID, err)
			}
			inserted[patchID] = true
			delete(pending, patchID)
			progress = true
		}
		if !progress {
			return fmt.Errorf("试验块追溯关系包含环或缺失父试验块")
		}
	}
	return nil
}

func insertFormula(ctx context.Context, tx *sql.Tx, f domain.FormulaRevision) error {
	ingredients, err := marshal(f.Ingredients)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO formula_revisions VALUES(?,?,?,?,?,?,?,?,?)`, f.FormulaID, f.CaseID, f.RevisionNumber, ingredients, f.ApplicationMethod, f.SubstrateConditions, f.ChangeReason, f.CreatedBy, timestamp(f.CreatedAt))
	return err
}

func insertPatch(ctx context.Context, tx *sql.Tx, p domain.TrialPatch) error {
	evaluation := any(nil)
	if p.EvaluationResult != nil {
		encoded, err := marshal(p.EvaluationResult)
		if err != nil {
			return err
		}
		evaluation = encoded
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO trial_patches VALUES(?,?,?,?,?,?,?,?,?)`, p.PatchID, p.CaseID, p.FormulaID, p.PatchCode, optional(p.ParentPatchID), p.CuringStage, evaluation, timestamp(p.CreatedAt), nullableTime(p.CompletedAt))
	return err
}

func insertObservation(ctx context.Context, tx *sql.Tx, o domain.Observation) error {
	defects, err := marshal(o.SurfaceDefects)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO observations VALUES(?,?,?,?,?,?,?,?,?,?)`, o.ObservationID, o.PatchID, o.Stage, o.ColorDifference, o.WaterAbsorption, o.AdhesionStrength, defects, timestamp(o.ObservedAt), o.EvidenceSummary, o.RecordedBy)
	return err
}

func insertDeviation(ctx context.Context, tx *sql.Tx, d domain.Deviation) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO deviations VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, d.DeviationID, d.CaseID, d.PatchID, d.Metric, d.MeasuredValue, d.Threshold, d.Severity, d.Cause, d.Disposition, d.Status, optional(d.ReplacementFormulaID), optional(d.RetestPatchID), nullableTime(d.ClosedAt))
	return err
}

type statementExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertApproval(ctx context.Context, executor statementExecutor, a domain.ApprovalRecord) error {
	_, err := executor.ExecContext(ctx, `INSERT INTO approval_records VALUES(?,?,?,?,?,?,?,?,?,?)`, a.ApprovalID, a.CaseID, a.Decision, a.ReviewComment, a.ApprovedFormulaID, a.FrozenCaseVersion, a.SnapshotDigest, a.SnapshotJSON, a.DecidedBy, timestamp(a.DecidedAt))
	return err
}

func insertEvent(ctx context.Context, tx *sql.Tx, e domain.AuditEvent) error {
	data, err := marshal(e.Data)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events VALUES(?,?,?,?,?,?,?)`, e.EventID, e.CaseID, e.EventType, e.Summary, e.Actor, data, timestamp(e.OccurredAt))
	return err
}

func readIdempotent(ctx context.Context, tx *sql.Tx, scope, key, hash string) (*domain.RestorationCase, bool, error) {
	if key == "" {
		return nil, false, domain.Invalid("idempotencyKey", "幂等键不能为空")
	}
	var storedHash, response string
	err := tx.QueryRowContext(ctx, `SELECT request_hash,response_json FROM idempotency_records WHERE scope=? AND idempotency_key=?`, scope, key).Scan(&storedHash, &response)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if storedHash != hash {
		return nil, false, domain.ErrIdempotencyReuse
	}
	var value domain.RestorationCase
	if err := json.Unmarshal([]byte(response), &value); err != nil {
		return nil, false, err
	}
	return &value, true, nil
}

func writeIdempotent(ctx context.Context, tx *sql.Tx, scope, key, hash string, value *domain.RestorationCase) error {
	response, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO idempotency_records VALUES(?,?,?,?,?)`, scope, key, hash, string(response), timestamp(time.Now()))
	return err
}

func isConstraint(err error) bool {
	return err != nil && (contains(err.Error(), "constraint") || contains(err.Error(), "UNIQUE"))
}
func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
