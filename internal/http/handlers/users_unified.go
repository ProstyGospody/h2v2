package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"h2v2/internal/http/render"
	"h2v2/internal/repository"
	runtimecore "h2v2/internal/runtime"
)

type credentialRequest struct {
	Protocol string `json:"protocol"`
	Identity string `json:"identity"`
	Secret   string `json:"secret"`
	DataJSON string `json:"data_json"`
}

type userRequest struct {
	Name              *string             `json:"name"`
	Enabled           *bool               `json:"enabled"`
	TrafficLimitBytes *int64              `json:"traffic_limit_bytes"`
	ExpireAt          *time.Time          `json:"expire_at"`
	Note              *string             `json:"note"`
	Credentials       []credentialRequest `json:"credentials"`
}

type usersStateRequest struct {
	IDs      []string `json:"ids"`
	Enabled  *bool    `json:"enabled"`
	Protocol *string  `json:"protocol"`
}

type usersDeleteRequest struct {
	IDs []string `json:"ids"`
}

type usersKickRequest struct {
	IDs []string `json:"ids"`
}

type inboundRequest struct {
	ID         *string `json:"id"`
	NodeID     *string `json:"node_id"`
	Name       *string `json:"name"`
	Protocol   *string `json:"protocol"`
	Transport  *string `json:"transport"`
	Security   *string `json:"security"`
	Host       *string `json:"host"`
	Port       *int    `json:"port"`
	Enabled    *bool   `json:"enabled"`
	ParamsJSON *string `json:"params_json"`
	RuntimeJSON *string `json:"runtime_json"`
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	limit, offset := h.parsePagination(r)
	protocol, err := parseProtocolOptional(r.URL.Query().Get("protocol"))
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	items, err := h.userManager.ListUsers(r.Context(), limit, offset, protocol)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list users", nil)
		return
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, h.serializeUnifiedUser(r.Context(), item, false))
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": result})
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	var req userRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	input, err := mapCreateUserInput(req)
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	created, err := h.userManager.CreateUser(r.Context(), input)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("user create failed", "error", err)
		}
		if repository.IsUniqueViolation(err) {
			h.renderError(w, http.StatusConflict, "validation", "name already exists", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to create user", nil)
		return
	}
	render.JSON(w, http.StatusCreated, h.serializeUnifiedUser(r.Context(), created, true))
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	item, err := h.userManager.GetUser(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load user", nil)
		return
	}
	render.JSON(w, http.StatusOK, h.serializeUnifiedUser(r.Context(), item, true))
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	current, err := h.userManager.GetUser(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load user", nil)
		return
	}

	var req userRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	input, err := mapUpdateUserInput(req, current)
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	updated, err := h.userManager.UpdateUser(r.Context(), id, input)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		if repository.IsUniqueViolation(err) {
			h.renderError(w, http.StatusConflict, "validation", "name already exists", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to update user", nil)
		return
	}
	render.JSON(w, http.StatusOK, h.serializeUnifiedUser(r.Context(), updated, true))
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		h.renderError(w, http.StatusBadRequest, "validation", "id is required", nil)
		return
	}
	if err := h.userManager.DeleteUsers(r.Context(), repository.BatchDeleteUsersInput{UserIDs: []string{id}}); err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to delete user", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) DeleteUsers(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	var req usersDeleteRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	ids := normalizeUserIDs(req.IDs)
	if len(ids) == 0 {
		h.renderError(w, http.StatusBadRequest, "validation", "ids must contain at least one user id", nil)
		return
	}
	if err := h.userManager.DeleteUsers(r.Context(), repository.BatchDeleteUsersInput{UserIDs: ids}); err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to delete users", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": len(ids)})
}

func (h *Handler) SetUsersState(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	var req usersStateRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	if req.Enabled == nil {
		h.renderError(w, http.StatusBadRequest, "validation", "enabled is required", nil)
		return
	}
	ids := normalizeUserIDs(req.IDs)
	if len(ids) == 0 {
		h.renderError(w, http.StatusBadRequest, "validation", "ids must contain at least one user id", nil)
		return
	}
	protocol, err := parseProtocolOptional(valueOrEmpty(req.Protocol))
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	updated, err := h.userManager.SetUsersStateBatch(r.Context(), repository.BatchUserStateInput{UserIDs: ids, Enabled: *req.Enabled, Protocol: protocol})
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to update users state", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true, "updated": updated, "enabled": *req.Enabled})
}

func (h *Handler) KickUser(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	kicked, err := h.userManager.KickUsers(r.Context(), []string{id})
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusBadGateway, "service", "failed to kick user session", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true, "kicked": kicked})
}

func (h *Handler) KickUsers(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	var req usersKickRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	ids := normalizeUserIDs(req.IDs)
	if len(ids) == 0 {
		h.renderError(w, http.StatusBadRequest, "validation", "ids must contain at least one user id", nil)
		return
	}
	kicked, err := h.userManager.KickUsers(r.Context(), ids)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusBadGateway, "service", "failed to kick user sessions", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true, "kicked": kicked})
}

func (h *Handler) GetUserSubscriptionToken(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	token, state, err := h.userManager.EnsureSubscriptionToken(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to issue subscription token", nil)
		return
	}
	url := strings.TrimRight(strings.TrimSpace(h.cfg.SubscriptionPublicURL), "/") + "/api/subscriptions/" + token
	render.JSON(w, http.StatusOK, map[string]any{"token": token, "url": url, "state": state})
}

func (h *Handler) UserQR(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}

	if rawValue := strings.TrimSpace(r.URL.Query().Get("value")); rawValue != "" {
		if err := renderQRCodePNG(w, rawValue, parseQRSize(r.URL.Query().Get("size"), 320)); err != nil {
			h.renderError(w, http.StatusInternalServerError, "runtime", "failed to render qr code", nil)
		}
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	user, err := h.userManager.GetUser(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load user", nil)
		return
	}
	if !user.Enabled {
		h.renderError(w, http.StatusConflict, "validation", "user is disabled; enable the user to generate an active QR code", nil)
		return
	}

	artifacts, subscriptionURL, err := h.userManager.BuildUserArtifacts(r.Context(), user)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to generate user artifacts", nil)
		return
	}

	qrKind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
	qrValue := ""
	if qrKind == "subscription" {
		qrValue = strings.TrimSpace(subscriptionURL)
	} else {
		protocolHint := strings.TrimSpace(r.URL.Query().Get("protocol"))
		qrValue = resolveUnifiedAccessQRValue(user, artifacts, protocolHint)
	}
	if qrValue == "" {
		h.renderError(w, http.StatusNotFound, "not_found", "qr source is empty", nil)
		return
	}

	if err := renderQRCodePNG(w, qrValue, parseQRSize(r.URL.Query().Get("size"), 320)); err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to render qr code", nil)
	}
}

func (h *Handler) RotateUserSubscriptionToken(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	token, state, err := h.userManager.RotateSubscriptionToken(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to rotate subscription token", nil)
		return
	}
	url := strings.TrimRight(strings.TrimSpace(h.cfg.SubscriptionPublicURL), "/") + "/api/subscriptions/" + token
	render.JSON(w, http.StatusOK, map[string]any{"token": token, "url": url, "state": state})
}

func (h *Handler) RevokeUserSubscriptionToken(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	state, err := h.userManager.RevokeSubscriptionToken(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to revoke subscription token", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"state": state})
}

func (h *Handler) RestoreUserSubscriptionToken(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	state, err := h.userManager.ClearSubscriptionRevocation(r.Context(), id)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to restore subscription token", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"state": state})
}

func (h *Handler) UserSubscription(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	rendered, err := h.userManager.RenderSubscription(r.Context(), token, format)
	if err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "subscription not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to render subscription", nil)
		return
	}

	w.Header().Set("Content-Type", rendered.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, rendered.Filename))
	for key, value := range rendered.Headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		w.Header().Set(key, value)
	}
	_, _ = w.Write(rendered.Body)
}

func (h *Handler) ListInbounds(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	protocol, err := parseProtocolOptional(r.URL.Query().Get("protocol"))
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	items, err := h.userManager.ListInbounds(r.Context(), protocol)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list inbounds", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetInbound(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	items, err := h.userManager.ListInbounds(r.Context(), nil)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load inbound", nil)
		return
	}
	for _, item := range items {
		if item.ID == id {
			render.JSON(w, http.StatusOK, item)
			return
		}
	}
	h.renderError(w, http.StatusNotFound, "not_found", "inbound not found", nil)
}

func (h *Handler) UpsertInbound(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	var req inboundRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	inbound, err := mapInboundRequest(req, chi.URLParam(r, "id"), h.cfg.PanelPublicHost)
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	saved, err := h.userManager.UpsertInbound(r.Context(), inbound)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to save inbound", nil)
		return
	}
	render.JSON(w, http.StatusOK, saved)
}

func (h *Handler) DeleteInbound(w http.ResponseWriter, r *http.Request) {
	if h.userManager == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "user manager is not configured", nil)
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if err := h.userManager.DeleteInbound(r.Context(), id); err != nil {
		if repository.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "inbound not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to delete inbound", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) serializeUnifiedUser(ctx context.Context, user repository.UserWithCredentials, includeArtifacts bool) map[string]any {
	response := map[string]any{"user": user}
	if !includeArtifacts {
		return response
	}
	artifacts, subscriptionURL, err := h.userManager.BuildUserArtifacts(ctx, user)
	if err != nil {
		response["artifacts"] = map[string]any{}
		response["subscription_url"] = ""
		response["artifacts_error"] = err.Error()
		return response
	}
	response["artifacts"] = artifacts
	response["subscription_url"] = subscriptionURL
	return response
}

func mapCreateUserInput(req userRequest) (repository.CreateUserInput, error) {
	if req.Name == nil {
		return repository.CreateUserInput{}, fmt.Errorf("name is required")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	limit := int64(0)
	if req.TrafficLimitBytes != nil {
		limit = *req.TrafficLimitBytes
	}
	credentials, err := mapCredentials(req.Credentials)
	if err != nil {
		return repository.CreateUserInput{}, err
	}
	return repository.CreateUserInput{
		Name:              strings.TrimSpace(*req.Name),
		Note:              req.Note,
		Enabled:           enabled,
		TrafficLimitBytes: limit,
		ExpireAt:          req.ExpireAt,
		Credentials:       credentials,
	}, nil
}

func mapUpdateUserInput(req userRequest, current repository.UserWithCredentials) (repository.UpdateUserInput, error) {
	name := current.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	enabled := current.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	limit := current.TrafficLimitBytes
	if req.TrafficLimitBytes != nil {
		limit = *req.TrafficLimitBytes
	}
	expireAt := current.ExpireAt
	if req.ExpireAt != nil {
		expireAt = req.ExpireAt
	}
	note := current.Note
	if req.Note != nil {
		note = req.Note
	}
	credentials := current.Credentials
	if len(req.Credentials) > 0 {
		mapped, err := mapCredentials(req.Credentials)
		if err != nil {
			return repository.UpdateUserInput{}, err
		}
		credentials = mergeCredentialsWithCurrent(mapped, current.Credentials)
	}
	return repository.UpdateUserInput{
		Name:              name,
		Note:              note,
		Enabled:           enabled,
		TrafficLimitBytes: limit,
		ExpireAt:          expireAt,
		Credentials:       credentials,
	}, nil
}

func mapCredentials(items []credentialRequest) ([]repository.Credential, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("credentials are required")
	}
	result := make([]repository.Credential, 0, len(items))
	for _, item := range items {
		protocol, err := parseProtocol(item.Protocol)
		if err != nil {
			return nil, err
		}
		result = append(result, repository.Credential{
			Protocol: protocol,
			Identity: strings.TrimSpace(item.Identity),
			Secret:   strings.TrimSpace(item.Secret),
			DataJSON: strings.TrimSpace(item.DataJSON),
		})
	}
	return result, nil
}

func mergeCredentialsWithCurrent(next []repository.Credential, current []repository.Credential) []repository.Credential {
	if len(next) == 0 {
		return next
	}
	indexed := make(map[repository.Protocol]repository.Credential, len(current))
	for _, item := range current {
		indexed[item.Protocol] = item
	}
	merged := make([]repository.Credential, 0, len(next))
	for _, item := range next {
		credential := item
		if previous, ok := indexed[item.Protocol]; ok {
			if credential.Protocol == repository.ProtocolHY2 && strings.TrimSpace(credential.Secret) == "" {
				credential.Secret = previous.Secret
			}
			if credential.Protocol == repository.ProtocolHY2 && strings.TrimSpace(credential.Identity) == "" {
				credential.Identity = previous.Identity
			}
			if credential.Protocol == repository.ProtocolVLESS && strings.TrimSpace(credential.Identity) == "" {
				credential.Identity = previous.Identity
			}
			if strings.TrimSpace(credential.DataJSON) == "" {
				credential.DataJSON = previous.DataJSON
			}
		}
		merged = append(merged, credential)
	}
	return merged
}

func mapInboundRequest(req inboundRequest, pathID string, fallbackHost string) (repository.Inbound, error) {
	protocol := repository.ProtocolHY2
	if req.Protocol != nil {
		parsed, err := parseProtocol(*req.Protocol)
		if err != nil {
			return repository.Inbound{}, err
		}
		protocol = parsed
	}
	id := strings.TrimSpace(pathID)
	if req.ID != nil {
		id = strings.TrimSpace(*req.ID)
	}
	name := ""
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	if name == "" {
		return repository.Inbound{}, fmt.Errorf("name is required")
	}
	nodeID := "local"
	if req.NodeID != nil && strings.TrimSpace(*req.NodeID) != "" {
		nodeID = strings.TrimSpace(*req.NodeID)
	}
	transport := "tcp"
	if req.Transport != nil && strings.TrimSpace(*req.Transport) != "" {
		transport = strings.ToLower(strings.TrimSpace(*req.Transport))
	}
	security := "none"
	if req.Security != nil && strings.TrimSpace(*req.Security) != "" {
		security = strings.ToLower(strings.TrimSpace(*req.Security))
	}
	host := strings.TrimSpace(fallbackHost)
	if req.Host != nil && strings.TrimSpace(*req.Host) != "" {
		host = strings.TrimSpace(*req.Host)
	}
	if host == "" {
		return repository.Inbound{}, fmt.Errorf("host is required")
	}
	port := 443
	if req.Port != nil && *req.Port > 0 {
		port = *req.Port
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	paramsJSON := "{}"
	if req.ParamsJSON != nil && strings.TrimSpace(*req.ParamsJSON) != "" {
		paramsJSON = strings.TrimSpace(*req.ParamsJSON)
	}
	runtimeJSON := "{}"
	if req.RuntimeJSON != nil && strings.TrimSpace(*req.RuntimeJSON) != "" {
		runtimeJSON = strings.TrimSpace(*req.RuntimeJSON)
	}

	return repository.Inbound{
		ID:          id,
		NodeID:      nodeID,
		Name:        name,
		Protocol:    protocol,
		Transport:   transport,
		Security:    security,
		Host:        host,
		Port:        port,
		Enabled:     enabled,
		ParamsJSON:  paramsJSON,
		RuntimeJSON: runtimeJSON,
	}, nil
}

func parseProtocolOptional(raw string) (*repository.Protocol, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := parseProtocol(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseProtocol(raw string) (repository.Protocol, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "hy2":
		return repository.ProtocolHY2, nil
	case "vless":
		return repository.ProtocolVLESS, nil
	default:
		return "", fmt.Errorf("unsupported protocol")
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeUserIDs(raw []string) []string {
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
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func resolveUnifiedAccessQRValue(
	user repository.UserWithCredentials,
	artifacts map[repository.Protocol]runtimecore.UserArtifacts,
	protocolHint string,
) string {
	candidates := make([]repository.Protocol, 0, 4)

	if parsed, err := parseProtocolOptional(protocolHint); err == nil && parsed != nil {
		candidates = append(candidates, *parsed)
	}
	for _, credential := range user.Credentials {
		candidates = append(candidates, credential.Protocol)
	}
	candidates = append(candidates, repository.ProtocolVLESS, repository.ProtocolHY2)

	seen := make(map[repository.Protocol]struct{}, len(candidates))
	for _, protocol := range candidates {
		if _, ok := seen[protocol]; ok {
			continue
		}
		seen[protocol] = struct{}{}
		raw, ok := artifacts[protocol]
		if !ok {
			continue
		}
		uri := strings.TrimSpace(raw.AccessURI)
		if uri != "" {
			return uri
		}
	}
	return ""
}
