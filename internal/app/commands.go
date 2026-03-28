package app

import (
	"context"
	"fmt"
	"strings"

	"proxy-panel/internal/config"
	"proxy-panel/internal/repository"
	"proxy-panel/internal/security"
)

func OpenRepository(cfg config.Config) (*repository.Repository, error) {
	repo, err := repository.New(cfg.StorageRoot, cfg.AuditDir, cfg.RuntimeDir)
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
