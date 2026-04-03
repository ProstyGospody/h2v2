package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	paneldomain "h2v2/internal/domain/panel"
)

func (r *SQLiteRepository) CreateUser(ctx context.Context, input CreateUserInput) (UserWithCredentials, error) {
	name, err := paneldomain.NormalizeUserName(input.Name)
	if err != nil {
		return UserWithCredentials{}, err
	}

	note := paneldomain.NormalizeNote(input.Note)
	now := time.Now().UTC()
	userID := uuid.NewString()
	subject := userID
	if input.ExpireAt != nil {
		ts := input.ExpireAt.UTC()
		input.ExpireAt = &ts
	}
	if err := paneldomain.ValidateLifecycle(input.Enabled, input.TrafficLimitBytes, 0, 0, input.ExpireAt, now); err != nil {
		return UserWithCredentials{}, err
	}

	credentials, err := normalizeCredentialsInput(input.Credentials, userID, now)
	if err != nil {
		return UserWithCredentials{}, err
	}
	if len(credentials) == 0 {
		return UserWithCredentials{}, fmt.Errorf("at least one credential is required")
	}

	tx, err := r.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return UserWithCredentials{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO users (
			id, name, name_normalized, enabled, traffic_limit_bytes, traffic_used_tx_bytes, traffic_used_rx_bytes,
			expire_at_ns, note, subject, created_at_ns, updated_at_ns, last_seen_at_ns
		) VALUES (?, ?, ?, ?, ?, 0, 0, ?, ?, ?, ?, ?, NULL)`,
		userID,
		name,
		name,
		sqliteBool(input.Enabled),
		input.TrafficLimitBytes,
		nullInt64(input.ExpireAt),
		nullValue(note),
		subject,
		toUnixNano(now),
		toUnixNano(now),
	)
	if err != nil {
		return UserWithCredentials{}, translateSQLiteErr(err)
	}

	for _, credential := range credentials {
		if _, err = tx.ExecContext(
			resolveCtx(ctx),
			`INSERT INTO credentials (
				id, user_id, protocol, credential_type, identity, secret, data_json, created_at_ns, updated_at_ns
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			credential.ID,
			credential.UserID,
			string(credential.Protocol),
			string(credential.Type),
			credential.Identity,
			credential.Secret,
			nullString(credential.DataJSON),
			toUnixNano(credential.CreatedAt),
			toUnixNano(credential.UpdatedAt),
		); err != nil {
			return UserWithCredentials{}, translateSQLiteErr(err)
		}
	}

	if _, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO subscription_tokens (user_id, subject, version, revoked, rotated_at_ns, updated_at_ns)
		 VALUES (?, ?, 1, 0, NULL, ?)
		 ON CONFLICT(user_id) DO UPDATE SET subject = excluded.subject, updated_at_ns = excluded.updated_at_ns`,
		userID,
		subject,
		toUnixNano(now),
	); err != nil {
		return UserWithCredentials{}, err
	}

	if err = tx.Commit(); err != nil {
		return UserWithCredentials{}, err
	}
	return r.GetUser(ctx, userID)
}

func (r *SQLiteRepository) ListUsers(ctx context.Context, limit int, offset int, protocol *Protocol) ([]UserWithCredentials, error) {
	query := `SELECT
		u.id,
		u.name,
		u.name_normalized,
		u.enabled,
		u.traffic_limit_bytes,
		u.traffic_used_tx_bytes,
		u.traffic_used_rx_bytes,
		u.expire_at_ns,
		u.note,
		u.subject,
		u.created_at_ns,
		u.updated_at_ns,
		u.last_seen_at_ns
	FROM users u`
	args := make([]any, 0, 4)
	if protocol != nil {
		query += ` WHERE EXISTS (SELECT 1 FROM credentials c WHERE c.user_id = u.id AND c.protocol = ?)`
		args = append(args, string(*protocol))
	}
	query += ` ORDER BY u.created_at_ns DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, maxInt(offset, 0))
	}

	rows, err := r.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	userIDs := make([]string, 0)
	for rows.Next() {
		user, err := r.scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
		userIDs = append(userIDs, user.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return []UserWithCredentials{}, nil
	}

	credentialsByUser, err := r.listCredentialsForUsers(ctx, userIDs, protocol)
	if err != nil {
		return nil, err
	}
	onlineByUser, err := r.listOnlineCountsForUsers(ctx, userIDs)
	if err != nil {
		onlineByUser = make(map[string]int, len(userIDs))
		for _, userID := range userIDs {
			onlineByUser[userID] = 0
		}
	}

	now := time.Now().UTC()
	items := make([]UserWithCredentials, 0, len(users))
	for _, user := range users {
		if user.ExpireAt != nil && !user.ExpireAt.After(now) {
			user.Enabled = false
		}
		if user.TrafficLimitBytes > 0 && (user.TrafficUsedTxBytes+user.TrafficUsedRxBytes) >= user.TrafficLimitBytes {
			user.Enabled = false
		}
		items = append(items, UserWithCredentials{
			User:        user,
			Credentials: credentialsByUser[user.ID],
			OnlineCount: onlineByUser[user.ID],
			DownloadBps: 0,
			UploadBps:   0,
		})
	}
	if limit <= 0 {
		items = paginate(items, limit, offset)
	}
	return items, nil
}

func (r *SQLiteRepository) GetUser(ctx context.Context, id string) (UserWithCredentials, error) {
	row := r.db.QueryRowContext(
		resolveCtx(ctx),
		`SELECT
			u.id,
			u.name,
			u.name_normalized,
			u.enabled,
			u.traffic_limit_bytes,
			u.traffic_used_tx_bytes,
			u.traffic_used_rx_bytes,
			u.expire_at_ns,
			u.note,
			u.subject,
			u.created_at_ns,
			u.updated_at_ns,
			u.last_seen_at_ns
		FROM users u
		WHERE u.id = ?
		LIMIT 1`,
		strings.TrimSpace(id),
	)
	user, err := r.scanUser(row)
	if err != nil {
		return UserWithCredentials{}, err
	}

	credentials, err := r.listCredentialsForUser(ctx, user.ID, nil)
	if err != nil {
		return UserWithCredentials{}, err
	}
	onlineByUser, err := r.listOnlineCountsForUsers(ctx, []string{user.ID})
	if err != nil {
		onlineByUser = map[string]int{user.ID: 0}
	}
	if user.ExpireAt != nil && !user.ExpireAt.After(time.Now().UTC()) {
		user.Enabled = false
	}
	if user.TrafficLimitBytes > 0 && (user.TrafficUsedTxBytes+user.TrafficUsedRxBytes) >= user.TrafficLimitBytes {
		user.Enabled = false
	}

	return UserWithCredentials{
		User:        user,
		Credentials: credentials,
		OnlineCount: onlineByUser[user.ID],
		DownloadBps: 0,
		UploadBps:   0,
	}, nil
}

func (r *SQLiteRepository) GetUserBySubject(ctx context.Context, subject string) (UserWithCredentials, error) {
	row := r.db.QueryRowContext(
		resolveCtx(ctx),
		`SELECT id FROM users WHERE subject = ? LIMIT 1`,
		strings.TrimSpace(subject),
	)
	var id string
	if err := row.Scan(&id); err != nil {
		return UserWithCredentials{}, translateSQLiteErr(err)
	}
	return r.GetUser(ctx, id)
}

func (r *SQLiteRepository) UpdateUser(ctx context.Context, id string, input UpdateUserInput) (UserWithCredentials, error) {
	current, err := r.GetUser(ctx, id)
	if err != nil {
		return UserWithCredentials{}, err
	}

	name, err := paneldomain.NormalizeUserName(input.Name)
	if err != nil {
		return UserWithCredentials{}, err
	}
	note := paneldomain.NormalizeNote(input.Note)
	now := time.Now().UTC()
	if input.ExpireAt != nil {
		ts := input.ExpireAt.UTC()
		input.ExpireAt = &ts
	}
	if err := paneldomain.ValidateLifecycle(input.Enabled, input.TrafficLimitBytes, current.TrafficUsedTxBytes, current.TrafficUsedRxBytes, input.ExpireAt, now); err != nil {
		return UserWithCredentials{}, err
	}

	credentials, err := normalizeCredentialsInput(input.Credentials, current.ID, now)
	if err != nil {
		return UserWithCredentials{}, err
	}
	if len(credentials) == 0 {
		return UserWithCredentials{}, fmt.Errorf("at least one credential is required")
	}

	tx, err := r.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return UserWithCredentials{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(
		resolveCtx(ctx),
		`UPDATE users
		 SET name = ?, name_normalized = ?, enabled = ?, traffic_limit_bytes = ?, expire_at_ns = ?, note = ?, updated_at_ns = ?
		 WHERE id = ?`,
		name,
		name,
		sqliteBool(input.Enabled),
		input.TrafficLimitBytes,
		nullInt64(input.ExpireAt),
		nullValue(note),
		toUnixNano(now),
		strings.TrimSpace(id),
	)
	if err != nil {
		return UserWithCredentials{}, translateSQLiteErr(err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return UserWithCredentials{}, err
	}
	if changed == 0 {
		return UserWithCredentials{}, ErrNotFound
	}

	for _, credential := range credentials {
		if _, err = tx.ExecContext(
			resolveCtx(ctx),
			`INSERT INTO credentials (
				id, user_id, protocol, credential_type, identity, secret, data_json, created_at_ns, updated_at_ns
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id, protocol) DO UPDATE SET
				credential_type = excluded.credential_type,
				identity = excluded.identity,
				secret = excluded.secret,
				data_json = excluded.data_json,
				updated_at_ns = excluded.updated_at_ns`,
			credential.ID,
			credential.UserID,
			string(credential.Protocol),
			string(credential.Type),
			credential.Identity,
			credential.Secret,
			nullString(credential.DataJSON),
			toUnixNano(credential.CreatedAt),
			toUnixNano(credential.UpdatedAt),
		); err != nil {
			return UserWithCredentials{}, translateSQLiteErr(err)
		}
	}

	if _, err = tx.ExecContext(resolveCtx(ctx), `DELETE FROM credentials WHERE user_id = ? AND protocol NOT IN (`+placeholders(len(credentials))+`)`, buildDeleteCredentialsArgs(id, credentials)...); err != nil {
		return UserWithCredentials{}, err
	}

	if _, err = tx.ExecContext(resolveCtx(ctx), `UPDATE subscription_tokens SET updated_at_ns = ? WHERE user_id = ?`, toUnixNano(now), strings.TrimSpace(id)); err != nil {
		return UserWithCredentials{}, err
	}

	if err = tx.Commit(); err != nil {
		return UserWithCredentials{}, err
	}
	return r.GetUser(ctx, id)
}

func (r *SQLiteRepository) DeleteUsers(ctx context.Context, input BatchDeleteUsersInput) error {
	ids := normalizeIDs(input.UserIDs)
	if len(ids) == 0 {
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

	for _, id := range ids {
		result, execErr := tx.ExecContext(resolveCtx(ctx), `DELETE FROM users WHERE id = ?`, id)
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

func (r *SQLiteRepository) SetUsersStateBatch(ctx context.Context, input BatchUserStateInput) (int, error) {
	ids := normalizeIDs(input.UserIDs)
	if len(ids) == 0 {
		return 0, ErrNotFound
	}

	tx, err := r.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	updated := 0
	now := nowNano()
	for _, id := range ids {
		if input.Protocol != nil {
			var exists string
			if scanErr := tx.QueryRowContext(
				resolveCtx(ctx),
				`SELECT c.user_id FROM credentials c WHERE c.user_id = ? AND c.protocol = ? LIMIT 1`,
				id,
				string(*input.Protocol),
			).Scan(&exists); scanErr != nil {
				if errors.Is(scanErr, sql.ErrNoRows) {
					err = ErrNotFound
					return 0, err
				}
				err = scanErr
				return 0, err
			}
		}

		result, execErr := tx.ExecContext(
			resolveCtx(ctx),
			`UPDATE users SET enabled = ?, updated_at_ns = ? WHERE id = ?`,
			sqliteBool(input.Enabled),
			now,
			id,
		)
		if execErr != nil {
			err = execErr
			return 0, err
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			err = rowsErr
			return 0, err
		}
		if rows == 0 {
			err = ErrNotFound
			return 0, err
		}
		updated += int(rows)

	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}
	return updated, nil
}

func (r *SQLiteRepository) KickUsers(ctx context.Context, ids []string) ([]string, error) {
	normalized := normalizeIDs(ids)
	if len(normalized) == 0 {
		return nil, ErrNotFound
	}
	result := make([]string, 0, len(normalized))
	for _, id := range normalized {
		var exists string
		if err := r.db.QueryRowContext(resolveCtx(ctx), `SELECT id FROM users WHERE id = ? LIMIT 1`, id).Scan(&exists); err != nil {
			return nil, translateSQLiteErr(err)
		}
		result = append(result, exists)
	}
	return result, nil
}

func (r *SQLiteRepository) ListInbounds(ctx context.Context, protocol *Protocol) ([]Inbound, error) {
	query := `SELECT id, node_id, name, protocol, transport, security, host, port, enabled, params_json, runtime_json, created_at_ns, updated_at_ns FROM inbounds`
	args := make([]any, 0, 1)
	if protocol != nil {
		query += ` WHERE protocol = ?`
		args = append(args, string(*protocol))
	}
	query += ` ORDER BY created_at_ns DESC, id DESC`

	rows, err := r.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Inbound, 0)
	for rows.Next() {
		item, err := r.scanInbound(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) GetInbound(ctx context.Context, id string) (Inbound, error) {
	row := r.db.QueryRowContext(
		resolveCtx(ctx),
		`SELECT id, node_id, name, protocol, transport, security, host, port, enabled, params_json, runtime_json, created_at_ns, updated_at_ns
		 FROM inbounds
		 WHERE id = ?
		 LIMIT 1`,
		strings.TrimSpace(id),
	)
	return r.scanInbound(row)
}

func (r *SQLiteRepository) UpsertInbound(ctx context.Context, inbound Inbound) (Inbound, error) {
	if err := paneldomain.ValidateProtocol(inbound.Protocol); err != nil {
		return Inbound{}, err
	}
	inbound.ID = strings.TrimSpace(inbound.ID)
	if inbound.ID == "" {
		inbound.ID = uuid.NewString()
	}
	inbound.NodeID = strings.TrimSpace(inbound.NodeID)
	if inbound.NodeID == "" {
		inbound.NodeID = "local"
	}
	inbound.Name = strings.TrimSpace(inbound.Name)
	if inbound.Name == "" {
		return Inbound{}, fmt.Errorf("inbound name is required")
	}
	inbound.Transport = strings.ToLower(strings.TrimSpace(inbound.Transport))
	if inbound.Transport == "" {
		inbound.Transport = "tcp"
	}
	inbound.Security = strings.ToLower(strings.TrimSpace(inbound.Security))
	if inbound.Security == "" {
		inbound.Security = "none"
	}
	inbound.Host = strings.TrimSpace(inbound.Host)
	if inbound.Host == "" {
		nodeAddress, nodeErr := r.nodeAddressByID(ctx, inbound.NodeID)
		if nodeErr == nil {
			inbound.Host = strings.TrimSpace(nodeAddress)
		}
	}
	if inbound.Host == "" {
		return Inbound{}, fmt.Errorf("inbound host is required")
	}
	if inbound.Port <= 0 {
		inbound.Port = 443
	}
	inbound.ParamsJSON = strings.TrimSpace(inbound.ParamsJSON)
	if inbound.ParamsJSON == "" {
		inbound.ParamsJSON = "{}"
	}
	inbound.RuntimeJSON = strings.TrimSpace(inbound.RuntimeJSON)
	if inbound.RuntimeJSON == "" {
		inbound.RuntimeJSON = "{}"
	}
	if err := normalizeInboundParamsForStorage(&inbound); err != nil {
		return Inbound{}, err
	}

	now := time.Now().UTC()
	if inbound.CreatedAt.IsZero() {
		inbound.CreatedAt = now
	}
	inbound.UpdatedAt = now

	_, err := r.db.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO inbounds (
			id, node_id, name, protocol, transport, security, host, port, enabled, params_json, runtime_json, created_at_ns, updated_at_ns
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			node_id = excluded.node_id,
			name = excluded.name,
			protocol = excluded.protocol,
			transport = excluded.transport,
			security = excluded.security,
			host = excluded.host,
			port = excluded.port,
			enabled = excluded.enabled,
			params_json = excluded.params_json,
			runtime_json = excluded.runtime_json,
			updated_at_ns = excluded.updated_at_ns`,
		inbound.ID,
		inbound.NodeID,
		inbound.Name,
		string(inbound.Protocol),
		inbound.Transport,
		inbound.Security,
		inbound.Host,
		inbound.Port,
		sqliteBool(inbound.Enabled),
		inbound.ParamsJSON,
		inbound.RuntimeJSON,
		toUnixNano(inbound.CreatedAt),
		toUnixNano(inbound.UpdatedAt),
	)
	if err != nil {
		return Inbound{}, translateSQLiteErr(err)
	}
	return r.GetInbound(ctx, inbound.ID)
}

func (r *SQLiteRepository) DeleteInbound(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(resolveCtx(ctx), `DELETE FROM inbounds WHERE id = ?`, strings.TrimSpace(id))
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

func (r *SQLiteRepository) GetSubscriptionToken(ctx context.Context, userID string) (SubscriptionToken, error) {
	row := r.db.QueryRowContext(
		resolveCtx(ctx),
		`SELECT user_id, subject, version, revoked, rotated_at_ns, updated_at_ns
		 FROM subscription_tokens
		 WHERE user_id = ?
		 LIMIT 1`,
		strings.TrimSpace(userID),
	)
	return r.scanSubscriptionToken(row)
}

func (r *SQLiteRepository) EnsureSubscriptionToken(ctx context.Context, userID string) (SubscriptionToken, error) {
	user, err := r.GetUser(ctx, userID)
	if err != nil {
		return SubscriptionToken{}, err
	}

	now := nowNano()
	_, err = r.db.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO subscription_tokens (user_id, subject, version, revoked, rotated_at_ns, updated_at_ns)
		 VALUES (?, ?, 1, 0, NULL, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
			subject = excluded.subject,
			updated_at_ns = excluded.updated_at_ns`,
		user.ID,
		user.Subject,
		now,
	)
	if err != nil {
		return SubscriptionToken{}, err
	}
	return r.GetSubscriptionToken(ctx, user.ID)
}

func (r *SQLiteRepository) RotateSubscriptionToken(ctx context.Context, userID string) (SubscriptionToken, error) {
	state, err := r.EnsureSubscriptionToken(ctx, userID)
	if err != nil {
		return SubscriptionToken{}, err
	}
	now := nowNano()
	_, err = r.db.ExecContext(
		resolveCtx(ctx),
		`UPDATE subscription_tokens
		 SET version = ?, revoked = 0, rotated_at_ns = ?, updated_at_ns = ?
		 WHERE user_id = ?`,
		state.Version+1,
		now,
		now,
		strings.TrimSpace(userID),
	)
	if err != nil {
		return SubscriptionToken{}, err
	}
	return r.GetSubscriptionToken(ctx, strings.TrimSpace(userID))
}

func (r *SQLiteRepository) RevokeSubscriptionToken(ctx context.Context, userID string) (SubscriptionToken, error) {
	if _, err := r.EnsureSubscriptionToken(ctx, userID); err != nil {
		return SubscriptionToken{}, err
	}
	_, err := r.db.ExecContext(
		resolveCtx(ctx),
		`UPDATE subscription_tokens SET revoked = 1, updated_at_ns = ? WHERE user_id = ?`,
		nowNano(),
		strings.TrimSpace(userID),
	)
	if err != nil {
		return SubscriptionToken{}, err
	}
	return r.GetSubscriptionToken(ctx, strings.TrimSpace(userID))
}

func (r *SQLiteRepository) ClearSubscriptionRevocation(ctx context.Context, userID string) (SubscriptionToken, error) {
	if _, err := r.EnsureSubscriptionToken(ctx, userID); err != nil {
		return SubscriptionToken{}, err
	}
	_, err := r.db.ExecContext(
		resolveCtx(ctx),
		`UPDATE subscription_tokens SET revoked = 0, updated_at_ns = ? WHERE user_id = ?`,
		nowNano(),
		strings.TrimSpace(userID),
	)
	if err != nil {
		return SubscriptionToken{}, err
	}
	return r.GetSubscriptionToken(ctx, strings.TrimSpace(userID))
}

func (r *SQLiteRepository) InsertTrafficCounters(ctx context.Context, counters []TrafficCounter) error {
	if len(counters) == 0 {
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

	stmt, err := tx.PrepareContext(resolveCtx(ctx), `INSERT INTO traffic_counters (user_id, protocol, tx_bytes, rx_bytes, online_count, snapshot_at_ns) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	touchedUsers := make(map[string]struct{}, len(counters))
	type stateKey struct {
		UserID   string
		Protocol Protocol
	}
	type stateValue struct {
		Online     int64
		SnapshotAt int64
	}
	runtimeState := make(map[stateKey]stateValue, len(counters))

	for _, item := range counters {
		ts := item.SnapshotAt
		if ts.IsZero() {
			ts = now
		}
		userID := strings.TrimSpace(item.UserID)
		if userID == "" {
			continue
		}
		protocol := item.Protocol
		if protocol == "" {
			protocol = ProtocolHY2
		}
		snapshotAt := toUnixNano(ts)
		if _, err = stmt.ExecContext(resolveCtx(ctx), userID, string(protocol), item.TxBytes, item.RxBytes, item.Online, snapshotAt); err != nil {
			return err
		}
		touchedUsers[userID] = struct{}{}
		key := stateKey{UserID: userID, Protocol: protocol}
		current, exists := runtimeState[key]
		if !exists || snapshotAt >= current.SnapshotAt {
			runtimeState[key] = stateValue{
				Online:     int64(item.Online),
				SnapshotAt: snapshotAt,
			}
		}
	}
	if len(touchedUsers) == 0 {
		return tx.Commit()
	}

	userIDs := make([]string, 0, len(touchedUsers))
	for userID := range touchedUsers {
		userIDs = append(userIDs, userID)
	}
	query := `SELECT
		tc.user_id,
		COALESCE(SUM(tc.tx_bytes), 0),
		COALESCE(SUM(tc.rx_bytes), 0),
		COALESCE(SUM(tc.online_count), 0),
		COALESCE(MAX(tc.snapshot_at_ns), 0)
	FROM traffic_counters tc
	JOIN (
		SELECT user_id, protocol, MAX(id) AS max_id
		FROM traffic_counters
		WHERE user_id IN (` + placeholders(len(userIDs)) + `)
		GROUP BY user_id, protocol
	) latest ON latest.max_id = tc.id
	GROUP BY tc.user_id`
	args := make([]any, 0, len(userIDs))
	for _, userID := range userIDs {
		args = append(args, userID)
	}

	type userTotals struct {
		TxBytes int64
		RxBytes int64
		Online  int64
		SeenAt  int64
	}
	totalsByUser := make(map[string]userTotals, len(userIDs))
	rows, err := tx.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var (
			userID string
			total  userTotals
		)
		if err = rows.Scan(&userID, &total.TxBytes, &total.RxBytes, &total.Online, &total.SeenAt); err != nil {
			_ = rows.Close()
			return err
		}
		totalsByUser[userID] = total
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()

	updateUserStmt, err := tx.PrepareContext(
		resolveCtx(ctx),
		`UPDATE users
		 SET
			traffic_used_tx_bytes = ?,
			traffic_used_rx_bytes = ?,
			last_seen_at_ns = CASE WHEN ? > 0 THEN ? ELSE last_seen_at_ns END,
			updated_at_ns = ?
		 WHERE id = ?`,
	)
	if err != nil {
		return err
	}
	defer updateUserStmt.Close()

	updatedAtNs := nowNano()
	for _, userID := range userIDs {
		total := totalsByUser[userID]
		if _, err = updateUserStmt.ExecContext(
			resolveCtx(ctx),
			total.TxBytes,
			total.RxBytes,
			total.Online,
			total.SeenAt,
			updatedAtNs,
			userID,
		); err != nil {
			return err
		}
	}

	stateStmt, err := tx.PrepareContext(
		resolveCtx(ctx),
		`INSERT INTO runtime_user_state (user_id, protocol, online_count, last_sync_at_ns, last_error)
		 VALUES (?, ?, ?, ?, NULL)
		 ON CONFLICT(user_id, protocol) DO UPDATE SET
			online_count = excluded.online_count,
			last_sync_at_ns = excluded.last_sync_at_ns,
			last_error = NULL`,
	)
	if err != nil {
		return err
	}
	defer stateStmt.Close()

	for key, state := range runtimeState {
		if _, err = stateStmt.ExecContext(
			resolveCtx(ctx),
			key.UserID,
			string(key.Protocol),
			state.Online,
			state.SnapshotAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *SQLiteRepository) ListTrafficCounters(ctx context.Context, userID string, protocol *Protocol, limit int, offset int) ([]TrafficCounter, error) {
	query := `SELECT id, user_id, protocol, tx_bytes, rx_bytes, online_count, snapshot_at_ns FROM traffic_counters`
	args := make([]any, 0, 4)
	filters := make([]string, 0, 2)
	if strings.TrimSpace(userID) != "" {
		filters = append(filters, "user_id = ?")
		args = append(args, strings.TrimSpace(userID))
	}
	if protocol != nil {
		filters = append(filters, "protocol = ?")
		args = append(args, string(*protocol))
	}
	if len(filters) > 0 {
		query += ` WHERE ` + strings.Join(filters, " AND ")
	}
	query += ` ORDER BY snapshot_at_ns DESC, id DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, maxInt(offset, 0))
	}

	rows, err := r.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]TrafficCounter, 0)
	for rows.Next() {
		item, err := r.scanTrafficCounter(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		items = paginate(items, limit, offset)
	}
	return items, nil
}

type userMetrics struct {
	TxBytes     int64
	RxBytes     int64
	Online      int
	DownloadBps float64
	UploadBps   float64
}

func (r *SQLiteRepository) collectUserMetrics(ctx context.Context, userID string, protocols []Protocol) (userMetrics, error) {
	if len(protocols) == 0 {
		protocols = []Protocol{ProtocolHY2, ProtocolVLESS}
	}
	result := userMetrics{}
	for _, protocol := range protocols {
		rows, err := r.db.QueryContext(
			resolveCtx(ctx),
			`SELECT tx_bytes, rx_bytes, online_count, snapshot_at_ns
			 FROM traffic_counters
			 WHERE user_id = ? AND protocol = ?
			 ORDER BY snapshot_at_ns DESC, id DESC
			 LIMIT 2`,
			strings.TrimSpace(userID),
			string(protocol),
		)
		if err != nil {
			return userMetrics{}, err
		}

		type sample struct {
			tx     int64
			rx     int64
			online int
			at     int64
		}
		collected := make([]sample, 0, 2)
		for rows.Next() {
			var item sample
			if scanErr := rows.Scan(&item.tx, &item.rx, &item.online, &item.at); scanErr != nil {
				_ = rows.Close()
				return userMetrics{}, scanErr
			}
			collected = append(collected, item)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return userMetrics{}, err
		}
		_ = rows.Close()
		if len(collected) == 0 {
			continue
		}

		latest := collected[0]
		result.TxBytes += latest.tx
		result.RxBytes += latest.rx
		result.Online += latest.online
		if len(collected) > 1 {
			previous := collected[1]
			down, up := computeSnapshotRates(latest.tx, latest.rx, latest.at, previous.tx, previous.rx, previous.at)
			result.DownloadBps += down
			result.UploadBps += up
		}
	}
	return result, nil
}

func (r *SQLiteRepository) listCredentialsForUser(ctx context.Context, userID string, protocol *Protocol) ([]Credential, error) {
	query := `SELECT id, user_id, protocol, credential_type, identity, secret, data_json, created_at_ns, updated_at_ns FROM credentials WHERE user_id = ?`
	args := []any{strings.TrimSpace(userID)}
	if protocol != nil {
		query += ` AND protocol = ?`
		args = append(args, string(*protocol))
	}
	query += ` ORDER BY protocol ASC, created_at_ns ASC`

	rows, err := r.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Credential, 0)
	for rows.Next() {
		item, err := r.scanCredential(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) listCredentialsForUsers(ctx context.Context, userIDs []string, protocol *Protocol) (map[string][]Credential, error) {
	ids := normalizeIDs(userIDs)
	result := make(map[string][]Credential, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	for _, id := range ids {
		result[id] = []Credential{}
	}

	query := `SELECT id, user_id, protocol, credential_type, identity, secret, data_json, created_at_ns, updated_at_ns
		FROM credentials
		WHERE user_id IN (` + placeholders(len(ids)) + `)`
	args := make([]any, 0, len(ids)+1)
	for _, id := range ids {
		args = append(args, id)
	}
	if protocol != nil {
		query += ` AND protocol = ?`
		args = append(args, string(*protocol))
	}
	query += ` ORDER BY user_id ASC, protocol ASC, created_at_ns ASC`

	rows, err := r.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item, err := r.scanCredential(rows)
		if err != nil {
			return nil, err
		}
		result[item.UserID] = append(result[item.UserID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *SQLiteRepository) listOnlineCountsForUsers(ctx context.Context, userIDs []string) (map[string]int, error) {
	ids := normalizeIDs(userIDs)
	result := make(map[string]int, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	for _, id := range ids {
		result[id] = 0
	}

	query := `SELECT user_id, COALESCE(SUM(online_count), 0) AS online_total
		FROM runtime_user_state
		WHERE user_id IN (` + placeholders(len(ids)) + `)
		GROUP BY user_id`
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}

	rows, err := r.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			userID string
			online int64
		)
		if err := rows.Scan(&userID, &online); err != nil {
			return nil, err
		}
		result[userID] = int(online)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *SQLiteRepository) scanUser(row scanner) (User, error) {
	var (
		out          User
		enabled      int64
		expireAt     sql.NullInt64
		note         sql.NullString
		createdAt    int64
		updatedAt    int64
		lastSeenAt   sql.NullInt64
	)
	if err := row.Scan(
		&out.ID,
		&out.Name,
		&out.NameNormalized,
		&enabled,
		&out.TrafficLimitBytes,
		&out.TrafficUsedTxBytes,
		&out.TrafficUsedRxBytes,
		&expireAt,
		&note,
		&out.Subject,
		&createdAt,
		&updatedAt,
		&lastSeenAt,
	); err != nil {
		return User{}, translateSQLiteErr(err)
	}
	out.Enabled = boolFromSQLite(enabled)
	out.Note = optionalString(note)
	out.CreatedAt = fromUnixNano(createdAt)
	out.UpdatedAt = fromUnixNano(updatedAt)
	out.LastSeenAt = optionalInt64(lastSeenAt)
	if expireAt.Valid {
		ts := fromUnixNano(expireAt.Int64)
		if !ts.IsZero() {
			out.ExpireAt = &ts
		}
	}
	return out, nil
}

func (r *SQLiteRepository) scanCredential(row scanner) (Credential, error) {
	var (
		out       Credential
		protocol  string
		kind      string
		dataJSON  sql.NullString
		createdAt int64
		updatedAt int64
	)
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&protocol,
		&kind,
		&out.Identity,
		&out.Secret,
		&dataJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Credential{}, translateSQLiteErr(err)
	}
	out.Protocol = Protocol(strings.TrimSpace(protocol))
	out.Type = CredentialType(strings.TrimSpace(kind))
	out.DataJSON = strings.TrimSpace(dataJSON.String)
	out.CreatedAt = fromUnixNano(createdAt)
	out.UpdatedAt = fromUnixNano(updatedAt)
	return out, nil
}

func (r *SQLiteRepository) scanInbound(row scanner) (Inbound, error) {
	var (
		out       Inbound
		protocol  string
		enabled   int64
		createdAt int64
		updatedAt int64
	)
	if err := row.Scan(
		&out.ID,
		&out.NodeID,
		&out.Name,
		&protocol,
		&out.Transport,
		&out.Security,
		&out.Host,
		&out.Port,
		&enabled,
		&out.ParamsJSON,
		&out.RuntimeJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Inbound{}, translateSQLiteErr(err)
	}
	out.Protocol = Protocol(strings.TrimSpace(protocol))
	out.Enabled = boolFromSQLite(enabled)
	out.CreatedAt = fromUnixNano(createdAt)
	out.UpdatedAt = fromUnixNano(updatedAt)
	return out, nil
}

func (r *SQLiteRepository) scanSubscriptionToken(row scanner) (SubscriptionToken, error) {
	var (
		out       SubscriptionToken
		revoked   int64
		rotatedAt sql.NullInt64
		updatedAt int64
	)
	if err := row.Scan(
		&out.UserID,
		&out.Subject,
		&out.Version,
		&revoked,
		&rotatedAt,
		&updatedAt,
	); err != nil {
		return SubscriptionToken{}, translateSQLiteErr(err)
	}
	out.Revoked = boolFromSQLite(revoked)
	out.UpdatedAt = fromUnixNano(updatedAt)
	if rotatedAt.Valid {
		ts := fromUnixNano(rotatedAt.Int64)
		if !ts.IsZero() {
			out.RotatedAt = &ts
		}
	}
	return out, nil
}

func (r *SQLiteRepository) scanTrafficCounter(row scanner) (TrafficCounter, error) {
	var (
		out       TrafficCounter
		protocol  string
		snapshot  int64
	)
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&protocol,
		&out.TxBytes,
		&out.RxBytes,
		&out.Online,
		&snapshot,
	); err != nil {
		return TrafficCounter{}, translateSQLiteErr(err)
	}
	out.Protocol = Protocol(strings.TrimSpace(protocol))
	out.SnapshotAt = fromUnixNano(snapshot)
	return out, nil
}

func normalizeCredentialsInput(items []Credential, userID string, now time.Time) ([]Credential, error) {
	if len(items) == 0 {
		return nil, nil
	}
	result := make([]Credential, 0, len(items))
	seen := make(map[Protocol]struct{}, len(items))
	for _, item := range items {
		credential := item
		if err := paneldomain.ValidateProtocol(credential.Protocol); err != nil {
			return nil, err
		}
		if _, exists := seen[credential.Protocol]; exists {
			return nil, fmt.Errorf("duplicate protocol credential")
		}
		seen[credential.Protocol] = struct{}{}

		credential.ID = strings.TrimSpace(credential.ID)
		if credential.ID == "" {
			credential.ID = uuid.NewString()
		}
		credential.UserID = userID
		credential.CreatedAt = now
		credential.UpdatedAt = now

		switch credential.Protocol {
		case ProtocolHY2:
			credential.Type = CredentialTypeUserPass
			identity := strings.TrimSpace(credential.Identity)
			if identity == "" {
				identity = uuid.NewString()
			}
			parsedIdentity, parseErr := uuid.Parse(identity)
			if parseErr != nil {
				identity = uuid.NewString()
				parsedIdentity, _ = uuid.Parse(identity)
			}
			credential.Identity = strings.ToLower(parsedIdentity.String())
			credential.Secret = strings.TrimSpace(credential.Secret)
			if credential.Secret == "" {
				credential.Secret = strings.ReplaceAll(uuid.NewString(), "-", "")
			}
			if len(credential.Secret) < 8 {
				return nil, fmt.Errorf("hy2 secret must be at least 8 chars")
			}
		case ProtocolVLESS:
			credential.Type = CredentialTypeUUID
			identity := strings.TrimSpace(credential.Identity)
			if identity == "" {
				identity = uuid.NewString()
			}
			if _, err := uuid.Parse(identity); err != nil {
				return nil, fmt.Errorf("vless uuid is invalid")
			}
			credential.Identity = strings.ToLower(identity)
			credential.Secret = ""
		default:
			return nil, fmt.Errorf("unsupported protocol")
		}

		result = append(result, credential)
	}
	return result, nil
}

func (r *SQLiteRepository) nodeAddressByID(ctx context.Context, id string) (string, error) {
	row := r.db.QueryRowContext(
		resolveCtx(ctx),
		`SELECT address FROM nodes WHERE id = ? LIMIT 1`,
		strings.TrimSpace(id),
	)
	var address string
	if err := row.Scan(&address); err != nil {
		return "", translateSQLiteErr(err)
	}
	return strings.TrimSpace(address), nil
}

func normalizeInboundParamsForStorage(inbound *Inbound) error {
	if inbound == nil {
		return nil
	}
	raw := strings.TrimSpace(inbound.ParamsJSON)
	if raw == "" {
		raw = "{}"
	}
	params := make(map[string]any)
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return fmt.Errorf("inbound params_json is invalid")
	}
	if inbound.Protocol == ProtocolVLESS && strings.EqualFold(strings.TrimSpace(inbound.Security), "reality") {
		normalized, _, err := normalizeLegacyRealityParams(params)
		if err != nil {
			return fmt.Errorf("vless reality params are invalid: %w", err)
		}
		params = normalized
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return err
	}
	inbound.ParamsJSON = string(encoded)
	return nil
}

func protocolsFromCredentials(credentials []Credential, filter *Protocol) []Protocol {
	if filter != nil {
		return []Protocol{*filter}
	}
	if len(credentials) == 0 {
		return nil
	}
	seen := make(map[Protocol]struct{}, len(credentials))
	out := make([]Protocol, 0, len(credentials))
	for _, item := range credentials {
		if _, ok := seen[item.Protocol]; ok {
			continue
		}
		seen[item.Protocol] = struct{}{}
		out = append(out, item.Protocol)
	}
	return out
}

func normalizeIDs(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		id := strings.TrimSpace(item)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func placeholders(count int) string {
	if count <= 0 {
		return "''"
	}
	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		parts = append(parts, "?")
	}
	return strings.Join(parts, ",")
}

func buildDeleteCredentialsArgs(userID string, credentials []Credential) []any {
	args := make([]any, 0, len(credentials)+1)
	args = append(args, strings.TrimSpace(userID))
	for _, credential := range credentials {
		args = append(args, string(credential.Protocol))
	}
	return args
}

func nullString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullInt64(value *time.Time) any {
	if value == nil {
		return nil
	}
	ts := value.UTC()
	if ts.IsZero() {
		return nil
	}
	return toUnixNano(ts)
}
