package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"h2v2/internal/http/render"
	"h2v2/internal/repository"
)

const sqliteRestoreUploadLimitBytes int64 = 512 << 20

func (h *Handler) DownloadSQLiteBackup(w http.ResponseWriter, r *http.Request) {
	repo, err := h.requireSQLiteRepository()
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}

	fileName := fmt.Sprintf("panel-%s.db", time.Now().UTC().Format("20060102-150405"))
	outPath := filepath.Join(h.cfg.StorageRoot, "backups", fileName)
	if err := repo.BackupTo(r.Context(), outPath); err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to create sqlite backup", nil)
		return
	}

	file, err := os.Open(outPath)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to read sqlite backup", nil)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to read sqlite backup metadata", nil)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, file); err != nil {
		h.logger.Warn("stream sqlite backup failed", "error", err)
		return
	}

	h.audit(r, "storage.sqlite.backup.download", "storage", nil, map[string]any{
		"path": outPath,
		"size": info.Size(),
	})
}

func (h *Handler) RestoreSQLiteBackup(w http.ResponseWriter, r *http.Request) {
	repo, err := h.requireSQLiteRepository()
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, sqliteRestoreUploadLimitBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid upload payload", nil)
		return
	}

	src, header, err := r.FormFile("file")
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "backup file is required", nil)
		return
	}
	defer src.Close()

	uploadsDir := filepath.Join(h.cfg.StorageRoot, "backups", "uploads")
	if err := os.MkdirAll(uploadsDir, 0o750); err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to prepare upload directory", nil)
		return
	}

	tempFile, err := os.CreateTemp(uploadsDir, "restore-*.db")
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to prepare uploaded backup", nil)
		return
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	if _, err := io.Copy(tempFile, src); err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to store uploaded backup", nil)
		return
	}
	if err := tempFile.Sync(); err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to finalize uploaded backup", nil)
		return
	}
	if err := tempFile.Close(); err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to finalize uploaded backup", nil)
		return
	}

	counts, err := repo.RestoreFromBackup(r.Context(), tempPath)
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "runtime", "failed to restore sqlite backup", map[string]any{"reason": strings.TrimSpace(err.Error())})
		return
	}

	h.audit(r, "storage.sqlite.restore.upload", "storage", nil, map[string]any{
		"filename": strings.TrimSpace(header.Filename),
		"counts":   counts,
	})
	render.JSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"counts": counts,
	})
}

func (h *Handler) requireSQLiteRepository() (*repository.SQLiteRepository, error) {
	repo, ok := h.repo.(*repository.SQLiteRepository)
	if ok {
		return repo, nil
	}
	if h.repo == nil {
		return nil, errors.New("repository is not configured")
	}
	return nil, errors.New("sqlite repository is not active")
}
