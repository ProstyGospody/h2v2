package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrateFileToSQLite(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	storageRoot := filepath.Join(tmpDir, "storage")
	auditDir := filepath.Join(tmpDir, "audit")
	runDir := filepath.Join(tmpDir, "run")
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	fileRepo, err := NewFileRepository(storageRoot, auditDir, runDir)
	if err != nil {
		t.Fatalf("open file repository: %v", err)
	}
	t.Cleanup(func() { _ = fileRepo.Close() })

	admin, err := fileRepo.UpsertAdmin(ctx, "admin@example.com", "hash", true)
	if err != nil {
		t.Fatalf("upsert admin: %v", err)
	}
	if _, err := fileRepo.CreateSession(ctx, admin.ID, "token-hash", time.Now().UTC().Add(1*time.Hour), "127.0.0.1", "agent"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	user, err := fileRepo.CreateHysteriaUser(ctx, "demo-user", "supersecret88", nil, nil)
	if err != nil {
		t.Fatalf("create hysteria user: %v", err)
	}
	if err := fileRepo.InsertHysteriaSnapshots(ctx, []HysteriaSnapshot{
		{UserID: user.ID, TxBytes: 10, RxBytes: 20, Online: 1, SnapshotAt: time.Now().UTC()},
	}); err != nil {
		t.Fatalf("insert hysteria snapshots: %v", err)
	}
	if _, err := fileRepo.InsertSystemSnapshot(ctx, SystemSnapshot{
		SnapshotAt:        time.Now().UTC(),
		CPUUsagePercent:   10,
		MemoryUsedPercent: 30,
		NetworkRxBps:      100,
		NetworkTxBps:      80,
	}); err != nil {
		t.Fatalf("insert system snapshot: %v", err)
	}
	if err := fileRepo.InsertAuditLog(ctx, &admin.ID, "seed", "test", &user.ID, map[string]any{"ok": true}); err != nil {
		t.Fatalf("insert audit log: %v", err)
	}
	if err := fileRepo.UpsertServiceState(ctx, "h2v2-api", "active", nil, `{"state":"active"}`); err != nil {
		t.Fatalf("upsert service state: %v", err)
	}

	report, err := MigrateFileToSQLite(ctx, storageRoot, auditDir, runDir, dbPath)
	if err != nil {
		t.Fatalf("migrate file->sqlite: %v", err)
	}
	if report.Source != report.Target {
		t.Fatalf("counts mismatch after migration: source=%+v target=%+v", report.Source, report.Target)
	}

	sqliteRepo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = sqliteRepo.Close() })

	if _, err := sqliteRepo.GetAdminByEmail(ctx, "admin@example.com"); err != nil {
		t.Fatalf("get migrated admin: %v", err)
	}
	if _, _, err := sqliteRepo.GetSessionWithAdminByTokenHash(ctx, "token-hash"); err != nil {
		t.Fatalf("get migrated session: %v", err)
	}
	overview, err := sqliteRepo.GetHysteriaStatsOverview(ctx)
	if err != nil {
		t.Fatalf("get migrated overview: %v", err)
	}
	if overview.EnabledUsers != 1 {
		t.Fatalf("expected 1 enabled user after migration, got %d", overview.EnabledUsers)
	}
	logs, err := sqliteRepo.ListAuditLogs(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list migrated logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 migrated log, got %d", len(logs))
	}

	report2, err := MigrateFileToSQLite(ctx, storageRoot, auditDir, runDir, dbPath)
	if err != nil {
		t.Fatalf("idempotent migration rerun: %v", err)
	}
	if report2.Source != report2.Target {
		t.Fatalf("counts mismatch after second migration: source=%+v target=%+v", report2.Source, report2.Target)
	}
}
