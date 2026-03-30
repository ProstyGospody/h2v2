package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type EntityCounts struct {
	Admins           int `json:"admins"`
	Sessions         int `json:"sessions"`
	HysteriaUsers    int `json:"hysteria_users"`
	HysteriaSnapshots int `json:"hysteria_snapshots"`
	SystemSnapshots  int `json:"system_snapshots"`
	AuditLogs        int `json:"audit_logs"`
	ServiceStates    int `json:"service_states"`
}

type MigrationReport struct {
	Driver     string       `json:"driver"`
	SQLitePath string       `json:"sqlite_path"`
	Source     EntityCounts `json:"source_counts"`
	Target     EntityCounts `json:"target_counts"`
}

type ExportPayload struct {
	Driver           string            `json:"driver"`
	SQLitePath       string            `json:"sqlite_path"`
	ExportedAt       time.Time         `json:"exported_at"`
	Counts           EntityCounts      `json:"counts"`
	Admins           []Admin           `json:"admins"`
	Sessions         []Session         `json:"sessions"`
	HysteriaUsers    []HysteriaUser    `json:"hysteria_users"`
	HysteriaSnapshots []HysteriaSnapshot `json:"hysteria_snapshots"`
	SystemSnapshots  []SystemSnapshot  `json:"system_snapshots"`
	AuditLogs        []AuditLog        `json:"audit_logs"`
	ServiceStates    []ServiceState    `json:"service_states"`
}

func MigrateFileToSQLite(ctx context.Context, storageRoot string, auditDir string, runDir string, sqlitePath string) (MigrationReport, error) {
	fileRepo, err := NewFileRepository(storageRoot, auditDir, runDir)
	if err != nil {
		return MigrationReport{}, err
	}
	defer fileRepo.Close()

	sqliteRepo, err := NewSQLiteRepository(sqlitePath)
	if err != nil {
		return MigrationReport{}, err
	}
	defer sqliteRepo.Close()

	snapshot, err := captureFileSnapshot(ctx, fileRepo)
	if err != nil {
		return MigrationReport{}, err
	}

	if err := sqliteRepo.migrateTable(ctx, "admins", func(tx *sql.Tx) error {
		return sqliteRepo.importAdminsTx(ctx, tx, snapshot.admins)
	}); err != nil {
		return MigrationReport{}, err
	}
	if err := sqliteRepo.migrateTable(ctx, "hysteria_users", func(tx *sql.Tx) error {
		return sqliteRepo.importHysteriaUsersTx(ctx, tx, snapshot.hysteriaUsers)
	}); err != nil {
		return MigrationReport{}, err
	}
	if err := sqliteRepo.migrateTable(ctx, "sessions", func(tx *sql.Tx) error {
		return sqliteRepo.importSessionsTx(ctx, tx, snapshot.sessions)
	}); err != nil {
		return MigrationReport{}, err
	}
	if err := sqliteRepo.migrateTable(ctx, "hysteria_snapshots", func(tx *sql.Tx) error {
		return sqliteRepo.importHysteriaSnapshotsTx(ctx, tx, snapshot.hysteriaSnapshots)
	}); err != nil {
		return MigrationReport{}, err
	}
	if err := sqliteRepo.migrateTable(ctx, "system_snapshots", func(tx *sql.Tx) error {
		return sqliteRepo.importSystemSnapshotsTx(ctx, tx, snapshot.systemSnapshots)
	}); err != nil {
		return MigrationReport{}, err
	}
	if err := sqliteRepo.migrateTable(ctx, "audit_logs", func(tx *sql.Tx) error {
		return sqliteRepo.importAuditLogsTx(ctx, tx, snapshot.auditLogs)
	}); err != nil {
		return MigrationReport{}, err
	}
	if err := sqliteRepo.migrateTable(ctx, "service_states", func(tx *sql.Tx) error {
		return sqliteRepo.importServiceStatesTx(ctx, tx, snapshot.serviceStates)
	}); err != nil {
		return MigrationReport{}, err
	}

	sourceCounts := snapshot.counts()
	targetCounts, err := sqliteRepo.countEntities(ctx)
	if err != nil {
		return MigrationReport{}, err
	}
	if sourceCounts != targetCounts {
		return MigrationReport{}, fmt.Errorf("migration validation failed: source=%+v target=%+v", sourceCounts, targetCounts)
	}

	return MigrationReport{
		Driver:     StorageDriverSQLite,
		SQLitePath: sqlitePath,
		Source:     sourceCounts,
		Target:     targetCounts,
	}, nil
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
	hysteriaUsers, err := r.listHysteriaUsers(ctx)
	if err != nil {
		return ExportPayload{}, err
	}
	hysteriaSnapshots, err := r.sortedHysteriaSnapshots(ctx)
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

	counts := EntityCounts{
		Admins:            len(admins),
		Sessions:          len(sessions),
		HysteriaUsers:     len(hysteriaUsers),
		HysteriaSnapshots: len(hysteriaSnapshots),
		SystemSnapshots:   len(systemSnapshots),
		AuditLogs:         len(auditLogs),
		ServiceStates:     len(serviceStates),
	}
	return ExportPayload{
		Driver:            StorageDriverSQLite,
		SQLitePath:        r.path,
		ExportedAt:        time.Now().UTC(),
		Counts:            counts,
		Admins:            admins,
		Sessions:          sessions,
		HysteriaUsers:     hysteriaUsers,
		HysteriaSnapshots: hysteriaSnapshots,
		SystemSnapshots:   systemSnapshots,
		AuditLogs:         auditLogs,
		ServiceStates:     serviceStates,
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
		`DELETE FROM hysteria_snapshots`,
		`DELETE FROM system_snapshots`,
		`DELETE FROM audit_logs`,
		`DELETE FROM service_states`,
		`DELETE FROM hysteria_users`,
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
	if err := r.importHysteriaUsersTx(ctx, tx, payload.HysteriaUsers); err != nil {
		return err
	}
	if err := r.importSessionsTx(ctx, tx, payload.Sessions); err != nil {
		return err
	}
	if err := r.importHysteriaSnapshotsTx(ctx, tx, payload.HysteriaSnapshots); err != nil {
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

func (r *SQLiteRepository) migrateTable(ctx context.Context, table string, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return fmt.Errorf("%s: begin transaction: %w", table, err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("%s: %w", table, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: commit transaction: %w", table, err)
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
	hUsers, err := getCount("hysteria_users")
	if err != nil {
		return EntityCounts{}, err
	}
	hSnapshots, err := getCount("hysteria_snapshots")
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
	return EntityCounts{
		Admins:            admins,
		Sessions:          sessions,
		HysteriaUsers:     hUsers,
		HysteriaSnapshots: hSnapshots,
		SystemSnapshots:   systemSnapshots,
		AuditLogs:         auditLogs,
		ServiceStates:     serviceStates,
	}, nil
}

func (r *SQLiteRepository) countEntities(ctx context.Context) (EntityCounts, error) {
	getCount := func(table string) (int, error) {
		var count int
		query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)
		if err := r.db.QueryRowContext(resolveCtx(ctx), query).Scan(&count); err != nil {
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
	hUsers, err := getCount("hysteria_users")
	if err != nil {
		return EntityCounts{}, err
	}
	hSnapshots, err := getCount("hysteria_snapshots")
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
	return EntityCounts{
		Admins:            admins,
		Sessions:          sessions,
		HysteriaUsers:     hUsers,
		HysteriaSnapshots: hSnapshots,
		SystemSnapshots:   systemSnapshots,
		AuditLogs:         auditLogs,
		ServiceStates:     serviceStates,
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

func (r *SQLiteRepository) listHysteriaUsers(ctx context.Context) ([]HysteriaUser, error) {
	rows, err := r.db.QueryContext(
		resolveCtx(ctx),
		`SELECT
			id, username, username_normalized, password, enabled, note, client_overrides_json, created_at_ns, updated_at_ns, last_seen_at_ns
		 FROM hysteria_users
		 ORDER BY created_at_ns ASC, id ASC`,
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

type fileSnapshot struct {
	admins            []Admin
	sessions          []Session
	hysteriaUsers     []HysteriaUser
	hysteriaSnapshots []HysteriaSnapshot
	systemSnapshots   []SystemSnapshot
	auditLogs         []AuditLog
	serviceStates     []ServiceState
}

func (s fileSnapshot) counts() EntityCounts {
	return EntityCounts{
		Admins:            len(s.admins),
		Sessions:          len(s.sessions),
		HysteriaUsers:     len(s.hysteriaUsers),
		HysteriaSnapshots: len(s.hysteriaSnapshots),
		SystemSnapshots:   len(s.systemSnapshots),
		AuditLogs:         len(s.auditLogs),
		ServiceStates:     len(s.serviceStates),
	}
}

func captureFileSnapshot(ctx context.Context, repo *FileRepository) (fileSnapshot, error) {
	out := fileSnapshot{}
	err := repo.withLock(ctx, func() error {
		var err error
		out.admins, err = repo.loadAdminsNoLock()
		if err != nil {
			return err
		}
		out.sessions, err = repo.loadSessionsNoLock()
		if err != nil {
			return err
		}
		out.hysteriaUsers, err = repo.loadHysteriaUsersNoLock()
		if err != nil {
			return err
		}
		out.hysteriaSnapshots, err = repo.loadHysteriaSnapshotsNoLock("")
		if err != nil {
			return err
		}
		out.systemSnapshots, err = loadEntities[SystemSnapshot](repo.systemSnapshotsDir)
		if err != nil {
			return err
		}
		out.auditLogs, err = repo.loadAuditLogsNoLock()
		if err != nil {
			return err
		}
		out.serviceStates, err = repo.loadServiceStatesNoLock()
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fileSnapshot{}, err
	}
	sort.Slice(out.hysteriaSnapshots, func(i, j int) bool { return out.hysteriaSnapshots[i].ID < out.hysteriaSnapshots[j].ID })
	sort.Slice(out.systemSnapshots, func(i, j int) bool { return out.systemSnapshots[i].ID < out.systemSnapshots[j].ID })
	sort.Slice(out.auditLogs, func(i, j int) bool { return out.auditLogs[i].ID < out.auditLogs[j].ID })
	sort.Slice(out.serviceStates, func(i, j int) bool {
		if out.serviceStates[i].ServiceName == out.serviceStates[j].ServiceName {
			return out.serviceStates[i].ID < out.serviceStates[j].ID
		}
		return out.serviceStates[i].ServiceName < out.serviceStates[j].ServiceName
	})
	return out, nil
}
