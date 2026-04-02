package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	hysteriadomain "h2v2/internal/domain/hysteria"
	"h2v2/internal/repository"
	"h2v2/internal/services"
)

type HY2Adapter struct {
	access      *services.HysteriaAccessManager
	client      *services.HysteriaClient
	services    *services.ServiceManager
	serviceName string
}

func NewHY2Adapter(access *services.HysteriaAccessManager, client *services.HysteriaClient, svc *services.ServiceManager, serviceName string) *HY2Adapter {
	name := strings.TrimSpace(serviceName)
	if name == "" {
		name = "hysteria-server"
	}
	return &HY2Adapter{access: access, client: client, services: svc, serviceName: name}
}

func (a *HY2Adapter) Protocol() repository.Protocol {
	return repository.ProtocolHY2
}

func (a *HY2Adapter) SyncConfig(ctx context.Context, _ []repository.Inbound, _ []repository.UserWithCredentials) error {
	return a.sync(ctx)
}

func (a *HY2Adapter) AddUser(ctx context.Context, _ repository.UserWithCredentials, _ []repository.Inbound) error {
	return a.sync(ctx)
}

func (a *HY2Adapter) UpdateUser(ctx context.Context, _ repository.UserWithCredentials, _ []repository.Inbound) error {
	return a.sync(ctx)
}

func (a *HY2Adapter) RemoveUser(ctx context.Context, _ repository.UserWithCredentials, _ []repository.Inbound) error {
	return a.sync(ctx)
}

func (a *HY2Adapter) SetUsersStateBatch(ctx context.Context, _ []repository.UserWithCredentials, _ bool, _ []repository.Inbound) error {
	return a.sync(ctx)
}

func (a *HY2Adapter) KickUser(ctx context.Context, user repository.UserWithCredentials) error {
	if a.client == nil {
		return fmt.Errorf("hy2 client is not configured")
	}
	credential, ok := userCredential(user, repository.ProtocolHY2)
	if !ok {
		return fmt.Errorf("hy2 credential is missing")
	}
	return a.client.Kick(ctx, credential.Identity)
}

func (a *HY2Adapter) CollectTraffic(ctx context.Context, users []repository.UserWithCredentials) ([]repository.TrafficCounter, error) {
	if a.client == nil {
		return nil, nil
	}
	traffic, err := a.client.FetchTraffic(ctx)
	if err != nil {
		return nil, err
	}
	online, err := a.client.FetchOnline(ctx)
	if err != nil {
		return nil, err
	}

	counters := make([]repository.TrafficCounter, 0, len(users))
	for _, user := range users {
		credential, ok := userCredential(user, repository.ProtocolHY2)
		if !ok {
			continue
		}
		stat := traffic[credential.Identity]
		counters = append(counters, repository.TrafficCounter{
			UserID:   user.ID,
			Protocol: repository.ProtocolHY2,
			TxBytes:  stat.TxBytes,
			RxBytes:  stat.RxBytes,
			Online:   online[credential.Identity],
		})
	}
	return counters, nil
}

func (a *HY2Adapter) CollectOnline(ctx context.Context, users []repository.UserWithCredentials) (map[string]int, error) {
	if a.client == nil {
		return map[string]int{}, nil
	}
	online, err := a.client.FetchOnline(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int, len(users))
	for _, user := range users {
		credential, ok := userCredential(user, repository.ProtocolHY2)
		if !ok {
			continue
		}
		result[user.ID] = online[credential.Identity]
	}
	return result, nil
}

func (a *HY2Adapter) BuildArtifacts(ctx context.Context, user repository.UserWithCredentials, _ []repository.Inbound, subscriptionURL string) (UserArtifacts, error) {
	if a.access == nil {
		return UserArtifacts{}, fmt.Errorf("hy2 access manager is not configured")
	}
	credential, ok := userCredential(user, repository.ProtocolHY2)
	if !ok {
		return UserArtifacts{}, fmt.Errorf("hy2 credential is missing")
	}

	overrides := decodeHY2Overrides(credential)
	legacy := repository.HysteriaUserView{User: repository.HysteriaUser{
		ID:              user.ID,
		Username:        credential.Identity,
		Password:        credential.Secret,
		Enabled:         user.Enabled,
		Note:            user.Note,
		ClientOverrides: overrides,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
		LastSeenAt:      user.LastSeenAt,
	}}

	artifacts, _, err := a.access.BuildUserArtifacts(legacy)
	if err != nil {
		return UserArtifacts{}, err
	}
	if strings.TrimSpace(subscriptionURL) == "" {
		subscriptionURL = artifacts.SubscriptionURL
	}

	return UserArtifacts{
		Protocol:     repository.ProtocolHY2,
		AccessURI:    firstNonEmpty(artifacts.URIHy2, artifacts.URI),
		Config:       strings.TrimSpace(artifacts.ClientYAML),
		Subscription: strings.TrimSpace(subscriptionURL),
		ClashNode:    renderHY2ClashNode(user, artifacts),
		SingBoxNode:  artifacts.SingBoxOutbound,
	}, nil
}

func (a *HY2Adapter) sync(ctx context.Context) error {
	if a.access == nil {
		return nil
	}
	if _, err := a.access.Sync(ctx); err != nil {
		return err
	}
	if a.services == nil {
		return nil
	}
	return a.services.Restart(ctx, a.serviceName)
}

func userCredential(user repository.UserWithCredentials, protocol repository.Protocol) (repository.Credential, bool) {
	for _, credential := range user.Credentials {
		if credential.Protocol == protocol {
			return credential, true
		}
	}
	return repository.Credential{}, false
}

func decodeHY2Overrides(credential repository.Credential) *hysteriadomain.ClientOverrides {
	trimmed := strings.TrimSpace(credential.DataJSON)
	if trimmed == "" {
		return nil
	}
	var overrides hysteriadomain.ClientOverrides
	if err := json.Unmarshal([]byte(trimmed), &overrides); err != nil {
		return nil
	}
	return hysteriadomain.NormalizeClientOverrides(&overrides)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func renderHY2ClashNode(user repository.UserWithCredentials, artifacts services.HysteriaUserArtifacts) string {
	params := artifacts.ClientParams
	if strings.TrimSpace(params.Server) == "" || params.Port <= 0 {
		return ""
	}
	password := extractHY2Password(firstNonEmpty(artifacts.URIHy2, artifacts.URI))
	if password == "" {
		return ""
	}
	lines := []string{
		"- name: " + firstNonEmpty(user.Name, "hy2"),
		"  type: hysteria2",
		"  server: " + strings.TrimSpace(params.Server),
		"  port: " + fmt.Sprintf("%d", params.Port),
		"  password: " + password,
	}
	if strings.TrimSpace(params.SNI) != "" {
		lines = append(lines, "  sni: "+strings.TrimSpace(params.SNI))
	}
	return strings.Join(lines, "\n")
}

func extractHY2Password(rawURI string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURI))
	if err != nil || parsed.User == nil {
		return ""
	}
	if value := strings.TrimSpace(parsed.User.String()); value != "" {
		return value
	}
	username := parsed.User.Username()
	password, ok := parsed.User.Password()
	if ok {
		return username + ":" + password
	}
	return strings.TrimSpace(username)
}
