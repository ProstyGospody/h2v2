package repository

import (
	"context"
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
