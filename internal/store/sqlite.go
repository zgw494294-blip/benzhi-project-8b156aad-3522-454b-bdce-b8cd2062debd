package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*SQLiteStore, error) {
	dsn := path
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		dsn = "file:" + path
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &SQLiteStore{db: db}
	if err := store.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) initialize(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA busy_timeout = 5000`,
		`CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL)`,
		`INSERT INTO schema_meta(version) SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM schema_meta)`,
		`CREATE TABLE IF NOT EXISTS restoration_cases (
			case_id TEXT PRIMARY KEY, site_name TEXT NOT NULL, building_area TEXT NOT NULL,
			stone_type TEXT NOT NULL, deterioration_summary TEXT NOT NULL, target_appearance TEXT NOT NULL,
			max_color_difference REAL NOT NULL, max_water_absorption REAL NOT NULL,
			min_adhesion_strength REAL NOT NULL, status TEXT NOT NULL, version INTEGER NOT NULL,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS formula_revisions (
			formula_id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES restoration_cases(case_id),
			revision_number INTEGER NOT NULL, ingredients_json TEXT NOT NULL, application_method TEXT NOT NULL,
			substrate_conditions TEXT NOT NULL, change_reason TEXT NOT NULL, created_by TEXT NOT NULL,
			created_at TEXT NOT NULL, UNIQUE(case_id, revision_number))`,
		`CREATE TABLE IF NOT EXISTS trial_patches (
			patch_id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES restoration_cases(case_id),
			formula_id TEXT NOT NULL REFERENCES formula_revisions(formula_id), patch_code TEXT NOT NULL,
			parent_patch_id TEXT REFERENCES trial_patches(patch_id), curing_stage TEXT NOT NULL,
			evaluation_json TEXT, created_at TEXT NOT NULL, completed_at TEXT, UNIQUE(case_id, patch_code))`,
		`CREATE TABLE IF NOT EXISTS observations (
			observation_id TEXT PRIMARY KEY, patch_id TEXT NOT NULL REFERENCES trial_patches(patch_id),
			stage TEXT NOT NULL, color_difference REAL NOT NULL, water_absorption REAL NOT NULL,
			adhesion_strength REAL NOT NULL, surface_defects_json TEXT NOT NULL, observed_at TEXT NOT NULL,
			evidence_summary TEXT NOT NULL, recorded_by TEXT NOT NULL, UNIQUE(patch_id, stage))`,
		`CREATE TABLE IF NOT EXISTS deviations (
			deviation_id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES restoration_cases(case_id),
			patch_id TEXT NOT NULL REFERENCES trial_patches(patch_id), metric TEXT NOT NULL,
			measured_value TEXT NOT NULL, threshold_value TEXT NOT NULL, severity TEXT NOT NULL,
			cause TEXT NOT NULL, disposition TEXT NOT NULL, status TEXT NOT NULL,
			replacement_formula_id TEXT, retest_patch_id TEXT, closed_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS approval_records (
			approval_id TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE REFERENCES restoration_cases(case_id),
			decision TEXT NOT NULL, review_comment TEXT NOT NULL, approved_formula_id TEXT NOT NULL,
			frozen_case_version INTEGER NOT NULL, snapshot_digest TEXT NOT NULL, snapshot_json TEXT NOT NULL,
			decided_by TEXT NOT NULL, decided_at TEXT NOT NULL)`,
		`CREATE TRIGGER IF NOT EXISTS approval_records_no_update
			BEFORE UPDATE ON approval_records BEGIN
			SELECT RAISE(ABORT, 'approval records are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS approval_records_no_delete
			BEFORE DELETE ON approval_records BEGIN
			SELECT RAISE(ABORT, 'approval records are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS idempotency_records (
			scope TEXT NOT NULL, idempotency_key TEXT NOT NULL, request_hash TEXT NOT NULL,
			response_json TEXT NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY(scope, idempotency_key))`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			event_id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES restoration_cases(case_id),
			event_type TEXT NOT NULL, summary TEXT NOT NULL, actor TEXT NOT NULL,
			data_json TEXT NOT NULL, occurred_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_events_case_time ON audit_events(case_id, occurred_at, event_id)`,
		`CREATE INDEX IF NOT EXISTS idx_patches_case ON trial_patches(case_id)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("初始化数据库: %w", err)
		}
	}
	var version int
	if err := s.db.QueryRowContext(ctx, `SELECT version FROM schema_meta LIMIT 1`).Scan(&version); err != nil {
		return err
	}
	if version != 1 {
		return fmt.Errorf("不支持的 schemaVersion: %d", version)
	}
	return nil
}
