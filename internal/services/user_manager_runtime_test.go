package services

import (
	"context"
	"path/filepath"
	"testing"

	"h2v2/internal/config"
	"h2v2/internal/repository"
	runtimecore "h2v2/internal/runtime"
)

type runtimeStatsAdapter struct {
	protocol repository.Protocol
	traffic  []repository.TrafficCounter
	online   map[string]int
}

func (a *runtimeStatsAdapter) Protocol() repository.Protocol {
	return a.protocol
}

func (a *runtimeStatsAdapter) SyncConfig(context.Context, []repository.Inbound, []repository.UserWithCredentials) error {
	return nil
}

func (a *runtimeStatsAdapter) AddUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *runtimeStatsAdapter) UpdateUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *runtimeStatsAdapter) RemoveUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *runtimeStatsAdapter) SetUsersStateBatch(context.Context, []repository.UserWithCredentials, bool, []repository.Inbound) error {
	return nil
}

func (a *runtimeStatsAdapter) KickUser(context.Context, repository.UserWithCredentials) error {
	return nil
}

func (a *runtimeStatsAdapter) CollectTraffic(context.Context, []repository.UserWithCredentials) ([]repository.TrafficCounter, error) {
	return a.traffic, nil
}

func (a *runtimeStatsAdapter) CollectOnline(context.Context, []repository.UserWithCredentials) (map[string]int, error) {
	return a.online, nil
}

func (a *runtimeStatsAdapter) BuildArtifacts(context.Context, repository.UserWithCredentials, []repository.Inbound, string) (runtimecore.UserArtifacts, error) {
	return runtimecore.UserArtifacts{}, nil
}

func TestUserManagerCollectRuntimeAppliesOnlineFromAdapter(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := repository.NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	user, err := repo.CreateUser(ctx, repository.CreateUserInput{
		Name:    "runtime-online-user",
		Enabled: true,
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
		},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	adapter := &runtimeStatsAdapter{
		protocol: repository.ProtocolVLESS,
		traffic: []repository.TrafficCounter{
			{
				UserID:   user.ID,
				Protocol: repository.ProtocolVLESS,
				TxBytes:  128,
				RxBytes:  256,
			},
		},
		online: map[string]int{
			user.ID: 2,
		},
	}

	manager := NewUserManager(
		config.Config{
			InternalAuthToken:     "secret",
			SubscriptionPublicURL: "https://panel.example.com",
		},
		repo,
		runtimecore.NewRuntime(adapter),
	)

	if err := manager.CollectRuntime(ctx); err != nil {
		t.Fatalf("collect runtime: %v", err)
	}

	updated, err := repo.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if updated.OnlineCount != 2 {
		t.Fatalf("unexpected online count: %d", updated.OnlineCount)
	}
	if updated.TrafficUsedTxBytes != 128 || updated.TrafficUsedRxBytes != 256 {
		t.Fatalf("unexpected traffic counters: tx=%d rx=%d", updated.TrafficUsedTxBytes, updated.TrafficUsedRxBytes)
	}
}

func TestApplyOnlineToCounters(t *testing.T) {
	counters := []repository.TrafficCounter{
		{UserID: "u1", Online: 0},
		{UserID: "u2", Online: 1},
	}
	onlineByUser := map[string]int{
		"u1": 3,
	}

	updated := applyOnlineToCounters(counters, onlineByUser)
	if updated[0].Online != 3 {
		t.Fatalf("unexpected online for u1: %d", updated[0].Online)
	}
	if updated[1].Online != 1 {
		t.Fatalf("unexpected online for u2: %d", updated[1].Online)
	}
}
