package handlers

import (
	"net/http"

	"h2v2/internal/http/render"
)

func (h *Handler) ListAudit(w http.ResponseWriter, r *http.Request) {
	limit, offset := h.parsePagination(r)
	items, err := h.repo.ListAuditLogs(r.Context(), limit, offset)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": items})
}

