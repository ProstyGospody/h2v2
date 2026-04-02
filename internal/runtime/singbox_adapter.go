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
	"strconv"
	"strings"
	"time"

	"h2v2/internal/fsutil"
	"h2v2/internal/repository"
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
)

type SingBoxAdapter struct {
	binaryPath   string
	configPath   string
	services     ServiceRestarter
	serviceName  string
	artifactHost string
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
		binaryPath:   bin,
		configPath:   strings.TrimSpace(configPath),
		services:     svc,
		serviceName:  name,
		artifactHost: normalizePublicEndpointHost(artifactHost),
	}
}

func (a *SingBoxAdapter) Protocol() repository.Protocol {
	return repository.ProtocolVLESS
}

func (a *SingBoxAdapter) SyncConfig(ctx context.Context, inbounds []repository.Inbound, users []repository.UserWithCredentials) error {
	config, err := buildSingBoxVLESSConfig(inbounds, users, a.artifactHost)
	if err != nil {
		return err
	}
	if err := a.applyConfig(config); err != nil {
		return err
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

func (a *SingBoxAdapter) CollectTraffic(context.Context, []repository.UserWithCredentials) ([]repository.TrafficCounter, error) {
	return nil, nil
}

func (a *SingBoxAdapter) CollectOnline(context.Context, []repository.UserWithCredentials) (map[string]int, error) {
	return map[string]int{}, nil
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
	serverName := resolveVLESSAccessServerName(runtimeConfig)

	uri, err := buildSingBoxVLESSURI(credential.Identity, serverHost, runtimeConfig.Port, runtimeConfig, serverName)
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
	host = normalizeHostOnly(raw)
	if host != "" {
		return host
	}
	return "127.0.0.1"
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

func buildSingBoxVLESSConfig(inbounds []repository.Inbound, users []repository.UserWithCredentials, artifactHost string) (map[string]any, error) {
	now := time.Now().UTC()
	renderedInbounds := make([]map[string]any, 0, len(inbounds))

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
			entry := map[string]any{
				"name": firstNonEmpty(user.Name, credential.Identity),
				"uuid": credential.Identity,
			}
			if runtimeConfig.Flow != "" {
				entry["flow"] = runtimeConfig.Flow
			}
			userEntries = append(userEntries, entry)
		}
		if len(userEntries) == 0 {
			continue
		}

		inboundPayload, err := renderSingBoxVLESSInbound(runtimeConfig, userEntries)
		if err != nil {
			return nil, fmt.Errorf("vless inbound %s is invalid: %w", runtimeConfig.Tag, err)
		}
		renderedInbounds = append(renderedInbounds, inboundPayload)
	}

	return map[string]any{
		"log": map[string]any{"level": "warn"},
		"inbounds": renderedInbounds,
		"outbounds": []map[string]any{
			{"type": "direct", "tag": "direct"},
			{"type": "block", "tag": "block"},
		},
		"route": map[string]any{"final": "direct"},
	}, nil
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

func buildSingBoxVLESSURI(uuidValue string, serverHost string, serverPort int, runtimeConfig singBoxVLESSRuntimeConfig, serverName string) (string, error) {
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

	uri := &url.URL{
		Scheme:   "vless",
		User:     url.User(uuidValue),
		Host:     fmt.Sprintf("%s:%d", strings.TrimSpace(serverHost), serverPort),
		RawQuery: query.Encode(),
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
