package repository

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (r *FileRepository) InsertAuditLog(ctx context.Context, adminID *string, action string, entityType string, entityID *string, payload any) error {
	return r.withLock(ctx, func() error {
		meta, err := r.loadMetaNoLock()
		if err != nil {
			return err
		}
		meta.NextAuditID++
		entry := AuditLog{ID: meta.NextAuditID, AdminID: cleanOptional(adminID), Action: strings.TrimSpace(action), EntityType: strings.TrimSpace(entityType), EntityID: cleanOptional(entityID), CreatedAt: time.Now().UTC()}
		payloadBytes := []byte("{}")
		if payload != nil {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			payloadBytes = encoded
		}
		entry.Payload = string(payloadBytes)
		if err := r.saveMetaNoLock(meta); err != nil {
			return err
		}
		return r.writeAuditLogNoLock(entry)
	})
}

func (r *FileRepository) ListAuditLogs(ctx context.Context, limit int, offset int) ([]AuditLog, error) {
	var out []AuditLog
	err := r.withLock(ctx, func() error {
		items, err := r.loadAuditLogsNoLock()
		if err != nil {
			return err
		}
		sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
		admins, err := r.loadAdminsNoLock()
		if err != nil {
			return err
		}
		adminMap := make(map[string]string, len(admins))
		for _, admin := range admins {
			adminMap[admin.ID] = admin.Email
		}
		for idx := range items {
			if items[idx].AdminID != nil {
				if email, ok := adminMap[*items[idx].AdminID]; ok {
					items[idx].AdminEmail = &email
				}
			}
		}
		out = paginate(items, limit, offset)
		return nil
	})
	return out, err
}

func (r *FileRepository) UpsertServiceState(ctx context.Context, serviceName string, status string, version *string, rawJSON string) error {
	return r.withLock(ctx, func() error {
		serviceName = strings.TrimSpace(serviceName)
		existing, err := r.loadServiceStateNoLock(serviceName)
		if err != nil && !IsNotFound(err) {
			return err
		}
		state := ServiceState{ServiceName: serviceName, Status: strings.TrimSpace(status), Version: cleanOptional(version), LastCheckAt: time.Now().UTC()}
		if trimmed := strings.TrimSpace(rawJSON); trimmed != "" {
			state.RawJSON = &trimmed
		}
		if err == nil {
			state.ID = existing.ID
		} else {
			meta, err := r.loadMetaNoLock()
			if err != nil {
				return err
			}
			meta.NextServiceStateID++
			state.ID = meta.NextServiceStateID
			if err := r.saveMetaNoLock(meta); err != nil {
				return err
			}
		}
		return r.writeServiceStateNoLock(state)
	})
}

func (r *FileRepository) GetServiceState(ctx context.Context, serviceName string) (ServiceState, error) {
	var out ServiceState
	err := r.withLock(ctx, func() error {
		state, err := r.loadServiceStateNoLock(serviceName)
		if err != nil {
			return err
		}
		out = state
		return nil
	})
	return out, err
}

func (r *FileRepository) loadAuditLogsNoLock() ([]AuditLog, error) { return loadEntities[AuditLog](r.auditDir) }
func (r *FileRepository) writeAuditLogNoLock(entry AuditLog) error { return writeJSONFile(filepath.Join(r.auditDir, numericJSONFile(entry.ID)), 0o640, entry) }
func (r *FileRepository) loadServiceStatesNoLock() ([]ServiceState, error) { return loadEntities[ServiceState](r.serviceStatesDir) }
func (r *FileRepository) loadServiceStateNoLock(serviceName string) (ServiceState, error) { return loadEntity[ServiceState](serviceStatePath(r.serviceStatesDir, serviceName)) }
func (r *FileRepository) writeServiceStateNoLock(state ServiceState) error { return writeJSONFile(serviceStatePath(r.serviceStatesDir, state.ServiceName), 0o600, state) }
