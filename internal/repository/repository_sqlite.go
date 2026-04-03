package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteRepository struct {
	db   *sql.DB
	path string
}

func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxIdleTime(0)
	db.SetConnMaxLifetime(0)

	repo := &SQLiteRepository{db: db, path: dbPath}
	if err := repo.applyPragmas(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := repo.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *SQLiteRepository) Path() string {
	return r.path
}

func (r *SQLiteRepository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *SQLiteRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(resolveCtx(ctx))
}

func (r *SQLiteRepository) applyPragmas(ctx context.Context) error {
	for _, stmt := range []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA foreign_keys=ON;`,
		`PRAGMA busy_timeout=5000;`,
	} {
		if _, err := r.db.ExecContext(resolveCtx(ctx), stmt); err != nil {
			return fmt.Errorf("apply sqlite pragma (%s): %w", strings.TrimSpace(stmt), err)
		}
	}
	return nil
}

func (r *SQLiteRepository) ensureSchema(ctx context.Context) error {
	if err := r.ensureBaseSchema(ctx); err != nil {
		return err
	}
	if err := r.ensureCoreSchema(ctx); err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) ensureBaseSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS admins (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			is_active INTEGER NOT NULL,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			admin_id TEXT NOT NULL,
			session_token_hash TEXT NOT NULL UNIQUE,
			expires_at_ns INTEGER NOT NULL,
			created_at_ns INTEGER NOT NULL,
			last_seen_at_ns INTEGER NOT NULL,
			ip TEXT NOT NULL,
			user_agent TEXT NOT NULL,
			FOREIGN KEY(admin_id) REFERENCES admins(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at_ns);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_admin ON sessions(admin_id);`,
		`CREATE TABLE IF NOT EXISTS service_states (
			id INTEGER PRIMARY KEY,
			service_name TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL,
			version TEXT,
			last_check_at_ns INTEGER NOT NULL,
			raw_json TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_service_states_checked ON service_states(last_check_at_ns DESC);`,
		`DROP INDEX IF EXISTS idx_audit_logs_created_at;`,
		`DROP TABLE IF EXISTS audit_logs;`,
		`CREATE TABLE IF NOT EXISTS system_snapshots (
			id INTEGER PRIMARY KEY,
			snapshot_at_ns INTEGER NOT NULL,
			cpu_usage_percent REAL NOT NULL,
			memory_used_percent REAL NOT NULL,
			network_rx_bps REAL NOT NULL,
			network_tx_bps REAL NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_system_snapshots_at ON system_snapshots(snapshot_at_ns DESC);`,
	}
	for _, stmt := range statements {
		if _, err := r.db.ExecContext(resolveCtx(ctx), stmt); err != nil {
			return fmt.Errorf("ensure sqlite base schema: %w", err)
		}
	}
	return nil
}

func (r *SQLiteRepository) ensureCoreSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS core_schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at_ns INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS core_servers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			public_host TEXT NOT NULL,
			panel_public_url TEXT NOT NULL,
			subscription_base_url TEXT NOT NULL,
			singbox_binary_path TEXT NOT NULL,
			singbox_config_path TEXT NOT NULL,
			singbox_service_name TEXT NOT NULL,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS core_inbounds (
			id TEXT PRIMARY KEY,
			server_id TEXT NOT NULL,
			name TEXT NOT NULL,
			tag TEXT NOT NULL,
			protocol TEXT NOT NULL CHECK(protocol IN ('vless', 'hysteria2')),
			listen TEXT NOT NULL,
			listen_port INTEGER NOT NULL,
			enabled INTEGER NOT NULL,
			template_key TEXT NOT NULL,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL,
			FOREIGN KEY(server_id) REFERENCES core_servers(id) ON DELETE CASCADE,
			UNIQUE(server_id, tag)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_core_inbounds_server ON core_inbounds(server_id, enabled);`,
		`CREATE TABLE IF NOT EXISTS core_inbound_vless_settings (
			inbound_id TEXT PRIMARY KEY,
			tls_enabled INTEGER NOT NULL,
			tls_server_name TEXT,
			tls_alpn_csv TEXT,
			tls_certificate_path TEXT,
			tls_key_path TEXT,
			reality_enabled INTEGER NOT NULL,
			reality_public_key TEXT,
			reality_private_key_enc TEXT,
			reality_short_id TEXT,
			reality_handshake_server TEXT,
			reality_handshake_server_port INTEGER,
			flow TEXT,
			transport_type TEXT NOT NULL,
			transport_host TEXT,
			transport_path TEXT,
			multiplex_enabled INTEGER NOT NULL,
			multiplex_protocol TEXT,
			multiplex_max_connections INTEGER,
			multiplex_min_streams INTEGER,
			multiplex_max_streams INTEGER,
			FOREIGN KEY(inbound_id) REFERENCES core_inbounds(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS core_inbound_hysteria2_settings (
			inbound_id TEXT PRIMARY KEY,
			tls_enabled INTEGER NOT NULL,
			tls_server_name TEXT,
			tls_certificate_path TEXT,
			tls_key_path TEXT,
			up_mbps INTEGER,
			down_mbps INTEGER,
			ignore_client_bandwidth INTEGER NOT NULL,
			obfs_type TEXT,
			obfs_password_enc TEXT,
			masquerade_json TEXT,
			bbr_profile TEXT,
			brutal_debug INTEGER NOT NULL,
			FOREIGN KEY(inbound_id) REFERENCES core_inbounds(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS core_users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			enabled INTEGER NOT NULL,
			traffic_limit_bytes INTEGER NOT NULL DEFAULT 0,
			traffic_used_up_bytes INTEGER NOT NULL DEFAULT 0,
			traffic_used_down_bytes INTEGER NOT NULL DEFAULT 0,
			expire_at_ns INTEGER,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_core_users_enabled ON core_users(enabled, updated_at_ns DESC);`,
		`CREATE TABLE IF NOT EXISTS core_user_access (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			inbound_id TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			vless_uuid TEXT,
			vless_flow_override TEXT,
			hy2_password_enc TEXT,
			traffic_limit_bytes_override INTEGER,
			expire_at_ns_override INTEGER,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL,
			FOREIGN KEY(user_id) REFERENCES core_users(id) ON DELETE CASCADE,
			FOREIGN KEY(inbound_id) REFERENCES core_inbounds(id) ON DELETE CASCADE,
			UNIQUE(user_id, inbound_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_core_user_access_user ON core_user_access(user_id, enabled);`,
		`CREATE INDEX IF NOT EXISTS idx_core_user_access_inbound ON core_user_access(inbound_id, enabled);`,
		`CREATE TABLE IF NOT EXISTS core_subscriptions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE,
			profile_name TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL,
			FOREIGN KEY(user_id) REFERENCES core_users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS core_subscription_tokens (
			id TEXT PRIMARY KEY,
			subscription_id TEXT NOT NULL,
			token_prefix TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			token_salt TEXT NOT NULL,
			revoked_at_ns INTEGER,
			expires_at_ns INTEGER,
			last_used_at_ns INTEGER,
			last_used_ip TEXT,
			created_at_ns INTEGER NOT NULL,
			FOREIGN KEY(subscription_id) REFERENCES core_subscriptions(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_core_subscription_tokens_prefix ON core_subscription_tokens(token_prefix);`,
		`CREATE TABLE IF NOT EXISTS core_config_revisions (
			id TEXT PRIMARY KEY,
			server_id TEXT NOT NULL,
			revision_no INTEGER NOT NULL,
			config_hash TEXT NOT NULL,
			rendered_json TEXT NOT NULL,
			check_ok INTEGER NOT NULL,
			check_error TEXT,
			applied_at_ns INTEGER,
			rollback_from_revision_id TEXT,
			created_at_ns INTEGER NOT NULL,
			FOREIGN KEY(server_id) REFERENCES core_servers(id) ON DELETE CASCADE,
			UNIQUE(server_id, revision_no)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_core_config_revisions_server ON core_config_revisions(server_id, revision_no DESC);`,
	}
	for _, stmt := range statements {
		if _, err := r.db.ExecContext(resolveCtx(ctx), stmt); err != nil {
			return fmt.Errorf("ensure sqlite core schema: %w", err)
		}
	}
	if _, err := r.db.ExecContext(resolveCtx(ctx), `INSERT OR IGNORE INTO core_schema_migrations(version, applied_at_ns) VALUES (1, ?);`, nowNano()); err != nil {
		return fmt.Errorf("record core schema version: %w", err)
	}
	return nil
}

func resolveCtx(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func nowNano() int64 {
	return time.Now().UTC().UnixNano()
}

func toUnixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UTC().UnixNano()
}

func fromUnixNano(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(0, ts).UTC()
}

func sqliteBool(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func boolFromSQLite(value int64) bool {
	return value != 0
}

func optionalString(raw sql.NullString) *string {
	if !raw.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(raw.String)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func translateSQLiteErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint failed") || strings.Contains(message, "constraint failed") {
		return ErrUniqueViolation
	}
	return err
}

func (r *SQLiteRepository) purgeExpiredSessions(ctx context.Context, now int64) error {
	_, err := r.db.ExecContext(resolveCtx(ctx), `DELETE FROM sessions WHERE expires_at_ns <= ?`, now)
	return err
}

func (r *SQLiteRepository) scanAdmin(row scanner) (Admin, error) {
	var (
		out       Admin
		isActive  int64
		createdAt int64
		updatedAt int64
	)
	if err := row.Scan(
		&out.ID,
		&out.Email,
		&out.PasswordHash,
		&isActive,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Admin{}, translateSQLiteErr(err)
	}
	out.IsActive = boolFromSQLite(isActive)
	out.CreatedAt = fromUnixNano(createdAt)
	out.UpdatedAt = fromUnixNano(updatedAt)
	return out, nil
}

func (r *SQLiteRepository) scanSession(row scanner) (Session, error) {
	var (
		out        Session
		expiresAt  int64
		createdAt  int64
		lastSeenAt int64
	)
	if err := row.Scan(
		&out.ID,
		&out.AdminID,
		&out.SessionTokenHash,
		&expiresAt,
		&createdAt,
		&lastSeenAt,
		&out.IP,
		&out.UserAgent,
	); err != nil {
		return Session{}, translateSQLiteErr(err)
	}
	out.ExpiresAt = fromUnixNano(expiresAt)
	out.CreatedAt = fromUnixNano(createdAt)
	out.LastSeenAt = fromUnixNano(lastSeenAt)
	return out, nil
}

func (r *SQLiteRepository) scanSystemSnapshot(row scanner) (SystemSnapshot, error) {
	var (
		out        SystemSnapshot
		snapshotAt int64
	)
	if err := row.Scan(
		&out.ID,
		&snapshotAt,
		&out.CPUUsagePercent,
		&out.MemoryUsedPercent,
		&out.NetworkRxBps,
		&out.NetworkTxBps,
	); err != nil {
		return SystemSnapshot{}, translateSQLiteErr(err)
	}
	out.SnapshotAt = fromUnixNano(snapshotAt)
	return out, nil
}

func (r *SQLiteRepository) scanServiceState(row scanner) (ServiceState, error) {
	var (
		out         ServiceState
		version     sql.NullString
		rawJSON     sql.NullString
		lastCheckAt int64
	)
	if err := row.Scan(
		&out.ID,
		&out.ServiceName,
		&out.Status,
		&version,
		&lastCheckAt,
		&rawJSON,
	); err != nil {
		return ServiceState{}, translateSQLiteErr(err)
	}
	out.Version = optionalString(version)
	out.LastCheckAt = fromUnixNano(lastCheckAt)
	out.RawJSON = optionalString(rawJSON)
	return out, nil
}

type scanner interface {
	Scan(dest ...any) error
}
