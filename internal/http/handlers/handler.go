package handlers

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"proxy-panel/internal/config"
	"proxy-panel/internal/http/middleware"
	"proxy-panel/internal/http/render"
	"proxy-panel/internal/repository"
	"proxy-panel/internal/services"
)

type Handler struct {
	cfg              config.Config
	logger           *slog.Logger
	repo             *repository.Repository
	rateLimiter      *middleware.LoginRateLimiter
	hy2Client        *services.HysteriaClient
	serviceManager   *services.ServiceManager
	hy2ConfigManager *services.HysteriaConfigManager
	hysteriaAccess   *services.HysteriaAccessManager
	systemMetrics    *services.SystemMetricsCollector
	protocolMu       sync.Mutex
	protocolSample   protocolPacketSample
	networkMu        sync.Mutex
	networkSample    networkByteSample
}

type protocolPacketSample struct {
	tcpPackets  int64
	udpPackets  int64
	collectedAt time.Time
}

type networkByteSample struct {
	rxBytes     int64
	txBytes     int64
	collectedAt time.Time
}

type systemTrendSample struct {
	Timestamp         time.Time `json:"timestamp"`
	CPUUsagePercent   float64   `json:"cpu_usage_percent"`
	MemoryUsedPercent float64   `json:"memory_used_percent"`
	NetworkRxBps      float64   `json:"network_rx_bps"`
	NetworkTxBps      float64   `json:"network_tx_bps"`
}

func New(
	cfg config.Config,
	logger *slog.Logger,
	repo *repository.Repository,
	rateLimiter *middleware.LoginRateLimiter,
	hy2Client *services.HysteriaClient,
	serviceManager *services.ServiceManager,
	hy2ConfigManager *services.HysteriaConfigManager,
	hysteriaAccess *services.HysteriaAccessManager,
	systemMetrics *services.SystemMetricsCollector,
) *Handler {
	return &Handler{
		cfg:              cfg,
		logger:           logger,
		repo:             repo,
		rateLimiter:      rateLimiter,
		hy2Client:        hy2Client,
		serviceManager:   serviceManager,
		hy2ConfigManager: hy2ConfigManager,
		hysteriaAccess:   hysteriaAccess,
		systemMetrics:    systemMetrics,
	}
}

func (h *Handler) setAuthCookies(w http.ResponseWriter, sessionToken string, csrfToken string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
		Secure:   h.cfg.SecureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.CSRFCookieName,
		Value:    csrfToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
		Secure:   h.cfg.SecureCookies,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   h.cfg.SecureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.CSRFCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   h.cfg.SecureCookies,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) requestIP(r *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			items := strings.Split(value, ",")
			if len(items) > 0 {
				value = strings.TrimSpace(items[0])
			}
		}
		if host, _, err := net.SplitHostPort(value); err == nil {
			value = host
		}
		if value != "" {
			return value
		}
	}
	remote := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remote); err == nil {
		return host
	}
	return remote
}

func (h *Handler) parsePagination(r *http.Request) (limit int, offset int) {
	limit = 50
	offset = 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return
}

func (h *Handler) audit(r *http.Request, action string, entityType string, entityID *string, payload any) {
	admin, ok := middleware.AdminFromContext(r.Context())
	var adminID *string
	if ok {
		adminID = &admin.ID
	}
	if err := h.repo.InsertAuditLog(r.Context(), adminID, action, entityType, entityID, payload); err != nil {
		h.logger.Warn("audit insert failed", "error", err, "action", action)
	}
}

func (h *Handler) renderError(w http.ResponseWriter, status int, errorType string, message string, details any) {
	payload := map[string]any{
		"error":      message,
		"error_type": strings.TrimSpace(errorType),
	}
	if details != nil {
		payload["details"] = details
	}
	render.JSON(w, status, payload)
}
