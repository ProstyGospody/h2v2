package handlers

import (
	"net/http"
	"strings"

	auditdomain "proxy-panel/internal/domain/audit"
	"proxy-panel/internal/http/render"
	"proxy-panel/internal/services"
)

type hysteriaSettingsValidateResponse struct {
	Settings           services.Hy2Settings           `json:"settings"`
	SettingsValidation services.Hy2SettingsValidation `json:"settings_validation"`
	ConfigValidation   services.Hy2ConfigValidation   `json:"config_validation"`
	RawYAML            string                         `json:"raw_yaml"`
	AccessMode         string                         `json:"access_mode"`
	AccessWarning      string                         `json:"access_warning"`
}

func (h *Handler) GetHysteriaSettings(w http.ResponseWriter, r *http.Request) {
	if h.hy2ConfigManager == nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "hysteria config manager is not configured", nil)
		return
	}
	content, err := h.hy2ConfigManager.Read()
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to read hysteria config", nil)
		return
	}
	settings, parseErr := h.hy2ConfigManager.ExtractSettingsWithError(content, h.cfg.Hy2Domain, h.cfg.Hy2Port)
	if parseErr != nil {
		settings = h.hy2ConfigManager.ExtractSettings("", h.cfg.Hy2Domain, h.cfg.Hy2Port)
	}
	settings.Auth = services.Hy2ServerAuth{Type: "userpass", UserPass: map[string]string{}}
	settingsValidation := h.hy2ConfigManager.ValidateSettings(settings)
	configValidation := h.hy2ConfigManager.Validate(content)
	if parseErr != nil {
		configValidation.Valid = false
		configValidation.Errors = append(configValidation.Errors, "failed to parse current YAML into structured settings")
	}
	render.JSON(w, http.StatusOK, map[string]any{
		"path":                h.cfg.Hy2ConfigPath,
		"raw_yaml":            content,
		"settings":            settings,
		"settings_validation": settingsValidation,
		"config_validation":   configValidation,
		"raw_only_paths":      configValidation.RawOnlyPaths,
		"access_mode":         "userpass",
		"access_warning":      "Access credentials are managed from Hysteria Users. Any auth changes in raw YAML will be overwritten on save/apply.",
	})
}

func (h *Handler) ValidateHysteriaSettings(w http.ResponseWriter, r *http.Request) {
	if h.hy2ConfigManager == nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "hysteria config manager is not configured", nil)
		return
	}
	var req services.Hy2Settings
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	req.Auth = services.Hy2ServerAuth{Type: "userpass", UserPass: map[string]string{}}
	current, err := h.hy2ConfigManager.Read()
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to read hysteria config", nil)
		return
	}
	next, settingsValidation := h.hy2ConfigManager.ApplySettings(current, req)
	if next, err = h.hysteriaAccess.InjectManagedAuth(r.Context(), next); err != nil {
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to inject managed hysteria auth", nil)
		return
	}
	configValidation := h.hy2ConfigManager.Validate(next)
	previewSettings, parseErr := h.hy2ConfigManager.ExtractSettingsWithError(next, h.cfg.Hy2Domain, h.cfg.Hy2Port)
	if parseErr != nil {
		previewSettings = req
	}
	previewSettings.Auth = services.Hy2ServerAuth{Type: "userpass", UserPass: map[string]string{}}
	render.JSON(w, http.StatusOK, hysteriaSettingsValidateResponse{
		Settings:           previewSettings,
		SettingsValidation: settingsValidation,
		ConfigValidation:   configValidation,
		RawYAML:            next,
		AccessMode:         "userpass",
		AccessWarning:      "Managed Hysteria users overwrite auth configuration during sync.",
	})
}

func (h *Handler) SaveHysteriaSettings(w http.ResponseWriter, r *http.Request) {
	if h.hy2ConfigManager == nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "hysteria config manager is not configured", nil)
		return
	}
	var req services.Hy2Settings
	if err := render.DecodeJSON(r, &req); err != nil {
		h.renderError(w, http.StatusBadRequest, "validation", "invalid request body", nil)
		return
	}
	req.Auth = services.Hy2ServerAuth{Type: "userpass", UserPass: map[string]string{}}
	current, err := h.hy2ConfigManager.Read()
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to read hysteria config", nil)
		return
	}
	next, settingsValidation := h.hy2ConfigManager.ApplySettings(current, req)
	if !settingsValidation.Valid {
		h.renderError(w, http.StatusBadRequest, "validation", "settings validation failed", settingsValidation)
		return
	}
	if next, err = h.hysteriaAccess.InjectManagedAuth(r.Context(), next); err != nil {
		h.renderError(w, http.StatusInternalServerError, "sync", "failed to inject managed hysteria auth", nil)
		return
	}
	configValidation := h.hy2ConfigManager.Validate(next)
	if !configValidation.Valid {
		h.renderError(w, http.StatusBadRequest, "validation", "generated config is invalid", configValidation)
		return
	}
	backupPath, err := h.hy2ConfigManager.Save(next)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to save hysteria settings", nil)
		return
	}
	updated, parseErr := h.hy2ConfigManager.ExtractSettingsWithError(next, h.cfg.Hy2Domain, h.cfg.Hy2Port)
	if parseErr != nil {
		updated = req
	}
	updated.Auth = services.Hy2ServerAuth{Type: "userpass", UserPass: map[string]string{}}
	h.audit(r, "hysteria.settings.save", auditdomain.EntityHysteriaConfig, nil, map[string]any{
		"path":   h.cfg.Hy2ConfigPath,
		"backup": backupPath,
		"listen": updated.Listen,
	})
	render.JSON(w, http.StatusOK, map[string]any{
		"ok":                  true,
		"path":                h.cfg.Hy2ConfigPath,
		"backup_path":         backupPath,
		"raw_yaml":            next,
		"settings":            updated,
		"settings_validation": settingsValidation,
		"config_validation":   configValidation,
		"raw_only_paths":      configValidation.RawOnlyPaths,
		"access_mode":         "userpass",
		"access_warning":      "Managed Hysteria users overwrite auth configuration during sync.",
	})
}

func (h *Handler) ApplyHysteriaSettings(w http.ResponseWriter, r *http.Request) {
	if h.hy2ConfigManager == nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "hysteria config manager is not configured", nil)
		return
	}
	if _, err := h.hysteriaAccess.Sync(r.Context()); err != nil {
		h.renderError(w, http.StatusBadRequest, "sync", "failed to synchronize hysteria config", nil)
		return
	}
	content, err := h.hy2ConfigManager.Read()
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to read hysteria config", nil)
		return
	}
	configValidation := h.hy2ConfigManager.Validate(content)
	if !configValidation.Valid {
		h.renderError(w, http.StatusBadRequest, "validation", "current hysteria config is invalid", configValidation)
		return
	}
	if err := h.serviceManager.Restart(r.Context(), "hysteria-server"); err != nil {
		h.renderError(w, http.StatusBadRequest, "service", "failed to restart hysteria-server: "+strings.TrimSpace(err.Error()), nil)
		return
	}
	status, statusErr := h.serviceManager.Status(r.Context(), "hysteria-server")
	if statusErr == nil {
		_ = h.repo.UpsertServiceState(r.Context(), "hysteria-server", status.StatusText, nil, h.serviceManager.ToJSON(status))
	}
	h.audit(r, "hysteria.settings.apply", auditdomain.EntityService, nil, map[string]any{"service": "hysteria-server"})
	if statusErr != nil {
		render.JSON(w, http.StatusOK, map[string]any{"ok": true, "config_validation": configValidation})
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"ok": true, "config_validation": configValidation, "service": status})
}
