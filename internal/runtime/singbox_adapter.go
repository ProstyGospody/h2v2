package runtime

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"h2v2/internal/fsutil"
	"h2v2/internal/repository"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultSingBoxServiceName   = "sing-box"
	defaultSingBoxBinaryPath    = "/usr/local/bin/sing-box"
	defaultVLESSListenPort      = 443
	defaultRealityServerName    = "www.cloudflare.com"
	defaultRealityHandshakePort = 443
	defaultVLESSFingerprint     = "chrome"
	defaultVLESSWSPath          = "/"
	defaultVLESSGRPCServiceName = "grpc"
	defaultSingBoxV2RayAPIListen = "127.0.0.1:10086"
	singBoxStatsQueryMethod      = "/v2ray.core.app.stats.command.StatsService/QueryStats"
	singBoxUserStatPrefix        = "user>>>"
	singBoxUserTrafficToken      = ">>>traffic>>>"
	singBoxFeatureV2RayAPI       = "with_v2ray_api"
)

type SingBoxAdapter struct {
	binaryPath         string
	configPath         string
	services           ServiceRestarter
	serviceName        string
	artifactHost       string
	v2rayAPIListen     string
	v2rayAPIEnabled    bool
	v2rayAPIUnsupported bool
	v2rayAPIMu         sync.RWMutex
}

type singBoxVLESSRuntimeConfig struct {
	Tag             string
	ListenHost      string
	Port            int
	Transport       singBoxVLESSTransport
	Security        string
	Flow            string
	ServerName      string
	TLSInsecure     bool
	TLSALPN         []string
	CertificatePath string
	KeyPath         string
	Reality         *singBoxRealitySettings
}

type singBoxVLESSTransport struct {
	Type        string
	WSPath      string
	WSHost      string
	GRPCService string
}

type singBoxRealitySettings struct {
	PrivateKey      string
	PublicKey       string
	ShortID         string
	HandshakeServer string
	HandshakePort   int
	Fingerprint     string
}

func NewSingBoxAdapter(binaryPath string, configPath string, svc ServiceRestarter, serviceName string, artifactHost string) *SingBoxAdapter {
	name := strings.TrimSpace(serviceName)
	if name == "" {
		name = defaultSingBoxServiceName
	}
	bin := strings.TrimSpace(binaryPath)
	if bin == "" {
		bin = defaultSingBoxBinaryPath
	}
	return &SingBoxAdapter{
		binaryPath:      bin,
		configPath:      strings.TrimSpace(configPath),
		services:        svc,
		serviceName:     name,
		artifactHost:    normalizePublicEndpointHost(artifactHost),
		v2rayAPIListen:  defaultSingBoxV2RayAPIListen,
		v2rayAPIEnabled: true,
	}
}

func (a *SingBoxAdapter) Protocol() repository.Protocol {
	return repository.ProtocolVLESS
}

func (a *SingBoxAdapter) SyncConfig(ctx context.Context, inbounds []repository.Inbound, users []repository.UserWithCredentials) error {
	enableStats := a.shouldUseV2RayStats()
	config, err := buildSingBoxVLESSConfigWithStats(inbounds, users, a.artifactHost, enableStats, a.v2rayAPIListen)
	if err != nil {
		return err
	}
	if err := a.applyConfig(config); err != nil {
		if enableStats && isSingBoxV2RayAPIUnsupportedError(err) {
			fallback, buildErr := buildSingBoxVLESSConfigWithStats(inbounds, users, a.artifactHost, false, "")
			if buildErr != nil {
				return buildErr
			}
			if fallbackErr := a.applyConfig(fallback); fallbackErr != nil {
				return fallbackErr
			}
			a.setV2RayStatsState(false, true)
			return a.restart(ctx)
		}
		return err
	}
	if enableStats {
		a.setV2RayStatsState(true, false)
	} else {
		a.setV2RayStatsEnabled(false)
	}
	return a.restart(ctx)
}

func (a *SingBoxAdapter) AddUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return fmt.Errorf("incremental vless mutation is not supported")
}

func (a *SingBoxAdapter) UpdateUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return fmt.Errorf("incremental vless mutation is not supported")
}

func (a *SingBoxAdapter) RemoveUser(context.Context, repository.UserWithCredentials, []repository.Inbound) error {
	return fmt.Errorf("incremental vless mutation is not supported")
}

func (a *SingBoxAdapter) SetUsersStateBatch(context.Context, []repository.UserWithCredentials, bool, []repository.Inbound) error {
	return fmt.Errorf("incremental vless mutation is not supported")
}

func (a *SingBoxAdapter) KickUser(context.Context, repository.UserWithCredentials) error {
	return fmt.Errorf("vless kick is not supported for sing-box runtime")
}

func (a *SingBoxAdapter) CollectTraffic(ctx context.Context, users []repository.UserWithCredentials) ([]repository.TrafficCounter, error) {
	if !a.v2rayStatsAvailable() || len(users) == 0 {
		return nil, nil
	}

	identityByName := mapVLESSStatsIdentity(users)
	if len(identityByName) == 0 {
		return nil, nil
	}

	statsByUser, err := a.queryV2RayUserStats(ctx)
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
			Protocol:   repository.ProtocolVLESS,
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

func (a *SingBoxAdapter) CollectOnline(ctx context.Context, users []repository.UserWithCredentials) (map[string]int, error) {
	if !a.v2rayStatsAvailable() || len(users) == 0 {
		return map[string]int{}, nil
	}

	identityByName := mapVLESSStatsIdentity(users)
	if len(identityByName) == 0 {
		return map[string]int{}, nil
	}

	statsByUser, err := a.queryV2RayUserStats(ctx)
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

func (a *SingBoxAdapter) BuildArtifacts(_ context.Context, user repository.UserWithCredentials, inbounds []repository.Inbound, subscriptionURL string) (UserArtifacts, error) {
	credential, ok := userCredential(user, repository.ProtocolVLESS)
	if !ok {
		return UserArtifacts{}, fmt.Errorf("vless credential is missing")
	}
	inbound, ok := selectEnabledVLESSInbound(inbounds)
	if !ok {
		return UserArtifacts{}, fmt.Errorf("enabled vless inbound is missing")
	}

	runtimeConfig, err := parseSingBoxVLESSInbound(inbound, a.artifactHost)
	if err != nil {
		return UserArtifacts{}, err
	}

	serverHost := a.resolveArtifactHost(inbound.Host)
	if serverHost == "" {
		return UserArtifacts{}, fmt.Errorf("public endpoint host is missing for vless artifacts")
	}
	serverName := resolveVLESSAccessServerName(runtimeConfig)

	uri, err := buildSingBoxVLESSURI(credential.Identity, user.Name, serverHost, runtimeConfig.Port, runtimeConfig, serverName)
	if err != nil {
		return UserArtifacts{}, err
	}
	singboxNode, err := renderSingBoxVLESSOutbound(user, credential, serverHost, runtimeConfig.Port, runtimeConfig, serverName)
	if err != nil {
		return UserArtifacts{}, err
	}
	clashNode := renderClashVLESSNode(user, credential, serverHost, runtimeConfig.Port, runtimeConfig, serverName)

	return UserArtifacts{
		Protocol:     repository.ProtocolVLESS,
		AccessURI:    uri,
		Config:       renderVLESSClientConfig(uri, clashNode, singboxNode),
		Subscription: strings.TrimSpace(subscriptionURL),
		ClashNode:    clashNode,
		SingBoxNode:  singboxNode,
	}, nil
}

func (a *SingBoxAdapter) resolveArtifactHost(raw string) string {
	host := normalizePublicEndpointHost(raw)
	if host != "" {
		return host
	}
	if a != nil && strings.TrimSpace(a.artifactHost) != "" {
		return a.artifactHost
	}
	return ""
}

func (a *SingBoxAdapter) applyConfig(config map[string]any) error {
	if strings.TrimSpace(a.configPath) == "" {
		return nil
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(a.configPath), 0o750); err != nil {
		return err
	}
	mode := os.FileMode(0o640)
	if st, err := os.Stat(a.configPath); err == nil {
		mode = st.Mode().Perm()
	}
	tempFile := filepath.Join(filepath.Dir(a.configPath), ".sing-box-config-test.json")
	if err := os.WriteFile(tempFile, data, mode); err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tempFile)
	}()
	if err := a.validateConfig(tempFile); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(a.configPath, data, mode)
}

func (a *SingBoxAdapter) validateConfig(path string) error {
	if strings.TrimSpace(a.binaryPath) == "" || strings.TrimSpace(path) == "" {
		return nil
	}
	ctxRun, cancelRun := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancelRun()
	out, err := exec.CommandContext(ctxRun, a.binaryPath, "check", "-c", path).CombinedOutput()
	if err == nil {
		return nil
	}
	details := strings.TrimSpace(string(out))
	if details == "" {
		details = "sing-box check command returned an empty error output"
	}
	return fmt.Errorf("sing-box config validation failed: %s", details)
}

func (a *SingBoxAdapter) restart(ctx context.Context) error {
	if a.services == nil {
		return nil
	}
	return a.services.Restart(ctx, a.serviceName)
}

func (a *SingBoxAdapter) v2rayStatsAvailable() bool {
	if a == nil {
		return false
	}
	a.v2rayAPIMu.RLock()
	defer a.v2rayAPIMu.RUnlock()
	return a.v2rayAPIEnabled
}

func (a *SingBoxAdapter) shouldUseV2RayStats() bool {
	if a == nil {
		return false
	}
	a.v2rayAPIMu.RLock()
	defer a.v2rayAPIMu.RUnlock()
	return !a.v2rayAPIUnsupported
}

func (a *SingBoxAdapter) setV2RayStatsState(enabled bool, unsupported bool) {
	if a == nil {
		return
	}
	a.v2rayAPIMu.Lock()
	a.v2rayAPIEnabled = enabled
	a.v2rayAPIUnsupported = unsupported
	a.v2rayAPIMu.Unlock()
}

func (a *SingBoxAdapter) setV2RayStatsEnabled(enabled bool) {
	if a == nil {
		return
	}
	a.v2rayAPIMu.Lock()
	a.v2rayAPIEnabled = enabled
	a.v2rayAPIMu.Unlock()
}

type singBoxUserTrafficStats struct {
	TxBytes int64
	RxBytes int64
	Online  int

	HasUplink   bool
	HasDownlink bool
	HasOnline   bool
}

func (s singBoxUserTrafficStats) hasTraffic() bool {
	return s.HasUplink || s.HasDownlink
}

func (a *SingBoxAdapter) queryV2RayUserStats(ctx context.Context) (map[string]singBoxUserTrafficStats, error) {
	listen := strings.TrimSpace(a.v2rayAPIListen)
	if listen == "" {
		listen = defaultSingBoxV2RayAPIListen
	}

	codec := singBoxV2RayCodec{}
	conn, err := grpc.DialContext(
		ctx,
		listen,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec)),
	)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	requests := []singBoxV2RayQueryStatsRequest{
		{},
		{Pattern: singBoxUserStatPrefix},
		{Pattern: "^user>>>.*", Regexp: true},
		{Patterns: []string{singBoxUserStatPrefix}},
	}

	response := &singBoxV2RayQueryStatsResponse{}
	var queryErr error
	hadSuccess := false
	for _, candidate := range requests {
		response.Stats = nil
		request := candidate
		if err := conn.Invoke(ctx, singBoxStatsQueryMethod, &request, response); err != nil {
			if !hadSuccess && queryErr == nil {
				queryErr = err
			}
			continue
		}
		hadSuccess = true
		queryErr = nil
		if len(response.Stats) > 0 {
			break
		}
	}
	if !hadSuccess && queryErr != nil {
		return nil, queryErr
	}

	stats := make(map[string]singBoxUserTrafficStats, len(response.Stats))
	for _, item := range response.Stats {
		userName, direction, ok := parseSingBoxUserTrafficName(item.Name)
		if ok {
			current := stats[userName]
			switch direction {
			case "uplink":
				current.TxBytes = item.Value
				current.HasUplink = true
			case "downlink":
				current.RxBytes = item.Value
				current.HasDownlink = true
			default:
				continue
			}
			stats[userName] = current
			continue
		}

		userName, ok = parseSingBoxUserOnlineName(item.Name)
		if !ok {
			continue
		}
		current := stats[userName]
		if item.Value < 0 {
			current.Online = 0
		} else {
			current.Online = int(item.Value)
		}
		current.HasOnline = true
		stats[userName] = current
	}
	return stats, nil
}

type singBoxV2RayQueryStatsRequest struct {
	Pattern  string
	Reset    bool
	Patterns []string
	Regexp   bool
}

type singBoxV2RayQueryStatsResponse struct {
	Stats []singBoxV2RayStat
}

type singBoxV2RayStat struct {
	Name  string
	Value int64
}

type singBoxV2RayCodec struct{}

func (singBoxV2RayCodec) Name() string {
	return "proto"
}

func (singBoxV2RayCodec) Marshal(value any) ([]byte, error) {
	request, ok := value.(*singBoxV2RayQueryStatsRequest)
	if !ok {
		return nil, fmt.Errorf("unsupported v2ray request type")
	}
	return marshalSingBoxQueryStatsRequest(*request), nil
}

func (singBoxV2RayCodec) Unmarshal(data []byte, value any) error {
	response, ok := value.(*singBoxV2RayQueryStatsResponse)
	if !ok {
		return fmt.Errorf("unsupported v2ray response type")
	}
	stats, err := unmarshalSingBoxQueryStatsResponse(data)
	if err != nil {
		return err
	}
	response.Stats = stats
	return nil
}

func buildSingBoxVLESSConfig(inbounds []repository.Inbound, users []repository.UserWithCredentials, artifactHost string) (map[string]any, error) {
	return buildSingBoxVLESSConfigWithStats(inbounds, users, artifactHost, false, "")
}

func buildSingBoxVLESSConfigWithStats(
	inbounds []repository.Inbound,
	users []repository.UserWithCredentials,
	artifactHost string,
	enableV2RayStats bool,
	v2rayStatsListen string,
) (map[string]any, error) {
	now := time.Now().UTC()
	renderedInbounds := make([]map[string]any, 0, len(inbounds))
	statsUsers := make(map[string]struct{}, len(users))
	statsInbounds := make(map[string]struct{}, len(inbounds))

	for _, inbound := range inbounds {
		if inbound.Protocol != repository.ProtocolVLESS || !inbound.Enabled {
			continue
		}

		runtimeConfig, err := parseSingBoxVLESSInbound(inbound, artifactHost)
		if err != nil {
			return nil, fmt.Errorf("vless inbound %s is invalid: %w", runtimeConfigTagFallback(inbound), err)
		}

		userEntries := make([]map[string]any, 0, len(users))
		for _, user := range users {
			if !isActiveRuntimeUser(user, now) {
				continue
			}
			credential, ok := userCredential(user, repository.ProtocolVLESS)
			if !ok {
				continue
			}
			uuidValue := strings.TrimSpace(credential.Identity)
			if uuidValue == "" {
				continue
			}
			statsName := resolveVLESSStatsName(user, credential)
			if statsName == "" {
				statsName = uuidValue
			}
			entry := map[string]any{
				"name": statsName,
				"uuid": uuidValue,
			}
			if runtimeConfig.Flow != "" {
				entry["flow"] = runtimeConfig.Flow
			}
			userEntries = append(userEntries, entry)
			if enableV2RayStats {
				statsCandidate := strings.TrimSpace(statsName)
				if statsCandidate != "" {
					statsUsers[statsCandidate] = struct{}{}
				}
			}
		}
		if len(userEntries) == 0 {
			continue
		}

		inboundPayload, err := renderSingBoxVLESSInbound(runtimeConfig, userEntries)
		if err != nil {
			return nil, fmt.Errorf("vless inbound %s is invalid: %w", runtimeConfig.Tag, err)
		}
		renderedInbounds = append(renderedInbounds, inboundPayload)
		if enableV2RayStats {
			statsInbounds[runtimeConfig.Tag] = struct{}{}
		}
	}

	payload := map[string]any{
		"log": map[string]any{"level": "warn"},
		"inbounds": renderedInbounds,
		"outbounds": []map[string]any{
			{"type": "direct", "tag": "direct"},
			{"type": "block", "tag": "block"},
		},
		"route": map[string]any{
			"auto_detect_interface": true,
			"final":                 "direct",
		},
	}

	if enableV2RayStats {
		listen := strings.TrimSpace(v2rayStatsListen)
		if listen == "" {
			listen = defaultSingBoxV2RayAPIListen
		}
		payload["experimental"] = map[string]any{
			"v2ray_api": map[string]any{
				"listen": listen,
				"stats": map[string]any{
					"enabled":  true,
					"inbounds": sortedKeys(statsInbounds),
					"users":    sortedKeys(statsUsers),
				},
			},
		}
	}

	return payload, nil
}

func parseSingBoxVLESSInbound(inbound repository.Inbound, _ string) (singBoxVLESSRuntimeConfig, error) {
	params := parseJSONMap(inbound.ParamsJSON)
	security := normalizeVLESSSecurity(firstNonEmpty(inbound.Security, readString(params, "security")))
	transport := normalizeVLESSTransport(inbound.Transport, params)
	flow, err := normalizeVLESSFlow(readString(params, "flow"))
	if err != nil {
		return singBoxVLESSRuntimeConfig{}, err
	}

	serverName := firstNonEmpty(
		readStringOrFirst(params, "sni"),
		readStringOrFirst(params, "server_name"),
		readStringOrFirst(params, "serverName"),
	)
	tlsALPN := readStringSlice(params, "alpn")
	certificatePath := firstNonEmpty(
		readString(params, "certificate_path"),
		readString(params, "certificatePath"),
		readString(params, "certPath"),
	)
	keyPath := firstNonEmpty(
		readString(params, "key_path"),
		readString(params, "keyPath"),
	)

	runtimeConfig := singBoxVLESSRuntimeConfig{
		Tag:             runtimeConfigTagFallback(inbound),
		ListenHost:      resolveSingBoxListenHost(inbound.Host),
		Port:            normalizeInboundPort(inbound.Port),
		Transport:       transport,
		Security:        security,
		Flow:            flow,
		ServerName:      serverName,
		TLSInsecure:     readBool(params, "insecure", false),
		TLSALPN:         tlsALPN,
		CertificatePath: certificatePath,
		KeyPath:         keyPath,
	}

	if runtimeConfig.Flow != "" {
		if runtimeConfig.Security == "none" {
			return singBoxVLESSRuntimeConfig{}, fmt.Errorf("vless flow requires tls or reality security")
		}
		if runtimeConfig.Transport.Type != "tcp" {
			return singBoxVLESSRuntimeConfig{}, fmt.Errorf("vless flow requires tcp transport")
		}
	}

	if security == "none" {
		return runtimeConfig, nil
	}

	if security == "tls" {
		if runtimeConfig.CertificatePath == "" || runtimeConfig.KeyPath == "" {
			return singBoxVLESSRuntimeConfig{}, fmt.Errorf("tls inbound requires certificate_path and key_path")
		}
		return runtimeConfig, nil
	}

	if security != "reality" {
		return singBoxVLESSRuntimeConfig{}, fmt.Errorf("unsupported vless security")
	}
	if runtimeConfig.Transport.Type != "tcp" {
		return singBoxVLESSRuntimeConfig{}, fmt.Errorf("reality requires tcp transport")
	}

	if err := normalizeRealityParams("reality", params); err != nil {
		return singBoxVLESSRuntimeConfig{}, err
	}

	reality, err := parseRealitySettings(params, firstNonEmpty(serverName, defaultRealityServerName))
	if err != nil {
		return singBoxVLESSRuntimeConfig{}, err
	}
	runtimeConfig.Reality = &reality

	if runtimeConfig.ServerName == "" {
		runtimeConfig.ServerName = inferDomainFromHost(reality.HandshakeServer)
	}
	return runtimeConfig, nil
}

func parseRealitySettings(params map[string]any, fallbackServerName string) (singBoxRealitySettings, error) {
	privateKey := strings.TrimSpace(readString(params, "privateKey"))
	if privateKey == "" {
		return singBoxRealitySettings{}, fmt.Errorf("reality private key is missing")
	}
	privateKeyDecoded, err := decodeRealityKey(privateKey)
	if err != nil {
		return singBoxRealitySettings{}, fmt.Errorf("reality private key is invalid")
	}
	privateKey = encodeRealityKey(privateKeyDecoded)

	publicKey := strings.TrimSpace(readString(params, "pbk"))
	if publicKey == "" {
		return singBoxRealitySettings{}, fmt.Errorf("reality public key is missing")
	}
	publicKeyDecoded, err := decodeRealityKey(publicKey)
	if err != nil {
		return singBoxRealitySettings{}, fmt.Errorf("reality public key is invalid")
	}
	publicKey = encodeRealityKey(publicKeyDecoded)

	shortID := strings.ToLower(strings.TrimSpace(readString(params, "sid")))
	if err := validateSingBoxRealityShortID(shortID); err != nil {
		return singBoxRealitySettings{}, err
	}

	handshakeServer, handshakePort := resolveRealityHandshakeTarget(params, fallbackServerName)
	if handshakeServer == "" {
		handshakeServer = defaultRealityServerName
	}
	if handshakePort <= 0 {
		handshakePort = defaultRealityHandshakePort
	}

	return singBoxRealitySettings{
		PrivateKey:      privateKey,
		PublicKey:       publicKey,
		ShortID:         shortID,
		HandshakeServer: handshakeServer,
		HandshakePort:   handshakePort,
		Fingerprint:     firstNonEmpty(readString(params, "fp"), defaultVLESSFingerprint),
	}, nil
}

func renderSingBoxVLESSInbound(runtimeConfig singBoxVLESSRuntimeConfig, users []map[string]any) (map[string]any, error) {
	inbound := map[string]any{
		"type":        "vless",
		"tag":         runtimeConfig.Tag,
		"listen":      runtimeConfig.ListenHost,
		"listen_port": runtimeConfig.Port,
		"users":       users,
	}

	if runtimeConfig.Transport.Type != "tcp" {
		transportMap := map[string]any{"type": runtimeConfig.Transport.Type}
		switch runtimeConfig.Transport.Type {
		case "ws":
			transportMap["path"] = firstNonEmpty(runtimeConfig.Transport.WSPath, defaultVLESSWSPath)
			if runtimeConfig.Transport.WSHost != "" {
				transportMap["headers"] = map[string]any{"Host": runtimeConfig.Transport.WSHost}
			}
		case "grpc":
			transportMap["service_name"] = firstNonEmpty(runtimeConfig.Transport.GRPCService, defaultVLESSGRPCServiceName)
		default:
			return nil, fmt.Errorf("unsupported vless transport")
		}
		inbound["transport"] = transportMap
	}

	tls, err := renderSingBoxInboundTLS(runtimeConfig)
	if err != nil {
		return nil, err
	}
	if len(tls) > 0 {
		inbound["tls"] = tls
	}

	return inbound, nil
}

func renderSingBoxInboundTLS(runtimeConfig singBoxVLESSRuntimeConfig) (map[string]any, error) {
	if runtimeConfig.Security == "none" {
		return nil, nil
	}

	tls := map[string]any{"enabled": true}

	if runtimeConfig.ServerName != "" {
		tls["server_name"] = runtimeConfig.ServerName
	}
	if len(runtimeConfig.TLSALPN) > 0 {
		tls["alpn"] = runtimeConfig.TLSALPN
	}
	if runtimeConfig.CertificatePath != "" {
		tls["certificate_path"] = runtimeConfig.CertificatePath
	}
	if runtimeConfig.KeyPath != "" {
		tls["key_path"] = runtimeConfig.KeyPath
	}

	if runtimeConfig.Security != "reality" {
		return tls, nil
	}
	if runtimeConfig.Reality == nil {
		return nil, fmt.Errorf("reality settings are missing")
	}

	reality := map[string]any{
		"enabled":     true,
		"private_key": runtimeConfig.Reality.PrivateKey,
		"handshake": map[string]any{
			"server":      runtimeConfig.Reality.HandshakeServer,
			"server_port": runtimeConfig.Reality.HandshakePort,
		},
	}
	if runtimeConfig.Reality.ShortID != "" {
		reality["short_id"] = []string{runtimeConfig.Reality.ShortID}
	}
	tls["reality"] = reality
	return tls, nil
}

func buildSingBoxVLESSURI(
	uuidValue string,
	displayName string,
	serverHost string,
	serverPort int,
	runtimeConfig singBoxVLESSRuntimeConfig,
	serverName string,
) (string, error) {
	query := url.Values{}
	query.Set("type", runtimeConfig.Transport.Type)
	query.Set("encryption", "none")
	query.Set("packetEncoding", "xudp")

	if runtimeConfig.Security == "tls" || runtimeConfig.Security == "reality" {
		query.Set("security", runtimeConfig.Security)
	}
	if serverName != "" {
		query.Set("sni", serverName)
	}
	if runtimeConfig.Flow != "" {
		query.Set("flow", runtimeConfig.Flow)
	}

	if runtimeConfig.Security == "reality" {
		if runtimeConfig.Reality == nil {
			return "", fmt.Errorf("reality settings are missing")
		}
		query.Set("pbk", runtimeConfig.Reality.PublicKey)
		if runtimeConfig.Reality.ShortID != "" {
			query.Set("sid", runtimeConfig.Reality.ShortID)
		}
		query.Set("fp", runtimeConfig.Reality.Fingerprint)
	}

	switch runtimeConfig.Transport.Type {
	case "ws":
		query.Set("path", firstNonEmpty(runtimeConfig.Transport.WSPath, defaultVLESSWSPath))
		if runtimeConfig.Transport.WSHost != "" {
			query.Set("host", runtimeConfig.Transport.WSHost)
		}
	case "grpc":
		query.Set("serviceName", firstNonEmpty(runtimeConfig.Transport.GRPCService, defaultVLESSGRPCServiceName))
	}

	tag := strings.TrimSpace(displayName)
	if tag == "" {
		tag = strings.TrimSpace(uuidValue)
	}

	uri := &url.URL{
		Scheme:   "vless",
		User:     url.User(uuidValue),
		Host:     fmt.Sprintf("%s:%d", strings.TrimSpace(serverHost), serverPort),
		RawQuery: query.Encode(),
		Fragment: tag,
	}
	return uri.String(), nil
}

func renderSingBoxVLESSOutbound(
	user repository.UserWithCredentials,
	credential repository.Credential,
	serverHost string,
	serverPort int,
	runtimeConfig singBoxVLESSRuntimeConfig,
	serverName string,
) (map[string]any, error) {
	outbound := map[string]any{
		"type":            "vless",
		"tag":             "vless-" + firstNonEmpty(user.Name, credential.Identity),
		"server":          strings.TrimSpace(serverHost),
		"server_port":     serverPort,
		"uuid":            credential.Identity,
		"packet_encoding": "xudp",
	}
	if runtimeConfig.Flow != "" {
		outbound["flow"] = runtimeConfig.Flow
	}

	if runtimeConfig.Security == "tls" || runtimeConfig.Security == "reality" {
		tls := map[string]any{"enabled": true}
		if serverName != "" {
			tls["server_name"] = serverName
		}
		if runtimeConfig.TLSInsecure {
			tls["insecure"] = true
		}
		if len(runtimeConfig.TLSALPN) > 0 {
			tls["alpn"] = runtimeConfig.TLSALPN
		}
		if runtimeConfig.Security == "reality" {
			if runtimeConfig.Reality == nil {
				return nil, fmt.Errorf("reality settings are missing")
			}
			reality := map[string]any{
				"enabled":    true,
				"public_key": runtimeConfig.Reality.PublicKey,
			}
			if runtimeConfig.Reality.ShortID != "" {
				reality["short_id"] = runtimeConfig.Reality.ShortID
			}
			tls["reality"] = reality
			tls["utls"] = map[string]any{
				"enabled":     true,
				"fingerprint": runtimeConfig.Reality.Fingerprint,
			}
		}
		outbound["tls"] = tls
	}

	if runtimeConfig.Transport.Type != "tcp" {
		transportMap := map[string]any{"type": runtimeConfig.Transport.Type}
		switch runtimeConfig.Transport.Type {
		case "ws":
			transportMap["path"] = firstNonEmpty(runtimeConfig.Transport.WSPath, defaultVLESSWSPath)
			if runtimeConfig.Transport.WSHost != "" {
				transportMap["headers"] = map[string]any{"Host": runtimeConfig.Transport.WSHost}
			}
		case "grpc":
			transportMap["service_name"] = firstNonEmpty(runtimeConfig.Transport.GRPCService, defaultVLESSGRPCServiceName)
		}
		outbound["transport"] = transportMap
	}

	return outbound, nil
}

func renderClashVLESSNode(
	user repository.UserWithCredentials,
	credential repository.Credential,
	serverHost string,
	serverPort int,
	runtimeConfig singBoxVLESSRuntimeConfig,
	serverName string,
) string {
	lines := []string{
		"- name: " + firstNonEmpty(user.Name, credential.Identity),
		"  type: vless",
		"  server: " + strings.TrimSpace(serverHost),
		"  port: " + strconv.Itoa(serverPort),
		"  uuid: " + credential.Identity,
		"  network: " + runtimeConfig.Transport.Type,
	}
	if runtimeConfig.Flow != "" {
		lines = append(lines, "  flow: "+runtimeConfig.Flow)
	}
	if runtimeConfig.Security == "none" {
		lines = append(lines, "  tls: false")
	} else {
		lines = append(lines, "  tls: true")
		if serverName != "" {
			lines = append(lines, "  servername: "+serverName)
		}
		if runtimeConfig.Security == "reality" && runtimeConfig.Reality != nil {
			lines = append(lines, "  reality-opts:")
			lines = append(lines, "    public-key: "+runtimeConfig.Reality.PublicKey)
			if runtimeConfig.Reality.ShortID != "" {
				lines = append(lines, "    short-id: "+runtimeConfig.Reality.ShortID)
			}
		}
	}
	switch runtimeConfig.Transport.Type {
	case "ws":
		lines = append(lines, "  ws-opts:")
		lines = append(lines, "    path: "+firstNonEmpty(runtimeConfig.Transport.WSPath, defaultVLESSWSPath))
		if runtimeConfig.Transport.WSHost != "" {
			lines = append(lines, "    headers:")
			lines = append(lines, "      Host: "+runtimeConfig.Transport.WSHost)
		}
	case "grpc":
		lines = append(lines, "  grpc-opts:")
		lines = append(lines, "    grpc-service-name: "+firstNonEmpty(runtimeConfig.Transport.GRPCService, defaultVLESSGRPCServiceName))
	}
	lines = append(lines, "  packet-encoding: xudp")
	return strings.Join(lines, "\n")
}

func resolveVLESSAccessServerName(runtimeConfig singBoxVLESSRuntimeConfig) string {
	if runtimeConfig.ServerName != "" {
		return runtimeConfig.ServerName
	}
	if runtimeConfig.Security != "reality" || runtimeConfig.Reality == nil {
		return ""
	}
	if inferred := inferDomainFromHost(runtimeConfig.Reality.HandshakeServer); inferred != "" {
		return inferred
	}
	return defaultRealityServerName
}

func selectEnabledVLESSInbound(inbounds []repository.Inbound) (repository.Inbound, bool) {
	for _, inbound := range inbounds {
		if inbound.Protocol == repository.ProtocolVLESS && inbound.Enabled {
			return inbound, true
		}
	}
	return repository.Inbound{}, false
}

func isActiveRuntimeUser(user repository.UserWithCredentials, now time.Time) bool {
	if !user.Enabled {
		return false
	}
	if user.ExpireAt != nil && !user.ExpireAt.After(now) {
		return false
	}
	if user.TrafficLimitBytes > 0 && (user.TrafficUsedTxBytes+user.TrafficUsedRxBytes) >= user.TrafficLimitBytes {
		return false
	}
	return true
}

func runtimeConfigTagFallback(inbound repository.Inbound) string {
	return firstNonEmpty(inbound.Name, inbound.ID, "vless")
}

func normalizeVLESSSecurity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "reality":
		return "reality"
	case "tls", "xtls":
		return "tls"
	default:
		return "none"
	}
}

func normalizeVLESSTransport(raw string, params map[string]any) singBoxVLESSTransport {
	transportType := strings.ToLower(strings.TrimSpace(raw))
	if transportType == "" {
		transportType = strings.ToLower(strings.TrimSpace(readString(params, "network")))
	}
	if transportType == "" {
		transportType = strings.ToLower(strings.TrimSpace(readString(params, "transport_type")))
	}
	switch transportType {
	case "ws":
		return singBoxVLESSTransport{
			Type:   "ws",
			WSPath: firstNonEmpty(readString(params, "path"), defaultVLESSWSPath),
			WSHost: strings.TrimSpace(readString(params, "host")),
		}
	case "grpc":
		return singBoxVLESSTransport{
			Type:        "grpc",
			GRPCService: firstNonEmpty(readString(params, "service_name"), readString(params, "serviceName"), defaultVLESSGRPCServiceName),
		}
	default:
		return singBoxVLESSTransport{Type: "tcp"}
	}
}

func normalizeVLESSFlow(raw string) (string, error) {
	flow := strings.TrimSpace(raw)
	if flow == "" {
		return "", nil
	}
	if flow != "xtls-rprx-vision" {
		return "", fmt.Errorf("unsupported vless flow: %s", flow)
	}
	return flow, nil
}

func normalizeInboundPort(port int) int {
	if port <= 0 {
		return defaultVLESSListenPort
	}
	return port
}

func resolveSingBoxListenHost(raw string) string {
	host := normalizeHostOnly(raw)
	if host == "" {
		return "0.0.0.0"
	}
	normalized := strings.ToLower(host)
	switch normalized {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0", "::":
		return "0.0.0.0"
	}
	if parsed := net.ParseIP(host); parsed != nil {
		if parsed.IsLoopback() || parsed.IsUnspecified() {
			return "0.0.0.0"
		}
		return host
	}
	return "0.0.0.0"
}

func resolveRealityHandshakeTarget(params map[string]any, fallbackHost string) (string, int) {
	host, port := parseHostPortLoose(readString(params, "dest"), defaultRealityHandshakePort)

	if host == "" {
		host = strings.TrimSpace(readString(params, "handshake_server"))
	}
	if host == "" {
		host = strings.TrimSpace(readString(params, "handshakeServer"))
	}

	if port <= 0 {
		port = readInt(params, "handshake_server_port", 0)
	}
	if port <= 0 {
		port = readInt(params, "handshakeServerPort", 0)
	}
	if port <= 0 {
		port = readInt(params, "handshake_port", 0)
	}

	host = normalizeHostOnly(host)
	if host == "" {
		host = normalizeHostOnly(fallbackHost)
	}
	if host == "" {
		host = defaultRealityServerName
	}
	if port <= 0 {
		port = defaultRealityHandshakePort
	}
	return host, port
}

func validateSingBoxRealityShortID(raw string) error {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return nil
	}
	if len(value) > 16 || len(value)%2 != 0 {
		return fmt.Errorf("reality short id is invalid")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("reality short id is invalid")
	}
	return nil
}

func inferDomainFromHost(raw string) string {
	host := strings.TrimSpace(normalizeHostOnly(raw))
	if host == "" {
		return ""
	}
	if net.ParseIP(host) != nil {
		return ""
	}
	return host
}

func parseHostPortLoose(raw string, fallbackPort int) (string, int) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fallbackPort
	}
	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil {
			value = parsed.Host
		}
	}
	if host, portRaw, err := net.SplitHostPort(value); err == nil {
		port, convErr := strconv.Atoi(strings.TrimSpace(portRaw))
		if convErr == nil && port > 0 {
			return strings.Trim(strings.TrimSpace(host), "[]"), port
		}
		return strings.Trim(strings.TrimSpace(host), "[]"), fallbackPort
	}
	if strings.Count(value, ":") == 1 {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 {
			port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err == nil && port > 0 {
				return strings.Trim(strings.TrimSpace(parts[0]), "[]"), port
			}
		}
	}
	return strings.Trim(strings.TrimSpace(value), "[]"), fallbackPort
}

func readBool(source map[string]any, key string, fallback bool) bool {
	if source == nil {
		return fallback
	}
	value, ok := source[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return fallback
		}
		return parsed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return fallback
	}
}

func readStringOrFirst(source map[string]any, key string) string {
	if source == nil {
		return ""
	}
	primary := strings.TrimSpace(readString(source, key))
	if primary != "" {
		return primary
	}
	parts := readStringSlice(source, key)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func detectSingBoxFeature(binaryPath string, feature string) bool {
	bin := strings.TrimSpace(binaryPath)
	required := strings.ToLower(strings.TrimSpace(feature))
	if bin == "" || required == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, bin, "version").CombinedOutput()
	if err != nil {
		return false
	}
	normalized := strings.ToLower(string(output))
	if strings.Contains(normalized, required) {
		return true
	}
	return false
}

func isSingBoxV2RayAPIUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "" {
		return false
	}
	if strings.Contains(message, singBoxFeatureV2RayAPI) {
		return true
	}
	switch {
	case strings.Contains(message, "unknown field \"v2ray_api\""),
		strings.Contains(message, "unknown field v2ray_api"),
		strings.Contains(message, "v2ray api is disabled"),
		strings.Contains(message, "v2ray api is not enabled"),
		strings.Contains(message, "v2ray api is unsupported"),
		strings.Contains(message, "v2ray api is unavailable"):
		return true
	default:
		return false
	}
}

func mapVLESSStatsIdentity(users []repository.UserWithCredentials) map[string]string {
	identityByName := make(map[string]string, len(users)*4)
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
		credential, ok := userCredential(user, repository.ProtocolVLESS)
		if !ok {
			continue
		}
		statsName := resolveVLESSStatsName(user, credential)
		mapIdentity(statsName, user.ID)
		mapIdentity(credential.Identity, user.ID)
		mapIdentity(user.Name, user.ID)
	}
	return identityByName
}

func resolveVLESSStatsName(user repository.UserWithCredentials, credential repository.Credential) string {
	uuidCandidate := strings.TrimSpace(credential.Identity)
	if uuidCandidate != "" {
		return uuidCandidate
	}
	return strings.TrimSpace(user.Name)
}

func parseSingBoxUserTrafficName(raw string) (string, string, bool) {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, singBoxUserStatPrefix) {
		return "", "", false
	}
	remaining := strings.TrimPrefix(value, singBoxUserStatPrefix)
	parts := strings.SplitN(remaining, singBoxUserTrafficToken, 2)
	if len(parts) == 2 {
		userName := strings.TrimSpace(parts[0])
		direction := strings.ToLower(strings.TrimSpace(parts[1]))
		if userName != "" && (direction == "uplink" || direction == "downlink") {
			return userName, direction, true
		}
	}

	segments := strings.Split(remaining, ">>>")
	if len(segments) < 3 {
		return "", "", false
	}
	userName := strings.TrimSpace(segments[0])
	if userName == "" {
		return "", "", false
	}
	for index := 1; index < len(segments)-1; index++ {
		token := strings.ToLower(strings.TrimSpace(segments[index]))
		if token != "traffic" {
			continue
		}
		direction := strings.ToLower(strings.TrimSpace(segments[index+1]))
		if direction == "uplink" || direction == "downlink" {
			return userName, direction, true
		}
	}
	return "", "", false
}

func parseSingBoxUserOnlineName(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, singBoxUserStatPrefix) {
		return "", false
	}
	remaining := strings.TrimPrefix(value, singBoxUserStatPrefix)
	if remaining == "" {
		return "", false
	}
	parts := strings.Split(remaining, ">>>")
	if len(parts) < 2 {
		return "", false
	}

	userName := strings.TrimSpace(parts[0])
	if userName == "" {
		return "", false
	}
	last := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	if last == "uplink" || last == "downlink" {
		return "", false
	}

	for _, tokenRaw := range parts[1:] {
		token := strings.ToLower(strings.TrimSpace(tokenRaw))
		token = strings.ReplaceAll(token, "_", "")
		token = strings.ReplaceAll(token, "-", "")
		if token == "uplink" || token == "downlink" {
			return "", false
		}
		switch token {
		case "online", "active", "connection", "connections", "conn", "session", "sessions":
			return userName, true
		}
		if strings.Contains(token, "online") || strings.Contains(token, "connection") || strings.Contains(token, "session") {
			return userName, true
		}
	}

	return "", false
}

func sortedKeys(items map[string]struct{}) []string {
	if len(items) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for key := range items {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func marshalSingBoxQueryStatsRequest(request singBoxV2RayQueryStatsRequest) []byte {
	out := make([]byte, 0, 96)
	if pattern := strings.TrimSpace(request.Pattern); pattern != "" {
		out = appendProtoStringField(out, 1, pattern)
	}
	if request.Reset {
		out = appendProtoBoolField(out, 2, true)
	}
	for _, raw := range request.Patterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}
		out = appendProtoStringField(out, 3, pattern)
	}
	if request.Regexp {
		out = appendProtoBoolField(out, 4, true)
	}
	return out
}

func unmarshalSingBoxQueryStatsResponse(data []byte) ([]singBoxV2RayStat, error) {
	cursor := data
	stats := make([]singBoxV2RayStat, 0, 8)

	for len(cursor) > 0 {
		fieldNumber, wireType, tagBytes, err := consumeProtoTag(cursor)
		if err != nil {
			return nil, err
		}
		cursor = cursor[tagBytes:]

		if fieldNumber == 1 && wireType == 2 {
			encodedStat, consumed, err := consumeProtoBytes(cursor)
			if err != nil {
				return nil, err
			}
			stat, err := unmarshalSingBoxStat(encodedStat)
			if err != nil {
				return nil, err
			}
			stats = append(stats, stat)
			cursor = cursor[consumed:]
			continue
		}

		consumed, err := skipProtoField(cursor, wireType)
		if err != nil {
			return nil, err
		}
		cursor = cursor[consumed:]
	}

	return stats, nil
}

func unmarshalSingBoxStat(data []byte) (singBoxV2RayStat, error) {
	cursor := data
	stat := singBoxV2RayStat{}

	for len(cursor) > 0 {
		fieldNumber, wireType, tagBytes, err := consumeProtoTag(cursor)
		if err != nil {
			return singBoxV2RayStat{}, err
		}
		cursor = cursor[tagBytes:]

		switch {
		case fieldNumber == 1 && wireType == 2:
			name, consumed, err := consumeProtoString(cursor)
			if err != nil {
				return singBoxV2RayStat{}, err
			}
			stat.Name = name
			cursor = cursor[consumed:]
		case fieldNumber == 2 && wireType == 0:
			value, consumed, err := consumeProtoVarint(cursor)
			if err != nil {
				return singBoxV2RayStat{}, err
			}
			stat.Value = int64(value)
			cursor = cursor[consumed:]
		default:
			consumed, err := skipProtoField(cursor, wireType)
			if err != nil {
				return singBoxV2RayStat{}, err
			}
			cursor = cursor[consumed:]
		}
	}

	return stat, nil
}

func appendProtoStringField(dst []byte, fieldNumber int, value string) []byte {
	dst = appendProtoTag(dst, fieldNumber, 2)
	dst = appendProtoVarint(dst, uint64(len(value)))
	dst = append(dst, value...)
	return dst
}

func appendProtoBoolField(dst []byte, fieldNumber int, value bool) []byte {
	dst = appendProtoTag(dst, fieldNumber, 0)
	if value {
		return append(dst, 1)
	}
	return append(dst, 0)
}

func appendProtoTag(dst []byte, fieldNumber int, wireType byte) []byte {
	tag := uint64(fieldNumber<<3) | uint64(wireType)
	return appendProtoVarint(dst, tag)
}

func appendProtoVarint(dst []byte, value uint64) []byte {
	for value >= 0x80 {
		dst = append(dst, byte(value)|0x80)
		value >>= 7
	}
	return append(dst, byte(value))
}

func consumeProtoTag(data []byte) (int, byte, int, error) {
	rawTag, consumed, err := consumeProtoVarint(data)
	if err != nil {
		return 0, 0, 0, err
	}
	fieldNumber := int(rawTag >> 3)
	if fieldNumber <= 0 {
		return 0, 0, 0, fmt.Errorf("invalid protobuf field number")
	}
	return fieldNumber, byte(rawTag & 0x7), consumed, nil
}

func consumeProtoBytes(data []byte) ([]byte, int, error) {
	length, consumed, err := consumeProtoVarint(data)
	if err != nil {
		return nil, 0, err
	}
	remaining := data[consumed:]
	if length > uint64(len(remaining)) {
		return nil, 0, fmt.Errorf("invalid protobuf bytes length")
	}
	size := int(length)
	total := consumed + size
	return remaining[:size], total, nil
}

func consumeProtoString(data []byte) (string, int, error) {
	value, consumed, err := consumeProtoBytes(data)
	if err != nil {
		return "", 0, err
	}
	return string(value), consumed, nil
}

func consumeProtoVarint(data []byte) (uint64, int, error) {
	var (
		value uint64
		shift uint
	)
	for index, current := range data {
		if index >= 10 {
			return 0, 0, fmt.Errorf("protobuf varint overflow")
		}
		value |= uint64(current&0x7f) << shift
		if current < 0x80 {
			return value, index + 1, nil
		}
		shift += 7
	}
	return 0, 0, fmt.Errorf("truncated protobuf varint")
}

func skipProtoField(data []byte, wireType byte) (int, error) {
	switch wireType {
	case 0:
		_, consumed, err := consumeProtoVarint(data)
		return consumed, err
	case 1:
		if len(data) < 8 {
			return 0, fmt.Errorf("truncated protobuf fixed64 field")
		}
		return 8, nil
	case 2:
		_, consumed, err := consumeProtoBytes(data)
		return consumed, err
	case 5:
		if len(data) < 4 {
			return 0, fmt.Errorf("truncated protobuf fixed32 field")
		}
		return 4, nil
	default:
		return 0, fmt.Errorf("unsupported protobuf wire type")
	}
}
