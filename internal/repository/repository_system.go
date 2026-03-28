package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	systemSnapshotMaxFiles   = 50000
	systemSnapshotPruneEvery = 120
)

func (r *Repository) InsertSystemSnapshot(ctx context.Context, snapshot SystemSnapshot) (SystemSnapshot, error) {
	var out SystemSnapshot
	err := r.withLock(ctx, func() error {
		meta, err := r.loadMetaNoLock()
		if err != nil {
			return err
		}

		meta.NextSystemSnapshotID++
		snapshot.ID = meta.NextSystemSnapshotID
		if snapshot.SnapshotAt.IsZero() {
			snapshot.SnapshotAt = time.Now().UTC()
		} else {
			snapshot.SnapshotAt = snapshot.SnapshotAt.UTC()
		}
		if snapshot.CPUUsagePercent < 0 {
			snapshot.CPUUsagePercent = 0
		} else if snapshot.CPUUsagePercent > 100 {
			snapshot.CPUUsagePercent = 100
		}
		if snapshot.MemoryUsedPercent < 0 {
			snapshot.MemoryUsedPercent = 0
		} else if snapshot.MemoryUsedPercent > 100 {
			snapshot.MemoryUsedPercent = 100
		}
		if snapshot.NetworkRxBps < 0 {
			snapshot.NetworkRxBps = 0
		}
		if snapshot.NetworkTxBps < 0 {
			snapshot.NetworkTxBps = 0
		}

		if err := r.saveMetaNoLock(meta); err != nil {
			return err
		}
		if err := writeJSONFile(filepath.Join(r.systemSnapshotsDir, numericJSONFile(snapshot.ID)), 0o600, snapshot); err != nil {
			return err
		}

		if snapshot.ID%systemSnapshotPruneEvery == 0 {
			_ = r.pruneSystemSnapshotsNoLock()
		}
		out = snapshot
		return nil
	})
	return out, err
}

func (r *Repository) ListSystemSnapshots(ctx context.Context, from time.Time, to time.Time, limit int) ([]SystemSnapshot, error) {
	if limit <= 0 {
		limit = 1000
	}

	if from.IsZero() {
		from = time.Time{}
	} else {
		from = from.UTC()
	}
	if to.IsZero() {
		to = time.Now().UTC()
	} else {
		to = to.UTC()
	}

	out := make([]SystemSnapshot, 0, limit)
	err := r.withLock(ctx, func() error {
		entries, err := os.ReadDir(r.systemSnapshotsDir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() > entries[j].Name()
		})

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}

			item, err := loadEntity[SystemSnapshot](filepath.Join(r.systemSnapshotsDir, entry.Name()))
			if err != nil {
				if IsNotFound(err) {
					continue
				}
				return err
			}
			ts := item.SnapshotAt.UTC()
			if !to.IsZero() && ts.After(to) {
				continue
			}
			if !from.IsZero() && ts.Before(from) {
				continue
			}

			out = append(out, item)
			if len(out) >= limit {
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (r *Repository) pruneSystemSnapshotsNoLock() error {
	entries, err := os.ReadDir(r.systemSnapshotsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	files := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		files = append(files, entry)
	}
	if len(files) <= systemSnapshotMaxFiles {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	excess := len(files) - systemSnapshotMaxFiles
	for idx := 0; idx < excess; idx++ {
		path := filepath.Join(r.systemSnapshotsDir, files[idx].Name())
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
