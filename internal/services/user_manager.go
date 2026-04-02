package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"h2v2/internal/config"
	"h2v2/internal/domain/panel"
	"h2v2/internal/repository"
	runtimecore "h2v2/internal/runtime"
)

type UserManager struct {
	cfg     config.Config
	repo    repository.Repository
	runtime *runtimecore.Runtime

	mu sync.Mutex

	subMu           sync.RWMutex
	subCacheVersion int64
	subCache        map[string]cachedSubscription
}

type cachedSubscription struct {
	version int64
	render  SubscriptionRender
}

type SubscriptionRender struct {
	User        repository.UserWithCredentials
	Body        []byte
	ContentType string
	Filename    string
	Headers     map[string]string
}

func NewUserManager(cfg config.Config, repo repository.Repository, runtime *runtimecore.Runtime) *UserManager {
	return &UserManager{
		cfg:      cfg,
		repo:     repo,
		runtime:  runtime,
		subCache: make(map[string]cachedSubscription),
	}
}

func (m *UserManager) ListUsers(ctx context.Context, limit int, offset int, protocol *repository.Protocol) ([]repository.UserWithCredentials, error) {
	return m.repo.ListUsers(ctx, limit, offset, protocol)
}

func (m *UserManager) GetUser(ctx context.Context, id string) (repository.UserWithCredentials, error) {
	return m.repo.GetUser(ctx, id)
}

func (m *UserManager) CreateUser(ctx context.Context, input repository.CreateUserInput) (repository.UserWithCredentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	created, err := m.repo.CreateUser(ctx, input)
	if err != nil {
		return repository.UserWithCredentials{}, err
	}
	if err := m.syncAllProtocols(ctx); err != nil {
		_ = m.repo.DeleteUsers(ctx, repository.BatchDeleteUsersInput{UserIDs: []string{created.ID}})
		_ = m.syncAllProtocols(ctx)
		return repository.UserWithCredentials{}, err
	}
	m.invalidateSubscriptions()
	return m.repo.GetUser(ctx, created.ID)
}

func (m *UserManager) UpdateUser(ctx context.Context, id string, input repository.UpdateUserInput) (repository.UserWithCredentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	previous, err := m.repo.GetUser(ctx, id)
	if err != nil {
		return repository.UserWithCredentials{}, err
	}
	updated, err := m.repo.UpdateUser(ctx, id, input)
	if err != nil {
		return repository.UserWithCredentials{}, err
	}
	if err := m.syncAllProtocols(ctx); err != nil {
		_, _ = m.repo.UpdateUser(ctx, id, updateInputFromUser(previous))
		_ = m.syncAllProtocols(ctx)
		return repository.UserWithCredentials{}, err
	}
	m.invalidateSubscriptions()
	return updated, nil
}

func (m *UserManager) SetUsersStateBatch(ctx context.Context, input repository.BatchUserStateInput) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := normalizeIDs(input.UserIDs)
	if len(ids) == 0 {
		return 0, repository.ErrNotFound
	}

	previous := make(map[string]repository.UserWithCredentials, len(ids))
	for _, id := range ids {
		item, err := m.repo.GetUser(ctx, id)
		if err != nil {
			return 0, err
		}
		previous[id] = item
	}

	updated, err := m.repo.SetUsersStateBatch(ctx, input)
	if err != nil {
		return 0, err
	}
	if err := m.syncAllProtocols(ctx); err != nil {
		for _, id := range ids {
			item := previous[id]
			_, _ = m.repo.SetUsersStateBatch(ctx, repository.BatchUserStateInput{UserIDs: []string{id}, Enabled: item.Enabled, Protocol: input.Protocol})
		}
		_ = m.syncAllProtocols(ctx)
		return 0, err
	}
	m.invalidateSubscriptions()
	return updated, nil
}

func (m *UserManager) DeleteUsers(ctx context.Context, input repository.BatchDeleteUsersInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := normalizeIDs(input.UserIDs)
	if len(ids) == 0 {
		return repository.ErrNotFound
	}

	previous := make(map[string]repository.UserWithCredentials, len(ids))
	enabledIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		item, err := m.repo.GetUser(ctx, id)
		if err != nil {
			return err
		}
		previous[id] = item
		if item.Enabled {
			enabledIDs = append(enabledIDs, id)
		}
	}

	if len(enabledIDs) > 0 {
		if _, err := m.repo.SetUsersStateBatch(ctx, repository.BatchUserStateInput{UserIDs: enabledIDs, Enabled: false}); err != nil {
			return err
		}
		if err := m.syncAllProtocols(ctx); err != nil {
			_, _ = m.repo.SetUsersStateBatch(ctx, repository.BatchUserStateInput{UserIDs: enabledIDs, Enabled: true})
			_ = m.syncAllProtocols(ctx)
			return err
		}
	}

	if err := m.repo.DeleteUsers(ctx, repository.BatchDeleteUsersInput{UserIDs: ids}); err != nil {
		if len(enabledIDs) > 0 {
			rollback := make([]string, 0, len(enabledIDs))
			for _, id := range enabledIDs {
				if previous[id].Enabled {
					rollback = append(rollback, id)
				}
			}
			if len(rollback) > 0 {
				_, _ = m.repo.SetUsersStateBatch(ctx, repository.BatchUserStateInput{UserIDs: rollback, Enabled: true})
				_ = m.syncAllProtocols(ctx)
			}
		}
		return err
	}

	m.invalidateSubscriptions()
	return nil
}

func (m *UserManager) KickUsers(ctx context.Context, ids []string) (int, error) {
	targets := normalizeIDs(ids)
	if len(targets) == 0 {
		return 0, repository.ErrNotFound
	}
	users := make([]repository.UserWithCredentials, 0, len(targets))
	for _, id := range targets {
		item, err := m.repo.GetUser(ctx, id)
		if err != nil {
			return 0, err
		}
		users = append(users, item)
	}

	kicked := 0
	var lastErr error
	for _, user := range users {
		for _, credential := range user.Credentials {
			adapter, ok := m.runtime.Adapter(credential.Protocol)
			if !ok {
				continue
			}
			if err := adapter.KickUser(ctx, user); err == nil {
				kicked++
				break
			} else {
				lastErr = err
			}
		}
	}
	if kicked == 0 && lastErr != nil {
		return 0, lastErr
	}
	return kicked, nil
}

func (m *UserManager) ListInbounds(ctx context.Context, protocol *repository.Protocol) ([]repository.Inbound, error) {
	return m.repo.ListInbounds(ctx, protocol)
}

func (m *UserManager) UpsertInbound(ctx context.Context, inbound repository.Inbound) (repository.Inbound, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var previous *repository.Inbound
	current, err := m.repo.GetInbound(ctx, inbound.ID)
	if err == nil {
		copy := current
		previous = &copy
	}

	saved, err := m.repo.UpsertInbound(ctx, inbound)
	if err != nil {
		return repository.Inbound{}, err
	}
	if err := m.syncAllProtocols(ctx); err != nil {
		if previous != nil {
			_, _ = m.repo.UpsertInbound(ctx, *previous)
		} else {
			_ = m.repo.DeleteInbound(ctx, saved.ID)
		}
		_ = m.syncAllProtocols(ctx)
		return repository.Inbound{}, err
	}
	m.invalidateSubscriptions()
	return saved, nil
}

func (m *UserManager) DeleteInbound(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	previous, err := m.repo.GetInbound(ctx, id)
	if err != nil {
		return err
	}
	if err := m.repo.DeleteInbound(ctx, id); err != nil {
		return err
	}
	if err := m.syncAllProtocols(ctx); err != nil {
		_, _ = m.repo.UpsertInbound(ctx, previous)
		_ = m.syncAllProtocols(ctx)
		return err
	}
	m.invalidateSubscriptions()
	return nil
}

func (m *UserManager) EnsureSubscriptionToken(ctx context.Context, userID string) (string, repository.SubscriptionToken, error) {
	state, err := m.repo.EnsureSubscriptionToken(ctx, userID)
	if err != nil {
		return "", repository.SubscriptionToken{}, err
	}
	token := buildSubscriptionTokenV2(m.cfg.InternalAuthToken, state.Subject, state.Version)
	return token, state, nil
}

func (m *UserManager) BuildUserArtifacts(ctx context.Context, user repository.UserWithCredentials) (map[repository.Protocol]runtimecore.UserArtifacts, string, error) {
	token, _, err := m.EnsureSubscriptionToken(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}
	subscriptionURL := strings.TrimRight(strings.TrimSpace(m.cfg.SubscriptionPublicURL), "/") + "/api/subscriptions/" + token
	inbounds, err := m.repo.ListInbounds(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	out := make(map[repository.Protocol]runtimecore.UserArtifacts, len(user.Credentials))
	buildErrors := make([]string, 0, len(user.Credentials))
	for _, credential := range user.Credentials {
		adapter, ok := m.runtime.Adapter(credential.Protocol)
		if !ok {
			buildErrors = append(buildErrors, "adapter "+string(credential.Protocol)+" is not configured")
			continue
		}
		artifact, artifactErr := adapter.BuildArtifacts(ctx, user, inbounds, subscriptionURL)
		if artifactErr != nil {
			buildErrors = append(buildErrors, string(credential.Protocol)+": "+artifactErr.Error())
			continue
		}
		out[credential.Protocol] = artifact
	}
	if len(out) == 0 {
		if len(buildErrors) == 0 {
			return nil, subscriptionURL, fmt.Errorf("failed to build user artifacts")
		}
		return nil, subscriptionURL, fmt.Errorf("failed to build user artifacts: %s", strings.Join(buildErrors, "; "))
	}
	return out, subscriptionURL, nil
}

func (m *UserManager) RotateSubscriptionToken(ctx context.Context, userID string) (string, repository.SubscriptionToken, error) {
	state, err := m.repo.RotateSubscriptionToken(ctx, userID)
	if err != nil {
		return "", repository.SubscriptionToken{}, err
	}
	m.invalidateSubscriptions()
	token := buildSubscriptionTokenV2(m.cfg.InternalAuthToken, state.Subject, state.Version)
	return token, state, nil
}

func (m *UserManager) RevokeSubscriptionToken(ctx context.Context, userID string) (repository.SubscriptionToken, error) {
	state, err := m.repo.RevokeSubscriptionToken(ctx, userID)
	if err != nil {
		return repository.SubscriptionToken{}, err
	}
	m.invalidateSubscriptions()
	return state, nil
}

func (m *UserManager) ClearSubscriptionRevocation(ctx context.Context, userID string) (repository.SubscriptionToken, error) {
	state, err := m.repo.ClearSubscriptionRevocation(ctx, userID)
	if err != nil {
		return repository.SubscriptionToken{}, err
	}
	m.invalidateSubscriptions()
	return state, nil
}

func (m *UserManager) RenderSubscription(ctx context.Context, token string, format string) (SubscriptionRender, error) {
	user, _, err := m.resolveToken(ctx, token)
	if err != nil {
		return SubscriptionRender{}, err
	}
	if !user.Enabled {
		return SubscriptionRender{}, repository.ErrNotFound
	}
	if err := panel.ValidateLifecycle(user.Enabled, user.TrafficLimitBytes, user.TrafficUsedTxBytes, user.TrafficUsedRxBytes, user.ExpireAt, time.Now().UTC()); err != nil {
		return SubscriptionRender{}, repository.ErrNotFound
	}

	inbounds, err := m.repo.ListInbounds(ctx, nil)
	if err != nil {
		return SubscriptionRender{}, err
	}
	state, err := m.repo.GetSubscriptionToken(ctx, user.ID)
	if err != nil {
		return SubscriptionRender{}, err
	}
	cacheKey := subscriptionCacheKey(token, format, user, state, inbounds)
	if cached, ok := m.readSubscriptionCache(cacheKey); ok {
		return cached, nil
	}
	items := make([]runtimecore.UserArtifacts, 0, len(user.Credentials))
	for _, credential := range user.Credentials {
		adapter, ok := m.runtime.Adapter(credential.Protocol)
		if !ok {
			continue
		}
		subURL := strings.TrimRight(strings.TrimSpace(m.cfg.SubscriptionPublicURL), "/") + "/api/subscriptions/" + urlEncode(token)
		artifacts, err := adapter.BuildArtifacts(ctx, user, inbounds, subURL)
		if err != nil {
			continue
		}
		items = append(items, artifacts)
	}
	if len(items) == 0 {
		return SubscriptionRender{}, repository.ErrNotFound
	}

	render := buildSubscriptionRender(user, items, format)
	m.writeSubscriptionCache(cacheKey, render)
	return render, nil
}

func (m *UserManager) CollectRuntime(ctx context.Context) error {
	for _, protocol := range m.sortedRuntimeProtocols() {
		adapter, ok := m.runtime.Adapter(protocol)
		if !ok {
			continue
		}
		users, err := m.repo.ListUsers(ctx, 0, 0, &protocol)
		if err != nil {
			return err
		}
		counters, err := adapter.CollectTraffic(ctx, users)
		if err != nil {
			return err
		}
		if protocol == repository.ProtocolHY2 {
			normalized := make([]repository.TrafficCounter, 0, len(counters))
			snapshots := make([]repository.HysteriaSnapshot, 0, len(counters))
			now := time.Now().UTC()
			for _, counter := range counters {
				snapshotAt := counter.SnapshotAt
				if snapshotAt.IsZero() {
					snapshotAt = now
				}
				normalized = append(normalized, repository.TrafficCounter{
					UserID:     counter.UserID,
					Protocol:   repository.ProtocolHY2,
					TxBytes:    counter.TxBytes,
					RxBytes:    counter.RxBytes,
					Online:     counter.Online,
					SnapshotAt: snapshotAt,
				})
				snapshots = append(snapshots, repository.HysteriaSnapshot{
					UserID:     counter.UserID,
					TxBytes:    counter.TxBytes,
					RxBytes:    counter.RxBytes,
					Online:     counter.Online,
					SnapshotAt: snapshotAt,
				})
				if counter.Online > 0 {
					_ = m.repo.TouchHysteriaUserLastSeen(ctx, counter.UserID, snapshotAt)
				}
			}
			if err := m.repo.InsertTrafficCounters(ctx, normalized); err != nil {
				return err
			}
			if err := m.repo.InsertHysteriaSnapshots(ctx, snapshots); err != nil {
				return err
			}
			continue
		}
		if err := m.repo.InsertTrafficCounters(ctx, counters); err != nil {
			return err
		}
	}
	return nil
}

func (m *UserManager) SyncAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.syncAllProtocols(ctx)
}

func (m *UserManager) syncAllProtocols(ctx context.Context) error {
	for _, protocol := range m.sortedRuntimeProtocols() {
		adapter, ok := m.runtime.Adapter(protocol)
		if !ok {
			continue
		}
		users, err := m.repo.ListUsers(ctx, 0, 0, &protocol)
		if err != nil {
			return err
		}
		inbounds, err := m.repo.ListInbounds(ctx, &protocol)
		if err != nil {
			return err
		}
		if err := adapter.SyncConfig(ctx, inbounds, users); err != nil {
			return err
		}
	}
	return nil
}

func (m *UserManager) sortedRuntimeProtocols() []repository.Protocol {
	protocols := m.runtime.Protocols()
	sort.Slice(protocols, func(i, j int) bool {
		return string(protocols[i]) < string(protocols[j])
	})
	return protocols
}

func (m *UserManager) resolveToken(ctx context.Context, token string) (repository.UserWithCredentials, repository.SubscriptionToken, error) {
	subject, version, ok := parseSubscriptionTokenV2(m.cfg.InternalAuthToken, token)
	if !ok {
		return repository.UserWithCredentials{}, repository.SubscriptionToken{}, repository.ErrNotFound
	}
	user, err := m.repo.GetUserBySubject(ctx, subject)
	if err != nil {
		return repository.UserWithCredentials{}, repository.SubscriptionToken{}, err
	}
	state, err := m.repo.GetSubscriptionToken(ctx, user.ID)
	if err != nil {
		return repository.UserWithCredentials{}, repository.SubscriptionToken{}, err
	}
	if state.Revoked || state.Subject != subject || state.Version != version {
		return repository.UserWithCredentials{}, repository.SubscriptionToken{}, repository.ErrNotFound
	}
	return user, state, nil
}

func buildSubscriptionRender(user repository.UserWithCredentials, artifacts []runtimecore.UserArtifacts, format string) SubscriptionRender {
	normalizedFormat := strings.ToLower(strings.TrimSpace(format))
	if normalizedFormat == "" {
		normalizedFormat = "uri"
	}
	headers := map[string]string{
		"Profile-Update-Interval": "6",
		"Profile-Title":           user.Name,
		"Subscription-Userinfo":   subscriptionUserInfo(user),
	}

	switch normalizedFormat {
	case "clash", "clash-meta":
		content := renderClashSubscription(artifacts)
		return SubscriptionRender{
			User:        user,
			Body:        []byte(content + "\n"),
			ContentType: "application/yaml; charset=utf-8",
			Filename:    user.Name + "-clash.yaml",
			Headers:     headers,
		}
	case "singbox", "sing-box":
		content := renderSingboxSubscription(artifacts)
		return SubscriptionRender{
			User:        user,
			Body:        []byte(content + "\n"),
			ContentType: "application/json; charset=utf-8",
			Filename:    user.Name + "-singbox.json",
			Headers:     headers,
		}
	default:
		lines := make([]string, 0, len(artifacts))
		for _, item := range artifacts {
			if strings.TrimSpace(item.AccessURI) == "" {
				continue
			}
			lines = append(lines, strings.TrimSpace(item.AccessURI))
		}
		return SubscriptionRender{
			User:        user,
			Body:        []byte(strings.Join(lines, "\n") + "\n"),
			ContentType: "text/plain; charset=utf-8",
			Filename:    user.Name + "-uri.txt",
			Headers:     headers,
		}
	}
}

func renderClashSubscription(artifacts []runtimecore.UserArtifacts) string {
	proxies := make([]string, 0, len(artifacts))
	names := make([]string, 0, len(artifacts))
	for index, item := range artifacts {
		node := strings.TrimSpace(item.ClashNode)
		if node == "" {
			continue
		}
		if strings.HasPrefix(node, "-") {
			proxies = append(proxies, "  "+strings.ReplaceAll(node, "\n", "\n  "))
			name := readProxyName(node)
			if name == "" {
				name = "proxy-" + strconv.Itoa(index+1)
			}
			names = append(names, name)
			continue
		}
		name := "proxy-" + strconv.Itoa(index+1)
		names = append(names, name)
		proxies = append(proxies, "  - name: "+name)
		proxies = append(proxies, "    type: url-test")
		proxies = append(proxies, "    url: "+node)
	}
	if len(proxies) == 0 {
		proxies = append(proxies, "  []")
	}
	if len(names) == 0 {
		names = append(names, "DIRECT")
	}
	proxyLines := make([]string, 0, len(names))
	for _, name := range names {
		proxyLines = append(proxyLines, "    - "+name)
	}
	return strings.Join([]string{
		"proxies:",
		strings.Join(proxies, "\n"),
		"proxy-groups:",
		"  - name: PROXY",
		"    type: select",
		"    proxies:",
		strings.Join(proxyLines, "\n"),
		"rules:",
		"  - MATCH,PROXY",
	}, "\n")
}

func renderSingboxSubscription(artifacts []runtimecore.UserArtifacts) string {
	outbounds := make([]map[string]any, 0, len(artifacts))
	for _, item := range artifacts {
		if len(item.SingBoxNode) == 0 {
			continue
		}
		outbounds = append(outbounds, item.SingBoxNode)
	}
	payload := map[string]any{"outbounds": outbounds}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return `{"outbounds":[]}`
	}
	return string(encoded)
}

func updateInputFromUser(user repository.UserWithCredentials) repository.UpdateUserInput {
	credentials := make([]repository.Credential, 0, len(user.Credentials))
	for _, item := range user.Credentials {
		credentials = append(credentials, item)
	}
	return repository.UpdateUserInput{
		Name:              user.Name,
		Note:              user.Note,
		Enabled:           user.Enabled,
		TrafficLimitBytes: user.TrafficLimitBytes,
		ExpireAt:          user.ExpireAt,
		Credentials:       credentials,
	}
}

func buildSubscriptionTokenV2(secret string, subject string, version int) string {
	payload := strings.TrimSpace(subject) + ":" + strconv.Itoa(version)
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := signSubscriptionTokenV2(strings.TrimSpace(secret), payload)
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func parseSubscriptionTokenV2(secret string, token string) (string, int, bool) {
	trimmed := strings.TrimSpace(token)
	parts := strings.Split(trimmed, ".")
	if len(parts) != 2 {
		return "", 0, false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", 0, false
	}
	signatureBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", 0, false
	}
	payload := strings.TrimSpace(string(payloadBytes))
	subject, versionRaw, ok := strings.Cut(payload, ":")
	if !ok {
		return "", 0, false
	}
	version, err := strconv.Atoi(strings.TrimSpace(versionRaw))
	if err != nil || version <= 0 {
		return "", 0, false
	}
	expected := signSubscriptionTokenV2(strings.TrimSpace(secret), payload)
	if !hmac.Equal(signatureBytes, expected) {
		return "", 0, false
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", 0, false
	}
	return subject, version, true
}

func signSubscriptionTokenV2(secret string, payload string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("sub-v2"))
	mac.Write([]byte("|"))
	mac.Write([]byte(strings.TrimSpace(payload)))
	return mac.Sum(nil)
}

func subscriptionUserInfo(user repository.UserWithCredentials) string {
	total := user.TrafficLimitBytes
	expires := int64(0)
	if user.ExpireAt != nil {
		expires = user.ExpireAt.UTC().Unix()
	}
	return "upload=" + strconv.FormatInt(user.TrafficUsedTxBytes, 10) +
		"; download=" + strconv.FormatInt(user.TrafficUsedRxBytes, 10) +
		"; total=" + strconv.FormatInt(total, 10) +
		"; expire=" + strconv.FormatInt(expires, 10)
}

func (m *UserManager) invalidateSubscriptions() {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	m.subCacheVersion++
	m.subCache = make(map[string]cachedSubscription)
}

func (m *UserManager) readSubscriptionCache(key string) (SubscriptionRender, bool) {
	m.subMu.RLock()
	defer m.subMu.RUnlock()
	entry, ok := m.subCache[key]
	if !ok || entry.version != m.subCacheVersion {
		return SubscriptionRender{}, false
	}
	return entry.render, true
}

func (m *UserManager) writeSubscriptionCache(key string, render SubscriptionRender) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	m.subCache[key] = cachedSubscription{version: m.subCacheVersion, render: render}
}

func normalizeIDs(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		id := strings.TrimSpace(item)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func urlEncode(value string) string {
	return url.PathEscape(strings.TrimSpace(value))
}

func readProxyName(node string) string {
	for _, line := range strings.Split(node, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- name:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:"))
		}
	}
	return ""
}

func subscriptionCacheKey(token string, format string, user repository.UserWithCredentials, state repository.SubscriptionToken, inbounds []repository.Inbound) string {
	maxInbound := int64(0)
	for _, inbound := range inbounds {
		ts := inbound.UpdatedAt.UTC().UnixNano()
		if ts > maxInbound {
			maxInbound = ts
		}
	}
	expireAt := int64(0)
	if user.ExpireAt != nil {
		expireAt = user.ExpireAt.UTC().UnixNano()
	}
	enabled := "0"
	if user.Enabled {
		enabled = "1"
	}
	return strings.TrimSpace(token) + "|" +
		strings.TrimSpace(strings.ToLower(format)) + "|" +
		strconv.FormatInt(user.UpdatedAt.UTC().UnixNano(), 10) + "|" +
		strconv.FormatInt(user.TrafficUsedTxBytes, 10) + "|" +
		strconv.FormatInt(user.TrafficUsedRxBytes, 10) + "|" +
		enabled + "|" +
		strconv.FormatInt(expireAt, 10) + "|" +
		strconv.FormatInt(state.UpdatedAt.UTC().UnixNano(), 10) + "|" +
		strconv.FormatInt(maxInbound, 10)
}
