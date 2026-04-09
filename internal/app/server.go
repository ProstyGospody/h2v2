package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"h2v2/internal/config"
	"h2v2/internal/core"
	httpserver "h2v2/internal/http"
	"h2v2/internal/http/handlers"
	"h2v2/internal/http/middleware"
	"h2v2/internal/repository"
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
	cancelJobs context.CancelFunc
}

func NewServer(cfg config.Config, logger *slog.Logger, repo repository.Repository) *Server {
	rateLimiter := middleware.NewLoginRateLimiter(cfg.RateLimitWindow, cfg.RateLimitBurst)
	serviceManager := services.NewServiceManager(cfg.SystemctlPath, cfg.SudoPath, cfg.JournalctlPath, cfg.ManagedServices, cfg.ServiceCommandTimeout)
	systemMetrics := services.NewSystemMetricsCollector()
	coreService, err := core.NewService(cfg, logger, serviceManager)
	if err != nil {
		logger.Warn("core service init failed", "error", err)
	}

	h := handlers.New(cfg, logger, repo, rateLimiter, serviceManager, systemMetrics, coreService)
	router := httpserver.NewRouter(cfg, logger, repo, h)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	jobs := scheduler.NewJobs(logger, cfg, repo, serviceManager, coreService)
	return &Server{
		cfg:        cfg,
		logger:     logger,
		repo:       repo,
		handler:    h,
		httpServer: httpSrv,
		jobs:       jobs,
	}
}

func (s *Server) Run(ctx context.Context) error {
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
	if s.handler != nil {
		_ = s.handler.Close()
	}
	err := s.httpServer.Shutdown(ctx)
	_ = s.repo.Close()
	return err
}
