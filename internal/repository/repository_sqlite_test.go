package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	admin, err := repo.UpsertAdmin(ctx, "admin@example.com", "hash-1", true)
	if err != nil {
		t.Fatalf("upsert admin: %v", err)
	}
	fetchedAdmin, err := repo.GetAdminByEmail(ctx, "admin@example.com")
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if fetchedAdmin.ID != admin.ID {
		t.Fatalf("admin id mismatch: got=%s want=%s", fetchedAdmin.ID, admin.ID)
	}

	session, err := repo.CreateSession(ctx, admin.ID, "token-hash-1", time.Now().UTC().Add(1*time.Hour), "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	storedSession, storedAdmin, err := repo.GetSessionWithAdminByTokenHash(ctx, "token-hash-1")
	if err != nil {
		t.Fatalf("get session by hash: %v", err)
	}
	if storedSession.ID != session.ID || storedAdmin.ID != admin.ID {
		t.Fatalf("session/admin mismatch")
	}
	if err := repo.TouchSession(ctx, session.ID); err != nil {
		t.Fatalf("touch session: %v", err)
	}

	user, err := repo.CreateHysteriaUser(ctx, "demo-user", "supersecret88", nil, nil)
	if err != nil {
		t.Fatalf("create hysteria user: %v", err)
	}
	if _, err := repo.UpdateHysteriaUser(ctx, user.ID, "demo-user", "supersecret89", nil, nil); err != nil {
		t.Fatalf("update hysteria user: %v", err)
	}
	if err := repo.InsertHysteriaSnapshots(ctx, []HysteriaSnapshot{
		{UserID: user.ID, TxBytes: 100, RxBytes: 200, Online: 1, SnapshotAt: time.Now().UTC()},
	}); err != nil {
		t.Fatalf("insert hysteria snapshot: %v", err)
	}
	overview, err := repo.GetHysteriaStatsOverview(ctx)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.EnabledUsers != 1 {
		t.Fatalf("expected 1 enabled user, got %d", overview.EnabledUsers)
	}
	if overview.TotalTxBytes != 100 || overview.TotalRxBytes != 200 {
		t.Fatalf("unexpected hysteria totals: %+v", overview)
	}

	if err := repo.InsertAuditLog(ctx, &admin.ID, "user.create", "hysteria_user", &user.ID, map[string]any{"ok": true}); err != nil {
		t.Fatalf("insert audit log: %v", err)
	}
	logs, err := repo.ListAuditLogs(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(logs))
	}

	if err := repo.UpsertServiceState(ctx, "h2v2-api", "active", nil, `{"state":"active"}`); err != nil {
		t.Fatalf("upsert service state: %v", err)
	}
	serviceState, err := repo.GetServiceState(ctx, "h2v2-api")
	if err != nil {
		t.Fatalf("get service state: %v", err)
	}
	if serviceState.Status != "active" {
		t.Fatalf("unexpected service state status: %s", serviceState.Status)
	}

	insertedSystem, err := repo.InsertSystemSnapshot(ctx, SystemSnapshot{
		SnapshotAt:        time.Now().UTC(),
		CPUUsagePercent:   15,
		MemoryUsedPercent: 42,
		NetworkRxBps:      1000,
		NetworkTxBps:      900,
	})
	if err != nil {
		t.Fatalf("insert system snapshot: %v", err)
	}
	systemSnapshots, err := repo.ListSystemSnapshots(ctx, insertedSystem.SnapshotAt.Add(-time.Minute), insertedSystem.SnapshotAt.Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("list system snapshots: %v", err)
	}
	if len(systemSnapshots) != 1 {
		t.Fatalf("expected 1 system snapshot, got %d", len(systemSnapshots))
	}
}
