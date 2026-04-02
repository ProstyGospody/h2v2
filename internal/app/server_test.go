package app

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"h2v2/internal/config"
	"h2v2/internal/repository"
	runtimecore "h2v2/internal/runtime"
	"h2v2/internal/services"
)

type startupSyncAdapter struct {
	protocol  repository.Protocol
	syncCalls int
}

func (a *startupSyncAdapter) Protocol() repository.Protocol {
	return a.protocol
}

func (a *startupSyncAdapter) SyncConfig(context.Context, []repository.Inbound, []repository.UserWithCredentials) error {
	a.syncCalls++
	return nil
}

func (a *startupSyncAdapter) AddUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *startupSyncAdapter) UpdateUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *startupSyncAdapter) RemoveUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *startupSyncAdapter) SetUsersStateBatch(context.Context, []repository.UserWithCredentials, bool, []repository.Inbound) error {
	return nil
}

func (a *startupSyncAdapter) KickUser(context.Context, repository.UserWithCredentials) error {
	return nil
}

func (a *startupSyncAdapter) CollectTraffic(context.Context, []repository.UserWithCredentials) ([]repository.TrafficCounter, error) {
	return nil, nil
}

func (a *startupSyncAdapter) CollectOnline(context.Context, []repository.UserWithCredentials) (map[string]int, error) {
	return map[string]int{}, nil
}

func (a *startupSyncAdapter) BuildArtifacts(context.Context, repository.UserWithCredentials, []repository.Inbound, string) (runtimecore.UserArtifacts, error) {
	return runtimecore.UserArtifacts{}, nil
}

func TestRunStartupTasksSyncsRuntimeAdaptersWhenUserManagerIsConfigured(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := repository.NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	adapter := &startupSyncAdapter{protocol: repository.ProtocolHY2}
	manager := services.NewUserManager(
		config.Config{
			InternalAuthToken:     "secret",
			SubscriptionPublicURL: "https://panel.example.com",
		},
		repo,
		runtimecore.NewRuntime(adapter),
	)

	server := &Server{
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		repo:        repo,
		userManager: manager,
	}

	server.runStartupTasks(ctx)
	if adapter.syncCalls == 0 {
		t.Fatalf("expected startup tasks to sync runtime adapters")
	}
}
