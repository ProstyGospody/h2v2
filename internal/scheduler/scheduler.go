package scheduler

import (
	"context"
	"log/slog"
	"time"

	"h2v2/internal/config"
	"h2v2/internal/repository"
	"h2v2/internal/services"
)

type Jobs struct {
	logger         *slog.Logger
	cfg            config.Config
	repo           repository.Repository
	serviceManager *services.ServiceManager
}

func NewJobs(
	logger *slog.Logger,
	cfg config.Config,
	repo repository.Repository,
	serviceManager *services.ServiceManager,
) *Jobs {
	return &Jobs{
		logger:         logger,
		cfg:            cfg,
		repo:           repo,
		serviceManager: serviceManager,
	}
}

func (j *Jobs) Start(ctx context.Context) {
	go j.runTicker(ctx, "services-poll", j.cfg.ServicePollInterval, true, j.pollServices)
}

func (j *Jobs) runTicker(ctx context.Context, name string, interval time.Duration, runImmediately bool, fn func(context.Context) error) {
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	if runImmediately {
		if err := fn(ctx); err != nil {
			j.logger.Warn("scheduler initial run failed", "job", name, "error", err)
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				j.logger.Warn("scheduler job failed", "job", name, "error", err)
			}
		}
	}
}

func (j *Jobs) pollServices(ctx context.Context) error {
	for service := range j.serviceManager.ManagedServices {
		details, err := j.serviceManager.Status(ctx, service)
		if err != nil {
			j.logger.Warn("service status failed", "service", service, "error", err)
			_ = j.repo.UpsertServiceState(ctx, service, "failed", nil, `{"error":"status failed"}`)
			continue
		}
		version := ""
		switch service {
		case j.cfg.SingBoxServiceName:
			version, _ = services.DetectBinaryVersion(ctx, j.cfg.SingBoxBinaryPath, "version")
		case "h2v2-api":
			version = "managed-by-systemd"
		case "h2v2-web":
			version = "managed-by-systemd"
		}
		var versionPtr *string
		if version != "" {
			versionPtr = &version
		}
		if err := j.repo.UpsertServiceState(ctx, service, details.StatusText, versionPtr, j.serviceManager.ToJSON(details)); err != nil {
			j.logger.Warn("upsert service state failed", "service", service, "error", err)
		}
	}
	return nil
}
