package app

import (
	"context"
	"fmt"
	"strings"

	"h2v2/internal/config"
	"h2v2/internal/repository"
	"h2v2/internal/security"
)

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
