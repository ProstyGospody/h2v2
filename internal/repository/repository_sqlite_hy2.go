package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	hysteriadomain "h2v2/internal/domain/hysteria"
)

func (r *SQLiteRepository) CreateHysteriaUser(ctx context.Context, username string, password string, note *string, overrides *hysteriadomain.ClientOverrides) (HysteriaUser, error) {
	normalizedUsername, err := hysteriadomain.NormalizeUsername(username)
	if err != nil {
		return HysteriaUser{}, err
	}
	normalizedPassword, err := hysteriadomain.NormalizePassword(password)
	if err != nil {
		return HysteriaUser{}, err
	}
	encodedOverrides, err := encodeClientOverrides(overrides)
	if err != nil {
		return HysteriaUser{}, err
	}

	now := time.Now().UTC()
	user := HysteriaUser{
		ID:                 uuid.NewString(),
		Username:           normalizedUsername,
		UsernameNormalized: normalizedUsername,
		Password:           normalizedPassword,
		Enabled:            true,
		Note:               hysteriadomain.NormalizeNote(note),
		ClientOverrides:    hysteriadomain.NormalizeClientOverrides(overrides),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	_, err = r.db.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO hysteria_users (
			id, username, username_normalized, password, enabled, note, client_overrides_json, created_at_ns, updated_at_ns, last_seen_at_ns
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		user.ID,
		user.Username,
		user.UsernameNormalized,
		user.Password,
		sqliteBool(user.Enabled),
		nullValue(user.Note),
		nullValue(encodedOverrides),
		toUnixNano(user.CreatedAt),
		toUnixNano(user.UpdatedAt),
	)
	if err != nil {
		return HysteriaUser{}, translateSQLiteErr(err)
	}
	return user, nil
}

func (r *SQLiteRepository) ListHysteriaUsers(ctx context.Context, limit int, offset int) ([]HysteriaUserView, error) {
	query := `
		SELECT
			u.id,
			u.username,
			u.username_normalized,
			u.password,
			u.enabled,
			u.note,
			u.client_overrides_json,
			u.created_at_ns,
			u.updated_at_ns,
			u.last_seen_at_ns,
			COALESCE(ls.tx_bytes, 0),
			COALESCE(ls.rx_bytes, 0),
			COALESCE(ls.online_count, 0),
			COALESCE(ls.snapshot_at_ns, 0),
			COALESCE(ps.tx_bytes, 0),
			COALESCE(ps.rx_bytes, 0),
			COALESCE(ps.snapshot_at_ns, 0)
		FROM hysteria_users u
		LEFT JOIN hysteria_snapshots ls
			ON ls.id = (
				SELECT hs.id
				FROM hysteria_snapshots hs
				WHERE hs.user_id = u.id
				ORDER BY hs.snapshot_at_ns DESC, hs.id DESC
				LIMIT 1
			)
		LEFT JOIN hysteria_snapshots ps
			ON ps.id = (
				SELECT hs.id
				FROM hysteria_snapshots hs
				WHERE hs.user_id = u.id
				ORDER BY hs.snapshot_at_ns DESC, hs.id DESC
				LIMIT 1 OFFSET 1
			)
		ORDER BY u.created_at_ns DESC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, maxInt(offset, 0))
	}

	rows, err := r.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]HysteriaUserView, 0)
	for rows.Next() {
		var (
			enabled         int64
			createdAt       int64
			updatedAt       int64
			lastSeenAt      sql.NullInt64
			note            sql.NullString
			overridesJSON   sql.NullString
			lastTxBytes     int64
			lastRxBytes     int64
			onlineCount     int
			lastSnapshotAt  int64
			prevTxBytes     int64
			prevRxBytes     int64
			prevSnapshotAt  int64
			item            HysteriaUserView
		)
		if err := rows.Scan(
			&item.ID,
			&item.Username,
			&item.UsernameNormalized,
			&item.Password,
			&enabled,
			&note,
			&overridesJSON,
			&createdAt,
			&updatedAt,
			&lastSeenAt,
			&lastTxBytes,
			&lastRxBytes,
			&onlineCount,
			&lastSnapshotAt,
			&prevTxBytes,
			&prevRxBytes,
			&prevSnapshotAt,
		); err != nil {
			return nil, err
		}
		decodedOverrides, err := decodeClientOverrides(overridesJSON)
		if err != nil {
			return nil, err
		}
		item.Enabled = boolFromSQLite(enabled)
		item.Note = optionalString(note)
		item.ClientOverrides = decodedOverrides
		item.CreatedAt = fromUnixNano(createdAt)
		item.UpdatedAt = fromUnixNano(updatedAt)
		item.LastSeenAt = optionalInt64(lastSeenAt)
		item.LastTxBytes = lastTxBytes
		item.LastRxBytes = lastRxBytes
		item.OnlineCount = onlineCount
		item.DownloadBps, item.UploadBps = computeSnapshotRates(lastTxBytes, lastRxBytes, lastSnapshotAt, prevTxBytes, prevRxBytes, prevSnapshotAt)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		out = paginate(out, limit, offset)
	}
	return out, nil
}

func (r *SQLiteRepository) ListEnabledHysteriaUsers(ctx context.Context) ([]HysteriaUser, error) {
	rows, err := r.db.QueryContext(
		resolveCtx(ctx),
		`SELECT
			id, username, username_normalized, password, enabled, note, client_overrides_json, created_at_ns, updated_at_ns, last_seen_at_ns
		 FROM hysteria_users
		 WHERE enabled = 1
		 ORDER BY created_at_ns ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]HysteriaUser, 0)
	for rows.Next() {
		item, err := r.scanHysteriaUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) GetHysteriaUser(ctx context.Context, id string) (HysteriaUserView, error) {
	row := r.db.QueryRowContext(
		resolveCtx(ctx),
		`SELECT
			u.id,
			u.username,
			u.username_normalized,
			u.password,
			u.enabled,
			u.note,
			u.client_overrides_json,
			u.created_at_ns,
			u.updated_at_ns,
			u.last_seen_at_ns,
			COALESCE(ls.tx_bytes, 0),
			COALESCE(ls.rx_bytes, 0),
			COALESCE(ls.online_count, 0),
			COALESCE(ls.snapshot_at_ns, 0),
			COALESCE(ps.tx_bytes, 0),
			COALESCE(ps.rx_bytes, 0),
			COALESCE(ps.snapshot_at_ns, 0)
		 FROM hysteria_users u
		 LEFT JOIN hysteria_snapshots ls
			ON ls.id = (
				SELECT hs.id
				FROM hysteria_snapshots hs
				WHERE hs.user_id = u.id
				ORDER BY hs.snapshot_at_ns DESC, hs.id DESC
				LIMIT 1
			)
		 LEFT JOIN hysteria_snapshots ps
			ON ps.id = (
				SELECT hs.id
				FROM hysteria_snapshots hs
				WHERE hs.user_id = u.id
				ORDER BY hs.snapshot_at_ns DESC, hs.id DESC
				LIMIT 1 OFFSET 1
			)
		 WHERE u.id = ?
		 LIMIT 1`,
		strings.TrimSpace(id),
	)
	var (
		enabled       int64
		createdAt     int64
		updatedAt     int64
		lastSeenAt    sql.NullInt64
		note          sql.NullString
		overridesJSON sql.NullString
		lastTxBytes   int64
		lastRxBytes   int64
		onlineCount   int
		lastSnapshot  int64
		prevTxBytes   int64
		prevRxBytes   int64
		prevSnapshot  int64
		item          HysteriaUserView
	)
	if err := row.Scan(
		&item.ID,
		&item.Username,
		&item.UsernameNormalized,
		&item.Password,
		&enabled,
		&note,
		&overridesJSON,
		&createdAt,
		&updatedAt,
		&lastSeenAt,
		&lastTxBytes,
		&lastRxBytes,
		&onlineCount,
		&lastSnapshot,
		&prevTxBytes,
		&prevRxBytes,
		&prevSnapshot,
	); err != nil {
		return HysteriaUserView{}, translateSQLiteErr(err)
	}
	decodedOverrides, err := decodeClientOverrides(overridesJSON)
	if err != nil {
		return HysteriaUserView{}, err
	}
	item.Enabled = boolFromSQLite(enabled)
	item.Note = optionalString(note)
	item.ClientOverrides = decodedOverrides
	item.CreatedAt = fromUnixNano(createdAt)
	item.UpdatedAt = fromUnixNano(updatedAt)
	item.LastSeenAt = optionalInt64(lastSeenAt)
	item.LastTxBytes = lastTxBytes
	item.LastRxBytes = lastRxBytes
	item.OnlineCount = onlineCount
	item.DownloadBps, item.UploadBps = computeSnapshotRates(lastTxBytes, lastRxBytes, lastSnapshot, prevTxBytes, prevRxBytes, prevSnapshot)
	return item, nil
}

func computeSnapshotRates(lastTxBytes int64, lastRxBytes int64, lastSnapshotAt int64, prevTxBytes int64, prevRxBytes int64, prevSnapshotAt int64) (float64, float64) {
	if lastSnapshotAt <= 0 || prevSnapshotAt <= 0 || lastSnapshotAt <= prevSnapshotAt {
		return 0, 0
	}

	intervalSeconds := float64(lastSnapshotAt-prevSnapshotAt) / float64(time.Second)
	if intervalSeconds < 1 {
		intervalSeconds = 1
	}

	rxDelta := lastRxBytes - prevRxBytes
	txDelta := lastTxBytes - prevTxBytes

	var downloadBps float64
	if rxDelta > 0 {
		downloadBps = float64(rxDelta) / intervalSeconds
	}

	var uploadBps float64
	if txDelta > 0 {
		uploadBps = float64(txDelta) / intervalSeconds
	}

	return downloadBps, uploadBps
}

func (r *SQLiteRepository) UpdateHysteriaUser(ctx context.Context, id string, username string, password string, note *string, overrides *hysteriadomain.ClientOverrides) (HysteriaUserView, error) {
	normalizedUsername, err := hysteriadomain.NormalizeUsername(username)
	if err != nil {
		return HysteriaUserView{}, err
	}
	normalizedPassword, err := hysteriadomain.NormalizePassword(password)
	if err != nil {
		return HysteriaUserView{}, err
	}
	encodedOverrides, err := encodeClientOverrides(overrides)
	if err != nil {
		return HysteriaUserView{}, err
	}

	now := time.Now().UTC().UnixNano()
	result, err := r.db.ExecContext(
		resolveCtx(ctx),
		`UPDATE hysteria_users
		 SET username = ?, username_normalized = ?, password = ?, note = ?, client_overrides_json = ?, updated_at_ns = ?
		 WHERE id = ?`,
		normalizedUsername,
		normalizedUsername,
		normalizedPassword,
		nullValue(hysteriadomain.NormalizeNote(note)),
		nullValue(encodedOverrides),
		now,
		strings.TrimSpace(id),
	)
	if err != nil {
		return HysteriaUserView{}, translateSQLiteErr(err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return HysteriaUserView{}, err
	}
	if rows == 0 {
		return HysteriaUserView{}, ErrNotFound
	}
	return r.GetHysteriaUser(ctx, id)
}

func (r *SQLiteRepository) DeleteHysteriaUser(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(resolveCtx(ctx), `DELETE FROM hysteria_users WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteRepository) DeleteHysteriaUsers(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	uniqueIDs := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return ErrNotFound
	}

	tx, err := r.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(resolveCtx(ctx), `DELETE FROM hysteria_users WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, id := range uniqueIDs {
		result, execErr := stmt.ExecContext(resolveCtx(ctx), id)
		if execErr != nil {
			err = execErr
			return err
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			err = rowsErr
			return err
		}
		if rows == 0 {
			err = ErrNotFound
			return err
		}
	}

	err = tx.Commit()
	return err
}

func (r *SQLiteRepository) SetHysteriaUserEnabled(ctx context.Context, id string, enabled bool) error {
	result, err := r.db.ExecContext(
		resolveCtx(ctx),
		`UPDATE hysteria_users SET enabled = ?, updated_at_ns = ? WHERE id = ?`,
		sqliteBool(enabled),
		nowNano(),
		strings.TrimSpace(id),
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteRepository) TouchHysteriaUserLastSeen(ctx context.Context, id string, seenAt time.Time) error {
	_, err := r.db.ExecContext(
		resolveCtx(ctx),
		`UPDATE hysteria_users SET last_seen_at_ns = ? WHERE id = ?`,
		toUnixNano(seenAt),
		strings.TrimSpace(id),
	)
	return err
}

func (r *SQLiteRepository) InsertHysteriaSnapshots(ctx context.Context, snapshots []HysteriaSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	stmt, err := tx.PrepareContext(resolveCtx(ctx), `INSERT INTO hysteria_snapshots (user_id, tx_bytes, rx_bytes, online_count, snapshot_at_ns) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, snapshot := range snapshots {
		ts := snapshot.SnapshotAt
		if ts.IsZero() {
			ts = now
		}
		if _, err = stmt.ExecContext(
			resolveCtx(ctx),
			strings.TrimSpace(snapshot.UserID),
			snapshot.TxBytes,
			snapshot.RxBytes,
			snapshot.Online,
			toUnixNano(ts),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLiteRepository) GetHysteriaStatsOverview(ctx context.Context) (HysteriaOverview, error) {
	users, err := r.ListHysteriaUsers(ctx, 100000, 0)
	if err != nil {
		return HysteriaOverview{}, err
	}
	out := HysteriaOverview{}
	for _, user := range users {
		if user.Enabled {
			out.EnabledUsers++
		}
		out.TotalTxBytes += user.LastTxBytes
		out.TotalRxBytes += user.LastRxBytes
		out.OnlineCount += int64(user.OnlineCount)
	}
	return out, nil
}

func (r *SQLiteRepository) ListHysteriaSnapshots(ctx context.Context, userID string, limit int, offset int) ([]HysteriaSnapshot, error) {
	baseQuery := `SELECT id, user_id, tx_bytes, rx_bytes, online_count, snapshot_at_ns FROM hysteria_snapshots`
	args := []any{}
	if trimmed := strings.TrimSpace(userID); trimmed != "" {
		baseQuery += ` WHERE user_id = ?`
		args = append(args, trimmed)
	}
	baseQuery += ` ORDER BY snapshot_at_ns DESC, id DESC`

	rows, err := r.db.QueryContext(resolveCtx(ctx), baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]HysteriaSnapshot, 0)
	for rows.Next() {
		item, err := r.scanHysteriaSnapshot(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if limit > 0 || offset > 0 {
		items = paginate(items, limit, offset)
	}
	return items, nil
}

func nullValue(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func maxInt(a int, b int) int {
	if a >= b {
		return a
	}
	return b
}

func (r *SQLiteRepository) importHysteriaUsersTx(ctx context.Context, tx *sql.Tx, items []HysteriaUser) error {
	stmt := `INSERT INTO hysteria_users (
			id, username, username_normalized, password, enabled, note, client_overrides_json, created_at_ns, updated_at_ns, last_seen_at_ns
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			username = excluded.username,
			username_normalized = excluded.username_normalized,
			password = excluded.password,
			enabled = excluded.enabled,
			note = excluded.note,
			client_overrides_json = excluded.client_overrides_json,
			created_at_ns = excluded.created_at_ns,
			updated_at_ns = excluded.updated_at_ns,
			last_seen_at_ns = excluded.last_seen_at_ns`
	for _, item := range items {
		encoded, err := encodeClientOverrides(item.ClientOverrides)
		if err != nil {
			return err
		}
		lastSeen := any(nil)
		if item.LastSeenAt != nil {
			lastSeen = toUnixNano(item.LastSeenAt.UTC())
		}
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.ID,
			item.Username,
			item.UsernameNormalized,
			item.Password,
			sqliteBool(item.Enabled),
			nullValue(item.Note),
			nullValue(encoded),
			toUnixNano(item.CreatedAt),
			toUnixNano(item.UpdatedAt),
			lastSeen,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) importHysteriaSnapshotsTx(ctx context.Context, tx *sql.Tx, items []HysteriaSnapshot) error {
	stmt := `INSERT INTO hysteria_snapshots (id, user_id, tx_bytes, rx_bytes, online_count, snapshot_at_ns)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_id = excluded.user_id,
			tx_bytes = excluded.tx_bytes,
			rx_bytes = excluded.rx_bytes,
			online_count = excluded.online_count,
			snapshot_at_ns = excluded.snapshot_at_ns`
	for _, item := range items {
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.ID,
			item.UserID,
			item.TxBytes,
			item.RxBytes,
			item.Online,
			toUnixNano(item.SnapshotAt),
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) sortedHysteriaSnapshots(ctx context.Context) ([]HysteriaSnapshot, error) {
	items, err := r.ListHysteriaSnapshots(ctx, "", 0, 0)
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].SnapshotAt.Equal(items[j].SnapshotAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].SnapshotAt.Before(items[j].SnapshotAt)
	})
	return items, nil
}

func (r *SQLiteRepository) requireHysteriaUser(ctx context.Context, id string) error {
	var value string
	if err := r.db.QueryRowContext(resolveCtx(ctx), `SELECT id FROM hysteria_users WHERE id = ? LIMIT 1`, strings.TrimSpace(id)).Scan(&value); err != nil {
		return translateSQLiteErr(err)
	}
	return nil
}

func (r *SQLiteRepository) validateHysteriaIntegrity(ctx context.Context) error {
	rows, err := r.db.QueryContext(resolveCtx(ctx), `SELECT DISTINCT user_id FROM hysteria_snapshots`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return err
		}
		if err := r.requireHysteriaUser(ctx, userID); err != nil {
			return fmt.Errorf("invalid hysteria snapshot user %s: %w", userID, err)
		}
	}
	return rows.Err()
}
