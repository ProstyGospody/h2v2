package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestTouchHysteriaUserLastSeenDoesNotRotateUpdatedAt(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	repo, err := New(filepath.Join(tmpDir, "storage"), filepath.Join(tmpDir, "audit"), filepath.Join(tmpDir, "run"))
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	created, err := repo.CreateHysteriaUser(ctx, "demo-user", "supersecret88", nil, nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	before, err := repo.GetHysteriaUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("get user before touch: %v", err)
	}

	seenAt := before.UpdatedAt.Add(5 * time.Minute)
	if err := repo.TouchHysteriaUserLastSeen(ctx, created.ID, seenAt); err != nil {
		t.Fatalf("touch last seen: %v", err)
	}

	after, err := repo.GetHysteriaUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("get user after touch: %v", err)
	}
	if after.LastSeenAt == nil || !after.LastSeenAt.UTC().Equal(seenAt.UTC()) {
		t.Fatalf("expected last_seen_at=%s, got %#v", seenAt.UTC().Format(time.RFC3339Nano), after.LastSeenAt)
	}
	if !after.UpdatedAt.UTC().Equal(before.UpdatedAt.UTC()) {
		t.Fatalf("updated_at must remain stable on last_seen touch: before=%s after=%s", before.UpdatedAt.UTC().Format(time.RFC3339Nano), after.UpdatedAt.UTC().Format(time.RFC3339Nano))
	}
}
