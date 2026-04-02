package repository

import (
	"context"
	"math"
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

func TestSQLiteRepositoryRestoreFromBackup(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "source", "h2v2.db")
	targetPath := filepath.Join(tmpDir, "target", "h2v2.db")

	sourceRepo, err := NewSQLiteRepository(sourcePath)
	if err != nil {
		t.Fatalf("open source sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = sourceRepo.Close() })

	targetRepo, err := NewSQLiteRepository(targetPath)
	if err != nil {
		t.Fatalf("open target sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = targetRepo.Close() })

	sourceAdmin, err := sourceRepo.UpsertAdmin(ctx, "source-admin@example.com", "hash-source", true)
	if err != nil {
		t.Fatalf("upsert source admin: %v", err)
	}
	if _, err := sourceRepo.CreateSession(ctx, sourceAdmin.ID, "source-token", time.Now().UTC().Add(2*time.Hour), "127.0.0.1", "source-agent"); err != nil {
		t.Fatalf("create source session: %v", err)
	}
	sourceUser, err := sourceRepo.CreateHysteriaUser(ctx, "source-user", "source-password-123", nil, nil)
	if err != nil {
		t.Fatalf("create source user: %v", err)
	}
	if err := sourceRepo.InsertHysteriaSnapshots(ctx, []HysteriaSnapshot{{UserID: sourceUser.ID, TxBytes: 128, RxBytes: 256, Online: 1, SnapshotAt: time.Now().UTC()}}); err != nil {
		t.Fatalf("insert source hysteria snapshot: %v", err)
	}
	if _, err := sourceRepo.InsertSystemSnapshot(ctx, SystemSnapshot{SnapshotAt: time.Now().UTC(), CPUUsagePercent: 11, MemoryUsedPercent: 33, NetworkRxBps: 444, NetworkTxBps: 222}); err != nil {
		t.Fatalf("insert source system snapshot: %v", err)
	}
	if err := sourceRepo.InsertAuditLog(ctx, &sourceAdmin.ID, "source.action", "source_entity", &sourceUser.ID, map[string]any{"source": true}); err != nil {
		t.Fatalf("insert source audit log: %v", err)
	}
	if err := sourceRepo.UpsertServiceState(ctx, "h2v2-api", "active", nil, `{"state":"active"}`); err != nil {
		t.Fatalf("insert source service state: %v", err)
	}

	if _, err := targetRepo.UpsertAdmin(ctx, "target-admin@example.com", "hash-target", true); err != nil {
		t.Fatalf("upsert target admin: %v", err)
	}
	targetUser, err := targetRepo.CreateHysteriaUser(ctx, "target-user", "target-password-123", nil, nil)
	if err != nil {
		t.Fatalf("create target user: %v", err)
	}
	if err := targetRepo.InsertHysteriaSnapshots(ctx, []HysteriaSnapshot{{UserID: targetUser.ID, TxBytes: 1, RxBytes: 2, Online: 0, SnapshotAt: time.Now().UTC()}}); err != nil {
		t.Fatalf("insert target hysteria snapshot: %v", err)
	}

	counts, err := targetRepo.RestoreFromBackup(ctx, sourcePath)
	if err != nil {
		t.Fatalf("restore from backup: %v", err)
	}
	if counts.Admins != 1 || counts.HysteriaUsers != 1 || counts.HysteriaSnapshots != 1 || counts.SystemSnapshots != 1 || counts.AuditLogs != 1 || counts.ServiceStates != 1 || counts.Sessions != 1 {
		t.Fatalf("unexpected restored counts: %+v", counts)
	}

	if _, err := targetRepo.GetAdminByEmail(ctx, "target-admin@example.com"); !IsNotFound(err) {
		t.Fatalf("expected target admin to be replaced, got err=%v", err)
	}

	restoredAdmin, err := targetRepo.GetAdminByEmail(ctx, "source-admin@example.com")
	if err != nil {
		t.Fatalf("get restored admin: %v", err)
	}
	if restoredAdmin.PasswordHash != "hash-source" {
		t.Fatalf("unexpected restored admin hash: %s", restoredAdmin.PasswordHash)
	}

	overview, err := targetRepo.GetHysteriaStatsOverview(ctx)
	if err != nil {
		t.Fatalf("get restored overview: %v", err)
	}
	if overview.EnabledUsers != 1 || overview.TotalTxBytes != 128 || overview.TotalRxBytes != 256 {
		t.Fatalf("unexpected restored overview: %+v", overview)
	}
}

func TestSQLiteRepositoryHysteriaUserRealtimeRates(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	user, err := repo.CreateHysteriaUser(ctx, "rate-user", "supersecret88", nil, nil)
	if err != nil {
		t.Fatalf("create hysteria user: %v", err)
	}

	baseTime := time.Now().UTC().Truncate(time.Second)
	err = repo.InsertHysteriaSnapshots(ctx, []HysteriaSnapshot{
		{UserID: user.ID, TxBytes: 1_000, RxBytes: 2_000, Online: 1, SnapshotAt: baseTime},
		{UserID: user.ID, TxBytes: 1_500, RxBytes: 2_800, Online: 1, SnapshotAt: baseTime.Add(10 * time.Second)},
	})
	if err != nil {
		t.Fatalf("insert hysteria snapshots: %v", err)
	}

	item, err := repo.GetHysteriaUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("get hysteria user: %v", err)
	}

	if math.Abs(item.DownloadBps-80) > 0.0001 {
		t.Fatalf("unexpected download bps: got=%v want=80", item.DownloadBps)
	}
	if math.Abs(item.UploadBps-50) > 0.0001 {
		t.Fatalf("unexpected upload bps: got=%v want=50", item.UploadBps)
	}

	list, err := repo.ListHysteriaUsers(ctx, 100, 0)
	if err != nil {
		t.Fatalf("list hysteria users: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one user, got %d", len(list))
	}
	if math.Abs(list[0].DownloadBps-80) > 0.0001 {
		t.Fatalf("unexpected list download bps: got=%v want=80", list[0].DownloadBps)
	}
	if math.Abs(list[0].UploadBps-50) > 0.0001 {
		t.Fatalf("unexpected list upload bps: got=%v want=50", list[0].UploadBps)
	}
}

func TestSQLiteRepositoryDeleteHysteriaUsers(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	userA, err := repo.CreateHysteriaUser(ctx, "delete-user-a", "supersecret88", nil, nil)
	if err != nil {
		t.Fatalf("create user A: %v", err)
	}
	userB, err := repo.CreateHysteriaUser(ctx, "delete-user-b", "supersecret88", nil, nil)
	if err != nil {
		t.Fatalf("create user B: %v", err)
	}
	userC, err := repo.CreateHysteriaUser(ctx, "delete-user-c", "supersecret88", nil, nil)
	if err != nil {
		t.Fatalf("create user C: %v", err)
	}

	if err := repo.DeleteHysteriaUsers(ctx, []string{userA.ID, userB.ID}); err != nil {
		t.Fatalf("delete users batch: %v", err)
	}
	if _, err := repo.GetHysteriaUser(ctx, userA.ID); !IsNotFound(err) {
		t.Fatalf("expected user A to be deleted, got err=%v", err)
	}
	if _, err := repo.GetHysteriaUser(ctx, userB.ID); !IsNotFound(err) {
		t.Fatalf("expected user B to be deleted, got err=%v", err)
	}
	if _, err := repo.GetHysteriaUser(ctx, userC.ID); err != nil {
		t.Fatalf("expected user C to remain after first batch, got err=%v", err)
	}

	err = repo.DeleteHysteriaUsers(ctx, []string{userC.ID, "missing-user-id"})
	if !IsNotFound(err) {
		t.Fatalf("expected not found from mixed batch delete, got err=%v", err)
	}
	if _, err := repo.GetHysteriaUser(ctx, userC.ID); err != nil {
		t.Fatalf("expected transaction rollback to keep user C, got err=%v", err)
	}
}
