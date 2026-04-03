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
)

type EntityCounts struct {
	Admins                       int `json:"admins"`
	Sessions                     int `json:"sessions"`
	ServiceStates                int `json:"service_states"`
	SystemSnapshots              int `json:"system_snapshots"`
	CoreSchemaMigrations         int `json:"core_schema_migrations"`
	CoreServers                  int `json:"core_servers"`
	CoreInbounds                 int `json:"core_inbounds"`
	CoreInboundVLESSSettings     int `json:"core_inbound_vless_settings"`
	CoreInboundHysteria2Settings int `json:"core_inbound_hysteria2_settings"`
	CoreUsers                    int `json:"core_users"`
	CoreSubscriptions            int `json:"core_subscriptions"`
	CoreUserAccess               int `json:"core_user_access"`
	CoreSubscriptionTokens       int `json:"core_subscription_tokens"`
	CoreConfigRevisions          int `json:"core_config_revisions"`
}

type ExportPayload struct {
	Driver     string                       `json:"driver"`
	SQLitePath string                       `json:"sqlite_path"`
	ExportedAt time.Time                    `json:"exported_at"`
	Counts     EntityCounts                 `json:"counts"`
	Tables     map[string][]map[string]any `json:"tables"`
}

type tableSpec struct {
	name    string
	orderBy string
}

var managedTableSpecs = []tableSpec{
	{name: "admins", orderBy: "created_at_ns ASC, id ASC"},
	{name: "sessions", orderBy: "created_at_ns ASC, id ASC"},
	{name: "service_states", orderBy: "service_name ASC"},
	{name: "system_snapshots", orderBy: "id ASC"},
	{name: "core_schema_migrations", orderBy: "version ASC"},
	{name: "core_servers", orderBy: "created_at_ns ASC, id ASC"},
	{name: "core_inbounds", orderBy: "created_at_ns ASC, id ASC"},
	{name: "core_inbound_vless_settings", orderBy: "inbound_id ASC"},
	{name: "core_inbound_hysteria2_settings", orderBy: "inbound_id ASC"},
	{name: "core_users", orderBy: "created_at_ns ASC, id ASC"},
	{name: "core_subscriptions", orderBy: "created_at_ns ASC, id ASC"},
	{name: "core_user_access", orderBy: "created_at_ns ASC, id ASC"},
	{name: "core_subscription_tokens", orderBy: "created_at_ns ASC, id ASC"},
	{name: "core_config_revisions", orderBy: "created_at_ns ASC, id ASC"},
}

var managedDeleteOrder = []string{
	"sessions",
	"core_config_revisions",
	"core_subscription_tokens",
	"core_user_access",
	"core_subscriptions",
	"core_inbound_hysteria2_settings",
	"core_inbound_vless_settings",
	"core_inbounds",
	"core_users",
	"core_servers",
	"core_schema_migrations",
	"system_snapshots",
	"service_states",
	"admins",
}

var managedInsertOrder = []string{
	"admins",
	"sessions",
	"service_states",
	"system_snapshots",
	"core_schema_migrations",
	"core_servers",
	"core_inbounds",
	"core_inbound_vless_settings",
	"core_inbound_hysteria2_settings",
	"core_users",
	"core_subscriptions",
	"core_user_access",
	"core_subscription_tokens",
	"core_config_revisions",
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
	payload := ExportPayload{
		Driver:     StorageDriverSQLite,
		SQLitePath: r.path,
		ExportedAt: time.Now().UTC(),
		Counts:     EntityCounts{},
		Tables:     make(map[string][]map[string]any, len(managedTableSpecs)),
	}

	for _, spec := range managedTableSpecs {
		exists, err := tableExistsOnDB(resolveCtx(ctx), r.db, "main", spec.name)
		if err != nil {
			return ExportPayload{}, err
		}
		if !exists {
			continue
		}
		rows, err := exportTableRows(resolveCtx(ctx), r.db, "main", spec.name, spec.orderBy)
		if err != nil {
			return ExportPayload{}, err
		}
		payload.Tables[spec.name] = rows
		setEntityCount(&payload.Counts, spec.name, len(rows))
	}

	return payload, nil
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
	ctx = resolveCtx(ctx)
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

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return EntityCounts{}, fmt.Errorf("restore begin transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
		_, _ = r.db.ExecContext(ctx, `DETACH DATABASE src`)
	}()

	if _, err := tx.ExecContext(ctx, `ATTACH DATABASE ? AS src`, absFrom); err != nil {
		return EntityCounts{}, fmt.Errorf("attach source database: %w", err)
	}

	hasAdmins, err := tableExistsOnTx(ctx, tx, "src", "admins")
	if err != nil {
		return EntityCounts{}, err
	}
	if hasAdmins {
		count, err := countTableOnTx(ctx, tx, "src", "admins")
		if err != nil {
			return EntityCounts{}, err
		}
		if count == 0 {
			return EntityCounts{}, fmt.Errorf("backup validation failed: admins must be greater than zero")
		}
	}

	if err := restoreManagedTablesTx(ctx, tx); err != nil {
		return EntityCounts{}, err
	}
	counts, err := countEntitiesTx(ctx, tx)
	if err != nil {
		return EntityCounts{}, err
	}

	if err := tx.Commit(); err != nil {
		return EntityCounts{}, fmt.Errorf("restore commit transaction: %w", err)
	}
	tx = nil
	return counts, nil
}

func restoreManagedTablesTx(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys=OFF;`); err != nil {
		return err
	}
	defer func() {
		_, _ = tx.ExecContext(ctx, `PRAGMA foreign_keys=ON;`)
	}()

	for _, table := range managedDeleteOrder {
		existsMain, err := tableExistsOnTx(ctx, tx, "main", table)
		if err != nil {
			return err
		}
		if !existsMain {
			continue
		}
		stmt := fmt.Sprintf(`DELETE FROM main.%s`, quoteIdentifier(table))
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	for _, table := range managedInsertOrder {
		existsMain, err := tableExistsOnTx(ctx, tx, "main", table)
		if err != nil {
			return err
		}
		existsSrc, err := tableExistsOnTx(ctx, tx, "src", table)
		if err != nil {
			return err
		}
		if !existsMain || !existsSrc {
			continue
		}
		columns, err := commonColumnsTx(ctx, tx, table)
		if err != nil {
			return err
		}
		if len(columns) == 0 {
			continue
		}
		columnList := joinIdentifiers(columns)
		stmt := fmt.Sprintf(
			`INSERT INTO main.%s (%s) SELECT %s FROM src.%s`,
			quoteIdentifier(table),
			columnList,
			columnList,
			quoteIdentifier(table),
		)
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

func countEntitiesTx(ctx context.Context, tx *sql.Tx) (EntityCounts, error) {
	counts := EntityCounts{}
	for _, spec := range managedTableSpecs {
		exists, err := tableExistsOnTx(ctx, tx, "main", spec.name)
		if err != nil {
			return EntityCounts{}, err
		}
		if !exists {
			continue
		}
		count, err := countTableOnTx(ctx, tx, "main", spec.name)
		if err != nil {
			return EntityCounts{}, err
		}
		setEntityCount(&counts, spec.name, count)
	}
	return counts, nil
}

func exportTableRows(ctx context.Context, db *sql.DB, schema string, table string, orderBy string) ([]map[string]any, error) {
	query := fmt.Sprintf(`SELECT * FROM %s.%s`, quoteIdentifier(schema), quoteIdentifier(table))
	if strings.TrimSpace(orderBy) != "" {
		query += " ORDER BY " + orderBy
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		item := make(map[string]any, len(columns))
		for i, name := range columns {
			item[name] = normalizeSQLValue(values[i])
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func tableExistsOnDB(ctx context.Context, db *sql.DB, schema string, table string) (bool, error) {
	query := fmt.Sprintf(`SELECT 1 FROM %s.sqlite_master WHERE type='table' AND name=? LIMIT 1`, schemaName(schema))
	var exists int
	err := db.QueryRowContext(ctx, query, strings.TrimSpace(table)).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func tableExistsOnTx(ctx context.Context, tx *sql.Tx, schema string, table string) (bool, error) {
	query := fmt.Sprintf(`SELECT 1 FROM %s.sqlite_master WHERE type='table' AND name=? LIMIT 1`, schemaName(schema))
	var exists int
	err := tx.QueryRowContext(ctx, query, strings.TrimSpace(table)).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func countTableOnTx(ctx context.Context, tx *sql.Tx, schema string, table string) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s`, schemaName(schema), quoteIdentifier(table))
	var count int
	if err := tx.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func tableColumnsTx(ctx context.Context, tx *sql.Tx, schema string, table string) ([]string, error) {
	query := fmt.Sprintf(`PRAGMA %s.table_info(%s);`, schemaName(schema), quoteIdentifier(table))
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make([]string, 0)
	for rows.Next() {
		var (
			cid        int
			name       string
			typeName   string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultVal, &pk); err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		columns = append(columns, trimmed)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}

func commonColumnsTx(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	mainColumns, err := tableColumnsTx(ctx, tx, "main", table)
	if err != nil {
		return nil, err
	}
	srcColumns, err := tableColumnsTx(ctx, tx, "src", table)
	if err != nil {
		return nil, err
	}
	if len(mainColumns) == 0 || len(srcColumns) == 0 {
		return nil, nil
	}
	srcSet := make(map[string]struct{}, len(srcColumns))
	for _, item := range srcColumns {
		srcSet[item] = struct{}{}
	}
	shared := make([]string, 0, len(mainColumns))
	for _, item := range mainColumns {
		if _, ok := srcSet[item]; !ok {
			continue
		}
		shared = append(shared, item)
	}
	return shared, nil
}

func normalizeSQLValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(typed)
	default:
		return typed
	}
}

func setEntityCount(counts *EntityCounts, table string, value int) {
	switch table {
	case "admins":
		counts.Admins = value
	case "sessions":
		counts.Sessions = value
	case "service_states":
		counts.ServiceStates = value
	case "system_snapshots":
		counts.SystemSnapshots = value
	case "core_schema_migrations":
		counts.CoreSchemaMigrations = value
	case "core_servers":
		counts.CoreServers = value
	case "core_inbounds":
		counts.CoreInbounds = value
	case "core_inbound_vless_settings":
		counts.CoreInboundVLESSSettings = value
	case "core_inbound_hysteria2_settings":
		counts.CoreInboundHysteria2Settings = value
	case "core_users":
		counts.CoreUsers = value
	case "core_subscriptions":
		counts.CoreSubscriptions = value
	case "core_user_access":
		counts.CoreUserAccess = value
	case "core_subscription_tokens":
		counts.CoreSubscriptionTokens = value
	case "core_config_revisions":
		counts.CoreConfigRevisions = value
	}
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

func quoteIdentifier(value string) string {
	trimmed := strings.TrimSpace(value)
	return `"` + strings.ReplaceAll(trimmed, `"`, `""`) + `"`
}

func schemaName(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "src" {
		return "src"
	}
	return "main"
}

func joinIdentifiers(values []string) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		parts = append(parts, quoteIdentifier(trimmed))
	}
	return strings.Join(parts, ", ")
}
