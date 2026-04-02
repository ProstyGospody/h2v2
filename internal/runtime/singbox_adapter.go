package runtime

import (
	"context"
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

type SingBoxAdapter struct {
	binaryPath  string
	configPath  string
	services    ServiceRestarter
	serviceName string
	artifactHost string
}

func NewSingBoxAdapter(binaryPath string, configPath string, svc ServiceRestarter, serviceName string, artifactHost string) *SingBoxAdapter {
	name := strings.TrimSpace(serviceName)
	if name == "" {
		name = "sing-box"
	}
	bin := strings.TrimSpace(binaryPath)
	if bin == "" {
		bin = "/usr/local/bin/sing-box"
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
	params := parseJSONMap(inbound.ParamsJSON)
	security := normalizeVLESSSecurity(inbound.Security)
	if security == "reality" {
		if err := normalizeRealityParams(security, params); err != nil {
			return UserArtifacts{}, err
		}
	}

	serverHost := a.resolveArtifactHost(inbound.Host)
	transport := normalizeVLESSTransport(inbound.Transport, params)
	serverPort := normalizeInboundPort(inbound.Port)
	flow := strings.TrimSpace(readString(params, "flow"))
	sni := resolveOutboundServerName(params, security, serverHost)

	uri, err := buildSingBoxVLESSURI(credential.Identity, serverHost, serverPort, transport, security, flow, sni, params)
	if err != nil {
		return UserArtifacts{}, err
	}
	singboxNode, err := renderSingBoxVLESSOutbound(user, credential, serverHost, serverPort, transport, security, flow, sni, params)
	if err != nil {
		return UserArtifacts{}, err
	}
	clashNode := renderClashVLESSNode(user, credential, serverHost, serverPort, transport, security, flow, sni, params)

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
	renderedInbounds := make([]map[string]any, 0)
	for _, inbound := range inbounds {
		if inbound.Protocol != repository.ProtocolVLESS || !inbound.Enabled {
			continue
		}
		params := parseJSONMap(inbound.ParamsJSON)
		security := normalizeVLESSSecurity(inbound.Security)
		transport := normalizeVLESSTransport(inbound.Transport, params)

		if security == "reality" {
			if err := normalizeRealityParams(security, params); err != nil {
				return nil, fmt.Errorf("vless inbound %s is invalid: %w", firstNonEmpty(inbound.Name, inbound.ID, "vless"), err)
			}
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
			if flow := strings.TrimSpace(readString(params, "flow")); flow != "" {
				entry["flow"] = flow
			}
			userEntries = append(userEntries, entry)
		}
		if len(userEntries) == 0 {
			continue
		}

		item := map[string]any{
			"type":        "vless",
			"tag":         firstNonEmpty(inbound.Name, inbound.ID, "vless"),
			"listen":      resolveSingBoxListenHost(inbound.Host),
			"listen_port": normalizeInboundPort(inbound.Port),
			"users":       userEntries,
		}

		if transport != "tcp" {
			transportMap := map[string]any{"type": transport}
			switch transport {
			case "ws":
				transportMap["path"] = firstNonEmpty(readString(params, "path"), "/")
				if host := strings.TrimSpace(readString(params, "host")); host != "" {
					transportMap["headers"] = map[string]any{"Host": host}
				}
			case "grpc":
				transportMap["service_name"] = firstNonEmpty(readString(params, "service_name"), readString(params, "serviceName"), "grpc")
			}
			item["transport"] = transportMap
		}

		tls, err := buildVLESSInboundTLS(params, security, inbound.Host, artifactHost)
		if err != nil {
			return nil, fmt.Errorf("vless inbound %s tls is invalid: %w", firstNonEmpty(inbound.Name, inbound.ID, "vless"), err)
		}
		if len(tls) > 0 {
			item["tls"] = tls
		}

		renderedInbounds = append(renderedInbounds, item)
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

func buildVLESSInboundTLS(params map[string]any, security string, inboundHost string, artifactHost string) (map[string]any, error) {
	if security == "none" {
		return nil, nil
	}
	tls := map[string]any{"enabled": true}
	serverName := strings.TrimSpace(readString(params, "server_name"))
	if serverName == "" {
		serverName = strings.TrimSpace(readString(params, "serverName"))
	}
	if serverName == "" {
		serverName = strings.TrimSpace(readString(params, "sni"))
	}
	if serverName != "" {
		tls["server_name"] = serverName
	}
	if alpn := readStringSlice(params, "alpn"); len(alpn) > 0 {
		tls["alpn"] = alpn
	}
	if certPath := firstNonEmpty(readString(params, "certificate_path"), readString(params, "certificatePath"), readString(params, "certPath")); certPath != "" {
		tls["certificate_path"] = certPath
	}
	if keyPath := firstNonEmpty(readString(params, "key_path"), readString(params, "keyPath")); keyPath != "" {
		tls["key_path"] = keyPath
	}
	if security != "reality" {
		return tls, nil
	}

	privateKey := strings.TrimSpace(readString(params, "privateKey"))
	if privateKey == "" {
		return nil, fmt.Errorf("reality private key is missing")
	}
	privateKeyDecoded, err := decodeRealityKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("reality private key is invalid")
	}
	privateKey = encodeRealityKey(privateKeyDecoded)

	fallbackHost := defaultRealityHandshakeServer(params, inboundHost, artifactHost)
	handshakeHost, handshakePort, err := resolveRealityHandshakeTarget(params, fallbackHost)
	if err != nil {
		return nil, err
	}
	reality := map[string]any{
		"enabled":     true,
		"private_key": privateKey,
		"handshake": map[string]any{
			"server":      handshakeHost,
			"server_port": handshakePort,
		},
	}
	if sid := strings.TrimSpace(readString(params, "sid")); sid != "" {
		reality["short_id"] = []string{sid}
	}
	tls["reality"] = reality
	return tls, nil
}

func buildSingBoxVLESSURI(uuidValue string, serverHost string, serverPort int, transport string, security string, flow string, sni string, params map[string]any) (string, error) {
	query := url.Values{}
	query.Set("type", transport)
	query.Set("encryption", "none")
	query.Set("packetEncoding", "xudp")
	if security == "tls" || security == "reality" {
		query.Set("security", security)
	}
	if strings.TrimSpace(sni) != "" {
		query.Set("sni", strings.TrimSpace(sni))
	}
	if flow != "" {
		query.Set("flow", flow)
	}
	if security == "reality" {
		publicKey := strings.TrimSpace(readString(params, "pbk"))
		if publicKey == "" {
			return "", fmt.Errorf("reality public key is missing")
		}
		query.Set("pbk", publicKey)
		if sid := strings.TrimSpace(readString(params, "sid")); sid != "" {
			query.Set("sid", sid)
		}
		query.Set("fp", firstNonEmpty(readString(params, "fp"), "chrome"))
	}
	switch transport {
	case "ws":
		query.Set("path", firstNonEmpty(readString(params, "path"), "/"))
		if host := strings.TrimSpace(readString(params, "host")); host != "" {
			query.Set("host", host)
		}
	case "grpc":
		query.Set("serviceName", firstNonEmpty(readString(params, "service_name"), readString(params, "serviceName"), "grpc"))
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
	transport string,
	security string,
	flow string,
	sni string,
	params map[string]any,
) (map[string]any, error) {
	out := map[string]any{
		"type":        "vless",
		"tag":         "vless-" + firstNonEmpty(user.Name, credential.Identity),
		"server":      strings.TrimSpace(serverHost),
		"server_port": serverPort,
		"uuid":        credential.Identity,
	}
	if flow != "" {
		out["flow"] = flow
	}
	out["packet_encoding"] = "xudp"
	if security == "tls" || security == "reality" {
		tls := map[string]any{"enabled": true}
		if strings.TrimSpace(sni) != "" {
			tls["server_name"] = strings.TrimSpace(sni)
		}
		if readBool(params, "insecure", false) {
			tls["insecure"] = true
		}
		if alpn := readStringSlice(params, "alpn"); len(alpn) > 0 {
			tls["alpn"] = alpn
		}
		if security == "reality" {
			publicKey := strings.TrimSpace(readString(params, "pbk"))
			if publicKey == "" {
				return nil, fmt.Errorf("reality public key is missing")
			}
			reality := map[string]any{
				"enabled":    true,
				"public_key": publicKey,
			}
			if sid := strings.TrimSpace(readString(params, "sid")); sid != "" {
				reality["short_id"] = sid
			}
			tls["reality"] = reality
			tls["utls"] = map[string]any{
				"enabled":     true,
				"fingerprint": firstNonEmpty(readString(params, "fp"), "chrome"),
			}
		}
		out["tls"] = tls
	}
	if transport != "tcp" {
		transportMap := map[string]any{"type": transport}
		switch transport {
		case "ws":
			transportMap["path"] = firstNonEmpty(readString(params, "path"), "/")
			if host := strings.TrimSpace(readString(params, "host")); host != "" {
				transportMap["headers"] = map[string]any{"Host": host}
			}
		case "grpc":
			transportMap["service_name"] = firstNonEmpty(readString(params, "service_name"), readString(params, "serviceName"), "grpc")
		}
		out["transport"] = transportMap
	}
	return out, nil
}

func renderClashVLESSNode(
	user repository.UserWithCredentials,
	credential repository.Credential,
	serverHost string,
	serverPort int,
	transport string,
	security string,
	flow string,
	sni string,
	params map[string]any,
) string {
	lines := []string{
		"- name: " + firstNonEmpty(user.Name, credential.Identity),
		"  type: vless",
		"  server: " + strings.TrimSpace(serverHost),
		"  port: " + strconv.Itoa(serverPort),
		"  uuid: " + credential.Identity,
		"  network: " + transport,
	}
	if flow != "" {
		lines = append(lines, "  flow: "+flow)
	}
	if security == "none" {
		lines = append(lines, "  tls: false")
	} else {
		lines = append(lines, "  tls: true")
		if strings.TrimSpace(sni) != "" {
			lines = append(lines, "  servername: "+strings.TrimSpace(sni))
		}
		if security == "reality" {
			lines = append(lines, "  reality-opts:")
			if publicKey := strings.TrimSpace(readString(params, "pbk")); publicKey != "" {
				lines = append(lines, "    public-key: "+publicKey)
			}
			if sid := strings.TrimSpace(readString(params, "sid")); sid != "" {
				lines = append(lines, "    short-id: "+sid)
			}
		}
	}
	switch transport {
	case "ws":
		lines = append(lines, "  ws-opts:")
		lines = append(lines, "    path: "+firstNonEmpty(readString(params, "path"), "/"))
		if host := strings.TrimSpace(readString(params, "host")); host != "" {
			lines = append(lines, "    headers:")
			lines = append(lines, "      Host: "+host)
		}
	case "grpc":
		lines = append(lines, "  grpc-opts:")
		lines = append(lines, "    grpc-service-name: "+firstNonEmpty(readString(params, "service_name"), readString(params, "serviceName"), "grpc"))
	}
	lines = append(lines, "  packet-encoding: xudp")
	return strings.Join(lines, "\n")
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

func normalizeVLESSTransport(raw string, params map[string]any) string {
	transport := strings.ToLower(strings.TrimSpace(raw))
	if transport == "" {
		transport = strings.ToLower(strings.TrimSpace(readString(params, "network")))
	}
	switch transport {
	case "ws", "grpc", "tcp":
		return transport
	default:
		return "tcp"
	}
}

func normalizeInboundPort(port int) int {
	if port <= 0 {
		return 443
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

func resolveOutboundServerName(params map[string]any, security string, serverHost string) string {
	serverName := firstNonEmpty(
		readString(params, "sni"),
		readString(params, "server_name"),
		readString(params, "serverName"),
	)
	if strings.TrimSpace(serverName) != "" {
		return strings.TrimSpace(serverName)
	}
	if security != "reality" {
		return ""
	}
	_ = serverHost
	return "www.cloudflare.com"
}

func defaultRealityHandshakeServer(params map[string]any, inboundHost string, artifactHost string) string {
	candidates := []string{
		readString(params, "handshake_server"),
		readString(params, "handshakeServer"),
		readString(params, "sni"),
		readString(params, "server_name"),
		readString(params, "serverName"),
	}
	for _, candidate := range candidates {
		host := normalizePublicEndpointHost(candidate)
		if host != "" {
			return host
		}
	}
	_ = inboundHost
	_ = artifactHost
	return "www.cloudflare.com"
}

func resolveRealityHandshakeTarget(params map[string]any, fallbackHost string) (string, int, error) {
	host, port := parseHostPortLoose(readString(params, "dest"), 443)
	if strings.TrimSpace(host) == "" {
		host = strings.TrimSpace(readString(params, "handshake_server"))
	}
	if strings.TrimSpace(host) == "" {
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
	if strings.TrimSpace(host) == "" {
		host = strings.TrimSpace(fallbackHost)
	}
	if strings.TrimSpace(host) == "" {
		host = "www.cloudflare.com"
	}
	if port <= 0 {
		port = 443
	}
	return host, port, nil
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
