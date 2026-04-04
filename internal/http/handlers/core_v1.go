package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"h2v2/internal/core"
	"h2v2/internal/http/render"
)

type coreServerRequest struct {
	ID                  *string `json:"id"`
	Name                *string `json:"name"`
	PublicHost          *string `json:"public_host"`
	PanelPublicURL      *string `json:"panel_public_url"`
	SubscriptionBaseURL *string `json:"subscription_base_url"`
	SingBoxBinaryPath   *string `json:"singbox_binary_path"`
	SingBoxConfigPath   *string `json:"singbox_config_path"`
	SingBoxServiceName  *string `json:"singbox_service_name"`
}

type coreInboundRequest struct {
	ID          *string                         `json:"id"`
	ServerID    *string                         `json:"server_id"`
	Name        *string                         `json:"name"`
	Tag         *string                         `json:"tag"`
	Protocol    *string                         `json:"protocol"`
	Listen      *string                         `json:"listen"`
	ListenPort  *int                            `json:"listen_port"`
	Enabled     *bool                           `json:"enabled"`
	TemplateKey *string                         `json:"template_key"`
	VLESS       *core.VLESSInboundSettings      `json:"vless"`
	Hysteria2   *core.Hysteria2InboundSettings  `json:"hysteria2"`
}

type coreUserRequest struct {
	ID                *string    `json:"id"`
	Username          *string    `json:"username"`
	Enabled           *bool      `json:"enabled"`
	TrafficLimitBytes *int64     `json:"traffic_limit_bytes"`
	ExpireAt          *time.Time `json:"expire_at"`
}

type coreAccessRequest struct {
	ID                        *string    `json:"id"`
	UserID                    *string    `json:"user_id"`
	InboundID                 *string    `json:"inbound_id"`
	Enabled                   *bool      `json:"enabled"`
	VLESSUUID                 *string    `json:"vless_uuid"`
	VLESSFlowOverride         *string    `json:"vless_flow_override"`
	Hysteria2Password         *string    `json:"hysteria2_password"`
	TrafficLimitBytesOverride *int64     `json:"traffic_limit_bytes_override"`
	ExpireAtOverride          *time.Time `json:"expire_at_override"`
}

type coreTokenRequest struct {
	ExpiresAt *time.Time `json:"expires_at"`
}

func (h *Handler) ensureCoreService(w http.ResponseWriter) *core.Service {
	if h.coreService == nil {
		h.renderError(w, http.StatusServiceUnavailable, "service", "core service is not configured", nil)
		return nil
	}
	return h.coreService
}

func parseCoreProtocol(value string) (core.InboundProtocol, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "vless":
		return core.InboundProtocolVLESS, nil
	case "hysteria2", "hy2":
		return core.InboundProtocolHysteria2, nil
	default:
		return "", fmt.Errorf("unsupported protocol")
	}
}

func pointerOrString(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return strings.TrimSpace(*value)
}

func pointerOrBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func pointerOrInt(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func pointerOrInt64(value *int64, fallback int64) int64 {
	if value == nil {
		return fallback
	}
	return *value
}

func decodeOptionalJSONBody(r *http.Request, dst any) error {
	if r.ContentLength == 0 {
		return nil
	}
	return render.DecodeJSON(r, dst)
}

func coreErrorStatus(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}
	if core.IsNotFound(err) {
		return http.StatusNotFound, "not_found"
	}
	if core.IsConflict(err) {
		return http.StatusConflict, "validation"
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "invalid") || strings.Contains(message, "required") || strings.Contains(message, "unsupported") || strings.Contains(message, "must be") {
		return http.StatusBadRequest, "validation"
	}
	return http.StatusInternalServerError, "runtime"
}

func normalizeCoreAccessProtocol(protocol core.InboundProtocol) string {
	switch protocol {
	case core.InboundProtocolVLESS:
		return "vless"
	case core.InboundProtocolHysteria2:
		return "hy2"
	default:
		return ""
	}
}

func collectCoreServerIDs(serverIDs ...string) []string {
	if len(serverIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(serverIDs))
	items := make([]string, 0, len(serverIDs))
	for _, raw := range serverIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		items = append(items, id)
	}
	return items
}

func (h *Handler) applyCoreServerConfigs(ctx context.Context, service *core.Service, serverIDs ...string) error {
	if service == nil {
		return fmt.Errorf("core service is not configured")
	}
	ids := collectCoreServerIDs(serverIDs...)
	if len(ids) == 0 {
		servers, err := service.ListServers(ctx)
		if err != nil {
			return err
		}
		for _, item := range servers {
			if strings.TrimSpace(item.ID) == "" {
				continue
			}
			ids = append(ids, item.ID)
		}
	}
	for _, serverID := range ids {
		rendered, err := service.RenderServerConfig(ctx, serverID, nil)
		if err != nil {
			return err
		}
		if _, err := service.ApplyServerConfig(ctx, serverID, rendered.Revision.ID); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) buildInboundProtocolMap(ctx context.Context, service *core.Service) (map[string]core.InboundProtocol, map[string]string, error) {
	items, err := service.ListInbounds(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	protocolByInboundID := make(map[string]core.InboundProtocol, len(items))
	serverByInboundID := make(map[string]string, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		protocolByInboundID[id] = item.Protocol
		serverByInboundID[id] = strings.TrimSpace(item.ServerID)
	}
	return protocolByInboundID, serverByInboundID, nil
}

func buildCoreAccessPayload(items []core.UserAccess, protocolByInboundID map[string]core.InboundProtocol) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		protocol := normalizeCoreAccessProtocol(protocolByInboundID[strings.TrimSpace(item.InboundID)])
		result = append(result, map[string]any{
			"id":                           item.ID,
			"user_id":                      item.UserID,
			"inbound_id":                   item.InboundID,
			"enabled":                      item.Enabled,
			"protocol":                     protocol,
			"vless_uuid":                   item.VLESSUUID,
			"vless_flow_override":          item.VLESSFlowOverride,
			"hysteria2_password":           item.Hysteria2Password,
			"traffic_limit_bytes_override": item.TrafficLimitBytesOverride,
			"expire_at_override":           item.ExpireAtOverride,
			"created_at":                   item.CreatedAt,
			"updated_at":                   item.UpdatedAt,
		})
	}
	return result
}
func (h *Handler) CoreDefaults(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, map[string]any{
		"hy2_port":      h.cfg.HY2Port,
		"hy2_domain":    h.cfg.HY2Domain,
		"hy2_cert_path": h.cfg.HY2CertPath,
		"hy2_key_path":  h.cfg.HY2KeyPath,
	})
}

func (h *Handler) ListCoreServers(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	items, err := service.ListServers(r.Context())
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list servers", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateCoreServer(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreServerRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	server := core.Server{
		ID:                  pointerOrString(req.ID, ""),
		Name:                pointerOrString(req.Name, ""),
		PublicHost:          pointerOrString(req.PublicHost, ""),
		PanelPublicURL:      pointerOrString(req.PanelPublicURL, ""),
		SubscriptionBaseURL: pointerOrString(req.SubscriptionBaseURL, ""),
		SingBoxBinaryPath:   pointerOrString(req.SingBoxBinaryPath, ""),
		SingBoxConfigPath:   pointerOrString(req.SingBoxConfigPath, ""),
		SingBoxServiceName:  pointerOrString(req.SingBoxServiceName, ""),
	}
	created, err := service.UpsertServer(r.Context(), server)
	if err != nil {
		status := http.StatusInternalServerError
		errorType := "runtime"
		if core.IsConflict(err) {
			status = http.StatusConflict
			errorType = "validation"
		}
		h.renderError(w, status, errorType, err.Error(), nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service, created.ID); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusCreated, created)
}

func (h *Handler) GetCoreServer(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	item, err := service.GetServer(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "server not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load server", nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpdateCoreServer(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	current, err := service.GetServer(r.Context(), id)
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "server not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load server", nil)
		return
	}
	var req coreServerRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	updated := current
	if req.Name != nil {
		updated.Name = pointerOrString(req.Name, updated.Name)
	}
	if req.PublicHost != nil {
		updated.PublicHost = pointerOrString(req.PublicHost, updated.PublicHost)
	}
	if req.PanelPublicURL != nil {
		updated.PanelPublicURL = pointerOrString(req.PanelPublicURL, updated.PanelPublicURL)
	}
	if req.SubscriptionBaseURL != nil {
		updated.SubscriptionBaseURL = pointerOrString(req.SubscriptionBaseURL, updated.SubscriptionBaseURL)
	}
	if req.SingBoxBinaryPath != nil {
		updated.SingBoxBinaryPath = pointerOrString(req.SingBoxBinaryPath, updated.SingBoxBinaryPath)
	}
	if req.SingBoxConfigPath != nil {
		updated.SingBoxConfigPath = pointerOrString(req.SingBoxConfigPath, updated.SingBoxConfigPath)
	}
	if req.SingBoxServiceName != nil {
		updated.SingBoxServiceName = pointerOrString(req.SingBoxServiceName, updated.SingBoxServiceName)
	}
	item, err := service.UpsertServer(r.Context(), updated)
	if err != nil {
		status := http.StatusInternalServerError
		errorType := "runtime"
		if core.IsConflict(err) {
			status = http.StatusConflict
			errorType = "validation"
		}
		h.renderError(w, status, errorType, err.Error(), nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service, item.ID); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCoreServer(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	serverID := strings.TrimSpace(chi.URLParam(r, "id"))
	if err := service.DeleteServer(r.Context(), serverID); err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "server not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to delete server", nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) ListCoreInbounds(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	items, err := service.ListInbounds(r.Context(), strings.TrimSpace(r.URL.Query().Get("server_id")))
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list inbounds", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateCoreInbound(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreInboundRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	protocol, err := parseCoreProtocol(pointerOrString(req.Protocol, ""))
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	inbound := core.Inbound{
		ID:          pointerOrString(req.ID, ""),
		ServerID:    pointerOrString(req.ServerID, ""),
		Name:        pointerOrString(req.Name, ""),
		Tag:         pointerOrString(req.Tag, ""),
		Protocol:    protocol,
		Listen:      pointerOrString(req.Listen, ""),
		ListenPort:  pointerOrInt(req.ListenPort, 443),
		Enabled:     pointerOrBool(req.Enabled, true),
		TemplateKey: pointerOrString(req.TemplateKey, ""),
		VLESS:       req.VLESS,
		Hysteria2:   req.Hysteria2,
	}
	saved, err := service.UpsertInbound(r.Context(), inbound)
	if err != nil {
		status, errorType := coreErrorStatus(err)
		h.renderError(w, status, errorType, err.Error(), nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service, saved.ServerID); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) GetCoreInbound(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	item, err := service.GetInbound(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "inbound not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load inbound", nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpdateCoreInbound(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	current, err := service.GetInbound(r.Context(), id)
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "inbound not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load inbound", nil)
		return
	}
	var req coreInboundRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	updated := current
	if req.ServerID != nil {
		updated.ServerID = pointerOrString(req.ServerID, updated.ServerID)
	}
	if req.Name != nil {
		updated.Name = pointerOrString(req.Name, updated.Name)
	}
	if req.Tag != nil {
		updated.Tag = pointerOrString(req.Tag, updated.Tag)
	}
	if req.Protocol != nil {
		protocol, err := parseCoreProtocol(*req.Protocol)
		if err != nil {
			h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
			return
		}
		updated.Protocol = protocol
	}
	if req.Listen != nil {
		updated.Listen = pointerOrString(req.Listen, updated.Listen)
	}
	if req.ListenPort != nil {
		updated.ListenPort = pointerOrInt(req.ListenPort, updated.ListenPort)
	}
	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}
	if req.TemplateKey != nil {
		updated.TemplateKey = pointerOrString(req.TemplateKey, updated.TemplateKey)
	}
	if req.VLESS != nil {
		updated.VLESS = req.VLESS
	}
	if req.Hysteria2 != nil {
		updated.Hysteria2 = req.Hysteria2
	}
	saved, err := service.UpsertInbound(r.Context(), updated)
	if err != nil {
		status, errorType := coreErrorStatus(err)
		h.renderError(w, status, errorType, err.Error(), nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service, saved.ServerID); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, saved)
}

func (h *Handler) DeleteCoreInbound(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	current, err := service.GetInbound(r.Context(), id)
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "inbound not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load inbound", nil)
		return
	}
	if err := service.DeleteInbound(r.Context(), id); err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "inbound not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to delete inbound", nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service, current.ServerID); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) ListCoreUsers(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	items, err := service.ListUsers(r.Context())
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list users", nil)
		return
	}
	includeAccess := true
	rawInclude := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("include_access")))
	if rawInclude == "0" || rawInclude == "false" || rawInclude == "no" {
		includeAccess = false
	}
	if !includeAccess {
		render.JSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}

	protocolByInboundID, _, err := h.buildInboundProtocolMap(r.Context(), service)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load inbounds", nil)
		return
	}

	payload := make([]map[string]any, 0, len(items))
	for _, user := range items {
		accessItems, accessErr := service.ListUserAccess(r.Context(), user.ID)
		if accessErr != nil {
			h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list access", nil)
			return
		}
		payload = append(payload, map[string]any{
			"user":   user,
			"access": buildCoreAccessPayload(accessItems, protocolByInboundID),
		})
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": payload})
}

func (h *Handler) ProvisionCoreUser(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreUserRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	username := strings.TrimSpace(pointerOrString(req.Username, ""))
	if username == "" {
		h.renderError(w, http.StatusBadRequest, "validation", "username is required", nil)
		return
	}
	user, accessItems, serverID, err := service.ProvisionUser(
		r.Context(),
		username,
		pointerOrInt64(req.TrafficLimitBytes, 0),
		req.ExpireAt,
	)
	if err != nil {
		status := http.StatusInternalServerError
		errorType := "runtime"
		if core.IsConflict(err) {
			status = http.StatusConflict
			errorType = "validation"
		}
		h.renderError(w, status, errorType, err.Error(), nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service, serverID); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	protocolByInboundID, _, err := h.buildInboundProtocolMap(r.Context(), service)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load inbounds", nil)
		return
	}
	render.JSON(w, http.StatusCreated, map[string]any{
		"user":   user,
		"access": buildCoreAccessPayload(accessItems, protocolByInboundID),
	})
}

func (h *Handler) CreateCoreUser(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreUserRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	user := core.User{
		ID:                pointerOrString(req.ID, ""),
		Username:          pointerOrString(req.Username, ""),
		Enabled:           pointerOrBool(req.Enabled, true),
		TrafficLimitBytes: pointerOrInt64(req.TrafficLimitBytes, 0),
		ExpireAt:          req.ExpireAt,
	}
	created, err := service.UpsertUser(r.Context(), user)
	if err != nil {
		status := http.StatusInternalServerError
		errorType := "runtime"
		if core.IsConflict(err) {
			status = http.StatusConflict
			errorType = "validation"
		}
		h.renderError(w, status, errorType, err.Error(), nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusCreated, created)
}

func (h *Handler) GetCoreUser(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	item, err := service.GetUser(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load user", nil)
		return
	}
	render.JSON(w, http.StatusOK, item)
}

func (h *Handler) UpdateCoreUser(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	current, err := service.GetUser(r.Context(), id)
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load user", nil)
		return
	}
	var req coreUserRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	updated := current
	if req.Username != nil {
		updated.Username = pointerOrString(req.Username, updated.Username)
	}
	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}
	if req.TrafficLimitBytes != nil {
		updated.TrafficLimitBytes = *req.TrafficLimitBytes
	}
	if req.ExpireAt != nil {
		updated.ExpireAt = req.ExpireAt
	}
	saved, err := service.UpsertUser(r.Context(), updated)
	if err != nil {
		status := http.StatusInternalServerError
		errorType := "runtime"
		if core.IsConflict(err) {
			status = http.StatusConflict
			errorType = "validation"
		}
		h.renderError(w, status, errorType, err.Error(), nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, saved)
}

func (h *Handler) DeleteCoreUser(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	if err := service.DeleteUser(r.Context(), chi.URLParam(r, "id")); err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to delete user", nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
func (h *Handler) ListCoreUserAccess(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	items, err := service.ListUserAccess(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list access", nil)
		return
	}
	protocolByInboundID, _, err := h.buildInboundProtocolMap(r.Context(), service)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load inbounds", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": buildCoreAccessPayload(items, protocolByInboundID)})
}

func (h *Handler) UpsertCoreAccess(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreAccessRequest
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	access := core.UserAccess{
		ID:                pointerOrString(req.ID, ""),
		UserID:            pointerOrString(req.UserID, ""),
		InboundID:         pointerOrString(req.InboundID, ""),
		Enabled:           pointerOrBool(req.Enabled, true),
		VLESSUUID:         pointerOrString(req.VLESSUUID, ""),
		VLESSFlowOverride: pointerOrString(req.VLESSFlowOverride, ""),
		Hysteria2Password: pointerOrString(req.Hysteria2Password, ""),
	}
	if req.TrafficLimitBytesOverride != nil {
		value := *req.TrafficLimitBytesOverride
		access.TrafficLimitBytesOverride = &value
	}
	if req.ExpireAtOverride != nil {
		value := req.ExpireAtOverride.UTC()
		access.ExpireAtOverride = &value
	}
	saved, err := service.UpsertUserAccess(r.Context(), access)
	if err != nil {
		status := http.StatusInternalServerError
		errorType := "runtime"
		if core.IsConflict(err) {
			status = http.StatusConflict
			errorType = "validation"
		}
		h.renderError(w, status, errorType, err.Error(), nil)
		return
	}
	protocolByInboundID, serverByInboundID, err := h.buildInboundProtocolMap(r.Context(), service)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load inbounds", nil)
		return
	}
	serverID := strings.TrimSpace(serverByInboundID[strings.TrimSpace(saved.InboundID)])
	if err := h.applyCoreServerConfigs(r.Context(), service, serverID); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	payload := buildCoreAccessPayload([]core.UserAccess{saved}, protocolByInboundID)
	render.JSON(w, http.StatusOK, payload[0])
}

func (h *Handler) DeleteCoreAccess(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	accessID := strings.TrimSpace(chi.URLParam(r, "id"))
	current, err := service.GetUserAccess(r.Context(), accessID)
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "access not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load access", nil)
		return
	}
	inbound, err := service.GetInbound(r.Context(), current.InboundID)
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "inbound not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load inbound", nil)
		return
	}
	if err := service.DeleteUserAccess(r.Context(), accessID); err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "access not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to delete access", nil)
		return
	}
	if err := h.applyCoreServerConfigs(r.Context(), service, inbound.ServerID); err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) CoreUserArtifacts(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	artifacts, err := service.BuildUserArtifacts(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to build artifacts", nil)
		return
	}
	render.JSON(w, http.StatusOK, artifacts)
}

func (h *Handler) ListCoreUserTokens(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	subscription, tokens, err := service.ListSubscriptionTokensByUser(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list subscription tokens", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"subscription": subscription, "tokens": tokens})
}

func (h *Handler) IssueCoreUserToken(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreTokenRequest
	if err := decodeOptionalJSONBody(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	subscription, issued, err := service.IssueAdditionalSubscriptionTokenByUser(r.Context(), chi.URLParam(r, "id"), req.ExpiresAt)
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to issue token", nil)
		return
	}
	base := strings.TrimRight(h.cfg.SubscriptionPublicURL, "/")
	if base == "" {
		base = strings.TrimRight(h.cfg.PublicPanelURL, "/")
	}
	profileURL := base + "/sub/" + issued.PlaintextToken + "/profile.singbox.json"
	render.JSON(w, http.StatusCreated, map[string]any{
		"subscription": subscription,
		"token":        issued.Token,
		"token_value":  issued.PlaintextToken,
		"profile_url":  profileURL,
	})
}

func (h *Handler) RotateCoreUserToken(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	var req coreTokenRequest
	if err := decodeOptionalJSONBody(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	subscription, issued, err := service.RotateSubscriptionTokenByUser(r.Context(), chi.URLParam(r, "id"), req.ExpiresAt)
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to rotate token", nil)
		return
	}
	base := strings.TrimRight(h.cfg.SubscriptionPublicURL, "/")
	if base == "" {
		base = strings.TrimRight(h.cfg.PublicPanelURL, "/")
	}
	profileURL := base + "/sub/" + issued.PlaintextToken + "/profile.singbox.json"
	render.JSON(w, http.StatusOK, map[string]any{
		"subscription": subscription,
		"token":        issued.Token,
		"token_value":  issued.PlaintextToken,
		"profile_url":  profileURL,
	})
}

func (h *Handler) RevokeCoreUserTokens(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	subscription, err := service.RevokeSubscriptionTokensByUser(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to revoke tokens", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"subscription": subscription, "revoked": true})
}

func (h *Handler) RenderCoreServerConfig(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	result, err := service.RenderServerConfig(r.Context(), chi.URLParam(r, "id"), nil)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, result)
}

func (h *Handler) ValidateCoreServerConfig(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	revisionID := strings.TrimSpace(r.URL.Query().Get("revision_id"))
	revision, err := service.ValidateServerConfig(r.Context(), chi.URLParam(r, "id"), revisionID)
	if err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true, "revision": revision})
}

func (h *Handler) ApplyCoreServerConfig(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	revisionID := strings.TrimSpace(r.URL.Query().Get("revision_id"))
	revision, err := service.ApplyServerConfig(r.Context(), chi.URLParam(r, "id"), revisionID)
	if err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true, "revision": revision})
}

func (h *Handler) ListCoreServerRevisions(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := service.ListServerConfigRevisions(r.Context(), chi.URLParam(r, "id"), limit)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to list revisions", nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) RollbackCoreServerConfig(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	revision, err := service.RollbackServerConfig(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "revisionID"))
	if err != nil {
		h.renderError(w, http.StatusBadGateway, "service", err.Error(), nil)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true, "revision": revision})
}

func (h *Handler) GetCoreServerConfigPreview(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	configBytes, checkWarning, err := service.PreviewServerConfig(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "server not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", err.Error(), nil)
		return
	}
	resp := map[string]any{
		"config_json": string(configBytes),
	}
	if checkWarning != "" {
		resp["check_warning"] = checkWarning
	}
	render.JSON(w, http.StatusOK, resp)
}

func (h *Handler) CoreSubscriptionProfile(w http.ResponseWriter, r *http.Request) {
	h.renderCoreSubscription(w, r, "profile")
}

func (h *Handler) CoreSubscriptionURIs(w http.ResponseWriter, r *http.Request) {
	h.renderCoreSubscription(w, r, "uris")
}

func (h *Handler) CoreSubscriptionQR(w http.ResponseWriter, r *http.Request) {
	h.renderCoreSubscription(w, r, "qr")
}

func (h *Handler) CoreSubscriptionClash(w http.ResponseWriter, r *http.Request) {
	h.renderCoreSubscription(w, r, "clash")
}

func (h *Handler) CoreSubscriptionBase64(w http.ResponseWriter, r *http.Request) {
	h.renderCoreSubscription(w, r, "base64")
}

func (h *Handler) CoreSubscriptionAuto(w http.ResponseWriter, r *http.Request) {
	kind := core.DetectSubscriptionKind(r.Header.Get("User-Agent"))
	h.renderCoreSubscription(w, r, kind)
}

func (h *Handler) renderCoreSubscription(w http.ResponseWriter, r *http.Request, kind string) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	content, err := service.RenderSubscriptionContentByToken(r.Context(), token, kind, h.requestIP(r), r.Header.Get("If-None-Match"))
	if err != nil {
		switch {
		case err == core.ErrRateLimited:
			h.renderError(w, http.StatusTooManyRequests, "rate_limit", "too many requests", nil)
		case err == core.ErrInvalidToken || err == core.ErrTokenRevoked || core.IsNotFound(err):
			h.renderError(w, http.StatusNotFound, "not_found", "subscription not found", nil)
		default:
			h.renderError(w, http.StatusInternalServerError, "runtime", "failed to render subscription", nil)
		}
		return
	}
	for key, value := range content.Headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		w.Header().Set(key, value)
	}
	if content.ETag != "" {
		w.Header().Set("ETag", content.ETag)
	}
	if content.StatusCode == http.StatusNotModified {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if content.ContentType != "" {
		w.Header().Set("Content-Type", content.ContentType)
	}
	if content.FileName != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, content.FileName))
	}
	if content.StatusCode <= 0 {
		content.StatusCode = http.StatusOK
	}
	w.WriteHeader(content.StatusCode)
	_, _ = w.Write(content.Body)
}

func (h *Handler) CoreUserProfileRaw(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	artifacts, err := service.BuildUserArtifacts(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to build profile", nil)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(artifacts.SingBoxProfileJSON))
}

func (h *Handler) CoreUserQR(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	if rawValue := strings.TrimSpace(r.URL.Query().Get("value")); rawValue != "" {
		if err := renderQRCodePNG(w, rawValue, parseQRSize(r.URL.Query().Get("size"), 320)); err != nil {
			h.renderError(w, http.StatusInternalServerError, "runtime", "failed to render qr", nil)
		}
		return
	}
	artifacts, err := service.BuildUserArtifacts(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to build qr", nil)
		return
	}
	qrKind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
	qrValue := strings.TrimSpace(artifacts.SubscriptionImportURL)
	if qrKind == "access" {
		qrValue = ""
		if len(artifacts.VLESSURIs) > 0 {
			qrValue = strings.TrimSpace(artifacts.VLESSURIs[0])
		}
		if qrValue == "" && len(artifacts.Hysteria2URIs) > 0 {
			qrValue = strings.TrimSpace(artifacts.Hysteria2URIs[0])
		}
		if qrValue == "" && len(artifacts.AllURIs) > 0 {
			qrValue = strings.TrimSpace(artifacts.AllURIs[0])
		}
	}
	if qrValue == "" {
		h.renderError(w, http.StatusNotFound, "not_found", "qr source is empty", nil)
		return
	}
	if err := renderQRCodePNG(w, qrValue, parseQRSize(r.URL.Query().Get("size"), 320)); err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to render qr", nil)
	}
}

func (h *Handler) CoreUserURIsRaw(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	artifacts, err := service.BuildUserArtifacts(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to build uris", nil)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(strings.Join(artifacts.AllURIs, "\n") + "\n"))
}

func (h *Handler) CoreUserProfileJSON(w http.ResponseWriter, r *http.Request) {
	service := h.ensureCoreService(w)
	if service == nil {
		return
	}
	artifacts, err := service.BuildUserArtifacts(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if core.IsNotFound(err) {
			h.renderError(w, http.StatusNotFound, "not_found", "user not found", nil)
			return
		}
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to build profile", nil)
		return
	}
	var payload any
	if err := json.Unmarshal([]byte(artifacts.SingBoxProfileJSON), &payload); err != nil {
		render.JSON(w, http.StatusOK, map[string]any{"profile_json": artifacts.SingBoxProfileJSON})
		return
	}
	render.JSON(w, http.StatusOK, payload)
}
