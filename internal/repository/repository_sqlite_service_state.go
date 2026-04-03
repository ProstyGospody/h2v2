package repository

import (
	"context"
	"database/sql"
	"strings"
)

func (r *SQLiteRepository) UpsertServiceState(ctx context.Context, serviceName string, status string, version *string, rawJSON string) error {
	trimmedService := strings.TrimSpace(serviceName)
	trimmedStatus := strings.TrimSpace(status)
	trimmedRawJSON := strings.TrimSpace(rawJSON)
	var versionValue any
	if version != nil {
		trimmed := strings.TrimSpace(*version)
		if trimmed != "" {
			versionValue = trimmed
		}
	}
	var rawJSONValue any
	if trimmedRawJSON != "" {
		rawJSONValue = trimmedRawJSON
	}
	_, err := r.db.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO service_states (service_name, status, version, last_check_at_ns, raw_json)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(service_name) DO UPDATE SET
			status = excluded.status,
			version = excluded.version,
			last_check_at_ns = excluded.last_check_at_ns,
			raw_json = excluded.raw_json`,
		trimmedService,
		trimmedStatus,
		versionValue,
		nowNano(),
		rawJSONValue,
	)
	return err
}

func (r *SQLiteRepository) GetServiceState(ctx context.Context, serviceName string) (ServiceState, error) {
	row := r.db.QueryRowContext(
		resolveCtx(ctx),
		`SELECT id, service_name, status, version, last_check_at_ns, raw_json
		 FROM service_states
		 WHERE service_name = ?
		 LIMIT 1`,
		strings.TrimSpace(serviceName),
	)
	return r.scanServiceState(row)
}

func (r *SQLiteRepository) importServiceStatesTx(ctx context.Context, tx *sql.Tx, items []ServiceState) error {
	stmt := `INSERT INTO service_states (id, service_name, status, version, last_check_at_ns, raw_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(service_name) DO UPDATE SET
			id = excluded.id,
			status = excluded.status,
			version = excluded.version,
			last_check_at_ns = excluded.last_check_at_ns,
			raw_json = excluded.raw_json`
	for _, item := range items {
		var versionValue any
		if item.Version != nil {
			trimmed := strings.TrimSpace(*item.Version)
			if trimmed != "" {
				versionValue = trimmed
			}
		}
		var rawJSONValue any
		if item.RawJSON != nil {
			trimmed := strings.TrimSpace(*item.RawJSON)
			if trimmed != "" {
				rawJSONValue = trimmed
			}
		}
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.ID,
			item.ServiceName,
			item.Status,
			versionValue,
			toUnixNano(item.LastCheckAt),
			rawJSONValue,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) listServiceStates(ctx context.Context) ([]ServiceState, error) {
	rows, err := r.db.QueryContext(
		resolveCtx(ctx),
		`SELECT id, service_name, status, version, last_check_at_ns, raw_json
		 FROM service_states
		 ORDER BY service_name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ServiceState, 0)
	for rows.Next() {
		item, err := r.scanServiceState(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

