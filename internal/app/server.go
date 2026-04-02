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
	runtimecore "h2v2/internal/runtime"
	"h2v2/internal/scheduler"
	"h2v2/internal/services"
)

type Server struct {
	cfg            config.Config
	logger         *slog.Logger
	repo           repository.Repository
	handler        *handlers.Handler
	httpServer     *http.Server
	jobs           *scheduler.Jobs
	hysteriaAccess *services.HysteriaAccessManager
	userManager    *services.UserManager
	serviceManager *services.ServiceManager
	cancelJobs     context.CancelFunc
}

type hy2AccessRuntimeAdapter struct {
	inner *services.HysteriaAccessManager
}

func (a hy2AccessRuntimeAdapter) Sync(ctx context.Context) error {
	if a.inner == nil {
		return nil
	}
	_, err := a.inner.Sync(ctx)
	return err
}

func (a hy2AccessRuntimeAdapter) BuildUserArtifacts(user repository.HysteriaUserView) (runtimecore.HY2UserArtifacts, error) {
	if a.inner == nil {
		return runtimecore.HY2UserArtifacts{}, fmt.Errorf("hysteria access manager is not configured")
	}
	artifacts, _, err := a.inner.BuildUserArtifacts(user)
	if err != nil {
		return runtimecore.HY2UserArtifacts{}, err
	}
	return runtimecore.HY2UserArtifacts{
		URI:             artifacts.URI,
		URIHy2:          artifacts.URIHy2,
		SubscriptionURL: artifacts.SubscriptionURL,
		ClientYAML:      artifacts.ClientYAML,
		ClientParams: runtimecore.HY2ClientParams{
			Server: artifacts.ClientParams.Server,
			Port:   artifacts.ClientParams.Port,
			SNI:    artifacts.ClientParams.SNI,
		},
		SingBoxOutbound: artifacts.SingBoxOutbound,
	}, nil
}

type hy2ClientRuntimeAdapter struct {
	inner *services.HysteriaClient
}

func (a hy2ClientRuntimeAdapter) Kick(ctx context.Context, identity string) error {
	if a.inner == nil {
		return fmt.Errorf("hy2 client is not configured")
	}
	return a.inner.Kick(ctx, identity)
}

func (a hy2ClientRuntimeAdapter) FetchTraffic(ctx context.Context) (map[string]runtimecore.HY2Traffic, error) {
	if a.inner == nil {
		return map[string]runtimecore.HY2Traffic{}, nil
	}
	traffic, err := a.inner.FetchTraffic(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]runtimecore.HY2Traffic, len(traffic))
	for identity, value := range traffic {
		out[identity] = runtimecore.HY2Traffic{
			TxBytes: value.TxBytes,
			RxBytes: value.RxBytes,
		}
	}
	return out, nil
}

func (a hy2ClientRuntimeAdapter) FetchOnline(ctx context.Context) (map[string]int, error) {
	if a.inner == nil {
		return map[string]int{}, nil
	}
	return a.inner.FetchOnline(ctx)
}

func NewServer(cfg config.Config, logger *slog.Logger, repo repository.Repository) *Server {
	rateLimiter := middleware.NewLoginRateLimiter(cfg.RateLimitWindow, cfg.RateLimitBurst)
	hy2Client := services.NewHysteriaClient(cfg.Hy2StatsURL, cfg.Hy2StatsSecret)
	serviceManager := services.NewServiceManager(cfg.SystemctlPath, cfg.SudoPath, cfg.JournalctlPath, cfg.ManagedServices, cfg.ServiceCommandTimeout)
	hy2ConfigManager := services.NewHysteriaConfigManager(cfg.Hy2ConfigPath)
	hysteriaAccess := services.NewHysteriaAccessManager(repo, cfg, hy2ConfigManager)
	hy2RuntimeAccess := hy2AccessRuntimeAdapter{inner: hysteriaAccess}
	hy2RuntimeClient := hy2ClientRuntimeAdapter{inner: hy2Client}
	runtime := runtimecore.NewRuntime(
		runtimecore.NewHY2Adapter(hy2RuntimeAccess, hy2RuntimeClient, serviceManager, "hysteria-server"),
		runtimecore.NewXrayAdapter(cfg.XrayConfigPath, cfg.XrayRuntimeURL, cfg.XrayRuntimeToken, serviceManager, cfg.XrayServiceName),
	)
	userManager := services.NewUserManager(cfg, repo, runtime)
	systemMetrics := services.NewSystemMetricsCollector()

	h := handlers.New(cfg, logger, repo, rateLimiter, hy2Client, serviceManager, hy2ConfigManager, hysteriaAccess, userManager, systemMetrics)
	router := httpserver.NewRouter(cfg, logger, repo, h)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	jobs := scheduler.NewJobs(logger, cfg, repo, hy2Client, serviceManager, userManager)
	return &Server{
		cfg:            cfg,
		logger:         logger,
		repo:           repo,
		handler:        h,
		httpServer:     httpSrv,
		jobs:           jobs,
		hysteriaAccess: hysteriaAccess,
		userManager:    userManager,
		serviceManager: serviceManager,
	}
}

func (s *Server) Run(ctx context.Context) error {
	if s.userManager != nil {
		if err := s.userManager.SyncAll(ctx); err != nil {
			s.logger.Warn("failed to sync runtime adapters on startup", "error", err)
		}
		if err := s.userManager.CollectRuntime(ctx); err != nil {
			s.logger.Warn("failed to collect initial runtime counters", "error", err)
		}
	} else if s.hysteriaAccess != nil {
		if syncResult, err := s.hysteriaAccess.Sync(ctx); err != nil {
			s.logger.Warn("failed to sync hysteria config on startup", "error", err)
		} else if syncResult.Changed && s.serviceManager != nil {
			if err := s.serviceManager.Restart(ctx, "hysteria-server"); err != nil {
				s.logger.Warn("failed to restart hysteria-server after startup sync", "error", err)
			} else if status, statusErr := s.serviceManager.Status(ctx, "hysteria-server"); statusErr == nil {
				_ = s.repo.UpsertServiceState(ctx, "hysteria-server", status.StatusText, nil, s.serviceManager.ToJSON(status))
			}
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
