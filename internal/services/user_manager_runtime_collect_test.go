package services

import (
	"context"
	"path/filepath"
	"testing"

	"h2v2/internal/config"
	"h2v2/internal/repository"
	runtimecore "h2v2/internal/runtime"
)

type collectRuntimeAdapter struct {
	protocol repository.Protocol
	counters []repository.TrafficCounter
	online   map[string]int
}

func (a *collectRuntimeAdapter) Protocol() repository.Protocol {
	return a.protocol
}

func (a *collectRuntimeAdapter) SyncConfig(context.Context, []repository.Inbound, []repository.UserWithCredentials) error {
	return nil
}

func (a *collectRuntimeAdapter) AddUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *collectRuntimeAdapter) UpdateUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *collectRuntimeAdapter) RemoveUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return nil
}

func (a *collectRuntimeAdapter) SetUsersStateBatch(context.Context, []repository.UserWithCredentials, bool, []repository.Inbound) error {
	return nil
}

func (a *collectRuntimeAdapter) KickUser(context.Context, repository.UserWithCredentials) error {
	return nil
}

func (a *collectRuntimeAdapter) CollectTraffic(context.Context, []repository.UserWithCredentials) ([]repository.TrafficCounter, error) {
	out := make([]repository.TrafficCounter, 0, len(a.counters))
	for _, item := range a.counters {
		out = append(out, item)
	}
	return out, nil
}

func (a *collectRuntimeAdapter) CollectOnline(context.Context, []repository.UserWithCredentials) (map[string]int, error) {
	out := make(map[string]int, len(a.online))
	for userID, online := range a.online {
		out[userID] = online
	}
	return out, nil
}

func (a *collectRuntimeAdapter) BuildArtifacts(context.Context, repository.UserWithCredentials, []repository.Inbound, string) (runtimecore.UserArtifacts, error) {
	return runtimecore.UserArtifacts{Protocol: a.protocol}, nil
}

func TestUserManagerCollectRuntimeAggregatesTrafficAndOnlineAcrossProtocols(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "h2v2.db")

	repo, err := repository.NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	created, err := repo.CreateUser(ctx, repository.CreateUserInput{
		Name:    "runtime-user",
		Enabled: true,
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolHY2, Secret: "supersecret88"},
			{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
		},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	vlessAdapter := &collectRuntimeAdapter{
		protocol: repository.ProtocolVLESS,
		counters: []repository.TrafficCounter{
			{UserID: created.ID, Protocol: repository.ProtocolVLESS, TxBytes: 50, RxBytes: 80},
		},
		online: map[string]int{created.ID: 2},
	}
	hy2Adapter := &collectRuntimeAdapter{
		protocol: repository.ProtocolHY2,
		counters: []repository.TrafficCounter{
			{UserID: created.ID, Protocol: repository.ProtocolHY2, TxBytes: 100, RxBytes: 200},
		},
		online: map[string]int{created.ID: 1},
	}

	manager := NewUserManager(
		config.Config{
			InternalAuthToken:     "secret",
			SubscriptionPublicURL: "https://panel.example.com",
		},
		repo,
		runtimecore.NewRuntime(vlessAdapter, hy2Adapter),
	)

	if err := manager.CollectRuntime(ctx); err != nil {
		t.Fatalf("collect runtime: %v", err)
	}

	updated, err := repo.GetUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("get user after runtime collect: %v", err)
	}
	if updated.TrafficUsedTxBytes != 150 || updated.TrafficUsedRxBytes != 280 {
		t.Fatalf("unexpected counters: tx=%d rx=%d", updated.TrafficUsedTxBytes, updated.TrafficUsedRxBytes)
	}
	if updated.OnlineCount != 3 {
		t.Fatalf("unexpected online count: %d", updated.OnlineCount)
	}
}
