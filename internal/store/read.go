package store

import (
	"context"
	"database/sql"
	"errors"
	"stone-restoration-trial/internal/domain"
)

type querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *SQLiteStore) GetCase(ctx context.Context, caseID string) (*domain.RestorationCase, error) {
	return loadCase(ctx, s.db, caseID)
}

func loadCase(ctx context.Context, q querier, caseID string) (*domain.RestorationCase, error) {
	c := &domain.RestorationCase{}
	var created, updated string
	err := q.QueryRowContext(ctx, `SELECT case_id,site_name,building_area,stone_type,deterioration_summary,target_appearance,max_color_difference,max_water_absorption,min_adhesion_strength,status,version,created_at,updated_at FROM restoration_cases WHERE case_id=?`, caseID).Scan(
		&c.CaseID, &c.SiteName, &c.BuildingArea, &c.StoneType, &c.DeteriorationSummary, &c.TargetAppearance,
		&c.AcceptanceThresholds.MaxColorDifference, &c.AcceptanceThresholds.MaxWaterAbsorption,
		&c.AcceptanceThresholds.MinAdhesionStrength, &c.Status, &c.Version, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt, err = parseTime(created)
	if err != nil {
		return nil, err
	}
	c.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return nil, err
	}
	if c.Formulas, err = loadFormulas(ctx, q, caseID); err != nil {
		return nil, err
	}
	if c.Patches, err = loadPatches(ctx, q, caseID); err != nil {
		return nil, err
	}
	if c.Deviations, err = loadDeviations(ctx, q, caseID); err != nil {
		return nil, err
	}
	approval, err := loadApproval(ctx, q, caseID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	c.Approval = approval
	if err := domain.PopulateDerivedEvidence(c); err != nil {
		return nil, err
	}
	return c, nil
}

func loadFormulas(ctx context.Context, q querier, caseID string) ([]domain.FormulaRevision, error) {
	rows, err := q.QueryContext(ctx, `SELECT formula_id,revision_number,ingredients_json,application_method,substrate_conditions,change_reason,created_by,created_at FROM formula_revisions WHERE case_id=? ORDER BY revision_number`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.FormulaRevision{}
	for rows.Next() {
		var f domain.FormulaRevision
		var ingredients, created string
		f.CaseID = caseID
		if err := rows.Scan(&f.FormulaID, &f.RevisionNumber, &ingredients, &f.ApplicationMethod, &f.SubstrateConditions, &f.ChangeReason, &f.CreatedBy, &created); err != nil {
			return nil, err
		}
		if err := unmarshal(ingredients, &f.Ingredients); err != nil {
			return nil, err
		}
		f.CreatedAt, err = parseTime(created)
		if err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

func loadPatches(ctx context.Context, q querier, caseID string) ([]domain.TrialPatch, error) {
	rows, err := q.QueryContext(ctx, `SELECT patch_id,formula_id,patch_code,COALESCE(parent_patch_id,''),curing_stage,COALESCE(evaluation_json,''),created_at,COALESCE(completed_at,'') FROM trial_patches WHERE case_id=? ORDER BY CASE WHEN parent_patch_id IS NULL THEN 0 ELSE 1 END,created_at,patch_id`, caseID)
	if err != nil {
		return nil, err
	}
	result := []domain.TrialPatch{}
	for rows.Next() {
		var p domain.TrialPatch
		var evaluation, created, completed string
		p.CaseID = caseID
		if err := rows.Scan(&p.PatchID, &p.FormulaID, &p.PatchCode, &p.ParentPatchID, &p.CuringStage, &evaluation, &created, &completed); err != nil {
			return nil, err
		}
		p.CreatedAt, err = parseTime(created)
		if err != nil {
			return nil, err
		}
		if completed != "" {
			value, parseErr := parseTime(completed)
			if parseErr != nil {
				return nil, parseErr
			}
			p.CompletedAt = &value
		}
		if evaluation != "" {
			p.EvaluationResult = &domain.EvaluationResult{}
			if err := unmarshal(evaluation, p.EvaluationResult); err != nil {
				return nil, err
			}
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range result {
		result[index].Observations, err = loadObservations(ctx, q, result[index].PatchID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func loadObservations(ctx context.Context, q querier, patchID string) ([]domain.Observation, error) {
	rows, err := q.QueryContext(ctx, `SELECT observation_id,stage,color_difference,water_absorption,adhesion_strength,surface_defects_json,observed_at,evidence_summary,recorded_by FROM observations WHERE patch_id=? ORDER BY CASE stage WHEN 'initial' THEN 1 WHEN 'stable' THEN 2 ELSE 3 END`, patchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Observation{}
	for rows.Next() {
		var o domain.Observation
		var defects, observed string
		o.PatchID = patchID
		if err := rows.Scan(&o.ObservationID, &o.Stage, &o.ColorDifference, &o.WaterAbsorption, &o.AdhesionStrength, &defects, &observed, &o.EvidenceSummary, &o.RecordedBy); err != nil {
			return nil, err
		}
		if err := unmarshal(defects, &o.SurfaceDefects); err != nil {
			return nil, err
		}
		o.ObservedAt, err = parseTime(observed)
		if err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

func loadDeviations(ctx context.Context, q querier, caseID string) ([]domain.Deviation, error) {
	rows, err := q.QueryContext(ctx, `SELECT deviation_id,patch_id,metric,measured_value,threshold_value,severity,cause,disposition,status,COALESCE(replacement_formula_id,''),COALESCE(retest_patch_id,''),COALESCE(closed_at,'') FROM deviations WHERE case_id=? ORDER BY rowid`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Deviation{}
	for rows.Next() {
		var d domain.Deviation
		var closed string
		d.CaseID = caseID
		if err := rows.Scan(&d.DeviationID, &d.PatchID, &d.Metric, &d.MeasuredValue, &d.Threshold, &d.Severity, &d.Cause, &d.Disposition, &d.Status, &d.ReplacementFormulaID, &d.RetestPatchID, &closed); err != nil {
			return nil, err
		}
		if closed != "" {
			value, parseErr := parseTime(closed)
			if parseErr != nil {
				return nil, parseErr
			}
			d.ClosedAt = &value
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func loadApproval(ctx context.Context, q querier, caseID string) (*domain.ApprovalRecord, error) {
	a := &domain.ApprovalRecord{}
	var decided string
	err := q.QueryRowContext(ctx, `SELECT approval_id,decision,review_comment,approved_formula_id,frozen_case_version,snapshot_digest,snapshot_json,decided_by,decided_at FROM approval_records WHERE case_id=?`, caseID).Scan(&a.ApprovalID, &a.Decision, &a.ReviewComment, &a.ApprovedFormulaID, &a.FrozenCaseVersion, &a.SnapshotDigest, &a.SnapshotJSON, &a.DecidedBy, &decided)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.CaseID = caseID
	a.DecidedAt, err = parseTime(decided)
	return a, err
}
