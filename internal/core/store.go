package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrNotFound        = errors.New("core: not found")
	ErrConflict        = errors.New("core: conflict")
	ErrInvalidToken    = errors.New("core: invalid subscription token")
	ErrTokenRevoked    = errors.New("core: subscription token revoked")
	ErrRateLimited     = errors.New("core: rate limited")
)

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}

type Store struct {
	db   *sql.DB
	path string
}

func NewStore(dbPath string) (*Store, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, fmt.Errorf("core sqlite path is required")
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
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	store := &Store{db: db, path: dbPath}
	if err := store.applyPragmas(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(resolveCtx(ctx))
}

func (s *Store) applyPragmas(ctx context.Context) error {
	for _, stmt := range []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA foreign_keys=ON;`,
		`PRAGMA busy_timeout=5000;`,
	} {
		if _, err := s.db.ExecContext(resolveCtx(ctx), stmt); err != nil {
			return fmt.Errorf("apply sqlite pragma (%s): %w", strings.TrimSpace(stmt), err)
		}
	}
	return nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
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
		if _, err := s.db.ExecContext(resolveCtx(ctx), stmt); err != nil {
			return fmt.Errorf("ensure core schema: %w", err)
		}
	}
	_, err := s.db.ExecContext(resolveCtx(ctx), `INSERT OR IGNORE INTO core_schema_migrations(version, applied_at_ns) VALUES (1, ?);`, nowNano())
	if err != nil {
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

func fromUnixNano(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.Unix(0, value).UTC()
}

func boolToInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func intToBool(value int64) bool {
	return value != 0
}

func normalizeString(value string) string {
	return strings.TrimSpace(value)
}

func optionalString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalTime(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	ts := fromUnixNano(value.Int64)
	if ts.IsZero() {
		return nil
	}
	return &ts
}

func nullIfEmpty(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func parseUnique(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint failed") || strings.Contains(message, "constraint failed") {
		return ErrConflict
	}
	return err
}

func generateID() string {
	return uuid.NewString()
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		size = 32
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "sub_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func randomHex(size int) (string, error) {
	if size <= 0 {
		size = 16
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func tokenPrefix(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) <= 12 {
		return trimmed
	}
	return trimmed[:12]
}

func tokenHash(salt string, token string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(salt) + "|" + strings.TrimSpace(token)))
	return hex.EncodeToString(h[:])
}

func hashEqual(expected string, got string) bool {
	expectedBytes, err1 := hex.DecodeString(strings.TrimSpace(expected))
	gotBytes, err2 := hex.DecodeString(strings.TrimSpace(got))
	if err1 != nil || err2 != nil {
		return false
	}
	if len(expectedBytes) != len(gotBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(expectedBytes, gotBytes) == 1
}

func splitCSV(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func joinCSV(values []string) string {
	if len(values) == 0 {
		return ""
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return strings.Join(normalized, ",")
}
