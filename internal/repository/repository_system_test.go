package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSystemSnapshotsInsertAndList(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	repo, err := New(filepath.Join(tmpDir, "storage"), filepath.Join(tmpDir, "audit"), filepath.Join(tmpDir, "run"))
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	base := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	samples := []SystemSnapshot{
		{SnapshotAt: base.Add(0 * time.Minute), CPUUsagePercent: 10, MemoryUsedPercent: 40, NetworkRxBps: 1000, NetworkTxBps: 800},
		{SnapshotAt: base.Add(1 * time.Minute), CPUUsagePercent: 20, MemoryUsedPercent: 45, NetworkRxBps: 1200, NetworkTxBps: 900},
		{SnapshotAt: base.Add(2 * time.Minute), CPUUsagePercent: 30, MemoryUsedPercent: 50, NetworkRxBps: 1400, NetworkTxBps: 1000},
	}
	for _, sample := range samples {
		if _, err := repo.InsertSystemSnapshot(ctx, sample); err != nil {
			t.Fatalf("insert snapshot: %v", err)
		}
	}

	items, err := repo.ListSystemSnapshots(ctx, base.Add(30*time.Second), base.Add(2*time.Minute), 10)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 snapshots in filtered range, got %d", len(items))
	}
	if !items[0].SnapshotAt.Before(items[1].SnapshotAt) {
		t.Fatalf("expected ascending timestamps")
	}
	if items[0].CPUUsagePercent != 20 || items[1].CPUUsagePercent != 30 {
		t.Fatalf("unexpected CPU sequence: %#v", items)
	}
}
