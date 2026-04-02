package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteRepositoryUnifiedUserLifecycle(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	user, err := repo.CreateUser(ctx, CreateUserInput{
		Name:              "mixed-user",
		Enabled:           true,
		TrafficLimitBytes: 8 * 1024 * 1024,
		Credentials: []Credential{
			{Protocol: ProtocolHY2, Identity: "mixed-user", Secret: "supersecret88"},
			{Protocol: ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
		},
	})
	if err != nil {
		t.Fatalf("create unified user: %v", err)
	}
	if len(user.Credentials) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(user.Credentials))
	}

	hy2Only, err := repo.ListUsers(ctx, 100, 0, ptrProtocol(ProtocolHY2))
	if err != nil {
		t.Fatalf("list hy2 users: %v", err)
	}
	if len(hy2Only) != 1 || hy2Only[0].ID != user.ID {
		t.Fatalf("hy2 filter mismatch: %+v", hy2Only)
	}
	vlessOnly, err := repo.ListUsers(ctx, 100, 0, ptrProtocol(ProtocolVLESS))
	if err != nil {
		t.Fatalf("list vless users: %v", err)
	}
	if len(vlessOnly) != 1 || vlessOnly[0].ID != user.ID {
		t.Fatalf("vless filter mismatch: %+v", vlessOnly)
	}

	issued, err := repo.EnsureSubscriptionToken(ctx, user.ID)
	if err != nil {
		t.Fatalf("ensure subscription token: %v", err)
	}
	if issued.Version != 1 || issued.Revoked {
		t.Fatalf("unexpected initial token state: %+v", issued)
	}
	rotated, err := repo.RotateSubscriptionToken(ctx, user.ID)
	if err != nil {
		t.Fatalf("rotate subscription token: %v", err)
	}
	if rotated.Version != 2 || rotated.Revoked {
		t.Fatalf("unexpected rotated token state: %+v", rotated)
	}
	revoked, err := repo.RevokeSubscriptionToken(ctx, user.ID)
	if err != nil {
		t.Fatalf("revoke subscription token: %v", err)
	}
	if !revoked.Revoked {
		t.Fatalf("expected token to be revoked: %+v", revoked)
	}
	restored, err := repo.ClearSubscriptionRevocation(ctx, user.ID)
	if err != nil {
		t.Fatalf("restore subscription token: %v", err)
	}
	if restored.Revoked {
		t.Fatalf("expected token to be restored: %+v", restored)
	}

	at := time.Now().UTC()
	if err := repo.InsertTrafficCounters(ctx, []TrafficCounter{
		{UserID: user.ID, Protocol: ProtocolHY2, TxBytes: 100, RxBytes: 200, Online: 1, SnapshotAt: at},
		{UserID: user.ID, Protocol: ProtocolVLESS, TxBytes: 50, RxBytes: 80, Online: 2, SnapshotAt: at},
	}); err != nil {
		t.Fatalf("insert unified traffic counters: %v", err)
	}
	updated, err := repo.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user after counters: %v", err)
	}
	if updated.TrafficUsedTxBytes != 150 || updated.TrafficUsedRxBytes != 280 {
		t.Fatalf("unexpected merged counters: tx=%d rx=%d", updated.TrafficUsedTxBytes, updated.TrafficUsedRxBytes)
	}
	if updated.OnlineCount != 3 {
		t.Fatalf("unexpected online count: %d", updated.OnlineCount)
	}
}

func TestSQLiteRepositorySetUsersStateBatchRollback(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	user, err := repo.CreateUser(ctx, CreateUserInput{
		Name:    "state-user",
		Enabled: true,
		Credentials: []Credential{
			{Protocol: ProtocolHY2, Identity: "state-user", Secret: "supersecret88"},
		},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if _, err := repo.SetUsersStateBatch(ctx, BatchUserStateInput{
		UserIDs: []string{user.ID, "missing-user-id"},
		Enabled: false,
	}); !IsNotFound(err) {
		t.Fatalf("expected not found for mixed state batch, got %v", err)
	}

	current, err := repo.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user after failed batch: %v", err)
	}
	if !current.Enabled {
		t.Fatalf("expected rollback to keep user enabled")
	}
}

func TestSQLiteRepositoryLegacyHysteriaCompatibility(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	unified, err := repo.CreateUser(ctx, CreateUserInput{
		Name:    "compat-user",
		Enabled: true,
		Credentials: []Credential{
			{Protocol: ProtocolHY2, Identity: "compat-user", Secret: "supersecret88"},
		},
	})
	if err != nil {
		t.Fatalf("create unified user: %v", err)
	}

	legacy, err := repo.GetHysteriaUser(ctx, unified.ID)
	if err != nil {
		t.Fatalf("get legacy user view: %v", err)
	}
	if legacy.User.Username != "compat-user" {
		t.Fatalf("unexpected legacy username: %s", legacy.User.Username)
	}

	updatedLegacy, err := repo.UpdateHysteriaUser(ctx, unified.ID, "compat-user-renamed", "supersecret99", nil, nil)
	if err != nil {
		t.Fatalf("update legacy user: %v", err)
	}
	updatedUnified, err := repo.GetUser(ctx, unified.ID)
	if err != nil {
		t.Fatalf("get unified user after legacy update: %v", err)
	}
	if updatedUnified.Name != "compat-user-renamed" {
		t.Fatalf("expected unified name to mirror legacy update, got %s", updatedUnified.Name)
	}
	if updatedLegacy.User.Password != "supersecret99" {
		t.Fatalf("expected legacy password to update")
	}

	if err := repo.DeleteHysteriaUser(ctx, unified.ID); err != nil {
		t.Fatalf("delete legacy user: %v", err)
	}
	if _, err := repo.GetUser(ctx, unified.ID); !IsNotFound(err) {
		t.Fatalf("expected unified user to be removed after legacy delete, got %v", err)
	}
}

func TestSQLiteRepositoryHY2SecretGenerationAndPreserve(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	created, err := repo.CreateUser(ctx, CreateUserInput{
		Name:    "generated-secret-user",
		Enabled: true,
		Credentials: []Credential{
			{Protocol: ProtocolHY2, Identity: "generated-secret-user"},
		},
	})
	if err != nil {
		t.Fatalf("create user without explicit secret: %v", err)
	}
	hy2Secret := ""
	for _, credential := range created.Credentials {
		if credential.Protocol == ProtocolHY2 {
			hy2Secret = credential.Secret
			break
		}
	}
	if len(hy2Secret) < 8 {
		t.Fatalf("expected generated hy2 secret, got %q", hy2Secret)
	}

	updated, err := repo.UpdateUser(ctx, created.ID, UpdateUserInput{
		Name:              created.Name,
		Enabled:           created.Enabled,
		TrafficLimitBytes: created.TrafficLimitBytes,
		ExpireAt:          created.ExpireAt,
		Note:              created.Note,
		Credentials: []Credential{
			{Protocol: ProtocolHY2, Identity: created.Name, Secret: ""},
		},
	})
	if err != nil {
		t.Fatalf("update user with empty secret: %v", err)
	}
	nextSecret := ""
	for _, credential := range updated.Credentials {
		if credential.Protocol == ProtocolHY2 {
			nextSecret = credential.Secret
			break
		}
	}
	if nextSecret == "" {
		t.Fatalf("expected hy2 secret to remain set")
	}
}

func ptrProtocol(value Protocol) *Protocol {
	return &value
}
