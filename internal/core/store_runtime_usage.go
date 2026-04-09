package core

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"h2v2/internal/services"
)

type runtimeUsageSnapshot struct {
	UserID            string
	ServiceInstanceID string
	RuntimeUpBytes    int64
	RuntimeDownBytes  int64
}

func (s *Store) SyncServerRuntimeUsage(
	ctx context.Context,
	serverID string,
	serviceInstanceID string,
	usageByUsername map[string]services.UserTrafficUsage,
	collectedAt time.Time,
) error {
	tx, err := s.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	users, err := s.listServerUsersTx(ctx, tx, serverID)
	if err != nil {
		return err
	}
	previous, err := s.listRuntimeUsageSnapshotsTx(ctx, tx, serverID)
	if err != nil {
		return err
	}

	collectedAtNS := toUnixNano(collectedAt)
	for _, user := range users {
		username := strings.TrimSpace(user.Username)
		current, hasCurrent := usageByUsername[username]
		prev, hasPrev := previous[user.ID]

		switch {
		case hasCurrent:
		case hasPrev && strings.TrimSpace(prev.ServiceInstanceID) == strings.TrimSpace(serviceInstanceID):
			current = services.UserTrafficUsage{
				UploadBytes:   prev.RuntimeUpBytes,
				DownloadBytes: prev.RuntimeDownBytes,
			}
		default:
			current = services.UserTrafficUsage{}
		}

		if current.UploadBytes < 0 {
			current.UploadBytes = 0
		}
		if current.DownloadBytes < 0 {
			current.DownloadBytes = 0
		}

		deltaUp, deltaDown := runtimeUsageDelta(prev, current, hasPrev, serviceInstanceID)
		if deltaUp != 0 || deltaDown != 0 {
			if _, err := tx.ExecContext(
				resolveCtx(ctx),
				`UPDATE core_users
				 SET traffic_used_up_bytes = traffic_used_up_bytes + ?,
				     traffic_used_down_bytes = traffic_used_down_bytes + ?
				 WHERE id = ?`,
				deltaUp,
				deltaDown,
				user.ID,
			); err != nil {
				return err
			}
		}

		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			`INSERT INTO core_runtime_usage_snapshots(
				server_id, user_id, service_instance_id, runtime_up_bytes, runtime_down_bytes, collected_at_ns, updated_at_ns
			) VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(server_id, user_id) DO UPDATE SET
				service_instance_id = excluded.service_instance_id,
				runtime_up_bytes = excluded.runtime_up_bytes,
				runtime_down_bytes = excluded.runtime_down_bytes,
				collected_at_ns = excluded.collected_at_ns,
				updated_at_ns = excluded.updated_at_ns`,
			normalizeString(serverID),
			user.ID,
			nullIfEmpty(strings.TrimSpace(serviceInstanceID)),
			current.UploadBytes,
			current.DownloadBytes,
			collectedAtNS,
			nowNano(),
		); err != nil {
			return err
		}

		delete(previous, user.ID)
	}

	for userID := range previous {
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			`DELETE FROM core_runtime_usage_snapshots WHERE server_id = ? AND user_id = ?`,
			normalizeString(serverID),
			userID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) listServerUsersTx(ctx context.Context, tx *sql.Tx, serverID string) ([]User, error) {
	rows, err := tx.QueryContext(
		resolveCtx(ctx),
		`SELECT DISTINCT
			u.id, u.username, u.enabled, u.traffic_limit_bytes, u.traffic_used_up_bytes, u.traffic_used_down_bytes,
			u.expire_at_ns, u.created_at_ns, u.updated_at_ns
		FROM core_users u
		JOIN core_user_access ua ON ua.user_id = u.id
		JOIN core_inbounds i ON i.id = ua.inbound_id
		WHERE i.server_id = ?
		ORDER BY u.created_at_ns DESC`,
		normalizeString(serverID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]User, 0)
	for rows.Next() {
		item, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) listRuntimeUsageSnapshotsTx(ctx context.Context, tx *sql.Tx, serverID string) (map[string]runtimeUsageSnapshot, error) {
	rows, err := tx.QueryContext(
		resolveCtx(ctx),
		`SELECT user_id, COALESCE(service_instance_id, ''), runtime_up_bytes, runtime_down_bytes
		 FROM core_runtime_usage_snapshots
		 WHERE server_id = ?`,
		normalizeString(serverID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]runtimeUsageSnapshot)
	for rows.Next() {
		var item runtimeUsageSnapshot
		if err := rows.Scan(&item.UserID, &item.ServiceInstanceID, &item.RuntimeUpBytes, &item.RuntimeDownBytes); err != nil {
			return nil, err
		}
		result[item.UserID] = item
	}
	return result, rows.Err()
}

func runtimeUsageDelta(
	previous runtimeUsageSnapshot,
	current services.UserTrafficUsage,
	hasPrevious bool,
	serviceInstanceID string,
) (int64, int64) {
	currentInstance := strings.TrimSpace(serviceInstanceID)
	previousInstance := strings.TrimSpace(previous.ServiceInstanceID)

	if !hasPrevious {
		return maxRuntimeUsageValue(current.UploadBytes), maxRuntimeUsageValue(current.DownloadBytes)
	}
	if currentInstance != "" && previousInstance != "" && currentInstance != previousInstance {
		return maxRuntimeUsageValue(current.UploadBytes), maxRuntimeUsageValue(current.DownloadBytes)
	}

	return runtimeUsageCounterDelta(previous.RuntimeUpBytes, current.UploadBytes),
		runtimeUsageCounterDelta(previous.RuntimeDownBytes, current.DownloadBytes)
}

func runtimeUsageCounterDelta(previous int64, current int64) int64 {
	if current < 0 {
		current = 0
	}
	if previous < 0 {
		previous = 0
	}
	if current >= previous {
		return current - previous
	}
	return current
}

func maxRuntimeUsageValue(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
