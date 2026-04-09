package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"h2v2/internal/core"
	"h2v2/internal/http/render"
)

type coreBulkUserRequest struct {
	IDs                []string   `json:"ids"`
	Enabled            *bool      `json:"enabled"`
	ExtendSeconds      int64      `json:"extend_seconds"`
	SetExpireAt        *time.Time `json:"set_expire_at"`
	ClearExpire        bool       `json:"clear_expire"`
	TrafficLimitBytes  *int64     `json:"traffic_limit_bytes"`
	ClientProfileID    *string    `json:"client_profile_id"`
	InboundID          *string    `json:"inbound_id"`
	RotateTokens       bool       `json:"rotate_tokens"`
	RegenerateArtifacts bool      `json:"regenerate_artifacts"`
	DeleteMode         string     `json:"delete_mode"`
}

type coreBulkAccessRequest struct {
	IDs                []string   `json:"ids"`
	Enabled            *bool      `json:"enabled"`
	ExtendSeconds      int64      `json:"extend_seconds"`
	SetExpireAt        *time.Time `json:"set_expire_at"`
	ClearExpire        bool       `json:"clear_expire"`
	TrafficLimitBytes  *int64     `json:"traffic_limit_bytes"`
	ClientProfileID    *string    `json:"client_profile_id"`
	InboundID          *string    `json:"inbound_id"`
	RotateCredentials  bool       `json:"rotate_credentials"`
	RegenerateArtifacts bool      `json:"regenerate_artifacts"`
	DeleteMode         string     `json:"delete_mode"`
}

func mapBulkUserPatch(req coreBulkUserRequest) core.BulkUserPatch {
	return core.BulkUserPatch{
		IDs:                 req.IDs,
		Enabled:             req.Enabled,
		ExtendSeconds:       req.ExtendSeconds,
		SetExpireAt:         req.SetExpireAt,
		ClearExpire:         req.ClearExpire,
		TrafficLimitBytes:   req.TrafficLimitBytes,
		ClientProfileID:     req.ClientProfileID,
		InboundID:           req.InboundID,
		RotateTokens:        req.RotateTokens,
		RegenerateArtifacts: req.RegenerateArtifacts,
		DeleteMode:          core.BulkDeleteMode(strings.ToLower(strings.TrimSpace(req.DeleteMode))),
	}
}

func mapBulkAccessPatch(req coreBulkAccessRequest) core.BulkAccessPatch {
	return core.BulkAccessPatch{
		IDs:                 req.IDs,
		Enabled:             req.Enabled,
		ExtendSeconds:       req.ExtendSeconds,
		SetExpireAt:         req.SetExpireAt,
		ClearExpire:         req.ClearExpire,
		TrafficLimitBytes:   req.TrafficLimitBytes,
		ClientProfileID:     req.ClientProfileID,
		InboundID:           req.InboundID,
		RotateCredentials:   req.RotateCredentials,
		RegenerateArtifacts: req.RegenerateArtifacts,
		DeleteMode:          core.BulkDeleteMode(strings.ToLower(strings.TrimSpace(req.DeleteMode))),
	}
}

func (h *Handler) BulkPreviewCoreUsersPatch(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreBulkUserRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	result, err := service.PreviewBulkUsers(r.Context(), mapBulkUserPatch(req))
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, result)
}

func (h *Handler) BulkApplyCoreUsersPatch(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreBulkUserRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	result, err := service.ApplyBulkUsers(r.Context(), mapBulkUserPatch(req))
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, result)
}

func (h *Handler) BulkPreviewCoreAccessPatch(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreBulkAccessRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	result, err := service.PreviewBulkAccess(r.Context(), mapBulkAccessPatch(req))
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, result)
}

func (h *Handler) BulkApplyCoreAccessPatch(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreBulkAccessRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	result, err := service.ApplyBulkAccess(r.Context(), mapBulkAccessPatch(req))
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, result)
}

func (h *Handler) GetCorePolicyUsage(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	usage, err := service.GetPolicyUsage(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "id"))
	if err != nil {
		status, errorType := coreErrorStatus(err)
		h.renderError(w, status, errorType, err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, usage)
}

func (h *Handler) GetCoreServerDraftState(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	state, err := service.GetDraftRevisionState(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, state)
}

func (h *Handler) RefreshCoreUserArtifacts(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	userID := strings.TrimSpace(chi.URLParam(r, "id"))
	subscription, err := service.EnsureSubscriptionForUser(r.Context(), userID)
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", err.Error(), nil)
		return
	}
	if err := service.Store().MarkSubscriptionArtifactsDirty(r.Context(), subscription.ID, "manual_refresh"); err != nil && !core.IsNotFound(err) {
		h.renderError(w, http.StatusInternalServerError, "runtime", err.Error(), nil)
		return
	}
	artifacts, err := service.BuildUserArtifacts(r.Context(), userID)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to refresh artifacts", nil)
		return
	}
	render.JSON(w, http.StatusOK, artifacts)
}
