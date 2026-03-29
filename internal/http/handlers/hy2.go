package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
		details := map[string]any{"sync_error": err.Error()}
		if rollbackErr := h.repo.DeleteHysteriaUser(r.Context(), user.ID); rollbackErr != nil {
			details["rollback_error"] = rollbackErr.Error()
		}
		if rollbackSyncErr := h.syncManagedHysteria(r.Context()); rollbackSyncErr != nil {
			details["rollback_sync_error"] = rollbackSyncErr.Error()
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
		details := map[string]any{"sync_error": err.Error()}
		if _, rollbackErr := h.repo.UpdateHysteriaUser(r.Context(), id, current.Username, current.Password, current.Note, current.ClientOverrides); rollbackErr != nil {
			details["rollback_error"] = rollbackErr.Error()
		}
		if rollbackSyncErr := h.syncManagedHysteria(r.Context()); rollbackSyncErr != nil {
			details["rollback_sync_error"] = rollbackSyncErr.Error()
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
			details := map[string]any{"sync_error": err.Error()}
			if rollbackSyncErr := h.syncManagedHysteria(r.Context()); rollbackSyncErr != nil {
				details["rollback_sync_error"] = rollbackSyncErr.Error()
				h.renderError(w, http.StatusInternalServerError, "sync", "failed to sync/restart hysteria config; revoke rollback failed to apply runtime state", details)
				return
			}
			h.renderError(w, http.StatusInternalServerError, "sync", "failed to sync/restart hysteria config; revoke was rolled back", details)
			return
		}
	} else {
		if err := h.syncManagedHysteria(r.Context()); err != nil {
			h.renderError(w, http.StatusInternalServerError, "sync", "failed to synchronize/restart managed hysteria config before delete", map[string]any{"sync_error": err.Error()})
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

func (h *Handler) setHysteriaUserState(w http.ResponseWriter, r *http.Request, enabled bool) {
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
		details := map[string]any{"sync_error": err.Error()}
		if rollbackSyncErr := h.syncManagedHysteria(r.Context()); rollbackSyncErr != nil {
			details["rollback_sync_error"] = rollbackSyncErr.Error()
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
	if err := h.serviceManager.Restart(ctx, "hysteria-server"); err != nil {
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
