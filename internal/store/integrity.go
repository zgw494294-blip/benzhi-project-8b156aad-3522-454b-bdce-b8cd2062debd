package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

var cachedSchemaVersionStatement struct {
	once      sync.Once
	statement *sql.Stmt
	err       error
}

func schemaVersionStatement(db *sql.DB) (*sql.Stmt, error) {
	cachedSchemaVersionStatement.once.Do(func() {
		cachedSchemaVersionStatement.statement, cachedSchemaVersionStatement.err = db.Prepare(`SELECT version FROM schema_meta LIMIT 1`)
	})
	return cachedSchemaVersionStatement.statement, cachedSchemaVersionStatement.err
}

// Check 验证健康端点依赖的持久化不变量。除连通性之外，
// 它还检查迁移版本、SQLite 完整性、外键和批准冻结状态。
func (s *SQLiteStore) Check(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("数据库连接不可用: %w", err)
	}
	statement, err := schemaVersionStatement(s.db)
	if err != nil {
		return fmt.Errorf("准备 schemaVersion 检查: %w", err)
	}
	var version int
	if err := statement.QueryRowContext(ctx).Scan(&version); err != nil {
		return fmt.Errorf("读取 schemaVersion: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("schemaVersion 不匹配: %d", version)
	}
	var integrity string
	if err := s.db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("执行 SQLite quick_check: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("SQLite 完整性异常: %s", integrity)
	}
	foreignRows, err := s.db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("执行外键检查: %w", err)
	}
	defer foreignRows.Close()
	if foreignRows.Next() {
		return fmt.Errorf("数据库存在外键不一致")
	}
	if err := foreignRows.Err(); err != nil {
		return fmt.Errorf("读取外键检查结果: %w", err)
	}
	return s.checkApprovalInvariants(ctx)
}

func (s *SQLiteStore) checkApprovalInvariants(ctx context.Context) error {
	var invalidApprovals int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM approval_records AS approvals
		JOIN restoration_cases AS cases ON cases.case_id = approvals.case_id
		WHERE cases.status <> 'approved'
		   OR cases.version <> approvals.frozen_case_version
		   OR length(approvals.snapshot_digest) <> 64
	`).Scan(&invalidApprovals)
	if err != nil {
		return fmt.Errorf("检查批准冻结不变量: %w", err)
	}
	if invalidApprovals != 0 {
		return fmt.Errorf("发现 %d 条批准记录未正确冻结", invalidApprovals)
	}
	var orphanEvents int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM audit_events AS events
		LEFT JOIN restoration_cases AS cases ON cases.case_id = events.case_id
		WHERE cases.case_id IS NULL
	`).Scan(&orphanEvents)
	if err != nil {
		return fmt.Errorf("检查审计事件归属: %w", err)
	}
	if orphanEvents != 0 {
		return fmt.Errorf("发现 %d 条无任务归属的审计事件", orphanEvents)
	}
	return nil
}
