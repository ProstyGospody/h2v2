package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	hysteriadomain "h2v2/internal/domain/hysteria"
)

const sqliteSchemaVersion = 1

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
		`CREATE TABLE IF NOT EXISTS hysteria_users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			username_normalized TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			note TEXT,
			client_overrides_json TEXT,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL,
			last_seen_at_ns INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_hysteria_users_created_at ON hysteria_users(created_at_ns DESC);`,
		`CREATE TABLE IF NOT EXISTS hysteria_snapshots (
			id INTEGER PRIMARY KEY,
			user_id TEXT NOT NULL,
			tx_bytes INTEGER NOT NULL,
			rx_bytes INTEGER NOT NULL,
			online_count INTEGER NOT NULL,
			snapshot_at_ns INTEGER NOT NULL,
			FOREIGN KEY(user_id) REFERENCES hysteria_users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_hysteria_snapshots_user_at ON hysteria_snapshots(user_id, snapshot_at_ns DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_hysteria_snapshots_at ON hysteria_snapshots(snapshot_at_ns DESC);`,
		`CREATE TABLE IF NOT EXISTS service_states (
			id INTEGER PRIMARY KEY,
			service_name TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL,
			version TEXT,
			last_check_at_ns INTEGER NOT NULL,
			raw_json TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_service_states_checked ON service_states(last_check_at_ns DESC);`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY,
			admin_id TEXT,
			action TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT,
			payload_json TEXT NOT NULL,
			created_at_ns INTEGER NOT NULL,
			FOREIGN KEY(admin_id) REFERENCES admins(id) ON DELETE SET NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at_ns DESC);`,
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
			return fmt.Errorf("ensure sqlite schema: %w", err)
		}
	}
	if _, err := r.db.ExecContext(resolveCtx(ctx), fmt.Sprintf(`PRAGMA user_version=%d;`, sqliteSchemaVersion)); err != nil {
		return fmt.Errorf("set sqlite user_version: %w", err)
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

func optionalInt64(raw sql.NullInt64) *time.Time {
	if !raw.Valid {
		return nil
	}
	ts := fromUnixNano(raw.Int64)
	if ts.IsZero() {
		return nil
	}
	return &ts
}

func encodeClientOverrides(value *hysteriadomain.ClientOverrides) (*string, error) {
	normalized := hysteriadomain.NormalizeClientOverrides(value)
	if normalized == nil {
		return nil, nil
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	encoded := string(data)
	return &encoded, nil
}

func decodeClientOverrides(value sql.NullString) (*hysteriadomain.ClientOverrides, error) {
	trimmed := strings.TrimSpace(value.String)
	if !value.Valid || trimmed == "" {
		return nil, nil
	}
	var out hysteriadomain.ClientOverrides
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, err
	}
	return hysteriadomain.NormalizeClientOverrides(&out), nil
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

func (r *SQLiteRepository) scanHysteriaUser(row scanner) (HysteriaUser, error) {
	var (
		out              HysteriaUser
		enabled          int64
		createdAt        int64
		updatedAt        int64
		lastSeenAt       sql.NullInt64
		note             sql.NullString
		clientOverrides  sql.NullString
	)
	if err := row.Scan(
		&out.ID,
		&out.Username,
		&out.UsernameNormalized,
		&out.Password,
		&enabled,
		&note,
		&clientOverrides,
		&createdAt,
		&updatedAt,
		&lastSeenAt,
	); err != nil {
		return HysteriaUser{}, translateSQLiteErr(err)
	}
	out.Enabled = boolFromSQLite(enabled)
	out.Note = optionalString(note)
	decoded, err := decodeClientOverrides(clientOverrides)
	if err != nil {
		return HysteriaUser{}, err
	}
	out.ClientOverrides = decoded
	out.CreatedAt = fromUnixNano(createdAt)
	out.UpdatedAt = fromUnixNano(updatedAt)
	out.LastSeenAt = optionalInt64(lastSeenAt)
	return out, nil
}

func (r *SQLiteRepository) scanHysteriaSnapshot(row scanner) (HysteriaSnapshot, error) {
	var (
		out        HysteriaSnapshot
		snapshotAt int64
	)
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.TxBytes,
		&out.RxBytes,
		&out.Online,
		&snapshotAt,
	); err != nil {
		return HysteriaSnapshot{}, translateSQLiteErr(err)
	}
	out.SnapshotAt = fromUnixNano(snapshotAt)
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

func (r *SQLiteRepository) scanAuditLog(row scanner) (AuditLog, error) {
	var (
		out       AuditLog
		adminID   sql.NullString
		entityID  sql.NullString
		adminMail sql.NullString
		createdAt int64
	)
	if err := row.Scan(
		&out.ID,
		&adminID,
		&out.Action,
		&out.EntityType,
		&entityID,
		&out.Payload,
		&createdAt,
		&adminMail,
	); err != nil {
		return AuditLog{}, translateSQLiteErr(err)
	}
	out.AdminID = optionalString(adminID)
	out.EntityID = optionalString(entityID)
	out.AdminEmail = optionalString(adminMail)
	out.CreatedAt = fromUnixNano(createdAt)
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
