package runtime

import (
	"context"

	"h2v2/internal/repository"
)

type UserArtifacts struct {
	Protocol    repository.Protocol `json:"protocol"`
	AccessURI   string              `json:"access_uri,omitempty"`
	Config      string              `json:"config,omitempty"`
	Subscription string             `json:"subscription,omitempty"`
	ClashNode   string              `json:"clash_node,omitempty"`
	SingBoxNode map[string]any      `json:"singbox_node,omitempty"`
}

type Adapter interface {
	Protocol() repository.Protocol
	SyncConfig(context.Context, []repository.Inbound, []repository.UserWithCredentials) error
	AddUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error
	UpdateUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error
	RemoveUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error
	SetUsersStateBatch(context.Context, []repository.UserWithCredentials, bool, []repository.Inbound) error
	KickUser(context.Context, repository.UserWithCredentials) error
	CollectTraffic(context.Context, []repository.UserWithCredentials) ([]repository.TrafficCounter, error)
	CollectOnline(context.Context, []repository.UserWithCredentials) (map[string]int, error)
	BuildArtifacts(context.Context, repository.UserWithCredentials, []repository.Inbound, string) (UserArtifacts, error)
}

type Runtime struct {
	adapters map[repository.Protocol]Adapter
}

type ServiceRestarter interface {
	Restart(context.Context, string) error
}

func NewRuntime(adapters ...Adapter) *Runtime {
	indexed := make(map[repository.Protocol]Adapter, len(adapters))
	for _, adapter := range adapters {
		if adapter == nil {
			continue
		}
		indexed[adapter.Protocol()] = adapter
	}
	return &Runtime{adapters: indexed}
}

func (r *Runtime) Adapter(protocol repository.Protocol) (Adapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.adapters[protocol]
	return adapter, ok
}

func (r *Runtime) Protocols() []repository.Protocol {
	if r == nil {
		return nil
	}
	out := make([]repository.Protocol, 0, len(r.adapters))
	for protocol := range r.adapters {
		out = append(out, protocol)
	}
	return out
}
