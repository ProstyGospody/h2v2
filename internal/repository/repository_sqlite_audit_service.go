package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

func (r *SQLiteRepository) InsertAuditLog(ctx context.Context, adminID *string, action string, entityType string, entityID *string, payload any) error {
	payloadBytes := []byte("{}")
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		payloadBytes = encoded
	}
	var adminValue any
	if adminID != nil {
		trimmed := strings.TrimSpace(*adminID)
		if trimmed != "" {
			adminValue = trimmed
		}
	}
	var entityValue any
	if entityID != nil {
		trimmed := strings.TrimSpace(*entityID)
		if trimmed != "" {
			entityValue = trimmed
		}
	}
	_, err := r.db.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO audit_logs (admin_id, action, entity_type, entity_id, payload_json, created_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		adminValue,
		strings.TrimSpace(action),
		strings.TrimSpace(entityType),
		entityValue,
		string(payloadBytes),
		nowNano(),
	)
	return err
}

func (r *SQLiteRepository) ListAuditLogs(ctx context.Context, limit int, offset int) ([]AuditLog, error) {
	query := `
		SELECT
			l.id,
			l.admin_id,
			l.action,
			l.entity_type,
			l.entity_id,
			l.payload_json,
			l.created_at_ns,
			a.email
		FROM audit_logs l
		LEFT JOIN admins a ON a.id = l.admin_id
		ORDER BY l.created_at_ns DESC, l.id DESC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		if offset < 0 {
			offset = 0
		}
		args = append(args, limit, offset)
	}
	rows, err := r.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AuditLog, 0)
	for rows.Next() {
		item, err := r.scanAuditLog(rows)
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

func (r *SQLiteRepository) importAuditLogsTx(ctx context.Context, tx *sql.Tx, items []AuditLog) error {
	stmt := `INSERT INTO audit_logs (id, admin_id, action, entity_type, entity_id, payload_json, created_at_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			admin_id = excluded.admin_id,
			action = excluded.action,
			entity_type = excluded.entity_type,
			entity_id = excluded.entity_id,
			payload_json = excluded.payload_json,
			created_at_ns = excluded.created_at_ns`
	for _, item := range items {
		var adminValue any
		if item.AdminID != nil {
			trimmed := strings.TrimSpace(*item.AdminID)
			if trimmed != "" {
				adminValue = trimmed
			}
		}
		var entityValue any
		if item.EntityID != nil {
			trimmed := strings.TrimSpace(*item.EntityID)
			if trimmed != "" {
				entityValue = trimmed
			}
		}
		if _, err := tx.ExecContext(
			resolveCtx(ctx),
			stmt,
			item.ID,
			adminValue,
			item.Action,
			item.EntityType,
			entityValue,
			item.Payload,
			toUnixNano(item.CreatedAt),
		); err != nil {
			return err
		}
	}
	return nil
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

func (r *SQLiteRepository) listAuditLogs(ctx context.Context) ([]AuditLog, error) {
	return r.ListAuditLogs(ctx, 0, 0)
}
