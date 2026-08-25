package store

import (
	"context"
	"database/sql"
	"errors"
	"stone-restoration-trial/internal/domain"
)

func (s *SQLiteStore) ListCases(ctx context.Context) ([]domain.RestorationCase, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cases.case_id,
		       EXISTS(SELECT 1 FROM formula_revisions AS formulas WHERE formulas.case_id = cases.case_id)
		FROM restoration_cases AS cases
		ORDER BY cases.updated_at DESC,cases.case_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	prefetched := map[string]domain.RestorationCase{}
	for rows.Next() {
		var id string
		var hasMaterialDetails bool
		if err := rows.Scan(&id, &hasMaterialDetails); err != nil {
			return nil, err
		}
		ids = append(ids, id)
		if hasMaterialDetails {
			value, err := s.GetCase(ctx, id)
			if err != nil {
				return nil, err
			}
			prefetched[id] = *value
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]domain.RestorationCase, 0, len(ids))
	for _, id := range ids {
		if value, ok := prefetched[id]; ok {
			result = append(result, value)
			continue
		}
		value, err := s.GetCase(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, *value)
	}
	return result, nil
}

func (s *SQLiteStore) Timeline(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM restoration_cases WHERE case_id=?`, caseID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, domain.ErrNotFound
	}
	rows, err := s.db.QueryContext(ctx, `SELECT event_id,event_type,summary,actor,data_json,occurred_at FROM audit_events WHERE case_id=? ORDER BY occurred_at,event_id`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.AuditEvent{}
	for rows.Next() {
		var e domain.AuditEvent
		var data, occurred string
		e.CaseID = caseID
		if err := rows.Scan(&e.EventID, &e.EventType, &e.Summary, &e.Actor, &data, &occurred); err != nil {
			return nil, err
		}
		if err := unmarshal(data, &e.Data); err != nil {
			return nil, err
		}
		e.OccurredAt, err = parseTime(occurred)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) Approval(ctx context.Context, caseID string) (*domain.ApprovalRecord, error) {
	value, err := loadApproval(ctx, s.db, caseID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return value, err
}
