package services

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"h2v2/internal/config"
	"h2v2/internal/repository"
	runtimecore "h2v2/internal/runtime"
)

type failingAdapter struct {
	protocol repository.Protocol
	syncErr  error
}

func (a *failingAdapter) Protocol() repository.Protocol {
	return a.protocol
}

func (a *failingAdapter) SyncConfig(context.Context, []repository.Inbound, []repository.UserWithCredentials) error {
	return a.syncErr
}

func (a *failingAdapter) AddUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *failingAdapter) UpdateUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *failingAdapter) RemoveUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *failingAdapter) SetUsersStateBatch(context.Context, []repository.UserWithCredentials, bool, []repository.Inbound) error {
	return nil
}

func (a *failingAdapter) KickUser(context.Context, repository.UserWithCredentials) error {
	return nil
}

func (a *failingAdapter) CollectTraffic(context.Context, []repository.UserWithCredentials) ([]repository.TrafficCounter, error) {
	return nil, nil
}

func (a *failingAdapter) CollectOnline(context.Context, []repository.UserWithCredentials) (map[string]int, error) {
	return map[string]int{}, nil
}

func (a *failingAdapter) BuildArtifacts(context.Context, repository.UserWithCredentials, []repository.Inbound, string) (runtimecore.UserArtifacts, error) {
	return runtimecore.UserArtifacts{}, nil
}

func TestUserManagerCreateUserRollbackOnSyncError(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := repository.NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	manager := NewUserManager(
		config.Config{
			InternalAuthToken:     "secret",
			SubscriptionPublicURL: "https://panel.example.com",
		},
		repo,
		runtimecore.NewRuntime(&failingAdapter{
			protocol: repository.ProtocolHY2,
			syncErr:  errors.New("sync failed"),
		}),
	)

	_, err = manager.CreateUser(ctx, repository.CreateUserInput{
		Name:    "rollback-create-user",
		Enabled: true,
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolHY2, Identity: "rollback-create-user", Secret: "supersecret88"},
		},
	})
	if err == nil {
		t.Fatalf("expected create user to fail when runtime sync fails")
	}

	list, err := repo.ListUsers(ctx, 0, 0, nil)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected rollback to remove created user, got %d users", len(list))
	}
}

func TestUserManagerSetUsersStateBatchRollbackOnSyncError(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := repository.NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	created, err := repo.CreateUser(ctx, repository.CreateUserInput{
		Name:    "rollback-state-user",
		Enabled: true,
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolHY2, Identity: "rollback-state-user", Secret: "supersecret88"},
		},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	manager := NewUserManager(
		config.Config{
			InternalAuthToken:     "secret",
			SubscriptionPublicURL: "https://panel.example.com",
		},
		repo,
		runtimecore.NewRuntime(&failingAdapter{
			protocol: repository.ProtocolHY2,
			syncErr:  errors.New("sync failed"),
		}),
	)

	_, err = manager.SetUsersStateBatch(ctx, repository.BatchUserStateInput{
		UserIDs: []string{created.ID},
		Enabled: false,
	})
	if err == nil {
		t.Fatalf("expected set state batch to fail when runtime sync fails")
	}

	current, err := repo.GetUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("get user after failed state batch: %v", err)
	}
	if !current.Enabled {
		t.Fatalf("expected rollback to keep user enabled")
	}
}
