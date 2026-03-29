package handlers

import (
	"net/http"
	"time"

	"h2v2/internal/http/render"
	"h2v2/internal/version"
)

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": version.Version,
		"time":    time.Now().UTC(),
	})
}

func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Ping(r.Context()); err != nil {
		render.Error(w, http.StatusServiceUnavailable, "file storage is unavailable")
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"status": "ready"})
}
