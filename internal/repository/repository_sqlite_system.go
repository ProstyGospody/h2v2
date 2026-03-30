package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	systemSnapshotMaxFiles  int64 = 20000
	systemSnapshotPruneEvery int64 = 200
)

func (r *SQLiteRepository) InsertSystemSnapshot(ctx context.Context, snapshot SystemSnapshot) (SystemSnapshot, error) {
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

	ctx = resolveCtx(ctx)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SystemSnapshot{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO system_snapshots (snapshot_at_ns, cpu_usage_percent, memory_used_percent, network_rx_bps, network_tx_bps)
		 VALUES (?, ?, ?, ?, ?)`,
		toUnixNano(snapshot.SnapshotAt),
		snapshot.CPUUsagePercent,
		snapshot.MemoryUsedPercent,
		snapshot.NetworkRxBps,
		snapshot.NetworkTxBps,
	)
	if err != nil {
		return SystemSnapshot{}, err
	}
	snapshot.ID, err = result.LastInsertId()
	if err != nil {
		return SystemSnapshot{}, err
	}

	if snapshot.ID%systemSnapshotPruneEvery == 0 {
		if err := r.pruneSystemSnapshotsTx(ctx, tx); err != nil {
			return SystemSnapshot{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return SystemSnapshot{}, err
	}
	return snapshot, nil
}

func (r *SQLiteRepository) ListSystemSnapshots(ctx context.Context, from time.Time, to time.Time, limit int) ([]SystemSnapshot, error) {
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

	rows, err := r.db.QueryContext(
		resolveCtx(ctx),
		`SELECT id, snapshot_at_ns, cpu_usage_percent, memory_used_percent, network_rx_bps, network_tx_bps
		 FROM system_snapshots
		 WHERE snapshot_at_ns >= ? AND snapshot_at_ns <= ?
		 ORDER BY snapshot_at_ns DESC, id DESC
		 LIMIT ?`,
		toUnixNano(from),
		toUnixNano(to),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SystemSnapshot, 0, limit)
	for rows.Next() {
		item, err := r.scanSystemSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (r *SQLiteRepository) pruneSystemSnapshotsTx(ctx context.Context, tx *sql.Tx) error {
	var count int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM system_snapshots`).Scan(&count); err != nil {
		return err
	}
	if count <= systemSnapshotMaxFiles {
		return nil
	}
	excess := count - systemSnapshotMaxFiles
	_, err := tx.ExecContext(
		ctx,
		`DELETE FROM system_snapshots
		 WHERE id IN (
			SELECT id
			FROM system_snapshots
			ORDER BY id ASC
			LIMIT ?
		 )`,
		excess,
	)
	return err
}

func (r *SQLiteRepository) importSystemSnapshotsTx(ctx context.Context, tx *sql.Tx, items []SystemSnapshot) error {
	stmt := `INSERT INTO system_snapshots (id, snapshot_at_ns, cpu_usage_percent, memory_used_percent, network_rx_bps, network_tx_bps)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			snapshot_at_ns = excluded.snapshot_at_ns,
			cpu_usage_percent = excluded.cpu_usage_percent,
			memory_used_percent = excluded.memory_used_percent,
			network_rx_bps = excluded.network_rx_bps,
			network_tx_bps = excluded.network_tx_bps`
	for _, item := range items {
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.ID,
			toUnixNano(item.SnapshotAt),
			item.CPUUsagePercent,
			item.MemoryUsedPercent,
			item.NetworkRxBps,
			item.NetworkTxBps,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) listSystemSnapshots(ctx context.Context) ([]SystemSnapshot, error) {
	rows, err := r.db.QueryContext(
		resolveCtx(ctx),
		`SELECT id, snapshot_at_ns, cpu_usage_percent, memory_used_percent, network_rx_bps, network_tx_bps
		 FROM system_snapshots
		 ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SystemSnapshot, 0)
	for rows.Next() {
		item, err := r.scanSystemSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) requireSystemSnapshotCount(ctx context.Context, expected int) error {
	var count int
	if err := r.db.QueryRowContext(resolveCtx(ctx), `SELECT COUNT(*) FROM system_snapshots`).Scan(&count); err != nil {
		return err
	}
	if count != expected {
		return fmt.Errorf("system snapshots mismatch: expected=%d got=%d", expected, count)
	}
	return nil
}
