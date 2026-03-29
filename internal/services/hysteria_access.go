package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"h2v2/internal/config"
	hysteriadomain "h2v2/internal/domain/hysteria"
	"h2v2/internal/repository"
)

type HysteriaUserArtifacts struct {
	URI             string                           `json:"uri"`
	URIHy2          string                           `json:"uri_hy2"`
	SubscriptionURL string                           `json:"subscription_url"`
	ClientYAML      string                           `json:"client_config"`
	ClientParams    Hy2ClientParams                  `json:"client_params"`
	ServerDefaults  Hy2ClientParams                  `json:"server_defaults"`
	ClientOverrides *hysteriadomain.ClientOverrides `json:"client_overrides,omitempty"`
	ServerOptions   HysteriaServerClientOptions      `json:"server_options"`
	SingBoxOutbound map[string]any                   `json:"singbox_outbound"`
}

type HysteriaClientDefaults struct {
	ClientParams  Hy2ClientParams             `json:"client_params"`
	ServerOptions HysteriaServerClientOptions `json:"server_options"`
}

type HysteriaServerClientOptions struct {
	TLSEnabled            bool   `json:"tls_enabled"`
	TLSMode               string `json:"tls_mode"`
	ObfsType              string `json:"obfs_type,omitempty"`
	MasqueradeType        string `json:"masquerade_type,omitempty"`
	BandwidthUp           string `json:"bandwidth_up,omitempty"`
	BandwidthDown         string `json:"bandwidth_down,omitempty"`
	IgnoreClientBandwidth bool   `json:"ignore_client_bandwidth"`
}

type HysteriaSyncResult struct {
	RawYAML    string
	BackupPath string
	Validation Hy2ConfigValidation
	Changed    bool
}

type HysteriaAccessManager struct {
	repo          *repository.Repository
	cfg           config.Config
	configManager *HysteriaConfigManager
}

var ErrInvalidSubscriptionToken = errors.New("invalid subscription token")

func NewHysteriaAccessManager(repo *repository.Repository, cfg config.Config, configManager *HysteriaConfigManager) *HysteriaAccessManager {
	return &HysteriaAccessManager{repo: repo, cfg: cfg, configManager: configManager}
}

func (m *HysteriaAccessManager) Sync(ctx context.Context) (HysteriaSyncResult, error) {
	if m.configManager == nil {
		return HysteriaSyncResult{}, fmt.Errorf("hysteria config manager is not configured")
	}
	current, err := m.configManager.Read()
	if err != nil {
		return HysteriaSyncResult{}, err
	}
	next, err := m.InjectManagedAuth(ctx, current)
	if err != nil {
		return HysteriaSyncResult{}, err
	}
	validation := m.configManager.Validate(next)
	if !validation.Valid {
		return HysteriaSyncResult{RawYAML: next, Validation: validation}, fmt.Errorf("managed hysteria config is invalid")
	}
	if strings.TrimSpace(current) == strings.TrimSpace(next) {
		return HysteriaSyncResult{RawYAML: next, Validation: validation, Changed: false}, nil
	}
	backupPath, err := m.configManager.Save(next)
	if err != nil {
		return HysteriaSyncResult{}, err
	}
	return HysteriaSyncResult{RawYAML: next, BackupPath: backupPath, Validation: validation, Changed: true}, nil
}

func (m *HysteriaAccessManager) InjectManagedAuth(ctx context.Context, content string) (string, error) {
	if m.configManager == nil {
		return "", fmt.Errorf("hysteria config manager is not configured")
	}
	root, err := parseYAMLMap(content)
	if err != nil {
		return "", err
	}
	auth, err := m.managedAuth(ctx)
	if err != nil {
		return "", err
	}
	root["auth"] = buildServerAuthMap(auth)
	return marshalYAMLMap(root)
}

func (m *HysteriaAccessManager) ClientDefaults(ctx context.Context) (HysteriaClientDefaults, error) {
	content, err := m.managedContent(ctx)
	if err != nil {
		return HysteriaClientDefaults{}, err
	}
	settings := m.configManager.ExtractSettings(content, m.preferredPublicHost(), m.cfg.Hy2Port)
	return HysteriaClientDefaults{
		ClientParams:  m.currentClientParamsFromContent(content),
		ServerOptions: m.serverClientOptionsFromSettings(settings),
	}, nil
}

func (m *HysteriaAccessManager) managedAuth(ctx context.Context) (Hy2ServerAuth, error) {
	users, err := m.repo.ListEnabledHysteriaUsers(ctx)
	if err != nil {
		return Hy2ServerAuth{}, err
	}
	userPass := make(map[string]string, len(users))
	for _, user := range users {
		userPass[user.Username] = user.Password
	}
	if len(userPass) == 0 {
		bootstrapPassword := strings.TrimSpace(m.cfg.InternalAuthToken)
		if bootstrapPassword == "" {
			bootstrapPassword = "h2v2-bootstrap"
		}
		userPass["__bootstrap__"] = bootstrapPassword
	}
	return Hy2ServerAuth{Type: "userpass", UserPass: userPass}, nil
}

func (m *HysteriaAccessManager) ValidateClientExportDraft(ctx context.Context, username string, password string, overrides *hysteriadomain.ClientOverrides) (Hy2ClientValidation, error) {
	content, err := m.managedContent(ctx)
	if err != nil {
		return Hy2ClientValidation{}, err
	}
	normalizedUsername, err := hysteriadomain.NormalizeUsername(username)
	if err != nil {
		return Hy2ClientValidation{}, err
	}
	normalizedPassword, err := hysteriadomain.NormalizePassword(password)
	if err != nil {
		return Hy2ClientValidation{}, err
	}
	draft := repository.HysteriaUserView{User: repository.HysteriaUser{
		Username:        normalizedUsername,
		Password:        normalizedPassword,
		ClientOverrides: hysteriadomain.NormalizeClientOverrides(overrides),
	}}
	profile := m.defaultClientProfileFromContent(content, draft)
	profile = applyClientOverrides(profile, draft.ClientOverrides)
	return m.configManager.ValidateClientProfile(profile), nil
}
func (m *HysteriaAccessManager) BuildUserArtifacts(user repository.HysteriaUserView) (HysteriaUserArtifacts, Hy2ClientValidation, error) {
	content, err := m.managedContent(context.Background())
	if err != nil {
		return HysteriaUserArtifacts{}, Hy2ClientValidation{}, err
	}
	settings := m.configManager.ExtractSettings(content, m.preferredPublicHost(), m.cfg.Hy2Port)
	baseProfile := m.baseClientProfileFromContent(content)
	serverDefaults := m.clientParamsFromProfile(baseProfile)
	profile := m.defaultClientProfileFromContent(content, user)
	effectiveProfile := applyClientOverrides(profile, user.ClientOverrides)

	artifacts, validation := m.configManager.GenerateClientArtifacts(effectiveProfile)
	if !validation.Valid {
		return HysteriaUserArtifacts{}, validation, fmt.Errorf("invalid hysteria client profile")
	}
	effectiveParams := m.clientParamsFromProfile(effectiveProfile)
	return HysteriaUserArtifacts{
		URI:             artifacts.URI,
		URIHy2:          artifacts.URIHy2,
		SubscriptionURL: m.BuildSubscriptionURL(user),
		ClientYAML:      artifacts.ClientYAML,
		ClientParams:    effectiveParams,
		ServerDefaults:  serverDefaults,
		ClientOverrides: hysteriadomain.NormalizeClientOverrides(user.ClientOverrides),
		ServerOptions:   m.serverClientOptionsFromSettings(settings),
		SingBoxOutbound: m.buildSingBoxOutbound(user, effectiveProfile, effectiveParams),
	}, validation, nil
}

func (m *HysteriaAccessManager) BuildSubscriptionURL(user repository.HysteriaUserView) string {
	base := strings.TrimRight(strings.TrimSpace(m.cfg.SubscriptionPublicURL), "/")
	if base == "" {
		return ""
	}
	token, err := buildSubscriptionToken(m.cfg.InternalAuthToken, user.ID, user.UpdatedAt)
	if err != nil {
		return ""
	}
	return base + "/api/hysteria/subscription/" + url.PathEscape(token)
}

func (m *HysteriaAccessManager) ResolveSubscriptionUser(ctx context.Context, token string) (repository.HysteriaUserView, error) {
	userID, err := parseSubscriptionTokenSubject(token)
	if err != nil {
		return repository.HysteriaUserView{}, ErrInvalidSubscriptionToken
	}
	user, err := m.repo.GetHysteriaUser(ctx, userID)
	if err != nil {
		return repository.HysteriaUserView{}, err
	}
	ok := verifySubscriptionToken(m.cfg.InternalAuthToken, token, user.ID, user.UpdatedAt)
	if !ok {
		return repository.HysteriaUserView{}, ErrInvalidSubscriptionToken
	}
	return user, nil
}

func (m *HysteriaAccessManager) managedContent(ctx context.Context) (string, error) {
	if m.configManager == nil {
		return "", fmt.Errorf("hysteria config manager is not configured")
	}
	content, err := m.configManager.Read()
	if err != nil {
		return "", err
	}
	return m.InjectManagedAuth(ctx, content)
}

func (m *HysteriaAccessManager) preferredPublicHost() string {
	return NormalizeHost(m.cfg.Hy2Domain)
}

func (m *HysteriaAccessManager) baseClientProfileFromContent(content string) Hy2ClientProfile {
	fallbackHost := m.preferredPublicHost()
	profile := m.configManager.DefaultClientProfile(content, fallbackHost, m.cfg.Hy2Port, "")

	if domainValue := strings.TrimSpace(m.cfg.Hy2Domain); isHy2ServerURI(domainValue) {
		profile.Server = mergeServerURIWithDefaults(domainValue, profile)
		profile.Auth = ""
		profile.Obfs = nil
		profile.TLS.SNI = ""
		profile.TLS.Insecure = false
		profile.TLS.PinSHA256 = ""
		return profile
	}

	if strings.TrimSpace(profile.Server) == "" {
		host := fallbackHost
		if host == "" {
			host = NormalizeHost(m.cfg.PanelPublicHost)
		}
		if host == "" {
			host = "127.0.0.1"
		}
		port := m.cfg.Hy2Port
		if port <= 0 {
			port = 443
		}
		profile.Server = fmt.Sprintf("%s:%d", host, port)
	}
	return profile
}

func mergeServerURIWithDefaults(rawServer string, defaults Hy2ClientProfile) string {
	value := strings.TrimSpace(rawServer)
	u, err := url.Parse(value)
	if err != nil {
		return rawServer
	}

	// Keep URI host, but always inherit the current managed port union.
	defaultHost, defaultPorts := splitServerForClient(strings.TrimSpace(defaults.Server))
	if validPortUnion(defaultPorts) {
		uriHost, _ := splitServerAuthority(strings.TrimSpace(u.Host))
		if strings.TrimSpace(uriHost) == "" {
			uriHost = strings.TrimSpace(defaultHost)
		}
		if strings.TrimSpace(uriHost) != "" {
			u.Host = joinServerValue(uriHost, defaultPorts)
		}
	}

	query := u.Query()

	defaultSNI := strings.TrimSpace(defaults.TLS.SNI)
	normalizedSNI := strings.ToLower(NormalizeHost(defaultSNI))
	if strings.TrimSpace(query.Get("sni")) == "" && normalizedSNI != "" && normalizedSNI != "127.0.0.1" && normalizedSNI != "::1" && normalizedSNI != "localhost" {
		query.Set("sni", defaultSNI)
	}
	if strings.TrimSpace(query.Get("insecure")) == "" && defaults.TLS.Insecure {
		query.Set("insecure", "1")
	}
	if strings.TrimSpace(query.Get("pinSHA256")) == "" {
		if pin := normalizeCertHash(strings.TrimSpace(defaults.TLS.PinSHA256)); pin != "" {
			query.Set("pinSHA256", pin)
		}
	}

	obfsType := strings.ToLower(strings.TrimSpace(query.Get("obfs")))
	if obfsType == "" && defaults.Obfs != nil {
		if strings.EqualFold(strings.TrimSpace(defaults.Obfs.Type), "salamander") {
			query.Set("obfs", "salamander")
			obfsType = "salamander"
		}
	}
	if obfsType == "salamander" && strings.TrimSpace(query.Get("obfs-password")) == "" && defaults.Obfs != nil && defaults.Obfs.Salamander != nil {
		if password := strings.TrimSpace(defaults.Obfs.Salamander.Password); password != "" {
			query.Set("obfs-password", password)
		}
	}

	u.RawQuery = query.Encode()
	u.Path = "/"
	u.Fragment = ""
	return u.String()
}

func (m *HysteriaAccessManager) currentClientParamsFromContent(content string) Hy2ClientParams {
	return m.clientParamsFromProfile(m.baseClientProfileFromContent(content))
}

func (m *HysteriaAccessManager) defaultClientProfileFromContent(content string, user repository.HysteriaUserView) Hy2ClientProfile {
	profile := m.baseClientProfileFromContent(content)
	profile.Name = user.Username
	credential := hysteriadomain.BuildCredential(user.User)
	if isHy2ServerURI(profile.Server) {
		profile.Server = withAuthOnServerURI(profile.Server, credential)
		profile.Auth = ""
		return profile
	}
	profile.Auth = credential
	return profile
}

func applyClientOverrides(profile Hy2ClientProfile, overrides *hysteriadomain.ClientOverrides) Hy2ClientProfile {
	normalized := hysteriadomain.NormalizeClientOverrides(overrides)
	if normalized == nil {
		return profile
	}

	if isHy2ServerURI(profile.Server) {
		u, err := url.Parse(strings.TrimSpace(profile.Server))
		if err != nil {
			return profile
		}
		query := u.Query()
		if normalized.SNI != nil {
			query.Set("sni", strings.TrimSpace(*normalized.SNI))
		}
		if normalized.Insecure != nil {
			if *normalized.Insecure {
				query.Set("insecure", "1")
			} else {
				query.Set("insecure", "0")
			}
		}
		if normalized.PinSHA256 != nil {
			pin := normalizeCertHash(strings.TrimSpace(*normalized.PinSHA256))
			if pin != "" {
				query.Set("pinSHA256", pin)
			}
		}
		if normalized.ObfsType != nil && strings.EqualFold(strings.TrimSpace(*normalized.ObfsType), "salamander") {
			password := strings.TrimSpace(query.Get("obfs-password"))
			if normalized.ObfsPassword != nil {
				password = strings.TrimSpace(*normalized.ObfsPassword)
			}
			query.Set("obfs", "salamander")
			if password != "" {
				query.Set("obfs-password", password)
			}
		}
		u.RawQuery = query.Encode()
		u.Path = "/"
		u.Fragment = ""
		profile.Server = u.String()
		profile.Auth = ""
		profile.Obfs = nil
		profile.TLS.SNI = ""
		profile.TLS.Insecure = false
		profile.TLS.PinSHA256 = ""
		return profile
	}

	if normalized.SNI != nil {
		profile.TLS.SNI = strings.TrimSpace(*normalized.SNI)
	}
	if normalized.Insecure != nil {
		profile.TLS.Insecure = *normalized.Insecure
	}
	if normalized.PinSHA256 != nil {
		profile.TLS.PinSHA256 = strings.TrimSpace(*normalized.PinSHA256)
	}
	if normalized.ObfsType != nil && strings.EqualFold(strings.TrimSpace(*normalized.ObfsType), "salamander") {
		password := ""
		if profile.Obfs != nil && strings.EqualFold(strings.TrimSpace(profile.Obfs.Type), "salamander") && profile.Obfs.Salamander != nil {
			password = strings.TrimSpace(profile.Obfs.Salamander.Password)
		}
		if normalized.ObfsPassword != nil {
			password = strings.TrimSpace(*normalized.ObfsPassword)
		}
		profile.Obfs = &Hy2ClientObfs{Type: "salamander", Salamander: &Hy2ClientSalamander{Password: password}}
	}
	return profile
}

func withAuthOnServerURI(rawServer string, auth string) string {
	value := strings.TrimSpace(rawServer)
	u, err := url.Parse(value)
	if err != nil {
		return rawServer
	}
	credential := strings.TrimSpace(auth)
	if credential != "" {
		u.User = buildURIUserInfo(credential)
	}
	u.Path = "/"
	u.Fragment = ""
	return u.String()
}

func (m *HysteriaAccessManager) clientParamsFromProfile(profile Hy2ClientProfile) Hy2ClientParams {
	host, ports := splitServerForClient(profile.Server)
	tls := profile.TLS
	obfs := profile.Obfs

	if isHy2ServerURI(profile.Server) {
		if resolved, err := parseClientServerURI(profile.Server); err == nil {
			host = resolved.Host
			ports = resolved.PortUnion
			if strings.TrimSpace(tls.SNI) == "" {
				tls.SNI = strings.TrimSpace(resolved.TLS.SNI)
			}
			if !tls.Insecure {
				tls.Insecure = resolved.TLS.Insecure
			}
			if strings.TrimSpace(tls.PinSHA256) == "" {
				tls.PinSHA256 = strings.TrimSpace(resolved.TLS.PinSHA256)
			}
			if obfs == nil {
				obfs = resolved.Obfs
			}
		}
	}

	if host == "" {
		host = strings.TrimSpace(profile.Server)
	}
	port := firstPortFromUnion(ports)
	if port <= 0 {
		port = m.cfg.Hy2Port
	}
	pin := strings.TrimSpace(tls.PinSHA256)
	obfsType := ""
	obfsPassword := ""
	if obfs != nil {
		obfsType = strings.TrimSpace(obfs.Type)
		if obfs.Salamander != nil {
			obfsPassword = strings.TrimSpace(obfs.Salamander.Password)
		}
	}
	return Hy2ClientParams{
		Server:       host,
		Port:         port,
		PortUnion:    ports,
		SNI:          strings.TrimSpace(tls.SNI),
		Insecure:     tls.Insecure,
		PinSHA256:    pin,
		ObfsType:     obfsType,
		ObfsPassword: obfsPassword,
	}
}

func (m *HysteriaAccessManager) serverClientOptionsFromSettings(settings Hy2Settings) HysteriaServerClientOptions {
	result := HysteriaServerClientOptions{
		TLSEnabled:            settings.TLSEnabled,
		TLSMode:               settings.TLSMode,
		IgnoreClientBandwidth: settings.IgnoreClientBandwidth,
	}
	if settings.Obfs != nil {
		result.ObfsType = strings.TrimSpace(settings.Obfs.Type)
	}
	if settings.Masquerade != nil {
		result.MasqueradeType = strings.TrimSpace(settings.Masquerade.Type)
	}
	if settings.Bandwidth != nil {
		result.BandwidthUp = strings.TrimSpace(settings.Bandwidth.Up)
		result.BandwidthDown = strings.TrimSpace(settings.Bandwidth.Down)
	}
	return result
}

func (m *HysteriaAccessManager) buildSingBoxOutbound(user repository.HysteriaUserView, profile Hy2ClientProfile, params Hy2ClientParams) map[string]any {
	tls := map[string]any{"enabled": true}
	if strings.TrimSpace(params.SNI) != "" {
		tls["server_name"] = strings.TrimSpace(params.SNI)
	}
	if params.Insecure {
		tls["insecure"] = true
	}
	serverPort := params.Port
	if serverPort <= 0 {
		serverPort = m.cfg.Hy2Port
	}
	serverHost := strings.TrimSpace(params.Server)
	if serverHost == "" {
		serverHost = m.preferredPublicHost()
	}
	if serverHost == "" {
		serverHost = "127.0.0.1"
	}

	serverAuth := strings.TrimSpace(profile.Auth)
	if isHy2ServerURI(profile.Server) {
		if resolved, err := parseClientServerURI(profile.Server); err == nil {
			serverAuth = strings.TrimSpace(resolved.Auth)
		}
	}
	if serverAuth == "" {
		serverAuth = hysteriadomain.BuildCredential(user.User)
	}

	outbound := map[string]any{
		"type":        "hysteria2",
		"tag":         "hy2-" + strings.TrimSpace(user.Username),
		"server":      serverHost,
		"server_port": serverPort,
		"password":    serverAuth,
		"tls":         tls,
	}
	if ports := strings.TrimSpace(params.PortUnion); ports != "" && (strings.Contains(ports, ",") || strings.Contains(ports, "-")) {
		segments := make([]string, 0, 4)
		for _, segment := range strings.Split(ports, ",") {
			segment = strings.TrimSpace(segment)
			if segment != "" {
				segments = append(segments, segment)
			}
		}
		if len(segments) > 0 {
			outbound["server_ports"] = segments
			delete(outbound, "server_port")
		}
	}
	if strings.TrimSpace(params.ObfsType) != "" {
		obfs := map[string]any{"type": strings.TrimSpace(params.ObfsType)}
		if strings.TrimSpace(params.ObfsPassword) != "" {
			obfs["password"] = strings.TrimSpace(params.ObfsPassword)
		}
		outbound["obfs"] = obfs
	}
	return outbound
}

func buildSubscriptionToken(secret string, userID string, updatedAt time.Time) (string, error) {
	subject := strings.TrimSpace(userID)
	key := strings.TrimSpace(secret)
	if subject == "" || key == "" {
		return "", fmt.Errorf("subscription token key is empty")
	}
	payload := base64.RawURLEncoding.EncodeToString([]byte(subject))
	signature := computeSubscriptionSignature(key, subject, updatedAt)
	return payload + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parseSubscriptionTokenSubject(token string) (string, error) {
	value := strings.TrimSpace(token)
	if value == "" {
		return "", ErrInvalidSubscriptionToken
	}
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return "", ErrInvalidSubscriptionToken
	}
	subjectBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", ErrInvalidSubscriptionToken
	}
	subject := strings.TrimSpace(string(subjectBytes))
	if subject == "" {
		return "", ErrInvalidSubscriptionToken
	}
	return subject, nil
}

func verifySubscriptionToken(secret string, token string, userID string, updatedAt time.Time) bool {
	value := strings.TrimSpace(token)
	key := strings.TrimSpace(secret)
	subject := strings.TrimSpace(userID)
	if value == "" || key == "" || subject == "" {
		return false
	}
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	if strings.TrimSpace(string(payloadBytes)) != subject {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	expected := computeSubscriptionSignature(key, subject, updatedAt)
	return hmac.Equal(signature, expected)
}

func computeSubscriptionSignature(secret string, userID string, updatedAt time.Time) []byte {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	mac.Write([]byte("hy2-sub-v1"))
	mac.Write([]byte("|"))
	mac.Write([]byte(strings.TrimSpace(userID)))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(updatedAt.UTC().Unix(), 10)))
	return mac.Sum(nil)
}
