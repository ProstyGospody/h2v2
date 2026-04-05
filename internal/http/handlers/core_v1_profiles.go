package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"h2v2/internal/core"
	"h2v2/internal/http/render"
)

// ─── Generic CRUD helpers ─────────────────────────────────────────────────────

func serverIDFromQuery(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("server_id"))
}

// ─── Outbounds ────────────────────────────────────────────────────────────────

func (h *Handler) ListCoreOutbounds(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	serverID := serverIDFromQuery(r)
	items, err := svc.ListOutbounds(r.Context(), serverID)
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *Handler) GetCoreOutbound(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	item, err := svc.GetOutbound(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpsertCoreOutbound(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	var req core.Outbound
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	if id := strings.TrimSpace(chi.URLParam(r, "id")); id != "" {
		req.ID = id
	}
	item, err := svc.UpsertOutbound(r.Context(), req)
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCoreOutbound(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	if err := svc.DeleteOutbound(r.Context(), chi.URLParam(r, "id")); err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── Route Rules ──────────────────────────────────────────────────────────────

func (h *Handler) ListCoreRouteRules(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	items, err := svc.ListRouteRules(r.Context(), serverIDFromQuery(r))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *Handler) GetCoreRouteRule(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	item, err := svc.GetRouteRule(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpsertCoreRouteRule(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	var req core.RouteRule
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	if id := strings.TrimSpace(chi.URLParam(r, "id")); id != "" {
		req.ID = id
	}
	item, err := svc.UpsertRouteRule(r.Context(), req)
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCoreRouteRule(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	if err := svc.DeleteRouteRule(r.Context(), chi.URLParam(r, "id")); err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── DNS Profiles ─────────────────────────────────────────────────────────────

func (h *Handler) ListCoreDNSProfiles(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	items, err := svc.ListDNSProfiles(r.Context(), serverIDFromQuery(r))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *Handler) GetCoreDNSProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	item, err := svc.GetDNSProfile(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpsertCoreDNSProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	var req core.DNSProfile
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	if id := strings.TrimSpace(chi.URLParam(r, "id")); id != "" {
		req.ID = id
	}
	item, err := svc.UpsertDNSProfile(r.Context(), req)
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCoreDNSProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	if err := svc.DeleteDNSProfile(r.Context(), chi.URLParam(r, "id")); err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── Log Profiles ─────────────────────────────────────────────────────────────

func (h *Handler) ListCoreLogProfiles(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	items, err := svc.ListLogProfiles(r.Context(), serverIDFromQuery(r))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *Handler) GetCoreLogProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	item, err := svc.GetLogProfile(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpsertCoreLogProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	var req core.LogProfile
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	if id := strings.TrimSpace(chi.URLParam(r, "id")); id != "" {
		req.ID = id
	}
	item, err := svc.UpsertLogProfile(r.Context(), req)
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCoreLogProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	if err := svc.DeleteLogProfile(r.Context(), chi.URLParam(r, "id")); err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── Reality Profiles ─────────────────────────────────────────────────────────

func (h *Handler) ListCoreRealityProfiles(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	items, err := svc.ListRealityProfiles(r.Context(), serverIDFromQuery(r))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	// Strip private key from list response.
	for i := range items {
		items[i].PrivateKey = ""
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *Handler) GetCoreRealityProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	item, err := svc.GetRealityProfile(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	item.PrivateKey = "" // never expose private key via API
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpsertCoreRealityProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	var req core.RealityProfile
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	if id := strings.TrimSpace(chi.URLParam(r, "id")); id != "" {
		req.ID = id
	}
	item, err := svc.UpsertRealityProfile(r.Context(), req)
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	item.PrivateKey = ""
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCoreRealityProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	if err := svc.DeleteRealityProfile(r.Context(), chi.URLParam(r, "id")); err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── Transport Profiles ───────────────────────────────────────────────────────

func (h *Handler) ListCoreTransportProfiles(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	items, err := svc.ListTransportProfiles(r.Context(), serverIDFromQuery(r))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *Handler) GetCoreTransportProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	item, err := svc.GetTransportProfile(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpsertCoreTransportProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	var req core.TransportProfile
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	if id := strings.TrimSpace(chi.URLParam(r, "id")); id != "" {
		req.ID = id
	}
	item, err := svc.UpsertTransportProfile(r.Context(), req)
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCoreTransportProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	if err := svc.DeleteTransportProfile(r.Context(), chi.URLParam(r, "id")); err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── Multiplex Profiles ───────────────────────────────────────────────────────

func (h *Handler) ListCoreMultiplexProfiles(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	items, err := svc.ListMultiplexProfiles(r.Context(), serverIDFromQuery(r))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *Handler) GetCoreMultiplexProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	item, err := svc.GetMultiplexProfile(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpsertCoreMultiplexProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	var req core.MultiplexProfile
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	if id := strings.TrimSpace(chi.URLParam(r, "id")); id != "" {
		req.ID = id
	}
	item, err := svc.UpsertMultiplexProfile(r.Context(), req)
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCoreMultiplexProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	if err := svc.DeleteMultiplexProfile(r.Context(), chi.URLParam(r, "id")); err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── HY2 Masquerade Profiles ──────────────────────────────────────────────────

func (h *Handler) ListCoreHY2MasqueradeProfiles(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	items, err := svc.ListHY2MasqueradeProfiles(r.Context(), serverIDFromQuery(r))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *Handler) GetCoreHY2MasqueradeProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	item, err := svc.GetHY2MasqueradeProfile(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpsertCoreHY2MasqueradeProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	var req core.HY2MasqueradeProfile
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	if id := strings.TrimSpace(chi.URLParam(r, "id")); id != "" {
		req.ID = id
	}
	item, err := svc.UpsertHY2MasqueradeProfile(r.Context(), req)
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCoreHY2MasqueradeProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	if err := svc.DeleteHY2MasqueradeProfile(r.Context(), chi.URLParam(r, "id")); err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── Client Profiles ──────────────────────────────────────────────────────────

func (h *Handler) ListCoreClientProfiles(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	items, err := svc.ListClientProfiles(r.Context(), serverIDFromQuery(r))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *Handler) GetCoreClientProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	item, err := svc.GetClientProfile(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpsertCoreClientProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	var req core.ClientProfile
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	if id := strings.TrimSpace(chi.URLParam(r, "id")); id != "" {
		req.ID = id
	}
	item, err := svc.UpsertClientProfile(r.Context(), req)
	if err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCoreClientProfile(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	if err := svc.DeleteClientProfile(r.Context(), chi.URLParam(r, "id")); err != nil {
		status, code := coreErrorStatus(err)
		h.renderError(w, status, code, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── Domain validation ────────────────────────────────────────────────────────

func (h *Handler) ValidateCoreServerDomain(w http.ResponseWriter, r *http.Request) {
	svc := h.ensureCoreService(w)
	if svc == nil {
		return
	}
	serverID := chi.URLParam(r, "id")
	errs := svc.ValidateDomainModel(r.Context(), serverID)
	render.JSON(w, http.StatusOK, map[string]any{
		"valid":  len(errs) == 0,
		"errors": errs,
	})
}
