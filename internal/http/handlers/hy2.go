package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	auditdomain "h2v2/internal/domain/audit"
	hysteriadomain "h2v2/internal/domain/hysteria"
	"h2v2/internal/http/render"
	"h2v2/internal/repository"
	"h2v2/internal/security"
	"h2v2/internal/services"
)

type createHysteriaUserRequest struct {
	Username        *string                          `json:"username"`
	Password        *string                          `json:"password"`
	AuthSecret      *string                          `json:"auth_secret"`
	Note            *string                          `json:"note"`
	ClientOverrides *hysteriadomain.ClientOverrides `json:"client_overrides"`
}

type updateHysteriaUserRequest struct {
	Username        *string                          `json:"username"`
	Password        *string                          `json:"password"`
	AuthSecret      *string                          `json:"auth_secret"`
	Note            *string                          `json:"note"`
	ClientOverrides *hysteriadomain.ClientOverrides `json:"client_overrides"`
}

type setHysteriaUsersStateRequest struct {
	IDs     []string `json:"ids"`
	Enabled *bool    `json:"enabled"`
}

type managedHysteriaSyncError struct {
	cause     error
	attempts  int
	status    services.ServiceDetails
	hasStatus bool
	logs      []string
	statusErr error
	logsErr   error
}

func (e *managedHysteriaSyncError) Error() string {
	return e.cause.Error()
}

func (e *managedHysteriaSyncError) Unwrap() error {
	return e.cause
}

func (h *Handler) ListHysteriaUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := h.parsePagination(r)
	items, err := h.repo.ListHysteriaUsers(r.Context(), limit, offset)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list hysteria users", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) HysteriaClientDefaults(w http.ResponseWriter, r *http.Request) {
	defaults, err := h.hysteriaAccess.ClientDefaults(r.Context())
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to resolve client defaults", nil)
		return
	}
	render.JSON(w, http.StatusOK, defaults)
}

func (h *Handler) CreateHysteriaUser(w http.ResponseWriter, r *http.Request) {
	var req createHysteriaUserRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}

	username := ""
	if req.Username != nil {
		username = strings.TrimSpace(*req.Username)
	}
	password, hasPassword := selectAuthSecret(req.AuthSecret, req.Password)
	if !hasPassword {
		generated, err := security.RandomHex(16)
		if err != nil {
			h.renderError(w, http.StatusInternalServerError, "runtime", "failed to generate password", nil)
			return
		}
		password = generated
	}

	validationErrors := hysteriadomain.ValidateUserInput(username, password)
	validationErrors = append(validationErrors, hysteriadomain.ValidateClientOverrides(req.ClientOverrides)...)
	if len(validationErrors) > 0 {
		h.renderError(w, http.StatusBadRequest, "validation", "hysteria user validation failed", validationErrors)
		return
	}

	exportValidation, err := h.hysteriaAccess.ValidateClientExportDraft(r.Context(), username, password, req.ClientOverrides)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to validate generated hysteria client config", nil)
		return
	}
	if !exportValidation.Valid {
		h.renderError(w, http.StatusBadRequest, "validation", "generated hysteria client config is invalid", exportValidation)
		return
	}

	user, err := h.repo.CreateHysteriaUser(r.Context(), username, password, req.Note, req.ClientOverrides)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			h.renderError(w, http.StatusConflict, "validation", "username already exists", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to create hysteria user", nil)
		return
	}
	if err := h.syncManagedHysteria(r.Context()); err != nil {
		details := map[string]any{}
		appendSyncErrorDetails(details, "sync_error", err)
		if rollbackErr := h.repo.DeleteHysteriaUser(r.Context(), user.ID); rollbackErr != nil {
			details["rollback_error"] = rollbackErr.Error()
		}
		if rollbackSyncErr := h.syncManagedHysteria(r.Context()); rollbackSyncErr != nil {
			appendSyncErrorDetails(details, "rollback_sync_error", rollbackSyncErr)
		}
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to sync/restart hysteria config; user creation was rolled back", details)
		return
	}

	item, err := h.repo.GetHysteriaUser(r.Context(), user.ID)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load hysteria user", nil)
		return
	}
	artifacts, _, err := h.hysteriaAccess.BuildUserArtifacts(item)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to generate hysteria artifacts", nil)
		return
	}

	h.audit(r, "hysteria.user.create", auditdomain.EntityHysteriaUser, &item.ID, map[string]any{"username": item.Username})
	render.JSON(w, http.StatusCreated, map[string]any{"user": item, "artifacts": artifacts})
}

func (h *Handler) GetHysteriaUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.repo.GetHysteriaUser(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to get hysteria user", nil)
		return
	}
	h.renderHysteriaUserPayload(w, http.StatusOK, item)
}

func (h *Handler) UpdateHysteriaUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	current, err := h.repo.GetHysteriaUser(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load hysteria user", nil)
		return
	}

	var req updateHysteriaUserRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}

	username := current.Username
	if req.Username != nil {
		username = strings.TrimSpace(*req.Username)
	}
	password := current.Password
	if requestedPassword, hasPassword := selectAuthSecret(req.AuthSecret, req.Password); hasPassword {
		password = requestedPassword
	}
	overrides := coalesceClientOverrides(req.ClientOverrides, current.ClientOverrides)

	validationErrors := hysteriadomain.ValidateUserInput(username, password)
	validationErrors = append(validationErrors, hysteriadomain.ValidateClientOverrides(overrides)...)
	if len(validationErrors) > 0 {
		h.renderError(w, http.StatusBadRequest, "validation", "hysteria user validation failed", validationErrors)
		return
	}

	exportValidation, err := h.hysteriaAccess.ValidateClientExportDraft(r.Context(), username, password, overrides)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to validate generated hysteria client config", nil)
		return
	}
	if !exportValidation.Valid {
		h.renderError(w, http.StatusBadRequest, "validation", "generated hysteria client config is invalid", exportValidation)
		return
	}

	updated, err := h.repo.UpdateHysteriaUser(r.Context(), id, username, password, coalesceNote(req.Note, current.Note), overrides)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", nil)
			return
		}
		if repository.IsUniqueViolation(err) {
			h.renderError(w, http.StatusConflict, "validation", "username already exists", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to update hysteria user", nil)
		return
	}
	if err := h.syncManagedHysteria(r.Context()); err != nil {
		details := map[string]any{}
		appendSyncErrorDetails(details, "sync_error", err)
		if _, rollbackErr := h.repo.UpdateHysteriaUser(r.Context(), id, current.Username, current.Password, current.Note, current.ClientOverrides); rollbackErr != nil {
			details["rollback_error"] = rollbackErr.Error()
		}
		if rollbackSyncErr := h.syncManagedHysteria(r.Context()); rollbackSyncErr != nil {
			appendSyncErrorDetails(details, "rollback_sync_error", rollbackSyncErr)
		}
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to sync/restart hysteria config; user update was rolled back", details)
		return
	}

	h.audit(r, "hysteria.user.update", auditdomain.EntityHysteriaUser, &id, map[string]any{"username": updated.Username})
	h.renderHysteriaUserPayload(w, http.StatusOK, updated)
}

func (h *Handler) DeleteHysteriaUser(w http.ResponseWriter, r *http.Request) {
	h.deleteHysteriaUser(w, r, "hysteria.user.delete")
}

func (h *Handler) RevokeHysteriaUser(w http.ResponseWriter, r *http.Request) {
	h.deleteHysteriaUser(w, r, "hysteria.user.revoke")
}

func (h *Handler) deleteHysteriaUser(w http.ResponseWriter, r *http.Request, action string) {
	id := chi.URLParam(r, "id")
	current, err := h.repo.GetHysteriaUser(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load hysteria user", nil)
		return
	}

	if current.Enabled {
		if err := h.repo.SetHysteriaUserEnabled(r.Context(), id, false); err != nil {
			h.renderError(w, http.StatusInternalServerError, "runtime", "failed to revoke hysteria user", nil)
			return
		}
		if err := h.syncManagedHysteria(r.Context()); err != nil {
			_ = h.repo.SetHysteriaUserEnabled(r.Context(), id, true)
			details := map[string]any{}
			appendSyncErrorDetails(details, "sync_error", err)
			if rollbackSyncErr := h.syncManagedHysteria(r.Context()); rollbackSyncErr != nil {
				appendSyncErrorDetails(details, "rollback_sync_error", rollbackSyncErr)
				h.renderError(w, http.StatusInternalServerError, "sync", "failed to sync/restart hysteria config; revoke rollback failed to apply runtime state", details)
				return
			}
			h.renderError(w, http.StatusInternalServerError, "sync", "failed to sync/restart hysteria config; revoke was rolled back", details)
			return
		}
	} else {
		if err := h.syncManagedHysteria(r.Context()); err != nil {
			details := map[string]any{}
			appendSyncErrorDetails(details, "sync_error", err)
			h.renderError(w, http.StatusInternalServerError, "sync", "failed to synchronize/restart managed hysteria config before delete", details)
			return
		}
	}

	if err := h.repo.DeleteHysteriaUser(r.Context(), id); err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to delete hysteria user record", nil)
		return
	}

	h.audit(r, action, auditdomain.EntityHysteriaUser, &id, map[string]any{"username": current.Username})
	render.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) EnableHysteriaUser(w http.ResponseWriter, r *http.Request) {
	h.setHysteriaUserState(w, r, true)
}

func (h *Handler) DisableHysteriaUser(w http.ResponseWriter, r *http.Request) {
	h.setHysteriaUserState(w, r, false)
}

func (h *Handler) SetHysteriaUsersState(w http.ResponseWriter, r *http.Request) {
	var req setHysteriaUsersStateRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	if req.Enabled == nil {
		h.renderError(w, http.StatusBadRequest, "validation", "enabled is required", nil)
		return
	}

	ids := normalizeHysteriaUserIDs(req.IDs)
	if len(ids) == 0 {
		h.renderError(w, http.StatusBadRequest, "validation", "ids must contain at least one user id", nil)
		return
	}
	if len(ids) > 500 {
		h.renderError(w, http.StatusBadRequest, "validation", "ids limit exceeded", map[string]any{"max": 500})
		return
	}

	h.hysteriaStateMu.Lock()
	defer h.hysteriaStateMu.Unlock()

	currentStates := make(map[string]bool, len(ids))
	changedIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		current, err := h.repo.GetHysteriaUser(r.Context(), id)
		if err != nil {
			if repository.IsNotFound(err) {
				h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", map[string]any{"id": id})
				return
			}
			h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load hysteria user", nil)
			return
		}
		currentStates[id] = current.Enabled
		if current.Enabled != *req.Enabled {
			changedIDs = append(changedIDs, id)
		}
	}

	if len(changedIDs) == 0 {
		render.JSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": *req.Enabled, "updated": 0})
		return
	}

	appliedIDs := make([]string, 0, len(changedIDs))
	for _, id := range changedIDs {
		if err := h.repo.SetHysteriaUserEnabled(r.Context(), id, *req.Enabled); err != nil {
			details := map[string]any{"id": id}
			if rollbackErr := h.rollbackHysteriaUsersState(r.Context(), appliedIDs, currentStates); rollbackErr != nil {
				details["rollback_error"] = rollbackErr.Error()
			}
			if repository.IsNotFound(err) {
				h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", details)
				return
			}
			h.renderError(w, http.StatusInternalServerError, "runtime", "failed to update hysteria user status", details)
			return
		}
		appliedIDs = append(appliedIDs, id)
	}

	if err := h.syncManagedHysteria(r.Context()); err != nil {
		details := map[string]any{}
		appendSyncErrorDetails(details, "sync_error", err)
		if rollbackErr := h.rollbackHysteriaUsersState(r.Context(), changedIDs, currentStates); rollbackErr != nil {
			details["rollback_error"] = rollbackErr.Error()
		}
		if rollbackSyncErr := h.syncManagedHysteria(r.Context()); rollbackSyncErr != nil {
			appendSyncErrorDetails(details, "rollback_sync_error", rollbackSyncErr)
			h.renderError(w, http.StatusInternalServerError, "sync", "failed to sync/restart hysteria config; state batch rollback failed to apply runtime state", details)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to sync/restart hysteria config; state batch was rolled back", details)
		return
	}

	action := "hysteria.user.disable"
	if *req.Enabled {
		action = "hysteria.user.enable"
	}
	for _, id := range changedIDs {
		h.audit(r, action, auditdomain.EntityHysteriaUser, &id, map[string]any{"enabled": *req.Enabled, "bulk": true})
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": *req.Enabled, "updated": len(changedIDs)})
}

func (h *Handler) setHysteriaUserState(w http.ResponseWriter, r *http.Request, enabled bool) {
	id := chi.URLParam(r, "id")
	h.hysteriaStateMu.Lock()
	defer h.hysteriaStateMu.Unlock()

	current, err := h.repo.GetHysteriaUser(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load hysteria user", nil)
		return
	}
	if current.Enabled == enabled {
		render.JSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": enabled})
		return
	}
	if err := h.repo.SetHysteriaUserEnabled(r.Context(), id, enabled); err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to update hysteria user status", nil)
		return
	}
	if err := h.syncManagedHysteria(r.Context()); err != nil {
		_ = h.repo.SetHysteriaUserEnabled(r.Context(), id, current.Enabled)
		details := map[string]any{}
		appendSyncErrorDetails(details, "sync_error", err)
		if rollbackSyncErr := h.syncManagedHysteria(r.Context()); rollbackSyncErr != nil {
			appendSyncErrorDetails(details, "rollback_sync_error", rollbackSyncErr)
			h.renderError(w, http.StatusInternalServerError, "sync", "failed to sync/restart hysteria config; state rollback failed to apply runtime state", details)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to sync/restart hysteria config; state change was rolled back", details)
		return
	}
	action := "hysteria.user.disable"
	if enabled {
		action = "hysteria.user.enable"
	}
	h.audit(r, action, auditdomain.EntityHysteriaUser, &id, map[string]any{"enabled": enabled})
	render.JSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": enabled})
}

func (h *Handler) HysteriaUserArtifacts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.repo.GetHysteriaUser(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load hysteria user", nil)
		return
	}
	if !item.Enabled {
		h.renderError(w, http.StatusConflict, "validation", "hysteria user is disabled; enable the user to generate active connection artifacts", nil)
		return
	}
	artifacts, _, err := h.hysteriaAccess.BuildUserArtifacts(item)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to generate hysteria artifacts", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"user": item, "artifacts": artifacts})
}

func (h *Handler) HysteriaUserSubscription(w http.ResponseWriter, r *http.Request) {
	if h.hysteriaAccess == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "hysteria access manager is not configured", nil)
		return
	}

	token := strings.TrimSpace(chi.URLParam(r, "token"))
	user, err := h.hysteriaAccess.ResolveSubscriptionUser(r.Context(), token)
	if err != nil {
		if errors.Is(err, services.ErrInvalidSubscriptionToken) || repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "subscription not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to resolve subscription", nil)
		return
	}
	if !user.Enabled {
		h.renderError(w, http.StatusNotFound, "not_found", "subscription not found", nil)
		return
	}

	artifacts, _, err := h.hysteriaAccess.BuildUserArtifacts(user)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to generate subscription artifacts", nil)
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s-hy2-subscription.txt"`, user.Username))

	if format == "yaml" || format == "client" {
		w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
		_, _ = w.Write([]byte(strings.TrimSpace(artifacts.ClientYAML) + "\n"))
		return
	}

	shareURI := strings.TrimSpace(artifacts.URIHy2)
	if shareURI == "" {
		shareURI = strings.TrimSpace(artifacts.URI)
	}
	if shareURI == "" {
		h.renderError(w, http.StatusNotFound, "not_found", "subscription endpoint has no active URI", nil)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(shareURI + "\n"))
}

func (h *Handler) HysteriaUserQR(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.repo.GetHysteriaUser(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load hysteria user", nil)
		return
	}
	if !item.Enabled {
		h.renderError(w, http.StatusConflict, "validation", "hysteria user is disabled; enable the user to generate an active QR code", nil)
		return
	}
	artifacts, _, err := h.hysteriaAccess.BuildUserArtifacts(item)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to generate hysteria artifacts", nil)
		return
	}

	qrValue := ""
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("kind")), "subscription") {
		qrValue = strings.TrimSpace(artifacts.SubscriptionURL)
	} else {
		qrValue = strings.TrimSpace(artifacts.URIHy2)
		if qrValue == "" {
			qrValue = strings.TrimSpace(artifacts.URI)
		}
	}
	if qrValue == "" {
		h.renderError(w, http.StatusNotFound, "not_found", "qr source is empty", nil)
		return
	}
	size := parseQRSize(r.URL.Query().Get("size"), 320)
	if err := renderQRCodePNG(w, qrValue, size); err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to render qr code", nil)
	}
}

func (h *Handler) KickHysteriaUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.repo.GetHysteriaUser(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "hysteria user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load hysteria user", nil)
		return
	}
	if h.hy2Client == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "hysteria live control is not configured", nil)
		return
	}
	if err := h.hy2Client.Kick(r.Context(), item.Username); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", "failed to kick hysteria session", nil)
		return
	}
	h.audit(r, "hysteria.user.kick", auditdomain.EntityHysteriaUser, &id, map[string]any{"username": item.Username})
	render.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) HysteriaStatsOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.repo.GetHysteriaStatsOverview(r.Context())
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to get hysteria stats overview", nil)
		return
	}
	render.JSON(w, http.StatusOK, overview)
}

func (h *Handler) HysteriaStatsHistory(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	limit, offset := h.parsePagination(r)
	items, err := h.repo.ListHysteriaSnapshots(r.Context(), userID, limit, offset)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list hysteria stats", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func selectAuthSecret(primary *string, fallback *string) (string, bool) {
	if primary != nil {
		return strings.TrimSpace(*primary), true
	}
	if fallback != nil {
		return strings.TrimSpace(*fallback), true
	}
	return "", false
}

func coalesceNote(next *string, current *string) *string {
	if next != nil {
		return next
	}
	return current
}

func coalesceClientOverrides(next *hysteriadomain.ClientOverrides, current *hysteriadomain.ClientOverrides) *hysteriadomain.ClientOverrides {
	if next != nil {
		return next
	}
	return current
}

func normalizeHysteriaUserIDs(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		id := strings.TrimSpace(item)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (h *Handler) rollbackHysteriaUsersState(ctx context.Context, ids []string, states map[string]bool) error {
	var rollbackErr error
	for _, id := range ids {
		state, ok := states[id]
		if !ok {
			continue
		}
		if err := h.repo.SetHysteriaUserEnabled(ctx, id, state); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
	}
	return rollbackErr
}

func appendSyncErrorDetails(details map[string]any, key string, err error) {
	if details == nil || err == nil {
		return
	}
	details[key] = err.Error()

	var syncErr *managedHysteriaSyncError
	if !errors.As(err, &syncErr) {
		return
	}

	prefix := strings.TrimSuffix(strings.TrimSpace(key), "_error")
	if prefix == "" {
		prefix = "sync"
	}
	if syncErr.attempts > 0 {
		details[prefix+"_attempts"] = syncErr.attempts
	}
	if syncErr.hasStatus {
		details[prefix+"_status"] = syncErr.status.StatusText
		details[prefix+"_active"] = syncErr.status.Active
		details[prefix+"_sub_state"] = syncErr.status.SubState
	} else if syncErr.statusErr != nil {
		details[prefix+"_status_error"] = syncErr.statusErr.Error()
	}
	if len(syncErr.logs) > 0 {
		logs := syncErr.logs
		if len(logs) > 20 {
			logs = logs[len(logs)-20:]
		}
		details[prefix+"_logs"] = logs
	} else if syncErr.logsErr != nil {
		details[prefix+"_logs_error"] = syncErr.logsErr.Error()
	}
}

func (h *Handler) withManagedHysteriaDiagnostics(err error, attempts int) error {
	if err == nil {
		return nil
	}
	result := &managedHysteriaSyncError{
		cause:    err,
		attempts: attempts,
	}
	if h.serviceManager == nil {
		return result
	}

	diagTimeout := 8 * time.Second
	if h.serviceManager.CommandTimeout > diagTimeout {
		diagTimeout = h.serviceManager.CommandTimeout
	}
	diagCtx, cancel := context.WithTimeout(context.Background(), diagTimeout)
	defer cancel()

	if status, statusErr := h.serviceManager.Status(diagCtx, "hysteria-server"); statusErr == nil {
		result.status = status
		result.hasStatus = true
	} else {
		result.statusErr = statusErr
	}

	if logs, logsErr := h.serviceManager.Logs(diagCtx, "hysteria-server", 30); logsErr == nil {
		result.logs = logs
	} else {
		result.logsErr = logsErr
	}
	return result
}

func (h *Handler) restartManagedHysteria(ctx context.Context) error {
	if h.serviceManager == nil {
		return fmt.Errorf("service manager is not configured")
	}
	const maxAttempts = 3

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := h.serviceManager.Restart(ctx, "hysteria-server"); err == nil {
			return nil
		} else {
			lastErr = err
			if h.isHysteriaServerRunning() {
				return nil
			}
		}

		if attempt == maxAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return h.withManagedHysteriaDiagnostics(fmt.Errorf("restart canceled: %w", ctx.Err()), attempt)
		case <-time.After(time.Duration(attempt) * 300 * time.Millisecond):
		}
	}
	return h.withManagedHysteriaDiagnostics(lastErr, maxAttempts)
}

func (h *Handler) isHysteriaServerRunning() bool {
	if h.serviceManager == nil {
		return false
	}
	diagCtx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	status, err := h.serviceManager.Status(diagCtx, "hysteria-server")
	if err != nil {
		return false
	}
	return status.Active == "active" && (status.SubState == "running" || status.SubState == "listening")
}

func (h *Handler) syncManagedHysteria(ctx context.Context) error {
	if h.hysteriaAccess == nil {
		return fmt.Errorf("hysteria access manager is not configured")
	}
	if _, err := h.hysteriaAccess.Sync(ctx); err != nil {
		return err
	}
	if h.serviceManager == nil {
		return fmt.Errorf("service manager is not configured")
	}
	if err := h.restartManagedHysteria(ctx); err != nil {
		return err
	}
	if h.repo != nil {
		if status, statusErr := h.serviceManager.Status(ctx, "hysteria-server"); statusErr == nil {
			_ = h.repo.UpsertServiceState(ctx, "hysteria-server", status.StatusText, nil, h.serviceManager.ToJSON(status))
		}
	}
	return nil
}

func (h *Handler) renderHysteriaUserPayload(w http.ResponseWriter, status int, item repository.HysteriaUserView) {
	response := map[string]any{"user": item}
	if !item.Enabled {
		response["artifacts"] = nil
		response["access_state"] = "disabled"
		response["access_message"] = "This user is disabled and is not present in the active Hysteria server auth config."
		render.JSON(w, status, response)
		return
	}
	artifacts, _, err := h.hysteriaAccess.BuildUserArtifacts(item)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to generate hysteria artifacts", nil)
		return
	}
	response["artifacts"] = artifacts
	render.JSON(w, status, response)
}
