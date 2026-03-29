package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"h2v2/internal/config"
	httpserver "h2v2/internal/http"
	"h2v2/internal/http/handlers"
	"h2v2/internal/http/middleware"
	"h2v2/internal/repository"
	"h2v2/internal/scheduler"
	"h2v2/internal/services"
)

type Server struct {
	cfg            config.Config
	logger         *slog.Logger
	repo           *repository.Repository
	handler        *handlers.Handler
	httpServer     *http.Server
	jobs           *scheduler.Jobs
	hysteriaAccess *services.HysteriaAccessManager
	serviceManager *services.ServiceManager
	cancelJobs     context.CancelFunc
}

func NewServer(cfg config.Config, logger *slog.Logger, repo *repository.Repository) *Server {
	rateLimiter := middleware.NewLoginRateLimiter(cfg.RateLimitWindow, cfg.RateLimitBurst)
	hy2Client := services.NewHysteriaClient(cfg.Hy2StatsURL, cfg.Hy2StatsSecret)
	serviceManager := services.NewServiceManager(cfg.SystemctlPath, cfg.SudoPath, cfg.JournalctlPath, cfg.ManagedServices)
	hy2ConfigManager := services.NewHysteriaConfigManager(cfg.Hy2ConfigPath)
	hysteriaAccess := services.NewHysteriaAccessManager(repo, cfg, hy2ConfigManager)
	systemMetrics := services.NewSystemMetricsCollector()

	h := handlers.New(cfg, logger, repo, rateLimiter, hy2Client, serviceManager, hy2ConfigManager, hysteriaAccess, systemMetrics)
	router := httpserver.NewRouter(cfg, logger, repo, h)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	jobs := scheduler.NewJobs(logger, cfg, repo, hy2Client, serviceManager)
	return &Server{
		cfg:            cfg,
		logger:         logger,
		repo:           repo,
		handler:        h,
		httpServer:     httpSrv,
		jobs:           jobs,
		hysteriaAccess: hysteriaAccess,
		serviceManager: serviceManager,
	}
}

func (s *Server) Run(ctx context.Context) error {
	if syncResult, err := s.hysteriaAccess.Sync(ctx); err != nil {
		s.logger.Warn("failed to sync hysteria config on startup", "error", err)
	} else if syncResult.Changed && s.serviceManager != nil {
		if err := s.serviceManager.Restart(ctx, "hysteria-server"); err != nil {
			s.logger.Warn("failed to restart hysteria-server after startup sync", "error", err)
		} else if status, statusErr := s.serviceManager.Status(ctx, "hysteria-server"); statusErr == nil {
			_ = s.repo.UpsertServiceState(ctx, "hysteria-server", status.StatusText, nil, s.serviceManager.ToJSON(status))
		}
	}
	jobsCtx, cancel := context.WithCancel(ctx)
	s.cancelJobs = cancel
	s.jobs.Start(jobsCtx)
	if s.handler != nil {
		s.handler.StartSystemTrendCollector(jobsCtx)
	}

	s.logger.Info("starting panel api", "listen_addr", s.cfg.ListenAddr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.cancelJobs != nil {
		s.cancelJobs()
	}
	err := s.httpServer.Shutdown(ctx)
	_ = s.repo.Close()
	return err
}
