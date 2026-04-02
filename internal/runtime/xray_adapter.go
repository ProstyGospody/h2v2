package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"

	"h2v2/internal/fsutil"
	"h2v2/internal/repository"
)

type XrayAdapter struct {
	binaryPath   string
	configPath   string
	runtimeURL   string
	runtimeToken string
	services     ServiceRestarter
	serviceName  string
	httpClient   *http.Client
	artifactHost string
}

func NewXrayAdapter(binaryPath string, configPath string, runtimeURL string, runtimeToken string, svc ServiceRestarter, serviceName string, artifactHost string) *XrayAdapter {
	name := strings.TrimSpace(serviceName)
	if name == "" {
		name = "xray"
	}
	bin := strings.TrimSpace(binaryPath)
	if bin == "" {
		bin = "/usr/local/bin/xray"
	}
	return &XrayAdapter{
		binaryPath:   bin,
		configPath:   strings.TrimSpace(configPath),
		runtimeURL:   strings.TrimRight(strings.TrimSpace(runtimeURL), "/"),
		runtimeToken: strings.TrimSpace(runtimeToken),
		services:     svc,
		serviceName:  name,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
		artifactHost: normalizePublicEndpointHost(artifactHost),
	}
}

func (a *XrayAdapter) Protocol() repository.Protocol {
	return repository.ProtocolVLESS
}

func (a *XrayAdapter) SyncConfig(ctx context.Context, inbounds []repository.Inbound, users []repository.UserWithCredentials) error {
	config, err := buildXrayConfig(inbounds, users)
	if err != nil {
		return err
	}
	if err := a.applyConfig(config); err != nil {
		return err
	}
	if err := a.hotReload(ctx); err != nil {
		return a.restart(ctx)
	}
	return nil
}

func (a *XrayAdapter) AddUser(ctx context.Context, user repository.UserWithCredentials, _ []repository.Inbound) error {
	if err := a.hotMutateUser(ctx, "add", user); err != nil {
		return a.restart(ctx)
	}
	return nil
}

func (a *XrayAdapter) UpdateUser(ctx context.Context, user repository.UserWithCredentials, _ []repository.Inbound) error {
	if err := a.hotMutateUser(ctx, "update", user); err != nil {
		return a.restart(ctx)
	}
	return nil
}

func (a *XrayAdapter) RemoveUser(ctx context.Context, user repository.UserWithCredentials, _ []repository.Inbound) error {
	if err := a.hotMutateUser(ctx, "remove", user); err != nil {
		return a.restart(ctx)
	}
	return nil
}

func (a *XrayAdapter) SetUsersStateBatch(ctx context.Context, _ []repository.UserWithCredentials, _ bool, inbounds []repository.Inbound) error {
	if err := a.hotReload(ctx); err != nil {
		return a.SyncConfig(ctx, inbounds, nil)
	}
	return nil
}

func (a *XrayAdapter) KickUser(ctx context.Context, user repository.UserWithCredentials) error {
	credential, ok := userCredential(user, repository.ProtocolVLESS)
	if !ok {
		return fmt.Errorf("vless credential is missing")
	}
	payload := map[string]any{"uuid": credential.Identity}
	return a.runtimePOST(ctx, "/kick", payload)
}

func (a *XrayAdapter) CollectTraffic(ctx context.Context, users []repository.UserWithCredentials) ([]repository.TrafficCounter, error) {
	response := struct {
		Users map[string]struct {
			Tx int64 `json:"tx"`
			Rx int64 `json:"rx"`
		} `json:"users"`
	}{}
	if err := a.runtimeGET(ctx, "/traffic", &response); err != nil {
		return nil, nil
	}
	counters := make([]repository.TrafficCounter, 0, len(users))
	for _, user := range users {
		credential, ok := userCredential(user, repository.ProtocolVLESS)
		if !ok {
			continue
		}
		stat, exists := response.Users[credential.Identity]
		if !exists {
			continue
		}
		counters = append(counters, repository.TrafficCounter{
			UserID:   user.ID,
			Protocol: repository.ProtocolVLESS,
			TxBytes:  stat.Tx,
			RxBytes:  stat.Rx,
		})
	}
	return counters, nil
}

func (a *XrayAdapter) CollectOnline(ctx context.Context, users []repository.UserWithCredentials) (map[string]int, error) {
	response := struct {
		Users map[string]int `json:"users"`
	}{}
	if err := a.runtimeGET(ctx, "/online", &response); err != nil {
		return map[string]int{}, nil
	}
	result := make(map[string]int, len(users))
	for _, user := range users {
		credential, ok := userCredential(user, repository.ProtocolVLESS)
		if !ok {
			continue
		}
		result[user.ID] = response.Users[credential.Identity]
	}
	return result, nil
}

func (a *XrayAdapter) BuildArtifacts(_ context.Context, user repository.UserWithCredentials, inbounds []repository.Inbound, subscriptionURL string) (UserArtifacts, error) {
	credential, ok := userCredential(user, repository.ProtocolVLESS)
	if !ok {
		return UserArtifacts{}, fmt.Errorf("vless credential is missing")
	}
	inbound, ok := selectVLESSInbound(inbounds)
	if !ok {
		return UserArtifacts{}, fmt.Errorf("vless inbound is missing")
	}
	params := parseJSONMap(inbound.ParamsJSON)
	if err := normalizeRealityParams(strings.ToLower(strings.TrimSpace(inbound.Security)), params); err != nil {
		return UserArtifacts{}, err
	}
	serverHost := a.resolveArtifactHost(inbound.Host)
	uri := buildVLESSURI(user, credential, inbound, serverHost, params)
	clashNode := renderClashVLESS(user, credential, inbound, serverHost, params)
	singboxNode := renderSingboxVLESS(user, credential, inbound, serverHost, params)
	return UserArtifacts{
		Protocol:     repository.ProtocolVLESS,
		AccessURI:    uri,
		Config:       renderVLESSClientConfig(uri, clashNode, singboxNode),
		Subscription: strings.TrimSpace(subscriptionURL),
		ClashNode:    clashNode,
		SingBoxNode:  singboxNode,
	}, nil
}

func (a *XrayAdapter) resolveArtifactHost(raw string) string {
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

func (a *XrayAdapter) applyConfig(config map[string]any) error {
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
	tempFile := filepath.Join(filepath.Dir(a.configPath), ".xray-config-test.json")
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

func (a *XrayAdapter) validateConfig(path string) error {
	if strings.TrimSpace(a.binaryPath) == "" || strings.TrimSpace(path) == "" {
		return nil
	}
	ctxRun, cancelRun := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancelRun()
	outRun, errRun := exec.CommandContext(ctxRun, a.binaryPath, "run", "-test", "-config", path).CombinedOutput()
	if errRun == nil {
		return nil
	}
	ctxFallback, cancelFallback := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancelFallback()
	outFallback, errFallback := exec.CommandContext(ctxFallback, a.binaryPath, "-test", "-config", path).CombinedOutput()
	if errFallback == nil {
		return nil
	}
	details := strings.TrimSpace(string(outFallback))
	if details == "" {
		details = strings.TrimSpace(string(outRun))
	}
	if details == "" {
		details = "xray test command returned an empty error output"
	}
	return fmt.Errorf("xray config validation failed: %s", details)
}

func (a *XrayAdapter) hotReload(ctx context.Context) error {
	if strings.TrimSpace(a.runtimeURL) == "" {
		return fmt.Errorf("xray runtime url is not configured")
	}
	if err := a.runtimePOST(ctx, "/reload", map[string]any{"reload": true}); err != nil {
		return err
	}
	return nil
}

func (a *XrayAdapter) hotMutateUser(ctx context.Context, action string, user repository.UserWithCredentials) error {
	if strings.TrimSpace(a.runtimeURL) == "" {
		return fmt.Errorf("xray runtime url is not configured")
	}
	credential, ok := userCredential(user, repository.ProtocolVLESS)
	if !ok {
		return fmt.Errorf("vless credential is missing")
	}
	payload := map[string]any{
		"action": action,
		"user": map[string]any{
			"id":      user.ID,
			"name":    user.Name,
			"enabled": user.Enabled,
			"uuid":    credential.Identity,
		},
	}
	return a.runtimePOST(ctx, "/users", payload)
}

func (a *XrayAdapter) restart(ctx context.Context) error {
	if a.services == nil {
		return nil
	}
	return a.services.Restart(ctx, a.serviceName)
}

func (a *XrayAdapter) runtimeGET(ctx context.Context, path string, out any) error {
	if strings.TrimSpace(a.runtimeURL) == "" {
		return fmt.Errorf("xray runtime url is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.runtimeURL+path, nil)
	if err != nil {
		return err
	}
	if a.runtimeToken != "" {
		request.Header.Set("Authorization", "Bearer "+a.runtimeToken)
	}
	response, err := a.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("xray runtime %s failed: status=%d body=%s", path, response.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(response.Body).Decode(out)
}

func (a *XrayAdapter) runtimePOST(ctx context.Context, path string, payload any) error {
	if strings.TrimSpace(a.runtimeURL) == "" {
		return fmt.Errorf("xray runtime url is not configured")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.runtimeURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if a.runtimeToken != "" {
		request.Header.Set("Authorization", "Bearer "+a.runtimeToken)
	}
	response, err := a.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("xray runtime %s failed: status=%d body=%s", path, response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func buildXrayConfig(inbounds []repository.Inbound, users []repository.UserWithCredentials) (map[string]any, error) {
	enabledInbounds := make([]map[string]any, 0)
	for _, inbound := range inbounds {
		if inbound.Protocol != repository.ProtocolVLESS || !inbound.Enabled {
			continue
		}

		publicHost := normalizePublicEndpointHost(inbound.Host)
		listenHost := resolveXrayListenHost(inbound.Host)
		listenPort := inbound.Port
		if listenPort <= 0 {
			listenPort = 443
		}

		params := parseJSONMap(inbound.ParamsJSON)
		clients := make([]map[string]any, 0)
		for _, user := range users {
			if !user.Enabled {
				continue
			}
			if user.ExpireAt != nil && !user.ExpireAt.After(time.Now().UTC()) {
				continue
			}
			if user.TrafficLimitBytes > 0 && (user.TrafficUsedTxBytes+user.TrafficUsedRxBytes) >= user.TrafficLimitBytes {
				continue
			}
			credential, ok := userCredential(user, repository.ProtocolVLESS)
			if !ok {
				continue
			}
			client := map[string]any{"id": credential.Identity}
			if flow := strings.TrimSpace(readString(params, "flow")); flow != "" {
				client["flow"] = flow
			}
			clients = append(clients, client)
		}

		network := firstNonEmpty(inbound.Transport, "tcp")
		if network != "tcp" && network != "ws" && network != "grpc" {
			network = "tcp"
		}
		security := firstNonEmpty(inbound.Security, "none")
		stream := map[string]any{
			"network":  network,
			"security": security,
		}
		switch network {
		case "ws":
			stream["wsSettings"] = map[string]any{"path": firstNonEmpty(readString(params, "path"), "/")}
		case "grpc":
			stream["grpcSettings"] = map[string]any{"serviceName": firstNonEmpty(readString(params, "serviceName"), "grpc")}
		}
		if security == "reality" {
			serverNames := readStringSlice(params, "sni")
			if len(serverNames) == 0 {
				if fallback := readString(params, "serverName"); fallback != "" {
					serverNames = []string{fallback}
				}
			}
			if len(serverNames) == 0 && publicHost != "" {
				serverNames = []string{publicHost}
			}
			privateKey := readString(params, "privateKey")
			if privateKey == "" || len(serverNames) == 0 {
				continue
			}
			stream["realitySettings"] = map[string]any{
				"show":        false,
				"dest":        firstNonEmpty(readString(params, "dest"), firstNonEmpty(publicHost, "www.cloudflare.com")+":443"),
				"xver":        readInt(params, "xver", 0),
				"serverNames": serverNames,
				"privateKey":  privateKey,
				"shortIds":    readStringSlice(params, "sid"),
			}
		}

		enabledInbounds = append(enabledInbounds, map[string]any{
			"tag":      firstNonEmpty(inbound.Name, inbound.ID, "vless"),
			"listen":   listenHost,
			"port":     listenPort,
			"protocol": "vless",
			"settings": map[string]any{
				"clients":    clients,
				"decryption": "none",
			},
			"streamSettings": stream,
		})
	}

	if len(enabledInbounds) == 0 {
		enabledInbounds = append(enabledInbounds, map[string]any{
			"tag":      "vless-default",
			"listen":   "127.0.0.1",
			"port":     24443,
			"protocol": "vless",
			"settings": map[string]any{"clients": []map[string]any{}, "decryption": "none"},
			"streamSettings": map[string]any{"network": "tcp", "security": "none"},
		})
	}

	return map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": enabledInbounds,
		"outbounds": []map[string]any{
			{"tag": "direct", "protocol": "freedom"},
			{"tag": "blocked", "protocol": "blackhole"},
		},
	}, nil
}

func buildVLESSURI(user repository.UserWithCredentials, credential repository.Credential, inbound repository.Inbound, serverHost string, params map[string]any) string {
	query := url.Values{}
	query.Set("type", firstNonEmpty(inbound.Transport, "tcp"))
	query.Set("encryption", "none")
	if inbound.Security != "" && inbound.Security != "none" {
		query.Set("security", inbound.Security)
	}
	if sni := firstNonEmpty(readString(params, "sni"), readString(params, "serverName")); sni != "" {
		query.Set("sni", sni)
	}
	if fp := readString(params, "fp"); fp != "" {
		query.Set("fp", fp)
	}
	if pbk := readString(params, "pbk"); pbk != "" {
		query.Set("pbk", pbk)
	}
	if sid := readString(params, "sid"); sid != "" {
		query.Set("sid", sid)
	}
	if flow := readString(params, "flow"); flow != "" {
		query.Set("flow", flow)
	}
	if inbound.Transport == "ws" {
		query.Set("path", firstNonEmpty(readString(params, "path"), "/"))
	}
	if inbound.Transport == "grpc" {
		query.Set("serviceName", firstNonEmpty(readString(params, "serviceName"), "grpc"))
	}

	host := strings.TrimSpace(serverHost)
	if host == "" {
		host = "127.0.0.1"
	}
	endpoint := host + ":" + strconv.Itoa(inbound.Port)
	tag := url.QueryEscape(firstNonEmpty(user.Name, credential.Identity))
	return "vless://" + credential.Identity + "@" + endpoint + "?" + query.Encode() + "#" + tag
}

func renderClashVLESS(user repository.UserWithCredentials, credential repository.Credential, inbound repository.Inbound, serverHost string, params map[string]any) string {
	name := firstNonEmpty(user.Name, credential.Identity)
	parts := []string{
		"- name: " + name,
		"  type: vless",
		"  server: " + firstNonEmpty(strings.TrimSpace(serverHost), "127.0.0.1"),
		"  port: " + strconv.Itoa(inbound.Port),
		"  uuid: " + credential.Identity,
		"  network: " + firstNonEmpty(inbound.Transport, "tcp"),
		"  tls: true",
	}
	if sni := firstNonEmpty(readString(params, "sni"), readString(params, "serverName")); sni != "" {
		parts = append(parts, "  servername: "+sni)
	}
	if inbound.Security == "reality" {
		parts = append(parts, "  reality-opts:")
		if pbk := readString(params, "pbk"); pbk != "" {
			parts = append(parts, "    public-key: "+pbk)
		}
		if sid := readString(params, "sid"); sid != "" {
			parts = append(parts, "    short-id: "+sid)
		}
	}
	return strings.Join(parts, "\n")
}

func renderSingboxVLESS(user repository.UserWithCredentials, credential repository.Credential, inbound repository.Inbound, serverHost string, params map[string]any) map[string]any {
	result := map[string]any{
		"type":       "vless",
		"tag":        "vless-" + firstNonEmpty(user.Name, credential.Identity),
		"server":     firstNonEmpty(strings.TrimSpace(serverHost), "127.0.0.1"),
		"server_port": inbound.Port,
		"uuid":       credential.Identity,
		"flow":       readString(params, "flow"),
		"packet_encoding": "xudp",
	}
	tls := map[string]any{"enabled": true}
	if sni := firstNonEmpty(readString(params, "sni"), readString(params, "serverName")); sni != "" {
		tls["server_name"] = sni
	}
	if inbound.Security == "reality" {
		reality := map[string]any{
			"enabled":    true,
			"public_key": readString(params, "pbk"),
		}
		if sid := readString(params, "sid"); sid != "" {
			reality["short_id"] = sid
		}
		tls["reality"] = reality
		tls["utls"] = map[string]any{
			"enabled":     true,
			"fingerprint": firstNonEmpty(readString(params, "fp"), "chrome"),
		}
	}
	result["tls"] = tls

	transport := map[string]any{"type": firstNonEmpty(inbound.Transport, "tcp")}
	if inbound.Transport == "ws" {
		transport["path"] = firstNonEmpty(readString(params, "path"), "/")
	}
	if inbound.Transport == "grpc" {
		transport["service_name"] = firstNonEmpty(readString(params, "serviceName"), "grpc")
	}
	result["transport"] = transport
	return result
}

func renderVLESSClientConfig(uri string, clashNode string, singboxNode map[string]any) string {
	parts := []string{"uri:", strings.TrimSpace(uri)}
	if strings.TrimSpace(clashNode) != "" {
		parts = append(parts, "", "clash:", strings.TrimSpace(clashNode))
	}
	if len(singboxNode) > 0 {
		payload, err := json.MarshalIndent(singboxNode, "", "  ")
		if err == nil {
			parts = append(parts, "", "singbox:", string(payload))
		}
	}
	return strings.Join(parts, "\n")
}

func selectVLESSInbound(inbounds []repository.Inbound) (repository.Inbound, bool) {
	for _, inbound := range inbounds {
		if inbound.Protocol == repository.ProtocolVLESS && inbound.Enabled {
			return inbound, true
		}
	}
	for _, inbound := range inbounds {
		if inbound.Protocol == repository.ProtocolVLESS {
			return inbound, true
		}
	}
	return repository.Inbound{}, false
}

func parseJSONMap(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func normalizeHostOnly(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		trimmed = strings.TrimPrefix(strings.TrimSuffix(trimmed, "]"), "[")
	}
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		trimmed = host
	}
	trimmed = strings.TrimSpace(trimmed)
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		trimmed = strings.TrimPrefix(strings.TrimSuffix(trimmed, "]"), "[")
	}
	return trimmed
}

func normalizePublicEndpointHost(raw string) string {
	host := strings.ToLower(strings.TrimSpace(normalizeHostOnly(raw)))
	switch host {
	case "", "localhost", "127.0.0.1", "::1", "0.0.0.0", "::":
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() {
			return ""
		}
	}
	return host
}

func resolveXrayListenHost(raw string) string {
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

func decodeRealityKey(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("reality key is empty")
	}
	encodings := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	for _, enc := range encodings {
		decoded, err := enc.DecodeString(trimmed)
		if err != nil {
			continue
		}
		if len(decoded) == 32 {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("reality key must decode to 32 bytes")
}

func encodeRealityKey(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func deriveRealityPublicKey(privateKey string) (string, error) {
	decoded, err := decodeRealityKey(privateKey)
	if err != nil {
		return "", err
	}
	derived, err := curve25519.X25519(decoded, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return encodeRealityKey(derived), nil
}

func normalizeRealityShortID(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}
	if len(value) > 32 || len(value)%2 != 0 {
		return "", fmt.Errorf("reality short id is invalid")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", fmt.Errorf("reality short id is invalid")
	}
	return value, nil
}

func normalizeRealityParams(security string, params map[string]any) error {
	if security != "reality" {
		return nil
	}
	if params == nil {
		return fmt.Errorf("reality params are missing")
	}
	privateKey := strings.TrimSpace(readString(params, "privateKey"))
	if privateKey != "" {
		decodedPrivate, err := decodeRealityKey(privateKey)
		if err != nil {
			return fmt.Errorf("reality private key is invalid")
		}
		privateKey = encodeRealityKey(decodedPrivate)
		params["privateKey"] = privateKey
	}
	publicKey := strings.TrimSpace(readString(params, "pbk"))
	if publicKey == "" {
		publicKey = strings.TrimSpace(readString(params, "publicKey"))
	}
	if publicKey != "" {
		decodedPublic, err := decodeRealityKey(publicKey)
		if err != nil {
			return fmt.Errorf("reality public key is invalid")
		}
		publicKey = encodeRealityKey(decodedPublic)
	}
	if publicKey == "" && privateKey != "" {
		derived, err := deriveRealityPublicKey(privateKey)
		if err != nil {
			return fmt.Errorf("reality public key is invalid")
		}
		publicKey = derived
	}
	if publicKey == "" {
		return fmt.Errorf("reality public key is missing")
	}
	params["pbk"] = publicKey
	delete(params, "publicKey")

	sid, err := normalizeRealityShortID(readString(params, "sid"))
	if err != nil {
		return err
	}
	if sid == "" {
		delete(params, "sid")
	} else {
		params["sid"] = sid
	}
	return nil
}

func readString(source map[string]any, key string) string {
	if source == nil {
		return ""
	}
	value, ok := source[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func readInt(source map[string]any, key string, fallback int) int {
	if source == nil {
		return fallback
	}
	value, ok := source[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func readStringSlice(source map[string]any, key string) []string {
	if source == nil {
		return nil
	}
	value, ok := source[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		parts := strings.Split(typed, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	default:
		return nil
	}
}
