package handlers

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"h2v2/internal/config"
	"h2v2/internal/core"
	"h2v2/internal/http/middleware"
	"h2v2/internal/http/render"
	"h2v2/internal/repository"
	"h2v2/internal/services"
)

type Handler struct {
	cfg              config.Config
	logger           *slog.Logger
	repo             repository.Repository
	rateLimiter      *middleware.LoginRateLimiter
	serviceManager   *services.ServiceManager
	userManager      *services.UserManager
	systemMetrics    *services.SystemMetricsCollector
	coreService      *core.Service
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
	repo repository.Repository,
	rateLimiter *middleware.LoginRateLimiter,
	serviceManager *services.ServiceManager,
	userManager *services.UserManager,
	systemMetrics *services.SystemMetricsCollector,
) *Handler {
	return &Handler{
		cfg:            cfg,
		logger:         logger,
		repo:           repo,
		rateLimiter:    rateLimiter,
		serviceManager: serviceManager,
		userManager:    userManager,
		systemMetrics:  systemMetrics,
		coreService:    buildCoreService(cfg, logger, serviceManager),
	}
}

func buildCoreService(cfg config.Config, logger *slog.Logger, serviceManager *services.ServiceManager) *core.Service {
	service, err := core.NewService(cfg, logger, serviceManager)
	if err != nil {
		logger.Warn("core service init failed", "error", err)
		return nil
	}
	return service
}

func (h *Handler) Close() error {
	if h == nil || h.coreService == nil {
		return nil
	}
	return h.coreService.Close()
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
