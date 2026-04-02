package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"
	_ "modernc.org/sqlite"

	hysteriadomain "h2v2/internal/domain/hysteria"
)

const sqliteSchemaVersion = 3

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

	version, err := r.sqliteUserVersion(ctx)
	if err != nil {
		return err
	}
	if version < 1 {
		if err := r.setSQLiteUserVersion(ctx, 1); err != nil {
			return err
		}
		version = 1
	}
	if version < 2 {
		if err := r.ensureUnifiedSchema(ctx); err != nil {
			return err
		}
		if err := r.backfillUnifiedSchema(ctx); err != nil {
			return err
		}
		if err := r.setSQLiteUserVersion(ctx, 2); err != nil {
			return err
		}
		version = 2
	}
	if version < 3 {
		if err := r.migrateVLESSRealityDefaults(ctx); err != nil {
			return err
		}
		if err := r.setSQLiteUserVersion(ctx, 3); err != nil {
			return err
		}
		version = 3
	}
	if version > sqliteSchemaVersion {
		return fmt.Errorf("sqlite schema version %d is newer than supported %d", version, sqliteSchemaVersion)
	}
	if version < sqliteSchemaVersion {
		if err := r.ensureUnifiedSchema(ctx); err != nil {
			return err
		}
		if err := r.setSQLiteUserVersion(ctx, sqliteSchemaVersion); err != nil {
			return err
		}
	}
	if err := r.ensureUnifiedSchema(ctx); err != nil {
		return err
	}
	if err := r.migrateVLESSRealityDefaults(ctx); err != nil {
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
			return fmt.Errorf("ensure sqlite base schema: %w", err)
		}
	}
	return nil
}

func (r *SQLiteRepository) ensureUnifiedSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			address TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			name_normalized TEXT NOT NULL UNIQUE,
			enabled INTEGER NOT NULL,
			traffic_limit_bytes INTEGER NOT NULL DEFAULT 0,
			traffic_used_tx_bytes INTEGER NOT NULL DEFAULT 0,
			traffic_used_rx_bytes INTEGER NOT NULL DEFAULT 0,
			expire_at_ns INTEGER,
			note TEXT,
			subject TEXT NOT NULL UNIQUE,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL,
			last_seen_at_ns INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at_ns DESC);`,
		`CREATE TABLE IF NOT EXISTS credentials (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			protocol TEXT NOT NULL,
			credential_type TEXT NOT NULL,
			identity TEXT NOT NULL,
			secret TEXT NOT NULL,
			data_json TEXT,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, protocol)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_credentials_protocol_identity ON credentials(protocol, identity);`,
		`CREATE TABLE IF NOT EXISTS inbounds (
			id TEXT PRIMARY KEY,
			node_id TEXT NOT NULL,
			name TEXT NOT NULL,
			protocol TEXT NOT NULL,
			transport TEXT NOT NULL,
			security TEXT NOT NULL,
			host TEXT NOT NULL,
			port INTEGER NOT NULL,
			enabled INTEGER NOT NULL,
			params_json TEXT,
			runtime_json TEXT,
			created_at_ns INTEGER NOT NULL,
			updated_at_ns INTEGER NOT NULL,
			FOREIGN KEY(node_id) REFERENCES nodes(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_inbounds_protocol ON inbounds(protocol);`,
		`CREATE TABLE IF NOT EXISTS subscription_tokens (
			user_id TEXT PRIMARY KEY,
			subject TEXT NOT NULL,
			version INTEGER NOT NULL,
			revoked INTEGER NOT NULL,
			rotated_at_ns INTEGER,
			updated_at_ns INTEGER NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS traffic_counters (
			id INTEGER PRIMARY KEY,
			user_id TEXT NOT NULL,
			protocol TEXT NOT NULL,
			tx_bytes INTEGER NOT NULL,
			rx_bytes INTEGER NOT NULL,
			online_count INTEGER NOT NULL,
			snapshot_at_ns INTEGER NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_traffic_counters_user_at ON traffic_counters(user_id, protocol, snapshot_at_ns DESC);`,
		`CREATE TABLE IF NOT EXISTS runtime_user_state (
			user_id TEXT NOT NULL,
			protocol TEXT NOT NULL,
			online_count INTEGER NOT NULL,
			last_sync_at_ns INTEGER NOT NULL,
			last_error TEXT,
			PRIMARY KEY(user_id, protocol),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TRIGGER IF NOT EXISTS trg_hy2_user_insert
		 AFTER INSERT ON hysteria_users
		 BEGIN
			 INSERT INTO users (
				 id, name, name_normalized, enabled, traffic_limit_bytes, traffic_used_tx_bytes, traffic_used_rx_bytes,
				 expire_at_ns, note, subject, created_at_ns, updated_at_ns, last_seen_at_ns
			 ) VALUES (
				 NEW.id, NEW.username, NEW.username_normalized, NEW.enabled, 0, 0, 0,
				 NULL, NEW.note, NEW.id, NEW.created_at_ns, NEW.updated_at_ns, NEW.last_seen_at_ns
			 )
			 ON CONFLICT(id) DO UPDATE SET
				 name = excluded.name,
				 name_normalized = excluded.name_normalized,
				 enabled = excluded.enabled,
				 note = excluded.note,
				 updated_at_ns = excluded.updated_at_ns,
				 last_seen_at_ns = excluded.last_seen_at_ns;
			 INSERT INTO credentials (
				 id, user_id, protocol, credential_type, identity, secret, data_json, created_at_ns, updated_at_ns
			 ) VALUES (
				 lower(hex(randomblob(16))), NEW.id, 'hy2', 'userpass', NEW.username, NEW.password, NEW.client_overrides_json, NEW.created_at_ns, NEW.updated_at_ns
			 )
			 ON CONFLICT(user_id, protocol) DO UPDATE SET
				 credential_type = excluded.credential_type,
				 identity = excluded.identity,
				 secret = excluded.secret,
				 data_json = excluded.data_json,
				 updated_at_ns = excluded.updated_at_ns;
			 INSERT INTO subscription_tokens (user_id, subject, version, revoked, rotated_at_ns, updated_at_ns)
			 VALUES (NEW.id, NEW.id, 1, 0, NULL, NEW.updated_at_ns)
			 ON CONFLICT(user_id) DO NOTHING;
		 END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_hy2_user_update
		 AFTER UPDATE ON hysteria_users
		 BEGIN
			 UPDATE users
			 SET
				 name = NEW.username,
				 name_normalized = NEW.username_normalized,
				 enabled = NEW.enabled,
				 note = NEW.note,
				 updated_at_ns = NEW.updated_at_ns,
				 last_seen_at_ns = NEW.last_seen_at_ns
			 WHERE id = NEW.id;
			 INSERT INTO credentials (
				 id, user_id, protocol, credential_type, identity, secret, data_json, created_at_ns, updated_at_ns
			 ) VALUES (
				 lower(hex(randomblob(16))), NEW.id, 'hy2', 'userpass', NEW.username, NEW.password, NEW.client_overrides_json, NEW.created_at_ns, NEW.updated_at_ns
			 )
			 ON CONFLICT(user_id, protocol) DO UPDATE SET
				 credential_type = excluded.credential_type,
				 identity = excluded.identity,
				 secret = excluded.secret,
				 data_json = excluded.data_json,
				 updated_at_ns = excluded.updated_at_ns;
			 UPDATE subscription_tokens
			 SET updated_at_ns = NEW.updated_at_ns
			 WHERE user_id = NEW.id;
		 END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_hy2_user_delete
		 AFTER DELETE ON hysteria_users
		 BEGIN
			 DELETE FROM users WHERE id = OLD.id;
		 END;`,
		`DROP TRIGGER IF EXISTS trg_hy2_snapshot_insert;`,
		`CREATE TRIGGER IF NOT EXISTS trg_hy2_snapshot_insert
		 AFTER INSERT ON hysteria_snapshots
		 BEGIN
			 INSERT INTO traffic_counters (user_id, protocol, tx_bytes, rx_bytes, online_count, snapshot_at_ns)
			 SELECT NEW.user_id, 'hy2', NEW.tx_bytes, NEW.rx_bytes, NEW.online_count, NEW.snapshot_at_ns
			 WHERE EXISTS (
				 SELECT 1 FROM users u
				 WHERE u.id = NEW.user_id
			 )
			   AND NOT EXISTS (
				 SELECT 1 FROM traffic_counters tc
				 WHERE tc.user_id = NEW.user_id
				   AND tc.protocol = 'hy2'
				   AND tc.snapshot_at_ns = NEW.snapshot_at_ns
			 );
		 END;`,
	}
	for _, stmt := range statements {
		if _, err := r.db.ExecContext(resolveCtx(ctx), stmt); err != nil {
			return fmt.Errorf("ensure sqlite unified schema: %w", err)
		}
	}
	return nil
}

func (r *SQLiteRepository) backfillUnifiedSchema(ctx context.Context) error {
	tx, err := r.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := nowNano()
	if _, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO nodes (id, name, address, enabled, created_at_ns, updated_at_ns)
		 VALUES ('local', 'local', '127.0.0.1', 1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			 name = excluded.name,
			 address = excluded.address,
			 enabled = excluded.enabled,
			 updated_at_ns = excluded.updated_at_ns`,
		now,
		now,
	); err != nil {
		return err
	}

	if _, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO users (
			id, name, name_normalized, enabled, traffic_limit_bytes, traffic_used_tx_bytes, traffic_used_rx_bytes,
			expire_at_ns, note, subject, created_at_ns, updated_at_ns, last_seen_at_ns
		)
		SELECT
			hu.id,
			hu.username,
			hu.username_normalized,
			hu.enabled,
			0,
			0,
			0,
			NULL,
			hu.note,
			hu.id,
			hu.created_at_ns,
			hu.updated_at_ns,
			hu.last_seen_at_ns
		FROM hysteria_users hu
		WHERE 1 = 1
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			name_normalized = excluded.name_normalized,
			enabled = excluded.enabled,
			note = excluded.note,
			updated_at_ns = excluded.updated_at_ns,
			last_seen_at_ns = excluded.last_seen_at_ns`,
	); err != nil {
		return err
	}

	if _, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO credentials (
			id, user_id, protocol, credential_type, identity, secret, data_json, created_at_ns, updated_at_ns
		)
		SELECT
			lower(hex(randomblob(16))),
			hu.id,
			'hy2',
			'userpass',
			hu.username,
			hu.password,
			hu.client_overrides_json,
			hu.created_at_ns,
			hu.updated_at_ns
		FROM hysteria_users hu
		WHERE EXISTS (
			SELECT 1 FROM users u
			WHERE u.id = hu.id
		)
		  AND NOT EXISTS (
			SELECT 1 FROM credentials c WHERE c.user_id = hu.id AND c.protocol = 'hy2'
		)`,
	); err != nil {
		return err
	}

	if _, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO subscription_tokens (user_id, subject, version, revoked, rotated_at_ns, updated_at_ns)
		SELECT
			u.id,
			u.subject,
			1,
			0,
			NULL,
			u.updated_at_ns
		FROM users u
		WHERE NOT EXISTS (
			SELECT 1 FROM subscription_tokens st WHERE st.user_id = u.id
		)`,
	); err != nil {
		return err
	}

	if _, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO traffic_counters (user_id, protocol, tx_bytes, rx_bytes, online_count, snapshot_at_ns)
		 SELECT hs.user_id, 'hy2', hs.tx_bytes, hs.rx_bytes, hs.online_count, hs.snapshot_at_ns
		 FROM hysteria_snapshots hs
		 WHERE EXISTS (
			 SELECT 1 FROM users u
			 WHERE u.id = hs.user_id
		 )
		   AND NOT EXISTS (
			 SELECT 1 FROM traffic_counters tc
			 WHERE tc.user_id = hs.user_id AND tc.protocol = 'hy2' AND tc.snapshot_at_ns = hs.snapshot_at_ns
		 )`,
	); err != nil {
		return err
	}

	if _, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO inbounds (
			id, node_id, name, protocol, transport, security, host, port, enabled, params_json, runtime_json, created_at_ns, updated_at_ns
		)
		SELECT 'hy2-default', 'local', 'HY2 Default', 'hy2', 'quic', 'tls', '127.0.0.1', 443, 1, '{}', '{}', ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM inbounds WHERE protocol = 'hy2')`,
		now,
		now,
	); err != nil {
		return err
	}

	if _, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO inbounds (
			id, node_id, name, protocol, transport, security, host, port, enabled, params_json, runtime_json, created_at_ns, updated_at_ns
		)
		SELECT
			'vless-default',
			'local',
			'VLESS Reality',
			'vless',
			'tcp',
			'reality',
			'127.0.0.1',
			443,
			1,
			'{"flow":"xtls-rprx-vision","pbk":"","sid":"","sni":"www.cloudflare.com","fp":"chrome","network":"tcp","security":"reality","dest":"www.cloudflare.com:443"}',
			'{}',
			?,
			?
		WHERE NOT EXISTS (SELECT 1 FROM inbounds WHERE protocol = 'vless')`,
		now,
		now,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *SQLiteRepository) migrateVLESSRealityDefaults(ctx context.Context) error {
	tx, err := r.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.QueryContext(
		resolveCtx(ctx),
		`SELECT id, params_json
		 FROM inbounds
		 WHERE protocol = 'vless' AND lower(security) = 'reality'`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type updateItem struct {
		ID         string
		ParamsJSON string
	}
	updates := make([]updateItem, 0)
	for rows.Next() {
		var (
			id        string
			paramsRaw sql.NullString
		)
		if err := rows.Scan(&id, &paramsRaw); err != nil {
			return err
		}
		params := make(map[string]any)
		if paramsRaw.Valid {
			trimmed := strings.TrimSpace(paramsRaw.String)
			if trimmed != "" {
				if err := json.Unmarshal([]byte(trimmed), &params); err != nil {
					params = make(map[string]any)
				}
			}
		}
		normalized, changed, normalizeErr := normalizeLegacyRealityParams(params)
		if normalizeErr != nil {
			return fmt.Errorf("normalize vless reality params for inbound %s: %w", id, normalizeErr)
		}
		if !changed {
			continue
		}
		encoded, encodeErr := json.Marshal(normalized)
		if encodeErr != nil {
			return encodeErr
		}
		updates = append(updates, updateItem{ID: id, ParamsJSON: string(encoded)})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range updates {
		if _, err = tx.ExecContext(
			resolveCtx(ctx),
			`UPDATE inbounds SET params_json = ?, updated_at_ns = ? WHERE id = ?`,
			item.ParamsJSON,
			nowNano(),
			item.ID,
		); err != nil {
			return err
		}
	}

	var enabledCount int
	if err = tx.QueryRowContext(
		resolveCtx(ctx),
		`SELECT COUNT(1) FROM inbounds WHERE protocol = 'vless' AND enabled = 1`,
	).Scan(&enabledCount); err != nil {
		return err
	}
	if enabledCount == 0 {
		if _, err = tx.ExecContext(
			resolveCtx(ctx),
			`UPDATE inbounds
			 SET enabled = 1, updated_at_ns = ?
			 WHERE id = (
			   SELECT id
			   FROM inbounds
			   WHERE protocol = 'vless'
			   ORDER BY CASE WHEN id = 'vless-default' THEN 0 ELSE 1 END, created_at_ns ASC, id ASC
			   LIMIT 1
			 )`,
			nowNano(),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func normalizeLegacyRealityParams(params map[string]any) (map[string]any, bool, error) {
	if params == nil {
		params = make(map[string]any)
	}
	changed := false
	readString := func(key string) string {
		value, ok := params[key]
		if !ok || value == nil {
			return ""
		}
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(typed)
		default:
			return strings.TrimSpace(fmt.Sprintf("%v", typed))
		}
	}
	setString := func(key string, value string) {
		trimmed := strings.TrimSpace(value)
		current := readString(key)
		if current != trimmed {
			changed = true
		}
		params[key] = trimmed
	}

	privateKeyRaw := readString("privateKey")
	privateKey := ""
	if privateKeyRaw != "" {
		decoded, err := decodeLegacyRealityKey(privateKeyRaw)
		if err == nil {
			privateKey = encodeLegacyRealityKey(decoded)
		}
	}
	publicKeyRaw := readString("pbk")
	if publicKeyRaw == "" {
		publicKeyRaw = readString("publicKey")
	}
	publicKey := ""
	if publicKeyRaw != "" {
		decoded, err := decodeLegacyRealityKey(publicKeyRaw)
		if err == nil {
			publicKey = encodeLegacyRealityKey(decoded)
		}
	}
	if privateKey != "" {
		derivedPublic, derivedErr := deriveLegacyRealityPublicKey(privateKey)
		if derivedErr != nil {
			return params, false, derivedErr
		}
		if publicKey != derivedPublic {
			publicKey = derivedPublic
			changed = true
		}
	}
	if privateKey == "" || publicKey == "" {
		var pairErr error
		privateKey, publicKey, pairErr = generateLegacyRealityKeyPair()
		if pairErr != nil {
			return params, false, pairErr
		}
		changed = true
	}
	setString("privateKey", privateKey)
	setString("pbk", publicKey)
	if _, exists := params["publicKey"]; exists {
		delete(params, "publicKey")
		changed = true
	}

	sidRaw := strings.ToLower(strings.TrimSpace(readString("sid")))
	sid := ""
	if sidRaw != "" {
		if len(sidRaw) <= 32 && len(sidRaw)%2 == 0 {
			if _, err := hex.DecodeString(sidRaw); err == nil {
				sid = sidRaw
			}
		}
	}
	if sid == "" {
		if _, exists := params["sid"]; exists {
			delete(params, "sid")
			changed = true
		}
	} else {
		setString("sid", sid)
	}

	if strings.TrimSpace(readString("security")) == "" {
		setString("security", "reality")
	}
	if strings.TrimSpace(readString("network")) == "" {
		setString("network", "tcp")
	}
	if strings.TrimSpace(readString("flow")) == "" {
		setString("flow", "xtls-rprx-vision")
	}
	if strings.TrimSpace(readString("fp")) == "" {
		setString("fp", "chrome")
	}
	return params, changed, nil
}

func decodeLegacyRealityKey(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("reality key is empty")
	}
	encodings := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	for _, enc := range encodings {
		decoded, err := enc.DecodeString(trimmed)
		if err != nil {
			continue
		}
		if len(decoded) == 32 {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("reality key must decode to 32 bytes")
}

func encodeLegacyRealityKey(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func deriveLegacyRealityPublicKey(privateKey string) (string, error) {
	decoded, err := decodeLegacyRealityKey(privateKey)
	if err != nil {
		return "", err
	}
	publicKey, err := curve25519.X25519(decoded, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return encodeLegacyRealityKey(publicKey), nil
}

func generateLegacyRealityKeyPair() (string, string, error) {
	privateKey := make([]byte, 32)
	if _, err := rand.Read(privateKey); err != nil {
		return "", "", err
	}
	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}
	return encodeLegacyRealityKey(privateKey), encodeLegacyRealityKey(publicKey), nil
}

func (r *SQLiteRepository) sqliteUserVersion(ctx context.Context) (int, error) {
	var version int
	if err := r.db.QueryRowContext(resolveCtx(ctx), `PRAGMA user_version;`).Scan(&version); err != nil {
		return 0, fmt.Errorf("get sqlite user_version: %w", err)
	}
	return version, nil
}

func (r *SQLiteRepository) setSQLiteUserVersion(ctx context.Context, version int) error {
	if _, err := r.db.ExecContext(resolveCtx(ctx), fmt.Sprintf(`PRAGMA user_version=%d;`, version)); err != nil {
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
