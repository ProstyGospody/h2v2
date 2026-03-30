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
	hy2Client      *services.HysteriaClient
	serviceManager *services.ServiceManager
}

func NewJobs(
	logger *slog.Logger,
	cfg config.Config,
	repo repository.Repository,
	hy2Client *services.HysteriaClient,
	serviceManager *services.ServiceManager,
) *Jobs {
	return &Jobs{
		logger:         logger,
		cfg:            cfg,
		repo:           repo,
		hy2Client:      hy2Client,
		serviceManager: serviceManager,
	}
}

func (j *Jobs) Start(ctx context.Context) {
	go j.runTicker(ctx, "hysteria-poll", j.cfg.Hy2PollInterval, j.pollHysteria)
	go j.runTicker(ctx, "services-poll", j.cfg.ServicePollInterval, j.pollServices)
}

func (j *Jobs) runTicker(ctx context.Context, name string, interval time.Duration, fn func(context.Context) error) {
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	if err := fn(ctx); err != nil {
		j.logger.Warn("scheduler initial run failed", "job", name, "error", err)
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

func (j *Jobs) pollHysteria(ctx context.Context) error {
	traffic, err := j.hy2Client.FetchTraffic(ctx)
	if err != nil {
		return err
	}
	online, err := j.hy2Client.FetchOnline(ctx)
	if err != nil {
		return err
	}
	users, err := j.repo.ListHysteriaUsers(ctx, 10000, 0)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	snapshots := make([]repository.HysteriaSnapshot, 0, len(users))
	for _, user := range users {
		stat := traffic[user.Username]
		onlineCount := online[user.Username]
		snapshots = append(snapshots, repository.HysteriaSnapshot{
			UserID:     user.ID,
			TxBytes:    stat.TxBytes,
			RxBytes:    stat.RxBytes,
			Online:     onlineCount,
			SnapshotAt: now,
		})
		if onlineCount > 0 {
			if err := j.repo.TouchHysteriaUserLastSeen(ctx, user.ID, now); err != nil {
				j.logger.Debug("failed to update hysteria last seen", "user_id", user.ID, "error", err)
			}
		}
	}
	return j.repo.InsertHysteriaSnapshots(ctx, snapshots)
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
		case "hysteria-server":
			version, _ = services.DetectBinaryVersion(ctx, j.cfg.Hy2BinaryPath, "version")
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
