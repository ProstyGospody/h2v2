package runtime

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"h2v2/internal/repository"
)

type SingBoxHY2Adapter struct {
	shared *SingBoxAdapter
}

func NewSingBoxHY2Adapter(shared *SingBoxAdapter) *SingBoxHY2Adapter {
	return &SingBoxHY2Adapter{shared: shared}
}

func (a *SingBoxHY2Adapter) Protocol() repository.Protocol {
	return repository.ProtocolHY2
}

func (a *SingBoxHY2Adapter) SyncConfig(context.Context, []repository.Inbound, []repository.UserWithCredentials) error {
	return nil
}

func (a *SingBoxHY2Adapter) AddUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return fmt.Errorf("incremental hy2 mutation is not supported")
}

func (a *SingBoxHY2Adapter) UpdateUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return fmt.Errorf("incremental hy2 mutation is not supported")
}

func (a *SingBoxHY2Adapter) RemoveUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return fmt.Errorf("incremental hy2 mutation is not supported")
}

func (a *SingBoxHY2Adapter) SetUsersStateBatch(context.Context, []repository.UserWithCredentials, bool, []repository.Inbound) error {
	return fmt.Errorf("incremental hy2 mutation is not supported")
}

func (a *SingBoxHY2Adapter) KickUser(context.Context, repository.UserWithCredentials) error {
	return fmt.Errorf("hy2 kick is not supported for sing-box runtime")
}

func (a *SingBoxHY2Adapter) CollectTraffic(ctx context.Context, users []repository.UserWithCredentials) ([]repository.TrafficCounter, error) {
	if a == nil || a.shared == nil || !a.shared.v2rayStatsAvailable() || len(users) == 0 {
		return nil, nil
	}

	identityByName := mapHY2StatsIdentity(users)
	if len(identityByName) == 0 {
		return nil, nil
	}

	statsByUser, err := a.shared.queryV2RayUserStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("sing-box v2ray stats query failed: %w", err)
	}

	now := time.Now().UTC()
	counters := make([]repository.TrafficCounter, 0, len(statsByUser))
	for statsUserName, traffic := range statsByUser {
		if !traffic.hasTraffic() {
			continue
		}
		userID, ok := identityByName[statsUserName]
		if !ok {
			userID, ok = identityByName[strings.ToLower(statsUserName)]
		}
		if !ok {
			continue
		}
		counter := repository.TrafficCounter{
			UserID:     userID,
			Protocol:   repository.ProtocolHY2,
			TxBytes:    traffic.TxBytes,
			RxBytes:    traffic.RxBytes,
			SnapshotAt: now,
		}
		if traffic.HasOnline {
			counter.Online = traffic.Online
		}
		counters = append(counters, counter)
	}

	return counters, nil
}

func (a *SingBoxHY2Adapter) CollectOnline(ctx context.Context, users []repository.UserWithCredentials) (map[string]int, error) {
	if a == nil || a.shared == nil || !a.shared.v2rayStatsAvailable() || len(users) == 0 {
		return map[string]int{}, nil
	}

	identityByName := mapHY2StatsIdentity(users)
	if len(identityByName) == 0 {
		return map[string]int{}, nil
	}

	statsByUser, err := a.shared.queryV2RayUserStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("sing-box v2ray online query failed: %w", err)
	}

	onlineByUser := make(map[string]int, len(users))
	for statsUserName, stat := range statsByUser {
		if !stat.HasOnline {
			continue
		}
		userID, ok := identityByName[statsUserName]
		if !ok {
			userID, ok = identityByName[strings.ToLower(statsUserName)]
		}
		if !ok {
			continue
		}
		current, exists := onlineByUser[userID]
		if !exists || stat.Online > current {
			onlineByUser[userID] = stat.Online
		}
	}

	return onlineByUser, nil
}

func (a *SingBoxHY2Adapter) BuildArtifacts(_ context.Context, user repository.UserWithCredentials, inbounds []repository.Inbound, subscriptionURL string) (UserArtifacts, error) {
	credential, ok := userCredential(user, repository.ProtocolHY2)
	if !ok {
		return UserArtifacts{}, fmt.Errorf("hy2 credential is missing")
	}
	password := strings.TrimSpace(credential.Secret)
	if password == "" {
		return UserArtifacts{}, fmt.Errorf("hy2 credential secret is missing")
	}

	inbound, ok := selectEnabledHY2Inbound(inbounds)
	if !ok {
		return UserArtifacts{}, fmt.Errorf("enabled hy2 inbound is missing")
	}

	runtimeConfig, err := parseSingBoxHY2Inbound(inbound)
	if err != nil {
		return UserArtifacts{}, err
	}

	serverHost := ""
	if a != nil && a.shared != nil {
		serverHost = a.shared.resolveArtifactHost(inbound.Host)
	}
	if serverHost == "" {
		serverHost = normalizePublicEndpointHost(inbound.Host)
	}
	if serverHost == "" {
		return UserArtifacts{}, fmt.Errorf("public endpoint host is missing for hy2 artifacts")
	}

	uri := buildSingBoxHY2URI(password, firstNonEmpty(user.Name, credential.Identity), serverHost, runtimeConfig)
	singBoxNode := renderSingBoxHY2Outbound(user, password, serverHost, runtimeConfig)
	clashNode := renderClashHY2Node(user, password, serverHost, runtimeConfig)

	return UserArtifacts{
		Protocol:     repository.ProtocolHY2,
		AccessURI:    uri,
		Config:       renderVLESSClientConfig(uri, clashNode, singBoxNode),
		Subscription: strings.TrimSpace(subscriptionURL),
		ClashNode:    clashNode,
		SingBoxNode:  singBoxNode,
	}, nil
}

func mapHY2StatsIdentity(users []repository.UserWithCredentials) map[string]string {
	identityByName := make(map[string]string, len(users)*3)
	mapIdentity := func(raw string, userID string) {
		key := strings.TrimSpace(raw)
		if key == "" {
			return
		}
		identityByName[key] = userID
		lowerKey := strings.ToLower(key)
		if lowerKey != key {
			identityByName[lowerKey] = userID
		}
		upperKey := strings.ToUpper(key)
		if upperKey != key {
			identityByName[upperKey] = userID
		}
	}
	for _, user := range users {
		credential, ok := userCredential(user, repository.ProtocolHY2)
		if !ok {
			continue
		}
		mapIdentity(credential.Identity, user.ID)
	}
	return identityByName
}

func selectEnabledHY2Inbound(inbounds []repository.Inbound) (repository.Inbound, bool) {
	for _, inbound := range inbounds {
		if inbound.Protocol == repository.ProtocolHY2 && inbound.Enabled {
			return inbound, true
		}
	}
	return repository.Inbound{}, false
}

func buildSingBoxHY2URI(password string, displayName string, serverHost string, runtimeConfig singBoxHY2RuntimeConfig) string {
	query := url.Values{}
	if runtimeConfig.ServerName != "" {
		query.Set("sni", runtimeConfig.ServerName)
	}
	if runtimeConfig.ObfsType != "" {
		query.Set("obfs", runtimeConfig.ObfsType)
		if runtimeConfig.ObfsPassword != "" {
			query.Set("obfs-password", runtimeConfig.ObfsPassword)
		}
	}
	if runtimeConfig.UpMbps > 0 {
		query.Set("upmbps", strconv.Itoa(runtimeConfig.UpMbps))
	}
	if runtimeConfig.DownMbps > 0 {
		query.Set("downmbps", strconv.Itoa(runtimeConfig.DownMbps))
	}
	if runtimeConfig.IgnoreClientBandwidth {
		query.Set("ignore-client-bandwidth", "1")
	}

	uri := url.URL{
		Scheme:   "hy2",
		User:     url.User(strings.TrimSpace(password)),
		Host:     strings.TrimSpace(serverHost) + ":" + strconv.Itoa(runtimeConfig.Port),
		RawQuery: query.Encode(),
		Fragment: firstNonEmpty(displayName, "hy2"),
	}
	return uri.String()
}

func renderSingBoxHY2Outbound(
	user repository.UserWithCredentials,
	password string,
	serverHost string,
	runtimeConfig singBoxHY2RuntimeConfig,
) map[string]any {
	outbound := map[string]any{
		"type":        "hysteria2",
		"tag":         "hy2-" + firstNonEmpty(user.Name, user.ID),
		"server":      strings.TrimSpace(serverHost),
		"server_port": runtimeConfig.Port,
		"password":    strings.TrimSpace(password),
		"tls": map[string]any{
			"enabled": true,
		},
	}
	tls := outbound["tls"].(map[string]any)
	if runtimeConfig.ServerName != "" {
		tls["server_name"] = runtimeConfig.ServerName
	}
	if runtimeConfig.ObfsType != "" {
		obfs := map[string]any{
			"type": runtimeConfig.ObfsType,
		}
		if runtimeConfig.ObfsPassword != "" {
			obfs["password"] = runtimeConfig.ObfsPassword
		}
		outbound["obfs"] = obfs
	}
	if runtimeConfig.UpMbps > 0 {
		outbound["up_mbps"] = runtimeConfig.UpMbps
	}
	if runtimeConfig.DownMbps > 0 {
		outbound["down_mbps"] = runtimeConfig.DownMbps
	}
	if runtimeConfig.IgnoreClientBandwidth {
		outbound["ignore_client_bandwidth"] = true
	}
	return outbound
}

func renderClashHY2Node(
	user repository.UserWithCredentials,
	password string,
	serverHost string,
	runtimeConfig singBoxHY2RuntimeConfig,
) string {
	lines := []string{
		"- name: " + firstNonEmpty(user.Name, "hy2"),
		"  type: hysteria2",
		"  server: " + strings.TrimSpace(serverHost),
		"  port: " + strconv.Itoa(runtimeConfig.Port),
		"  password: " + strings.TrimSpace(password),
	}
	if runtimeConfig.ServerName != "" {
		lines = append(lines, "  sni: "+runtimeConfig.ServerName)
	}
	if runtimeConfig.ObfsType != "" {
		lines = append(lines, "  obfs: "+runtimeConfig.ObfsType)
		if runtimeConfig.ObfsPassword != "" {
			lines = append(lines, "  obfs-password: "+runtimeConfig.ObfsPassword)
		}
	}
	if runtimeConfig.UpMbps > 0 {
		lines = append(lines, "  up: "+strconv.Itoa(runtimeConfig.UpMbps))
	}
	if runtimeConfig.DownMbps > 0 {
		lines = append(lines, "  down: "+strconv.Itoa(runtimeConfig.DownMbps))
	}
	return strings.Join(lines, "\n")
}
