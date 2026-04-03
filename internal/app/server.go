package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"net/http"
	"strings"
	"time"

	"h2v2/internal/config"
	httpserver "h2v2/internal/http"
	"h2v2/internal/http/handlers"
	"h2v2/internal/http/middleware"
	"h2v2/internal/repository"
	runtimecore "h2v2/internal/runtime"
	"h2v2/internal/scheduler"
	"h2v2/internal/services"
)

type Server struct {
	cfg        config.Config
	logger     *slog.Logger
	repo       repository.Repository
	handler    *handlers.Handler
	httpServer *http.Server
	jobs       *scheduler.Jobs
	userManager *services.UserManager
	cancelJobs context.CancelFunc
}

func NewServer(cfg config.Config, logger *slog.Logger, repo repository.Repository) *Server {
	rateLimiter := middleware.NewLoginRateLimiter(cfg.RateLimitWindow, cfg.RateLimitBurst)
	serviceManager := services.NewServiceManager(cfg.SystemctlPath, cfg.SudoPath, cfg.JournalctlPath, cfg.ManagedServices, cfg.ServiceCommandTimeout)
	singBoxAdapter := runtimecore.NewSingBoxAdapter(cfg.SingBoxBinaryPath, cfg.SingBoxConfigPath, serviceManager, cfg.SingBoxServiceName, resolveRuntimeArtifactHost(cfg))
	runtime := runtimecore.NewRuntime(
		singBoxAdapter,
		runtimecore.NewSingBoxHY2Adapter(singBoxAdapter),
	)
	userManager := services.NewUserManager(cfg, repo, runtime)
	systemMetrics := services.NewSystemMetricsCollector()

	h := handlers.New(cfg, logger, repo, rateLimiter, serviceManager, userManager, systemMetrics)
	router := httpserver.NewRouter(cfg, logger, repo, h)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	jobs := scheduler.NewJobs(logger, cfg, repo, serviceManager, userManager)
	return &Server{
		cfg:         cfg,
		logger:      logger,
		repo:        repo,
		handler:     h,
		httpServer:  httpSrv,
		jobs:        jobs,
		userManager: userManager,
	}
}

func resolveRuntimeArtifactHost(cfg config.Config) string {
	candidates := []string{
		strings.TrimSpace(cfg.SubscriptionPublicURL),
		strings.TrimSpace(cfg.PublicPanelURL),
		strings.TrimSpace(cfg.PanelPublicHost),
	}
	for _, candidate := range candidates {
		host := normalizeRuntimeHost(candidate)
		if host == "" {
			continue
		}
		if isLoopbackOrUnspecifiedHost(host) {
			continue
		}
		return host
	}
	return normalizeRuntimeHost(strings.TrimSpace(cfg.PanelPublicHost))
}

func normalizeRuntimeHost(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return ""
		}
		value = strings.TrimSpace(parsed.Host)
	}
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.TrimSpace(strings.Trim(value, "[]"))
	value = strings.TrimSuffix(value, ".")
	return strings.ToLower(value)
}

func isLoopbackOrUnspecifiedHost(raw string) bool {
	host := strings.TrimSpace(raw)
	if host == "" {
		return true
	}
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0", "::":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsUnspecified()
	}
	return false
}

func (s *Server) Run(ctx context.Context) error {
	jobsCtx, cancel := context.WithCancel(ctx)
	s.cancelJobs = cancel
	s.jobs.Start(jobsCtx)
	if s.handler != nil {
		s.handler.StartSystemTrendCollector(jobsCtx)
	}
	go s.runStartupTasks(jobsCtx)

	s.logger.Info("starting panel api", "listen_addr", s.cfg.ListenAddr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve: %w", err)
	}
	return nil
}

func (s *Server) runStartupTasks(ctx context.Context) {
	startupCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	if s.userManager != nil {
		if err := s.userManager.SyncAll(startupCtx); err != nil {
			s.logger.Warn("failed to sync runtime adapters on startup", "error", err)
		}
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.cancelJobs != nil {
		s.cancelJobs()
	}
	if s.handler != nil {
		_ = s.handler.Close()
	}
	err := s.httpServer.Shutdown(ctx)
	_ = s.repo.Close()
	return err
}
