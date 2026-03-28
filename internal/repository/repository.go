package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"proxy-panel/internal/fsutil"
)

var (
	ErrNotFound        = errors.New("repository: not found")
	ErrUniqueViolation = errors.New("repository: unique violation")
)

const currentSchemaVersion = 3

type Repository struct {
	mu sync.Mutex

	rootDir              string
	stateDir             string
	adminsDir            string
	sessionsDir          string
	hysteriaUsersDir     string
	serviceStatesDir     string
	hysteriaSnapshotsDir string
	systemSnapshotsDir   string
	backupsDir           string
	auditDir             string
	runDir               string
	lockPath             string
	metaPath             string
}

type metaState struct {
	SchemaVersion     int       `json:"schema_version"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	NextAuditID       int64     `json:"next_audit_id"`
	NextHy2SnapshotID int64     `json:"next_hy2_snapshot_id"`
	NextServiceStateID int64    `json:"next_service_state_id"`
	NextSystemSnapshotID int64  `json:"next_system_snapshot_id"`
}

func New(storageRoot string, auditDir string, runDir string) (*Repository, error) {
	storageRoot = strings.TrimSpace(storageRoot)
	auditDir = strings.TrimSpace(auditDir)
	runDir = strings.TrimSpace(runDir)
	if storageRoot == "" {
		return nil, fmt.Errorf("storage root is required")
	}
	if auditDir == "" {
		return nil, fmt.Errorf("audit dir is required")
	}
	if runDir == "" {
		return nil, fmt.Errorf("runtime dir is required")
	}

	r := &Repository{
		rootDir:              storageRoot,
		stateDir:             filepath.Join(storageRoot, "state"),
		adminsDir:            filepath.Join(storageRoot, "state", "admins"),
		sessionsDir:          filepath.Join(storageRoot, "state", "sessions"),
		hysteriaUsersDir:     filepath.Join(storageRoot, "state", "hysteria-users"),
		serviceStatesDir:     filepath.Join(storageRoot, "state", "service-states"),
		hysteriaSnapshotsDir: filepath.Join(storageRoot, "snapshots", "hy2"),
		systemSnapshotsDir:   filepath.Join(storageRoot, "snapshots", "system"),
		backupsDir:           filepath.Join(storageRoot, "backups"),
		auditDir:             auditDir,
		runDir:               runDir,
		lockPath:             filepath.Join(runDir, "locks", "repository.lock"),
		metaPath:             filepath.Join(storageRoot, "state", "meta.json"),
	}
	if err := r.ensureLayout(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Repository) Close() error {
	return nil
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.withLock(ctx, func() error {
		_, err := r.loadMetaNoLock()
		return err
	})
}

func (r *Repository) withLock(ctx context.Context, fn func() error) error {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := fsutil.LockFile(r.lockPath)
	if err != nil {
		return err
	}
	defer unlock()

	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return fn()
}

func (r *Repository) ensureLayout() error {
	for _, dir := range []string{
		r.rootDir,
		r.stateDir,
		r.adminsDir,
		r.sessionsDir,
		r.hysteriaUsersDir,
		r.serviceStatesDir,
		r.hysteriaSnapshotsDir,
		r.systemSnapshotsDir,
		r.backupsDir,
		r.auditDir,
		filepath.Join(r.runDir, "locks"),
		filepath.Join(r.runDir, "tmp"),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create repository directory %s: %w", dir, err)
		}
	}
	_, err := r.loadMetaNoLock()
	return err
}

func (r *Repository) loadMetaNoLock() (metaState, error) {
	var meta metaState
	if err := readJSONFile(r.metaPath, &meta); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return meta, err
		}
		now := time.Now().UTC()
		meta = metaState{
			SchemaVersion: currentSchemaVersion,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := r.saveMetaNoLock(meta); err != nil {
			return meta, err
		}
		return meta, nil
	}
	if meta.SchemaVersion == 0 {
		meta.SchemaVersion = currentSchemaVersion
	}
	if meta.SchemaVersion > currentSchemaVersion {
		return meta, fmt.Errorf("unsupported schema version %d", meta.SchemaVersion)
	}
	return meta, nil
}

func (r *Repository) saveMetaNoLock(meta metaState) error {
	meta.SchemaVersion = currentSchemaVersion
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now().UTC()
	}
	meta.UpdatedAt = time.Now().UTC()
	return writeJSONFile(r.metaPath, 0o600, meta)
}

func loadEntity[T any](path string) (T, error) {
	var out T
	if err := readJSONFile(path, &out); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return out, ErrNotFound
		}
		return out, err
	}
	return out, nil
}

func loadEntities[T any](dir string) ([]T, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []T{}, nil
		}
		return nil, err
	}
	out := make([]T, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		item, err := loadEntity[T](filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func writeJSONFile(path string, perm fs.FileMode, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fsutil.WriteFileAtomic(path, data, perm)
}

func paginate[T any](items []T, limit int, offset int) []T {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = len(items)
	}
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]T(nil), items[offset:end]...)
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func adminPath(dir string, id string) string { return filepath.Join(dir, strings.TrimSpace(id)+".json") }
func sessionPath(dir string, id string) string { return filepath.Join(dir, strings.TrimSpace(id)+".json") }
func hysteriaUserPath(dir string, id string) string { return filepath.Join(dir, strings.TrimSpace(id)+".json") }

func serviceStatePath(dir string, serviceName string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(strings.TrimSpace(serviceName))
	return filepath.Join(dir, safe+".json")
}

func numericJSONFile(id int64) string {
	return fmt.Sprintf("%020d.json", id)
}

func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
func IsUniqueViolation(err error) bool { return errors.Is(err, ErrUniqueViolation) }
