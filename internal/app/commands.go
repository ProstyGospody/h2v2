package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"h2v2/internal/config"
	"h2v2/internal/core"
	"h2v2/internal/repository"
	"h2v2/internal/security"
	"h2v2/internal/services"
)

func newServiceManager(cfg config.Config) *services.ServiceManager {
	return services.NewServiceManager(
		cfg.SystemctlPath,
		cfg.SudoPath,
		cfg.JournalctlPath,
		cfg.ManagedServices,
		cfg.ServiceCommandTimeout,
	)
}

func OpenRepository(cfg config.Config) (repository.Repository, error) {
	repo, err := repository.Open(repository.OpenOptions{
		StorageRoot: cfg.StorageRoot,
		SQLitePath:  cfg.SQLitePath,
	})
	if err != nil {
		return nil, err
	}
	return repo, nil
}

func BootstrapAdmin(ctx context.Context, cfg config.Config, email string, password string) error {
	repo, err := OpenRepository(cfg)
	if err != nil {
		return err
	}
	defer repo.Close()

	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = repo.UpsertAdmin(ctx, strings.TrimSpace(strings.ToLower(email)), hash, true)
	if err != nil {
		return fmt.Errorf("upsert admin: %w", err)
	}
	return nil
}

// BootstrapInbounds creates the default server record and VLESS Reality +
// Hysteria2 inbounds with stable parameters, auto-generating Reality keys.
// Idempotent — safe to run on every install.
func BootstrapInbounds(ctx context.Context, cfg config.Config) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	svc, err := core.NewService(cfg, logger, newServiceManager(cfg))
	if err != nil {
		return fmt.Errorf("open core service: %w", err)
	}
	defer svc.Close()
	if _, _, _, err := svc.EnsureDefaultInbounds(ctx); err != nil {
		return fmt.Errorf("ensure default inbounds: %w", err)
	}
	return nil
}

// RefreshInbounds re-normalizes all stored inbounds (re-applying defaults and
// regenerating missing Reality keys) and reloads sing-box on each server.
func RefreshInbounds(ctx context.Context, cfg config.Config) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	svc, err := core.NewService(cfg, logger, newServiceManager(cfg))
	if err != nil {
		return fmt.Errorf("open core service: %w", err)
	}
	defer svc.Close()
	if err := svc.RefreshInbounds(ctx); err != nil {
		return fmt.Errorf("refresh inbounds: %w", err)
	}
	return nil
}
