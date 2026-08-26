package health_statement_reopen_test

import (
	"context"
	"path/filepath"
	"testing"

	"stone-restoration-trial/internal/store"
)

func TestHealthCheckSurvivesStoreReopen(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "restoration.db")

	first, err := store.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("打开首个 SQLiteStore: %v", err)
	}
	if err := first.Check(ctx); err != nil {
		first.Close()
		t.Fatalf("首个 SQLiteStore 健康检查失败: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("关闭首个 SQLiteStore: %v", err)
	}

	second, err := store.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("重开 SQLiteStore: %v", err)
	}
	defer second.Close()
	if err := second.Check(ctx); err != nil {
		t.Fatalf("重开后的 SQLiteStore 健康检查应继续成功: %v", err)
	}
}
