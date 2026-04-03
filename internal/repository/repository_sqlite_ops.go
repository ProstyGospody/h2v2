package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EntityCounts struct {
	Admins              int `json:"admins"`
	Sessions            int `json:"sessions"`
	SystemSnapshots     int `json:"system_snapshots"`
	AuditLogs           int `json:"audit_logs"`
	ServiceStates       int `json:"service_states"`
	Nodes               int `json:"nodes"`
	Users               int `json:"users"`
	Credentials         int `json:"credentials"`
	Inbounds            int `json:"inbounds"`
	SubscriptionTokens  int `json:"subscription_tokens"`
	TrafficCounters     int `json:"traffic_counters"`
	RuntimeUserState    int `json:"runtime_user_state"`
}

type ExportPayload struct {
	Driver             string              `json:"driver"`
	SQLitePath         string              `json:"sqlite_path"`
	ExportedAt         time.Time           `json:"exported_at"`
	Counts             EntityCounts        `json:"counts"`
	Admins             []Admin             `json:"admins"`
	Sessions           []Session           `json:"sessions"`
	SystemSnapshots    []SystemSnapshot    `json:"system_snapshots"`
	AuditLogs          []AuditLog          `json:"audit_logs"`
	ServiceStates      []ServiceState      `json:"service_states"`
	Nodes              []map[string]any    `json:"nodes"`
	Users              []User              `json:"users"`
	Credentials        []Credential        `json:"credentials"`
	Inbounds           []Inbound           `json:"inbounds"`
	SubscriptionTokens []SubscriptionToken `json:"subscription_tokens"`
	TrafficCounters    []TrafficCounter    `json:"traffic_counters"`
	RuntimeUserState   []map[string]any    `json:"runtime_user_state"`
}

func (r *SQLiteRepository) BackupTo(ctx context.Context, outPath string) error {
	outPath = strings.TrimSpace(outPath)
	if outPath == "" {
		return fmt.Errorf("backup output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing backup: %w", err)
	}
	escaped := strings.ReplaceAll(outPath, `'`, `''`)
	_, err := r.db.ExecContext(resolveCtx(ctx), fmt.Sprintf(`VACUUM INTO '%s';`, escaped))
	if err != nil {
		return fmt.Errorf("sqlite backup failed: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) ExportToJSON(ctx context.Context, outPath string) (EntityCounts, error) {
	payload, err := r.ExportPayload(ctx)
	if err != nil {
		return EntityCounts{}, err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return EntityCounts{}, err
	}
	data = append(data, '\n')
	outPath = strings.TrimSpace(outPath)
	if outPath == "" {
		return EntityCounts{}, fmt.Errorf("export output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return EntityCounts{}, err
	}
	if err := os.WriteFile(outPath, data, 0o640); err != nil {
		return EntityCounts{}, err
	}
	return payload.Counts, nil
}

func (r *SQLiteRepository) ExportPayload(ctx context.Context) (ExportPayload, error) {
	admins, err := r.listAdmins(ctx)
	if err != nil {
		return ExportPayload{}, err
	}
	sessions, err := r.listSessions(ctx)
	if err != nil {
		return ExportPayload{}, err
	}
	systemSnapshots, err := r.listSystemSnapshots(ctx)
	if err != nil {
		return ExportPayload{}, err
	}
	auditLogs, err := r.listAuditLogs(ctx)
	if err != nil {
		return ExportPayload{}, err
	}
	serviceStates, err := r.listServiceStates(ctx)
	if err != nil {
		return ExportPayload{}, err
	}
	nodes, err := r.listNodesRaw(ctx)
	if err != nil {
		return ExportPayload{}, err
	}
	users, err := r.listUsersRaw(ctx)
	if err != nil {
		return ExportPayload{}, err
	}
	credentials, err := r.listCredentialsRaw(ctx)
	if err != nil {
		return ExportPayload{}, err
	}
	inbounds, err := r.ListInbounds(ctx, nil)
	if err != nil {
		return ExportPayload{}, err
	}
	subscriptionTokens, err := r.listSubscriptionTokensRaw(ctx)
	if err != nil {
		return ExportPayload{}, err
	}
	trafficCounters, err := r.ListTrafficCounters(ctx, "", nil, 0, 0)
	if err != nil {
		return ExportPayload{}, err
	}
	runtimeUserState, err := r.listRuntimeUserStateRaw(ctx)
	if err != nil {
		return ExportPayload{}, err
	}

	counts := EntityCounts{
		Admins:             len(admins),
		Sessions:           len(sessions),
		SystemSnapshots:    len(systemSnapshots),
		AuditLogs:          len(auditLogs),
		ServiceStates:      len(serviceStates),
		Nodes:              len(nodes),
		Users:              len(users),
		Credentials:        len(credentials),
		Inbounds:           len(inbounds),
		SubscriptionTokens: len(subscriptionTokens),
		TrafficCounters:    len(trafficCounters),
		RuntimeUserState:   len(runtimeUserState),
	}
	return ExportPayload{
		Driver:             StorageDriverSQLite,
		SQLitePath:         r.path,
		ExportedAt:         time.Now().UTC(),
		Counts:             counts,
		Admins:             admins,
		Sessions:           sessions,
		SystemSnapshots:    systemSnapshots,
		AuditLogs:          auditLogs,
		ServiceStates:      serviceStates,
		Nodes:              nodes,
		Users:              users,
		Credentials:        credentials,
		Inbounds:           inbounds,
		SubscriptionTokens: subscriptionTokens,
		TrafficCounters:    trafficCounters,
		RuntimeUserState:   runtimeUserState,
	}, nil
}

func SQLiteRestore(dbPath string, fromPath string) (string, error) {
	dbPath = strings.TrimSpace(dbPath)
	fromPath = strings.TrimSpace(fromPath)
	if dbPath == "" || fromPath == "" {
		return "", fmt.Errorf("both --db and --from are required")
	}
	absDB, err := filepath.Abs(dbPath)
	if err != nil {
		return "", err
	}
	absFrom, err := filepath.Abs(fromPath)
	if err != nil {
		return "", err
	}
	if absDB == absFrom {
		return "", fmt.Errorf("source and destination databases must differ")
	}
	if _, err := os.Stat(absFrom); err != nil {
		return "", fmt.Errorf("backup database is not accessible: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absDB), 0o750); err != nil {
		return "", err
	}

	rollbackPath := ""
	if _, err := os.Stat(absDB); err == nil {
		rollbackPath = absDB + ".pre-restore-" + time.Now().UTC().Format("20060102-150405")
		if err := copyFile(absDB, rollbackPath, 0o640); err != nil {
			return "", fmt.Errorf("create rollback backup: %w", err)
		}
	}

	if err := os.Remove(absDB + "-wal"); err != nil && !os.IsNotExist(err) {
		return rollbackPath, err
	}
	if err := os.Remove(absDB + "-shm"); err != nil && !os.IsNotExist(err) {
		return rollbackPath, err
	}
	temp := absDB + ".restore.tmp"
	if err := copyFile(absFrom, temp, 0o640); err != nil {
		return rollbackPath, err
	}
	if err := os.Rename(temp, absDB); err != nil {
		_ = os.Remove(temp)
		return rollbackPath, err
	}
	return rollbackPath, nil
}

func (r *SQLiteRepository) RestoreFromBackup(ctx context.Context, fromPath string) (EntityCounts, error) {
	fromPath = strings.TrimSpace(fromPath)
	if fromPath == "" {
		return EntityCounts{}, fmt.Errorf("backup database path is required")
	}

	absFrom, err := filepath.Abs(fromPath)
	if err != nil {
		return EntityCounts{}, err
	}
	absTarget, err := filepath.Abs(r.path)
	if err != nil {
		return EntityCounts{}, err
	}
	if absFrom == absTarget {
		return EntityCounts{}, fmt.Errorf("source and destination databases must differ")
	}
	if _, err := os.Stat(absFrom); err != nil {
		return EntityCounts{}, fmt.Errorf("backup database is not accessible: %w", err)
	}

	source, err := NewSQLiteRepository(absFrom)
	if err != nil {
		return EntityCounts{}, err
	}
	defer source.Close()

	payload, err := source.ExportPayload(ctx)
	if err != nil {
		return EntityCounts{}, err
	}
	if payload.Counts.Admins == 0 {
		return EntityCounts{}, fmt.Errorf("backup validation failed: admins must be greater than zero")
	}

	tx, err := r.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return EntityCounts{}, fmt.Errorf("restore begin transaction: %w", err)
	}
	if err := r.restoreDataTx(ctx, tx, payload); err != nil {
		_ = tx.Rollback()
		return EntityCounts{}, err
	}
	counts, err := countEntitiesTx(ctx, tx)
	if err != nil {
		_ = tx.Rollback()
		return EntityCounts{}, err
	}
	if counts != payload.Counts {
		_ = tx.Rollback()
		return EntityCounts{}, fmt.Errorf("restore validation failed: source=%+v target=%+v", payload.Counts, counts)
	}
	if err := tx.Commit(); err != nil {
		return EntityCounts{}, fmt.Errorf("restore commit transaction: %w", err)
	}
	return counts, nil
}

func (r *SQLiteRepository) restoreDataTx(ctx context.Context, tx *sql.Tx, payload ExportPayload) error {
	clearStatements := []string{
		`DELETE FROM sessions`,
		`DELETE FROM traffic_counters`,
		`DELETE FROM runtime_user_state`,
		`DELETE FROM subscription_tokens`,
		`DELETE FROM credentials`,
		`DELETE FROM inbounds`,
		`DELETE FROM users`,
		`DELETE FROM nodes`,
		`DELETE FROM system_snapshots`,
		`DELETE FROM audit_logs`,
		`DELETE FROM service_states`,
		`DELETE FROM admins`,
	}
	for _, stmt := range clearStatements {
		if _, err := tx.ExecContext(resolveCtx(ctx), stmt); err != nil {
			return err
		}
	}

	if err := r.importAdminsTx(ctx, tx, payload.Admins); err != nil {
		return err
	}
	if err := r.importSessionsTx(ctx, tx, payload.Sessions); err != nil {
		return err
	}
	if err := r.importNodesRawTx(ctx, tx, payload.Nodes); err != nil {
		return err
	}
	if err := r.importUsersRawTx(ctx, tx, payload.Users); err != nil {
		return err
	}
	if err := r.importCredentialsRawTx(ctx, tx, payload.Credentials); err != nil {
		return err
	}
	if err := r.importInboundsRawTx(ctx, tx, payload.Inbounds); err != nil {
		return err
	}
	if err := r.importSubscriptionTokensRawTx(ctx, tx, payload.SubscriptionTokens); err != nil {
		return err
	}
	if err := r.importTrafficCountersRawTx(ctx, tx, payload.TrafficCounters); err != nil {
		return err
	}
	if err := r.importRuntimeUserStateRawTx(ctx, tx, payload.RuntimeUserState); err != nil {
		return err
	}
	if err := r.importSystemSnapshotsTx(ctx, tx, payload.SystemSnapshots); err != nil {
		return err
	}
	if err := r.importAuditLogsTx(ctx, tx, payload.AuditLogs); err != nil {
		return err
	}
	if err := r.importServiceStatesTx(ctx, tx, payload.ServiceStates); err != nil {
		return err
	}
	return nil
}

func copyFile(src string, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) importAdminsTx(ctx context.Context, tx *sql.Tx, items []Admin) error {
	stmt := `INSERT INTO admins (id, email, password_hash, is_active, created_at_ns, updated_at_ns)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			email = excluded.email,
			password_hash = excluded.password_hash,
			is_active = excluded.is_active,
			created_at_ns = excluded.created_at_ns,
			updated_at_ns = excluded.updated_at_ns`
	for _, item := range items {
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.ID,
			strings.ToLower(strings.TrimSpace(item.Email)),
			item.PasswordHash,
			sqliteBool(item.IsActive),
			toUnixNano(item.CreatedAt),
			toUnixNano(item.UpdatedAt),
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) importSessionsTx(ctx context.Context, tx *sql.Tx, items []Session) error {
	stmt := `INSERT INTO sessions (id, admin_id, session_token_hash, expires_at_ns, created_at_ns, last_seen_at_ns, ip, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			admin_id = excluded.admin_id,
			session_token_hash = excluded.session_token_hash,
			expires_at_ns = excluded.expires_at_ns,
			created_at_ns = excluded.created_at_ns,
			last_seen_at_ns = excluded.last_seen_at_ns,
			ip = excluded.ip,
			user_agent = excluded.user_agent`
	for _, item := range items {
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.ID,
			item.AdminID,
			item.SessionTokenHash,
			toUnixNano(item.ExpiresAt),
			toUnixNano(item.CreatedAt),
			toUnixNano(item.LastSeenAt),
			item.IP,
			item.UserAgent,
		); err != nil {
			return err
		}
	}
	return nil
}

func countEntitiesTx(ctx context.Context, tx *sql.Tx) (EntityCounts, error) {
	getCount := func(table string) (int, error) {
		var count int
		query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)
		if err := tx.QueryRowContext(resolveCtx(ctx), query).Scan(&count); err != nil {
			return 0, err
		}
		return count, nil
	}
	admins, err := getCount("admins")
	if err != nil {
		return EntityCounts{}, err
	}
	sessions, err := getCount("sessions")
	if err != nil {
		return EntityCounts{}, err
	}
	systemSnapshots, err := getCount("system_snapshots")
	if err != nil {
		return EntityCounts{}, err
	}
	auditLogs, err := getCount("audit_logs")
	if err != nil {
		return EntityCounts{}, err
	}
	serviceStates, err := getCount("service_states")
	if err != nil {
		return EntityCounts{}, err
	}
	nodes, err := getCount("nodes")
	if err != nil {
		return EntityCounts{}, err
	}
	users, err := getCount("users")
	if err != nil {
		return EntityCounts{}, err
	}
	credentials, err := getCount("credentials")
	if err != nil {
		return EntityCounts{}, err
	}
	inbounds, err := getCount("inbounds")
	if err != nil {
		return EntityCounts{}, err
	}
	subscriptionTokens, err := getCount("subscription_tokens")
	if err != nil {
		return EntityCounts{}, err
	}
	trafficCounters, err := getCount("traffic_counters")
	if err != nil {
		return EntityCounts{}, err
	}
	runtimeUserState, err := getCount("runtime_user_state")
	if err != nil {
		return EntityCounts{}, err
	}
	return EntityCounts{
		Admins:             admins,
		Sessions:           sessions,
		SystemSnapshots:    systemSnapshots,
		AuditLogs:          auditLogs,
		ServiceStates:      serviceStates,
		Nodes:              nodes,
		Users:              users,
		Credentials:        credentials,
		Inbounds:           inbounds,
		SubscriptionTokens: subscriptionTokens,
		TrafficCounters:    trafficCounters,
		RuntimeUserState:   runtimeUserState,
	}, nil
}

func (r *SQLiteRepository) listAdmins(ctx context.Context) ([]Admin, error) {
	rows, err := r.db.QueryContext(resolveCtx(ctx), `SELECT id, email, password_hash, is_active, created_at_ns, updated_at_ns FROM admins ORDER BY created_at_ns ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Admin, 0)
	for rows.Next() {
		item, err := r.scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) listSessions(ctx context.Context) ([]Session, error) {
	rows, err := r.db.QueryContext(resolveCtx(ctx), `SELECT id, admin_id, session_token_hash, expires_at_ns, created_at_ns, last_seen_at_ns, ip, user_agent FROM sessions ORDER BY created_at_ns ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Session, 0)
	for rows.Next() {
		item, err := r.scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) countTable(ctx context.Context, table string) (int, error) {
	var count int
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)
	if err := r.db.QueryRowContext(resolveCtx(ctx), query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *SQLiteRepository) listNodesRaw(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.db.QueryContext(resolveCtx(ctx), `SELECT id, name, address, enabled, created_at_ns, updated_at_ns FROM nodes ORDER BY created_at_ns ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var (
			id        string
			name      string
			address   string
			enabled   int64
			createdAt int64
			updatedAt int64
		)
		if err := rows.Scan(&id, &name, &address, &enabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id":            id,
			"name":          name,
			"address":       address,
			"enabled":       enabled,
			"created_at_ns": createdAt,
			"updated_at_ns": updatedAt,
		})
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) listUsersRaw(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(
		resolveCtx(ctx),
		`SELECT id, name, name_normalized, enabled, traffic_limit_bytes, traffic_used_tx_bytes, traffic_used_rx_bytes, expire_at_ns, note, subject, created_at_ns, updated_at_ns, last_seen_at_ns
		 FROM users
		 ORDER BY created_at_ns ASC, id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]User, 0)
	for rows.Next() {
		item, err := r.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) listCredentialsRaw(ctx context.Context) ([]Credential, error) {
	rows, err := r.db.QueryContext(
		resolveCtx(ctx),
		`SELECT id, user_id, protocol, credential_type, identity, secret, data_json, created_at_ns, updated_at_ns
		 FROM credentials
		 ORDER BY created_at_ns ASC, id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Credential, 0)
	for rows.Next() {
		item, err := r.scanCredential(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) listSubscriptionTokensRaw(ctx context.Context) ([]SubscriptionToken, error) {
	rows, err := r.db.QueryContext(
		resolveCtx(ctx),
		`SELECT user_id, subject, version, revoked, rotated_at_ns, updated_at_ns
		 FROM subscription_tokens
		 ORDER BY updated_at_ns ASC, user_id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SubscriptionToken, 0)
	for rows.Next() {
		item, err := r.scanSubscriptionToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) importNodesRawTx(ctx context.Context, tx *sql.Tx, items []map[string]any) error {
	stmt := `INSERT INTO nodes (id, name, address, enabled, created_at_ns, updated_at_ns)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			address = excluded.address,
			enabled = excluded.enabled,
			created_at_ns = excluded.created_at_ns,
			updated_at_ns = excluded.updated_at_ns`
	for _, item := range items {
		id, _ := item["id"].(string)
		name, _ := item["name"].(string)
		address, _ := item["address"].(string)
		enabled, _ := item["enabled"].(int64)
		createdAt, _ := item["created_at_ns"].(int64)
		updatedAt, _ := item["updated_at_ns"].(int64)
		if _, err := tx.ExecContext(resolveCtx(ctx), stmt, id, name, address, enabled, createdAt, updatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) importUsersRawTx(ctx context.Context, tx *sql.Tx, items []User) error {
	stmt := `INSERT INTO users (
			id, name, name_normalized, enabled, traffic_limit_bytes, traffic_used_tx_bytes, traffic_used_rx_bytes,
			expire_at_ns, note, subject, created_at_ns, updated_at_ns, last_seen_at_ns
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			name_normalized = excluded.name_normalized,
			enabled = excluded.enabled,
			traffic_limit_bytes = excluded.traffic_limit_bytes,
			traffic_used_tx_bytes = excluded.traffic_used_tx_bytes,
			traffic_used_rx_bytes = excluded.traffic_used_rx_bytes,
			expire_at_ns = excluded.expire_at_ns,
			note = excluded.note,
			subject = excluded.subject,
			created_at_ns = excluded.created_at_ns,
			updated_at_ns = excluded.updated_at_ns,
			last_seen_at_ns = excluded.last_seen_at_ns`
	for _, item := range items {
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.ID,
			item.Name,
			item.NameNormalized,
			sqliteBool(item.Enabled),
			item.TrafficLimitBytes,
			item.TrafficUsedTxBytes,
			item.TrafficUsedRxBytes,
			nullInt64(item.ExpireAt),
			nullString(item.Note),
			item.Subject,
			toUnixNano(item.CreatedAt),
			toUnixNano(item.UpdatedAt),
			nullInt64(item.LastSeenAt),
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) importCredentialsRawTx(ctx context.Context, tx *sql.Tx, items []Credential) error {
	stmt := `INSERT INTO credentials (
			id, user_id, protocol, credential_type, identity, secret, data_json, created_at_ns, updated_at_ns
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, protocol) DO UPDATE SET
			id = excluded.id,
			credential_type = excluded.credential_type,
			identity = excluded.identity,
			secret = excluded.secret,
			data_json = excluded.data_json,
			created_at_ns = excluded.created_at_ns,
			updated_at_ns = excluded.updated_at_ns`
	for _, item := range items {
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.ID,
			item.UserID,
			string(item.Protocol),
			string(item.Type),
			item.Identity,
			item.Secret,
			nullString(item.DataJSON),
			toUnixNano(item.CreatedAt),
			toUnixNano(item.UpdatedAt),
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) importInboundsRawTx(ctx context.Context, tx *sql.Tx, items []Inbound) error {
	stmt := `INSERT INTO inbounds (
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
			created_at_ns = excluded.created_at_ns,
			updated_at_ns = excluded.updated_at_ns`
	for _, item := range items {
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.ID,
			item.NodeID,
			item.Name,
			string(item.Protocol),
			item.Transport,
			item.Security,
			item.Host,
			item.Port,
			sqliteBool(item.Enabled),
			item.ParamsJSON,
			item.RuntimeJSON,
			toUnixNano(item.CreatedAt),
			toUnixNano(item.UpdatedAt),
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) importSubscriptionTokensRawTx(ctx context.Context, tx *sql.Tx, items []SubscriptionToken) error {
	stmt := `INSERT INTO subscription_tokens (user_id, subject, version, revoked, rotated_at_ns, updated_at_ns)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			subject = excluded.subject,
			version = excluded.version,
			revoked = excluded.revoked,
			rotated_at_ns = excluded.rotated_at_ns,
			updated_at_ns = excluded.updated_at_ns`
	for _, item := range items {
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.UserID,
			item.Subject,
			item.Version,
			sqliteBool(item.Revoked),
			nullInt64(item.RotatedAt),
			toUnixNano(item.UpdatedAt),
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) importTrafficCountersRawTx(ctx context.Context, tx *sql.Tx, items []TrafficCounter) error {
	stmt := `INSERT INTO traffic_counters (id, user_id, protocol, tx_bytes, rx_bytes, online_count, snapshot_at_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_id = excluded.user_id,
			protocol = excluded.protocol,
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
			string(item.Protocol),
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

func (r *SQLiteRepository) listRuntimeUserStateRaw(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.db.QueryContext(
		resolveCtx(ctx),
		`SELECT user_id, protocol, online_count, last_sync_at_ns, last_error
		 FROM runtime_user_state
		 ORDER BY last_sync_at_ns ASC, user_id ASC, protocol ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var (
			userID     string
			protocol   string
			online     int64
			lastSyncAt int64
			lastError  sql.NullString
		)
		if err := rows.Scan(&userID, &protocol, &online, &lastSyncAt, &lastError); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"user_id":         userID,
			"protocol":        protocol,
			"online_count":    online,
			"last_sync_at_ns": lastSyncAt,
			"last_error":      strings.TrimSpace(lastError.String),
		})
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) importRuntimeUserStateRawTx(ctx context.Context, tx *sql.Tx, items []map[string]any) error {
	stmt := `INSERT INTO runtime_user_state (user_id, protocol, online_count, last_sync_at_ns, last_error)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, protocol) DO UPDATE SET
			online_count = excluded.online_count,
			last_sync_at_ns = excluded.last_sync_at_ns,
			last_error = excluded.last_error`
	for _, item := range items {
		userID, _ := item["user_id"].(string)
		protocol, _ := item["protocol"].(string)
		online, _ := item["online_count"].(int64)
		lastSyncAt, _ := item["last_sync_at_ns"].(int64)
		lastError, _ := item["last_error"].(string)
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			userID,
			protocol,
			online,
			lastSyncAt,
			nullString(lastError),
		); err != nil {
			return err
		}
	}
	return nil
}
