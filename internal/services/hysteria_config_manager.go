package services

import (
	"crypto/sha256"
	"crypto/x509"
	"crypto/rand"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"proxy-panel/internal/fsutil"

	"gopkg.in/yaml.v3"
)

type Hy2ConfigSummary struct {
	Listen                string `json:"listen"`
	TLSEnabled            bool   `json:"tlsEnabled"`
	TLSMode               string `json:"tlsMode,omitempty"`
	AuthType              string `json:"authType,omitempty"`
	ObfsType              string `json:"obfsType,omitempty"`
	MasqueradeType        string `json:"masqueradeType,omitempty"`
	QUICEnabled           bool   `json:"quicEnabled"`
	BandwidthUp           string `json:"bandwidthUp,omitempty"`
	BandwidthDown         string `json:"bandwidthDown,omitempty"`
	IgnoreClientBandwidth bool   `json:"ignoreClientBandwidth"`
	SpeedTest             bool   `json:"speedTest"`
	DisableUDP            bool   `json:"disableUDP"`
	UDPIdleTimeout        string `json:"udpIdleTimeout,omitempty"`
	RawOnlyPathsCount     int    `json:"rawOnlyPathsCount"`
}

type Hy2ConfigValidation struct {
	Valid        bool             `json:"valid"`
	Errors       []string         `json:"errors"`
	Warnings     []string         `json:"warnings"`
	Summary      Hy2ConfigSummary `json:"summary"`
	RawOnlyPaths []string         `json:"rawOnlyPaths,omitempty"`
}

type Hy2Settings struct {
	Listen                string               `json:"listen"`
	TLSEnabled            bool                 `json:"tlsEnabled"`
	TLSMode               string               `json:"tlsMode"`
	ClientTLSInsecure     bool                 `json:"clientTLSInsecure"`
	TLS                   *Hy2ServerTLS        `json:"tls,omitempty"`
	ACME                  *Hy2ServerACME       `json:"acme,omitempty"`
	Auth                  Hy2ServerAuth        `json:"auth"`
	Obfs                  *Hy2ServerObfs       `json:"obfs,omitempty"`
	Masquerade            *Hy2ServerMasquerade `json:"masquerade,omitempty"`
	Bandwidth             *Hy2ServerBandwidth  `json:"bandwidth,omitempty"`
	IgnoreClientBandwidth bool                 `json:"ignoreClientBandwidth"`
	SpeedTest             bool                 `json:"speedTest"`
	DisableUDP            bool                 `json:"disableUDP"`
	UDPIdleTimeout        string               `json:"udpIdleTimeout,omitempty"`
	QUICEnabled           bool                 `json:"quicEnabled"`
	QUIC                  *Hy2ServerQUIC       `json:"quic,omitempty"`
}

type Hy2ServerTLS struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

type Hy2ServerACME struct {
	Domains []string `json:"domains,omitempty"`
	Email   string   `json:"email,omitempty"`
}

type Hy2ServerBandwidth struct {
	Up   string `json:"up,omitempty"`
	Down string `json:"down,omitempty"`
}

type Hy2ServerQUIC struct {
	InitStreamReceiveWindow int    `json:"initStreamReceiveWindow,omitempty"`
	MaxStreamReceiveWindow  int    `json:"maxStreamReceiveWindow,omitempty"`
	InitConnReceiveWindow   int    `json:"initConnReceiveWindow,omitempty"`
	MaxConnReceiveWindow    int    `json:"maxConnReceiveWindow,omitempty"`
	MaxIdleTimeout          string `json:"maxIdleTimeout,omitempty"`
	MaxIncomingStreams      int    `json:"maxIncomingStreams,omitempty"`
	DisablePathMTUDiscovery bool   `json:"disablePathMTUDiscovery,omitempty"`
}

type Hy2ServerAuth struct {
	Type     string             `json:"type,omitempty"`
	Password string             `json:"password,omitempty"`
	UserPass map[string]string  `json:"userpass,omitempty"`
	HTTP     *Hy2ServerAuthHTTP `json:"http,omitempty"`
	Command  string             `json:"command,omitempty"`
}

type Hy2ServerAuthHTTP struct {
	URL      string `json:"url,omitempty"`
	Insecure bool   `json:"insecure,omitempty"`
}

type Hy2ServerObfs struct {
	Type       string               `json:"type,omitempty"`
	Salamander *Hy2ServerSalamander `json:"salamander,omitempty"`
}

type Hy2ServerSalamander struct {
	Password string `json:"password,omitempty"`
}

type Hy2ServerMasquerade struct {
	Type        string                     `json:"type,omitempty"`
	File        *Hy2ServerMasqueradeFile   `json:"file,omitempty"`
	Proxy       *Hy2ServerMasqueradeProxy  `json:"proxy,omitempty"`
	String      *Hy2ServerMasqueradeString `json:"string,omitempty"`
	ListenHTTP  string                     `json:"listenHTTP,omitempty"`
	ListenHTTPS string                     `json:"listenHTTPS,omitempty"`
	ForceHTTPS  bool                       `json:"forceHTTPS,omitempty"`
}

type Hy2ServerMasqueradeFile struct {
	Dir string `json:"dir,omitempty"`
}

type Hy2ServerMasqueradeProxy struct {
	URL         string `json:"url,omitempty"`
	RewriteHost bool   `json:"rewriteHost,omitempty"`
	Insecure    bool   `json:"insecure,omitempty"`
}

type Hy2ServerMasqueradeString struct {
	Content    string            `json:"content,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	StatusCode int               `json:"statusCode,omitempty"`
}

type Hy2SettingsValidation struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

type Hy2ClientParams struct {
	Server       string `json:"server"`
	Port         int    `json:"port"`
	PortUnion    string `json:"portUnion,omitempty"`
	SNI          string `json:"sni,omitempty"`
	Insecure     bool   `json:"insecure"`
	PinSHA256    string `json:"pinSHA256,omitempty"`
	ObfsType     string `json:"obfsType,omitempty"`
	ObfsPassword string `json:"obfsPassword,omitempty"`
}

type HysteriaConfigManager struct {
	Path string
}

type Hy2ClientProfile struct {
	Name      string               `json:"name,omitempty"`
	Server    string               `json:"server"`
	Auth      string               `json:"auth"`
	AuthType  string               `json:"authType,omitempty"`
	TLS       Hy2ClientTLS         `json:"tls"`
	Obfs      *Hy2ClientObfs       `json:"obfs,omitempty"`
	QUIC      *Hy2ClientQUIC       `json:"quic,omitempty"`
	Bandwidth *Hy2ClientBandwidth  `json:"bandwidth,omitempty"`
	Socks5    *Hy2ClientSocks5Mode `json:"socks5,omitempty"`
	HTTP      *Hy2ClientHTTPMode   `json:"http,omitempty"`
}

type Hy2ClientTLS struct {
	SNI               string `json:"sni,omitempty"`
	Insecure          bool   `json:"insecure,omitempty"`
	PinSHA256         string `json:"pinSHA256,omitempty"`
	CA                string `json:"ca,omitempty"`
	ClientCertificate string `json:"clientCertificate,omitempty"`
	ClientKey         string `json:"clientKey,omitempty"`
}

type Hy2ClientObfs struct {
	Type       string               `json:"type,omitempty"`
	Salamander *Hy2ClientSalamander `json:"salamander,omitempty"`
}

type Hy2ClientSalamander struct {
	Password string `json:"password,omitempty"`
}

type Hy2ClientQUIC struct {
	InitStreamReceiveWindow int    `json:"initStreamReceiveWindow,omitempty"`
	MaxStreamReceiveWindow  int    `json:"maxStreamReceiveWindow,omitempty"`
	InitConnReceiveWindow   int    `json:"initConnReceiveWindow,omitempty"`
	MaxConnReceiveWindow    int    `json:"maxConnReceiveWindow,omitempty"`
	MaxIdleTimeout          string `json:"maxIdleTimeout,omitempty"`
	DisablePathMTUDiscovery bool   `json:"disablePathMTUDiscovery,omitempty"`
}

type Hy2ClientBandwidth struct {
	Up   string `json:"up,omitempty"`
	Down string `json:"down,omitempty"`
}

type Hy2ClientSocks5Mode struct {
	Listen     string `json:"listen,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	DisableUDP bool   `json:"disableUDP,omitempty"`
}

type Hy2ClientHTTPMode struct {
	Listen   string `json:"listen,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Realm    string `json:"realm,omitempty"`
}

type Hy2ClientValidation struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

type Hy2ClientArtifacts struct {
	URI        string `json:"uri"`
	URIHy2     string `json:"uriHy2"`
	ClientYAML string `json:"clientYAML"`
}

type schemaNode struct {
	Fields   map[string]*schemaNode
	ListItem *schemaNode
	AnyMap   bool
}

var (
	emptySchema  = &schemaNode{}
	serverSchema = buildServerSchema()
)

func NewHysteriaConfigManager(path string) *HysteriaConfigManager {
	return &HysteriaConfigManager{Path: strings.TrimSpace(path)}
}

func (m *HysteriaConfigManager) Read() (string, error) {
	if strings.TrimSpace(m.Path) == "" {
		return "", fmt.Errorf("hysteria config path is empty")
	}
	content, err := os.ReadFile(m.Path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (m *HysteriaConfigManager) Save(content string) (string, error) {
	if strings.TrimSpace(m.Path) == "" {
		return "", fmt.Errorf("hysteria config path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(m.Path), 0o750); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	mode := os.FileMode(0o640)
	backupPath := ""
	if st, err := os.Stat(m.Path); err == nil {
		mode = st.Mode().Perm()
		if previous, readErr := os.ReadFile(m.Path); readErr == nil {
			backupPath = fmt.Sprintf("%s.bak.%d", m.Path, time.Now().UTC().Unix())
			if writeErr := os.WriteFile(backupPath, previous, mode); writeErr != nil {
				return "", fmt.Errorf("write backup file: %w", writeErr)
			}
		}
	}

	normalized := strings.TrimRight(content, "\n") + "\n"
	if err := fsutil.WriteFileAtomic(m.Path, []byte(normalized), mode); err != nil {
		return "", fmt.Errorf("write config file: %w", err)
	}
	return backupPath, nil
}

func (m *HysteriaConfigManager) Parse(content string) Hy2ConfigSummary {
	settings, err := m.ExtractSettingsWithError(content, "", 443)
	if err != nil {
		return Hy2ConfigSummary{}
	}
	unknown := m.RawOnlyPaths(content)
	return configSummaryFromSettings(settings, len(unknown))
}

func (m *HysteriaConfigManager) Validate(content string) Hy2ConfigValidation {
	v := Hy2ConfigValidation{Errors: []string{}, Warnings: []string{}}
	root, err := parseYAMLMap(content)
	if err != nil {
		v.Errors = append(v.Errors, "invalid YAML: "+strings.TrimSpace(err.Error()))
		v.Valid = false
		return v
	}

	settings := parseSettingsFromMap(root, "", 443)
	sv := validateSettings(settings)
	v.Errors = append(v.Errors, sv.Errors...)
	v.Warnings = append(v.Warnings, sv.Warnings...)
	_, hasTLS := toStringAnyMap(root["tls"])
	_, hasACME := toStringAnyMap(root["acme"])
	if !hasTLS && !hasACME {
		v.Errors = append(v.Errors, "either tls or acme section is required by Hysteria 2")
	}
	v.RawOnlyPaths = collectUnknown(root, serverSchema)

	v.Summary = configSummaryFromSettings(settings, len(v.RawOnlyPaths))
	v.Valid = len(v.Errors) == 0
	return v
}

func (m *HysteriaConfigManager) RawOnlyPaths(content string) []string {
	root, err := parseYAMLMap(content)
	if err != nil {
		return nil
	}
	return collectUnknown(root, serverSchema)
}

func (m *HysteriaConfigManager) ExtractSettings(content string, fallbackHost string, fallbackPort int) Hy2Settings {
	settings, err := m.ExtractSettingsWithError(content, fallbackHost, fallbackPort)
	if err != nil {
		return defaultSettings(fallbackHost, fallbackPort)
	}
	return settings
}

func (m *HysteriaConfigManager) ExtractSettingsWithError(content string, fallbackHost string, fallbackPort int) (Hy2Settings, error) {
	root, err := parseYAMLMap(content)
	if err != nil {
		return Hy2Settings{}, err
	}
	return parseSettingsFromMap(root, fallbackHost, fallbackPort), nil
}

func (m *HysteriaConfigManager) ValidateSettings(input Hy2Settings) Hy2SettingsValidation {
	return validateSettings(normalizeSettings(input))
}

func (m *HysteriaConfigManager) ApplySettings(content string, input Hy2Settings) (string, Hy2SettingsValidation) {
	settings := normalizeSettings(input)
	autoGenerateManagedSecrets(&settings)
	sv := validateSettings(settings)
	if !sv.Valid {
		return content, sv
	}

	currentMap, err := parseYAMLMap(content)
	if err != nil {
		sv.Valid = false
		sv.Errors = append(sv.Errors, "current YAML is invalid; fix it in the raw editor before using structured save")
		return content, sv
	}
	desiredMap := buildSettingsMap(settings)
	merged := mergeKnownValues(currentMap, desiredMap, serverSchema)
	mergedMap, ok := merged.(map[string]any)
	if !ok {
		sv.Valid = false
		sv.Errors = append(sv.Errors, "failed to build resulting config")
		return content, sv
	}

	next, marshalErr := marshalYAMLMap(mergedMap)
	if marshalErr != nil {
		sv.Valid = false
		sv.Errors = append(sv.Errors, "failed to render YAML: "+strings.TrimSpace(marshalErr.Error()))
		return content, sv
	}
	return next, sv
}
func (m *HysteriaConfigManager) ClientParams(content string, fallbackHost string, fallbackPort int) Hy2ClientParams {
	settings := m.ExtractSettings(content, fallbackHost, fallbackPort)
	profile := m.DefaultClientProfileFromSettings(settings, fallbackHost, fallbackPort, "")
	resolved, validation := resolveClientProfile(profile)

	host := strings.TrimSpace(resolved.Host)
	ports := strings.TrimSpace(resolved.PortUnion)
	if !validation.Valid {
		host, ports = splitServerForClient(profile.Server)
	}

	port := firstPortFromUnion(ports)
	if port <= 0 {
		port = fallbackPort
	}
	if host == "" {
		host = sanitizePublicHost(fallbackHost)
	}
	if host == "" {
		host = "127.0.0.1"
	}

	obfsType := ""
	obfsPassword := ""
	if resolved.Obfs != nil {
		obfsType = strings.TrimSpace(resolved.Obfs.Type)
		if resolved.Obfs.Salamander != nil {
			obfsPassword = strings.TrimSpace(resolved.Obfs.Salamander.Password)
		}
	}

	return Hy2ClientParams{
		Server:       host,
		Port:         port,
		PortUnion:    ports,
		SNI:          strings.TrimSpace(resolved.TLS.SNI),
		Insecure:     resolved.TLS.Insecure,
		PinSHA256:    strings.TrimSpace(resolved.TLS.PinSHA256),
		ObfsType:     obfsType,
		ObfsPassword: obfsPassword,
	}
}

func (m *HysteriaConfigManager) DefaultClientProfile(content string, fallbackHost string, fallbackPort int, auth string) Hy2ClientProfile {
	settings := m.ExtractSettings(content, fallbackHost, fallbackPort)
	return m.DefaultClientProfileFromSettings(settings, fallbackHost, fallbackPort, auth)
}

func (m *HysteriaConfigManager) DefaultClientProfileFromSettings(settings Hy2Settings, fallbackHost string, fallbackPort int, auth string) Hy2ClientProfile {
	settings = normalizeSettings(settings)
	listenHost, listenPorts, ok := parseListen(settings.Listen)
	if !ok || !validPortUnion(listenPorts) {
		listenPorts = strconv.Itoa(maxInt(fallbackPort, 443))
	}

	publicHost := sanitizePublicHost(fallbackHost)
	acmeHost := ""
	if settings.ACME != nil && len(settings.ACME.Domains) > 0 {
		acmeHost = sanitizePublicHost(settings.ACME.Domains[0])
	}
	if publicHost == "" {
		publicHost = acmeHost
	}
	if publicHost == "" {
		publicHost = sanitizePublicHost(listenHost)
	}
	if publicHost == "" {
		publicHost = "127.0.0.1"
	}

	sni := ""
	if settings.TLSEnabled {
		sni = acmeHost
		if sni == "" {
			sni = publicHost
		}
	}

	profile := Hy2ClientProfile{
		Server:   publicHost + ":" + listenPorts,
		Auth:     strings.TrimSpace(auth),
		AuthType: strings.ToLower(strings.TrimSpace(settings.Auth.Type)),
		TLS:      Hy2ClientTLS{SNI: sni, Insecure: settings.TLSEnabled && settings.ClientTLSInsecure},
		Socks5:   &Hy2ClientSocks5Mode{Listen: "127.0.0.1:1080"},
	}
	if settings.TLSEnabled {
		profile.TLS.PinSHA256 = m.defaultTLSPinSHA256(settings)
	}

	if settings.Obfs != nil && strings.EqualFold(strings.TrimSpace(settings.Obfs.Type), "salamander") {
		password := ""
		if settings.Obfs.Salamander != nil {
			password = strings.TrimSpace(settings.Obfs.Salamander.Password)
		}
		profile.Obfs = &Hy2ClientObfs{Type: "salamander", Salamander: &Hy2ClientSalamander{Password: password}}
	}

	if settings.QUICEnabled && settings.QUIC != nil {
		profile.QUIC = &Hy2ClientQUIC{
			InitStreamReceiveWindow: settings.QUIC.InitStreamReceiveWindow,
			MaxStreamReceiveWindow:  settings.QUIC.MaxStreamReceiveWindow,
			InitConnReceiveWindow:   settings.QUIC.InitConnReceiveWindow,
			MaxConnReceiveWindow:    settings.QUIC.MaxConnReceiveWindow,
			MaxIdleTimeout:          settings.QUIC.MaxIdleTimeout,
			DisablePathMTUDiscovery: settings.QUIC.DisablePathMTUDiscovery,
		}
	}

	return normalizeClientProfile(profile)
}

func (m *HysteriaConfigManager) defaultTLSPinSHA256(settings Hy2Settings) string {
	if settings.TLS == nil {
		return ""
	}

	certPath := strings.TrimSpace(settings.TLS.Cert)
	if certPath == "" {
		return ""
	}

	certPEM, err := m.readManagedFile(certPath)
	if err != nil {
		return ""
	}

	cert, err := parseFirstCertificate(certPEM)
	if err != nil {
		return ""
	}

	publicKey, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return ""
	}

	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:])
}

func (m *HysteriaConfigManager) readManagedFile(pathValue string) ([]byte, error) {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" {
		return nil, fmt.Errorf("path is empty")
	}

	if filepath.IsAbs(pathValue) {
		return os.ReadFile(pathValue)
	}

	// Try direct relative path first.
	content, directErr := os.ReadFile(pathValue)
	if directErr == nil {
		return content, nil
	}

	// Then resolve relative to the managed config directory.
	configPath := strings.TrimSpace(m.Path)
	if configPath == "" {
		return nil, directErr
	}
	return os.ReadFile(filepath.Join(filepath.Dir(configPath), pathValue))
}

func parseFirstCertificate(content []byte) (*x509.Certificate, error) {
	remaining := content
	for len(remaining) > 0 {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		remaining = rest
		if !strings.EqualFold(strings.TrimSpace(block.Type), "CERTIFICATE") {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err == nil {
			return cert, nil
		}
	}

	// Also support raw DER files.
	if cert, err := x509.ParseCertificate(content); err == nil {
		return cert, nil
	}

	return nil, fmt.Errorf("certificate not found")
}

func (m *HysteriaConfigManager) ValidateClientProfile(profile Hy2ClientProfile) Hy2ClientValidation {
	return validateClientProfile(profile)
}

func (m *HysteriaConfigManager) GenerateClientArtifacts(profile Hy2ClientProfile) (Hy2ClientArtifacts, Hy2ClientValidation) {
	normalized := normalizeClientProfile(profile)
	v := validateClientProfile(normalized)
	if !v.Valid {
		return Hy2ClientArtifacts{}, v
	}

	uri, uriHy2 := buildClientURI(normalized)
	clientYAML, err := buildClientYAML(normalized)
	if err != nil {
		v.Valid = false
		v.Errors = append(v.Errors, "failed to build client YAML: "+strings.TrimSpace(err.Error()))
		return Hy2ClientArtifacts{}, v
	}
	return Hy2ClientArtifacts{URI: uri, URIHy2: uriHy2, ClientYAML: clientYAML}, v
}

func defaultSettings(fallbackHost string, fallbackPort int) Hy2Settings {
	port := maxInt(fallbackPort, 443)
	host := sanitizePublicHost(fallbackHost)
	domains := []string{}
	if host != "" && net.ParseIP(host) == nil {
		domains = append(domains, host)
	}
	return Hy2Settings{
		Listen:            fmt.Sprintf(":%d", port),
		TLSEnabled:        true,
		TLSMode:           "acme",
		ClientTLSInsecure: false,
		ACME:              &Hy2ServerACME{Domains: domains, Email: ""},
		Auth:              Hy2ServerAuth{Type: "userpass", UserPass: map[string]string{}},
		QUICEnabled:       false,
	}
}

func parseSettingsFromMap(root map[string]any, fallbackHost string, fallbackPort int) Hy2Settings {
	settings := defaultSettings(fallbackHost, fallbackPort)
	settings.Listen = firstNonEmpty(toString(root["listen"]), settings.Listen)
	settings.TLS = nil
	settings.ACME = nil
	settings.TLSEnabled = false
	settings.QUIC = nil
	settings.QUICEnabled = false

	if m, ok := toStringAnyMap(root["tls"]); ok {
		settings.TLS = parseServerTLS(m)
	}
	if m, ok := toStringAnyMap(root["acme"]); ok {
		settings.ACME = parseServerACME(m)
	}
	switch {
	case settings.TLS != nil && settings.ACME != nil:
		settings.TLSMode = "conflict"
		settings.TLSEnabled = true
	case settings.TLS != nil:
		settings.TLSMode = "tls"
		settings.TLSEnabled = true
	case settings.ACME != nil:
		settings.TLSMode = "acme"
		settings.TLSEnabled = true
	default:
		settings.TLSMode = "acme"
		settings.TLSEnabled = false
	}

	if m, ok := toStringAnyMap(root["auth"]); ok {
		settings.Auth = parseServerAuth(m)
	}
	if m, ok := toStringAnyMap(root["obfs"]); ok {
		settings.Obfs = parseServerObfs(m)
	}
	if m, ok := toStringAnyMap(root["masquerade"]); ok {
		settings.Masquerade = parseServerMasquerade(m)
	}
	if m, ok := toStringAnyMap(root["bandwidth"]); ok {
		settings.Bandwidth = parseServerBandwidth(m)
	}
	settings.ClientTLSInsecure = toBool(root["clientTLSInsecure"])
	settings.IgnoreClientBandwidth = toBool(root["ignoreClientBandwidth"])
	settings.SpeedTest = toBool(root["speedTest"]) || toBool(root["speedtest"])
	settings.DisableUDP = toBool(root["disableUDP"])
	settings.UDPIdleTimeout = strings.TrimSpace(toString(root["udpIdleTimeout"]))
	if m, ok := toStringAnyMap(root["quic"]); ok {
		settings.QUIC = parseServerQUIC(m)
		settings.QUICEnabled = settings.QUIC != nil
	}

	return normalizeSettings(settings)
}

func parseServerTLS(m map[string]any) *Hy2ServerTLS {
	cfg := &Hy2ServerTLS{
		Cert: strings.TrimSpace(toString(m["cert"])),
		Key:  strings.TrimSpace(toString(m["key"])),
	}
	if cfg.Cert == "" && cfg.Key == "" {
		return nil
	}
	return cfg
}

func parseServerACME(m map[string]any) *Hy2ServerACME {
	cfg := &Hy2ServerACME{
		Domains: trimStringSlice(toStringSlice(m["domains"])),
		Email:   strings.TrimSpace(toString(m["email"])),
	}
	if len(cfg.Domains) == 0 && cfg.Email == "" {
		return nil
	}
	return cfg
}

func parseServerBandwidth(m map[string]any) *Hy2ServerBandwidth {
	cfg := &Hy2ServerBandwidth{
		Up:   strings.TrimSpace(toString(m["up"])),
		Down: strings.TrimSpace(toString(m["down"])),
	}
	if cfg.Up == "" && cfg.Down == "" {
		return nil
	}
	return cfg
}
func parseServerQUIC(m map[string]any) *Hy2ServerQUIC {
	cfg := &Hy2ServerQUIC{
		InitStreamReceiveWindow: toInt(m["initStreamReceiveWindow"]),
		MaxStreamReceiveWindow:  toInt(m["maxStreamReceiveWindow"]),
		InitConnReceiveWindow:   toInt(m["initConnReceiveWindow"]),
		MaxConnReceiveWindow:    toInt(m["maxConnReceiveWindow"]),
		MaxIdleTimeout:          strings.TrimSpace(toString(m["maxIdleTimeout"])),
		MaxIncomingStreams:      toInt(m["maxIncomingStreams"]),
		DisablePathMTUDiscovery: toBool(m["disablePathMTUDiscovery"]),
	}

	if cfg.InitStreamReceiveWindow <= 0 {
		cfg.InitStreamReceiveWindow = 0
	}
	if cfg.MaxStreamReceiveWindow <= 0 {
		cfg.MaxStreamReceiveWindow = 0
	}
	if cfg.InitConnReceiveWindow <= 0 {
		cfg.InitConnReceiveWindow = 0
	}
	if cfg.MaxConnReceiveWindow <= 0 {
		cfg.MaxConnReceiveWindow = 0
	}
	if cfg.MaxIncomingStreams <= 0 {
		cfg.MaxIncomingStreams = 0
	}

	if cfg.InitStreamReceiveWindow == 0 && cfg.MaxStreamReceiveWindow == 0 && cfg.InitConnReceiveWindow == 0 && cfg.MaxConnReceiveWindow == 0 && cfg.MaxIdleTimeout == "" && cfg.MaxIncomingStreams == 0 && !cfg.DisablePathMTUDiscovery {
		return nil
	}
	return cfg
}

func parseServerAuth(m map[string]any) Hy2ServerAuth {
	auth := Hy2ServerAuth{
		Type:     strings.ToLower(strings.TrimSpace(toString(m["type"]))),
		Password: strings.TrimSpace(toString(m["password"])),
		UserPass: trimStringStringMap(toStringStringMap(m["userpass"])),
		Command:  strings.TrimSpace(toString(m["command"])),
	}
	if mm, ok := toStringAnyMap(m["http"]); ok {
		auth.HTTP = &Hy2ServerAuthHTTP{URL: strings.TrimSpace(toString(mm["url"])), Insecure: toBool(mm["insecure"])}
	}
	return auth
}

func parseServerObfs(m map[string]any) *Hy2ServerObfs {
	t := strings.ToLower(strings.TrimSpace(toString(m["type"])))
	cfg := &Hy2ServerObfs{Type: t}
	if sm, ok := toStringAnyMap(m["salamander"]); ok {
		cfg.Salamander = &Hy2ServerSalamander{Password: strings.TrimSpace(toString(sm["password"]))}
	}
	if cfg.Type == "" && cfg.Salamander != nil {
		cfg.Type = "salamander"
	}
	if cfg.Type == "" {
		return nil
	}
	return cfg
}

func parseServerMasquerade(m map[string]any) *Hy2ServerMasquerade {
	cfg := &Hy2ServerMasquerade{Type: strings.ToLower(strings.TrimSpace(toString(m["type"]))), ListenHTTP: strings.TrimSpace(toString(m["listenHTTP"])), ListenHTTPS: strings.TrimSpace(toString(m["listenHTTPS"])), ForceHTTPS: toBool(m["forceHTTPS"])}
	if mm, ok := toStringAnyMap(m["file"]); ok {
		cfg.File = &Hy2ServerMasqueradeFile{Dir: strings.TrimSpace(toString(mm["dir"]))}
	}
	if mm, ok := toStringAnyMap(m["proxy"]); ok {
		cfg.Proxy = &Hy2ServerMasqueradeProxy{URL: strings.TrimSpace(toString(mm["url"])), RewriteHost: toBool(mm["rewriteHost"]), Insecure: toBool(mm["insecure"])}
	}
	if mm, ok := toStringAnyMap(m["string"]); ok {
		cfg.String = &Hy2ServerMasqueradeString{Content: strings.TrimSpace(toString(mm["content"])), Headers: toStringStringMap(mm["headers"]), StatusCode: toInt(mm["statusCode"])}
	}
	if cfg.Type == "" && cfg.File == nil && cfg.Proxy == nil && cfg.String == nil && cfg.ListenHTTP == "" && cfg.ListenHTTPS == "" && !cfg.ForceHTTPS {
		return nil
	}
	if cfg.Type == "" {
		switch {
		case cfg.Proxy != nil:
			cfg.Type = "proxy"
		case cfg.File != nil:
			cfg.Type = "file"
		case cfg.String != nil:
			cfg.Type = "string"
		}
	}
	return cfg
}

func normalizeSettings(input Hy2Settings) Hy2Settings {
	settings := input
	settings.Listen = normalizeListenInput(settings.Listen)
	settings.TLSMode = strings.ToLower(strings.TrimSpace(settings.TLSMode))

	if settings.TLS != nil {
		settings.TLS.Cert = strings.TrimSpace(settings.TLS.Cert)
		settings.TLS.Key = strings.TrimSpace(settings.TLS.Key)
		if settings.TLS.Cert == "" && settings.TLS.Key == "" {
			settings.TLS = nil
		}
	}

	if settings.ACME != nil {
		settings.ACME.Email = strings.TrimSpace(settings.ACME.Email)
		settings.ACME.Domains = trimStringSlice(settings.ACME.Domains)
		if len(settings.ACME.Domains) == 0 && settings.ACME.Email == "" {
			settings.ACME = nil
		}
	}

	if settings.TLS != nil || settings.ACME != nil {
		settings.TLSEnabled = true
	}
	if settings.TLSMode == "" {
		switch {
		case settings.TLS != nil && settings.ACME != nil:
			settings.TLSMode = "conflict"
		case settings.TLS != nil:
			settings.TLSMode = "tls"
		case settings.ACME != nil:
			settings.TLSMode = "acme"
		default:
			settings.TLSMode = "acme"
		}
	}
	if settings.TLSMode != "tls" && settings.TLSMode != "acme" && settings.TLSMode != "conflict" {
		settings.TLSMode = "acme"
	}

	if !settings.TLSEnabled {
		settings.TLS = nil
		settings.ACME = nil
	} else {
		switch settings.TLSMode {
		case "tls":
			settings.ACME = nil
		case "acme":
			settings.TLS = nil
		}
	}

	settings.Auth.Type = strings.ToLower(strings.TrimSpace(settings.Auth.Type))
	settings.Auth.Password = strings.TrimSpace(settings.Auth.Password)
	settings.Auth.UserPass = trimStringStringMap(settings.Auth.UserPass)
	settings.Auth.Command = strings.TrimSpace(settings.Auth.Command)
	if settings.Auth.HTTP != nil {
		settings.Auth.HTTP.URL = strings.TrimSpace(settings.Auth.HTTP.URL)
		if settings.Auth.HTTP.URL == "" && !settings.Auth.HTTP.Insecure {
			settings.Auth.HTTP = nil
		}
	}
	if settings.Auth.Type == "" {
		settings.Auth.Type = "userpass"
	}
	switch settings.Auth.Type {
	case "userpass":
		settings.Auth.Password = ""
		settings.Auth.HTTP = nil
		settings.Auth.Command = ""
		if settings.Auth.UserPass == nil {
			settings.Auth.UserPass = map[string]string{}
		}
	case "password":
		settings.Auth.UserPass = nil
		settings.Auth.HTTP = nil
		settings.Auth.Command = ""
	case "http":
		settings.Auth.Password = ""
		settings.Auth.UserPass = nil
		settings.Auth.Command = ""
	case "command":
		settings.Auth.Password = ""
		settings.Auth.UserPass = nil
		settings.Auth.HTTP = nil
	default:
		settings.Auth.Type = "userpass"
		settings.Auth.Password = ""
		settings.Auth.HTTP = nil
		settings.Auth.Command = ""
		if settings.Auth.UserPass == nil {
			settings.Auth.UserPass = map[string]string{}
		}
	}

	if settings.Obfs != nil {
		settings.Obfs.Type = strings.ToLower(strings.TrimSpace(settings.Obfs.Type))
		if settings.Obfs.Salamander != nil {
			settings.Obfs.Salamander.Password = strings.TrimSpace(settings.Obfs.Salamander.Password)
		}
		if settings.Obfs.Type == "" && settings.Obfs.Salamander != nil && settings.Obfs.Salamander.Password != "" {
			settings.Obfs.Type = "salamander"
		}
		if settings.Obfs.Type == "" {
			settings.Obfs = nil
		}
	}

	if settings.Masquerade != nil {
		settings.Masquerade.Type = strings.ToLower(strings.TrimSpace(settings.Masquerade.Type))
		settings.Masquerade.ListenHTTP = strings.TrimSpace(settings.Masquerade.ListenHTTP)
		settings.Masquerade.ListenHTTPS = strings.TrimSpace(settings.Masquerade.ListenHTTPS)
		if settings.Masquerade.File != nil {
			settings.Masquerade.File.Dir = strings.TrimSpace(settings.Masquerade.File.Dir)
			if settings.Masquerade.File.Dir == "" {
				settings.Masquerade.File = nil
			}
		}
		if settings.Masquerade.Proxy != nil {
			settings.Masquerade.Proxy.URL = strings.TrimSpace(settings.Masquerade.Proxy.URL)
			if settings.Masquerade.Proxy.URL == "" && !settings.Masquerade.Proxy.RewriteHost && !settings.Masquerade.Proxy.Insecure {
				settings.Masquerade.Proxy = nil
			}
		}
		if settings.Masquerade.String != nil {
			settings.Masquerade.String.Content = strings.TrimSpace(settings.Masquerade.String.Content)
			settings.Masquerade.String.Headers = trimStringStringMap(settings.Masquerade.String.Headers)
			if settings.Masquerade.String.Content == "" && len(settings.Masquerade.String.Headers) == 0 && settings.Masquerade.String.StatusCode <= 0 {
				settings.Masquerade.String = nil
			}
		}
		if settings.Masquerade.Type == "" {
			switch {
			case settings.Masquerade.Proxy != nil:
				settings.Masquerade.Type = "proxy"
			case settings.Masquerade.File != nil:
				settings.Masquerade.Type = "file"
			case settings.Masquerade.String != nil:
				settings.Masquerade.Type = "string"
			}
		}
		if settings.Masquerade.Type == "" {
			settings.Masquerade = nil
		}
	}

	settings.UDPIdleTimeout = strings.TrimSpace(settings.UDPIdleTimeout)
	if settings.Bandwidth != nil {
		settings.Bandwidth.Up = normalizeBandwidthInput(settings.Bandwidth.Up)
		settings.Bandwidth.Down = normalizeBandwidthInput(settings.Bandwidth.Down)
		if settings.Bandwidth.Up == "" && settings.Bandwidth.Down == "" {
			settings.Bandwidth = nil
		}
	}
	if settings.QUIC != nil {
		settings.QUIC.InitStreamReceiveWindow = positiveIntOrZero(settings.QUIC.InitStreamReceiveWindow)
		settings.QUIC.MaxStreamReceiveWindow = positiveIntOrZero(settings.QUIC.MaxStreamReceiveWindow)
		settings.QUIC.InitConnReceiveWindow = positiveIntOrZero(settings.QUIC.InitConnReceiveWindow)
		settings.QUIC.MaxConnReceiveWindow = positiveIntOrZero(settings.QUIC.MaxConnReceiveWindow)
		settings.QUIC.MaxIncomingStreams = positiveIntOrZero(settings.QUIC.MaxIncomingStreams)
		settings.QUIC.MaxIdleTimeout = strings.TrimSpace(settings.QUIC.MaxIdleTimeout)
		if settings.QUIC.InitStreamReceiveWindow == 0 && settings.QUIC.MaxStreamReceiveWindow == 0 && settings.QUIC.InitConnReceiveWindow == 0 && settings.QUIC.MaxConnReceiveWindow == 0 && settings.QUIC.MaxIdleTimeout == "" && settings.QUIC.MaxIncomingStreams == 0 && !settings.QUIC.DisablePathMTUDiscovery {
			settings.QUIC = nil
		}
	}
	if settings.QUIC != nil {
		settings.QUICEnabled = true
	}
	if !settings.QUICEnabled {
		settings.QUIC = nil
	}

	if settings.Listen == "" {
		settings.Listen = ":443"
	}

	return settings
}

func autoGenerateManagedSecrets(settings *Hy2Settings) {
	if settings == nil || settings.Obfs == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(settings.Obfs.Type), "salamander") {
		return
	}
	if settings.Obfs.Salamander == nil {
		settings.Obfs.Salamander = &Hy2ServerSalamander{}
	}
	if strings.TrimSpace(settings.Obfs.Salamander.Password) != "" {
		return
	}
	if generated, err := randomHex(16); err == nil {
		settings.Obfs.Salamander.Password = generated
	}
}

func validateSettings(input Hy2Settings) Hy2SettingsValidation {
	settings := normalizeSettings(input)
	v := Hy2SettingsValidation{Errors: []string{}, Warnings: []string{}}

	if settings.Listen == "" {
		v.Errors = append(v.Errors, "listen is required")
	} else if !validListenAddress(settings.Listen) {
		v.Errors = append(v.Errors, "listen must be host:port or :port with a valid port or port union")
	}

	if settings.TLSEnabled {
		switch settings.TLSMode {
		case "tls":
			if settings.TLS == nil {
				v.Errors = append(v.Errors, "tls mode requires tls section")
			} else {
				if settings.TLS.Cert == "" {
					v.Errors = append(v.Errors, "tls.cert is required in tls mode")
				}
				if settings.TLS.Key == "" {
					v.Errors = append(v.Errors, "tls.key is required in tls mode")
				}
			}
		case "acme":
			if settings.ACME == nil {
				v.Errors = append(v.Errors, "acme mode requires acme section")
			} else {
				if len(settings.ACME.Domains) == 0 {
					v.Errors = append(v.Errors, "acme.domains must contain at least one domain")
				}
				if settings.ACME.Email == "" {
					v.Warnings = append(v.Warnings, "acme.email is empty")
				}
			}
		case "conflict":
			v.Errors = append(v.Errors, "tls and acme are mutually exclusive")
		default:
			v.Errors = append(v.Errors, "tlsMode must be either tls or acme")
		}
	} else {
		v.Warnings = append(v.Warnings, "managed TLS is disabled")
	}

	switch settings.Auth.Type {
	case "userpass":
		if len(settings.Auth.UserPass) == 0 {
			v.Warnings = append(v.Warnings, "auth.userpass is empty; access is controlled by managed Hysteria users")
		}
	case "password":
		if settings.Auth.Password == "" {
			v.Errors = append(v.Errors, "auth.password is required when auth.type=password")
		}
	case "http":
		if settings.Auth.HTTP == nil || !isValidAbsURL(settings.Auth.HTTP.URL) {
			v.Errors = append(v.Errors, "auth.http.url must be a valid absolute URL")
		}
	case "command":
		if strings.TrimSpace(settings.Auth.Command) == "" {
			v.Errors = append(v.Errors, "auth.command is required when auth.type=command")
		}
	default:
		v.Errors = append(v.Errors, "auth.type must be userpass, password, http, or command")
	}

	if settings.Obfs != nil {
		if settings.Obfs.Type != "salamander" {
			v.Errors = append(v.Errors, "obfs.type must be salamander")
		} else if settings.Obfs.Salamander == nil || strings.TrimSpace(settings.Obfs.Salamander.Password) == "" {
			v.Warnings = append(v.Warnings, "obfs.salamander.password is empty and will be generated on save")
		}
	}

	if settings.Obfs != nil && settings.Masquerade != nil {
		v.Errors = append(v.Errors, "obfs and masquerade cannot be enabled together in managed mode")
	}

	if settings.Masquerade != nil {
		if settings.Masquerade.ListenHTTP != "" && !validListenAddress(settings.Masquerade.ListenHTTP) {
			v.Errors = append(v.Errors, "masquerade.listenHTTP must be host:port or :port")
		}
		if settings.Masquerade.ListenHTTPS != "" && !validListenAddress(settings.Masquerade.ListenHTTPS) {
			v.Errors = append(v.Errors, "masquerade.listenHTTPS must be host:port or :port")
		}
		switch settings.Masquerade.Type {
		case "file":
			if settings.Masquerade.File == nil || strings.TrimSpace(settings.Masquerade.File.Dir) == "" {
				v.Errors = append(v.Errors, "masquerade.file.dir is required when masquerade.type=file")
			}
		case "proxy":
			if settings.Masquerade.Proxy == nil || !isValidAbsURL(settings.Masquerade.Proxy.URL) {
				v.Errors = append(v.Errors, "masquerade.proxy.url must be a valid absolute URL when masquerade.type=proxy")
			}
		case "string":
			if settings.Masquerade.String == nil {
				v.Errors = append(v.Errors, "masquerade.string block is required when masquerade.type=string")
			} else if settings.Masquerade.String.StatusCode != 0 && (settings.Masquerade.String.StatusCode < 100 || settings.Masquerade.String.StatusCode > 999) {
				v.Errors = append(v.Errors, "masquerade.string.statusCode must be between 100 and 999")
			}
		case "":
			v.Errors = append(v.Errors, "masquerade.type is required when masquerade section is present")
		default:
			v.Errors = append(v.Errors, "masquerade.type must be file, proxy, or string")
		}
	}

	if settings.Bandwidth != nil {
		if settings.Bandwidth.Up != "" && !validBandwidthValue(settings.Bandwidth.Up) {
			v.Errors = append(v.Errors, "bandwidth.up must be a positive value (for example, 100 or 100 mbps)")
		}
		if settings.Bandwidth.Down != "" && !validBandwidthValue(settings.Bandwidth.Down) {
			v.Errors = append(v.Errors, "bandwidth.down must be a positive value (for example, 200 or 200 mbps)")
		}
	}
	if settings.IgnoreClientBandwidth && settings.Bandwidth == nil {
		v.Warnings = append(v.Warnings, "ignoreClientBandwidth is enabled while bandwidth is empty")
	}
	if settings.UDPIdleTimeout != "" {
		if _, err := time.ParseDuration(settings.UDPIdleTimeout); err != nil {
			v.Errors = append(v.Errors, "udpIdleTimeout must be a valid duration (for example, 30s or 2m)")
		}
	}
	if settings.DisableUDP && settings.UDPIdleTimeout != "" {
		v.Warnings = append(v.Warnings, "udpIdleTimeout is ignored when disableUDP is enabled")
	}
	if settings.QUICEnabled {
		if settings.QUIC == nil {
			v.Errors = append(v.Errors, "quic settings are enabled but quic section is empty")
		} else {
			hasCustomValue := false
			if settings.QUIC.InitStreamReceiveWindow > 0 {
				hasCustomValue = true
			}
			if settings.QUIC.MaxStreamReceiveWindow > 0 {
				hasCustomValue = true
			}
			if settings.QUIC.InitConnReceiveWindow > 0 {
				hasCustomValue = true
			}
			if settings.QUIC.MaxConnReceiveWindow > 0 {
				hasCustomValue = true
			}
			if settings.QUIC.MaxIncomingStreams > 0 {
				hasCustomValue = true
			}
			if settings.QUIC.MaxIdleTimeout != "" {
				if _, err := time.ParseDuration(settings.QUIC.MaxIdleTimeout); err != nil {
					v.Errors = append(v.Errors, "quic.maxIdleTimeout must be a valid duration (for example, 30s or 2m)")
				} else {
					hasCustomValue = true
				}
			}
			if settings.QUIC.DisablePathMTUDiscovery {
				hasCustomValue = true
			}
			if !hasCustomValue {
				v.Errors = append(v.Errors, "quic settings are enabled but no custom values are provided")
			}
		}
	}

	v.Valid = len(v.Errors) == 0
	return v
}

func buildSettingsMap(settings Hy2Settings) map[string]any {
	settings = normalizeSettings(settings)
	out := map[string]any{}
	if settings.Listen != "" {
		out["listen"] = settings.Listen
	}
	if settings.TLSEnabled {
		if settings.TLSMode == "tls" {
			if m := buildServerTLSMap(settings.TLS); len(m) > 0 {
				out["tls"] = m
			}
		}
		if settings.TLSMode == "acme" {
			if m := buildServerACMEMap(settings.ACME); len(m) > 0 {
				out["acme"] = m
			}
		}
	}
	if m := buildServerAuthMap(settings.Auth); len(m) > 0 {
		out["auth"] = m
	}
	if m := buildServerObfsMap(settings.Obfs); len(m) > 0 {
		out["obfs"] = m
	}
	if m := buildServerMasqueradeMap(settings.Masquerade); len(m) > 0 {
		out["masquerade"] = m
	}
	if m := buildServerBandwidthMap(settings.Bandwidth); len(m) > 0 {
		out["bandwidth"] = m
	}
	if settings.ClientTLSInsecure {
		out["clientTLSInsecure"] = true
	}
	if settings.IgnoreClientBandwidth {
		out["ignoreClientBandwidth"] = true
	}
	if settings.SpeedTest {
		out["speedTest"] = true
	}
	if settings.DisableUDP {
		out["disableUDP"] = true
	}
	if settings.UDPIdleTimeout != "" {
		out["udpIdleTimeout"] = settings.UDPIdleTimeout
	}
	if settings.QUICEnabled {
		if m := buildServerQUICMap(settings.QUIC); len(m) > 0 {
			out["quic"] = m
		}
	}

	normalized, ok := normalizeYAMLValue(out).(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return normalized
}

func buildServerTLSMap(cfg *Hy2ServerTLS) map[string]any {
	if cfg == nil {
		return nil
	}
	out := map[string]any{}
	if cfg.Cert != "" {
		out["cert"] = cfg.Cert
	}
	if cfg.Key != "" {
		out["key"] = cfg.Key
	}
	return out
}

func buildServerACMEMap(cfg *Hy2ServerACME) map[string]any {
	if cfg == nil {
		return nil
	}
	out := map[string]any{}
	if domains := trimStringSlice(cfg.Domains); len(domains) > 0 {
		out["domains"] = domains
	}
	if cfg.Email != "" {
		out["email"] = cfg.Email
	}
	return out
}

func buildServerBandwidthMap(cfg *Hy2ServerBandwidth) map[string]any {
	if cfg == nil {
		return nil
	}
	out := map[string]any{}
	if strings.TrimSpace(cfg.Up) != "" {
		out["up"] = strings.TrimSpace(cfg.Up)
	}
	if strings.TrimSpace(cfg.Down) != "" {
		out["down"] = strings.TrimSpace(cfg.Down)
	}
	return out
}
func buildServerQUICMap(cfg *Hy2ServerQUIC) map[string]any {
	if cfg == nil {
		return nil
	}
	out := map[string]any{}
	if cfg.InitStreamReceiveWindow > 0 {
		out["initStreamReceiveWindow"] = cfg.InitStreamReceiveWindow
	}
	if cfg.MaxStreamReceiveWindow > 0 {
		out["maxStreamReceiveWindow"] = cfg.MaxStreamReceiveWindow
	}
	if cfg.InitConnReceiveWindow > 0 {
		out["initConnReceiveWindow"] = cfg.InitConnReceiveWindow
	}
	if cfg.MaxConnReceiveWindow > 0 {
		out["maxConnReceiveWindow"] = cfg.MaxConnReceiveWindow
	}
	if strings.TrimSpace(cfg.MaxIdleTimeout) != "" {
		out["maxIdleTimeout"] = strings.TrimSpace(cfg.MaxIdleTimeout)
	}
	if cfg.MaxIncomingStreams > 0 {
		out["maxIncomingStreams"] = cfg.MaxIncomingStreams
	}
	if cfg.DisablePathMTUDiscovery {
		out["disablePathMTUDiscovery"] = true
	}
	return out
}

func buildServerAuthMap(cfg Hy2ServerAuth) map[string]any {
	auth := normalizeSettings(Hy2Settings{Auth: cfg}).Auth
	if auth.Type == "" {
		return nil
	}
	out := map[string]any{"type": auth.Type}
	switch auth.Type {
	case "userpass":
		userPass := map[string]any{}
		for username, password := range auth.UserPass {
			userPass[strings.TrimSpace(username)] = strings.TrimSpace(password)
		}
		out["userpass"] = userPass
	case "password":
		out["password"] = auth.Password
	case "http":
		hm := map[string]any{}
		if auth.HTTP != nil {
			if auth.HTTP.URL != "" {
				hm["url"] = auth.HTTP.URL
			}
			if auth.HTTP.Insecure {
				hm["insecure"] = true
			}
		}
		out["http"] = hm
	case "command":
		out["command"] = strings.TrimSpace(auth.Command)
	}
	return out
}

func buildServerObfsMap(cfg *Hy2ServerObfs) map[string]any {
	if cfg == nil || cfg.Type == "" {
		return nil
	}
	if !strings.EqualFold(cfg.Type, "salamander") {
		return nil
	}
	password := ""
	if cfg.Salamander != nil {
		password = cfg.Salamander.Password
	}
	return map[string]any{"type": "salamander", "salamander": map[string]any{"password": password}}
}

func buildServerMasqueradeMap(cfg *Hy2ServerMasquerade) map[string]any {
	if cfg == nil {
		return nil
	}
	out := map[string]any{}
	if cfg.Type != "" {
		out["type"] = cfg.Type
	}
	if cfg.File != nil && cfg.File.Dir != "" {
		out["file"] = map[string]any{"dir": cfg.File.Dir}
	}
	if cfg.Proxy != nil {
		proxy := map[string]any{}
		if cfg.Proxy.URL != "" {
			proxy["url"] = cfg.Proxy.URL
		}
		if cfg.Proxy.RewriteHost {
			proxy["rewriteHost"] = true
		}
		if cfg.Proxy.Insecure {
			proxy["insecure"] = true
		}
		if len(proxy) > 0 {
			out["proxy"] = proxy
		}
	}
	if cfg.String != nil {
		strMap := map[string]any{}
		if cfg.String.Content != "" {
			strMap["content"] = cfg.String.Content
		}
		if len(cfg.String.Headers) > 0 {
			strMap["headers"] = cfg.String.Headers
		}
		if cfg.String.StatusCode > 0 {
			strMap["statusCode"] = cfg.String.StatusCode
		}
		if len(strMap) > 0 {
			out["string"] = strMap
		}
	}
	if cfg.ListenHTTP != "" {
		out["listenHTTP"] = cfg.ListenHTTP
	}
	if cfg.ListenHTTPS != "" {
		out["listenHTTPS"] = cfg.ListenHTTPS
	}
	if cfg.ForceHTTPS {
		out["forceHTTPS"] = true
	}
	return out
}

func normalizeClientProfile(input Hy2ClientProfile) Hy2ClientProfile {
	profile := input
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Server = strings.TrimSpace(profile.Server)
	profile.Auth = strings.TrimSpace(profile.Auth)
	profile.AuthType = strings.ToLower(strings.TrimSpace(profile.AuthType))
	profile.TLS.SNI = strings.TrimSpace(profile.TLS.SNI)
	profile.TLS.PinSHA256 = strings.TrimSpace(profile.TLS.PinSHA256)
	profile.TLS.CA = strings.TrimSpace(profile.TLS.CA)
	profile.TLS.ClientCertificate = strings.TrimSpace(profile.TLS.ClientCertificate)
	profile.TLS.ClientKey = strings.TrimSpace(profile.TLS.ClientKey)

	if profile.Obfs != nil {
		profile.Obfs.Type = strings.ToLower(strings.TrimSpace(profile.Obfs.Type))
		if profile.Obfs.Salamander != nil {
			profile.Obfs.Salamander.Password = strings.TrimSpace(profile.Obfs.Salamander.Password)
		}
		if profile.Obfs.Type == "" {
			profile.Obfs = nil
		}
	}

	if profile.QUIC != nil {
		profile.QUIC.InitStreamReceiveWindow = positiveIntOrZero(profile.QUIC.InitStreamReceiveWindow)
		profile.QUIC.MaxStreamReceiveWindow = positiveIntOrZero(profile.QUIC.MaxStreamReceiveWindow)
		profile.QUIC.InitConnReceiveWindow = positiveIntOrZero(profile.QUIC.InitConnReceiveWindow)
		profile.QUIC.MaxConnReceiveWindow = positiveIntOrZero(profile.QUIC.MaxConnReceiveWindow)
		profile.QUIC.MaxIdleTimeout = strings.TrimSpace(profile.QUIC.MaxIdleTimeout)
		if profile.QUIC.InitStreamReceiveWindow == 0 && profile.QUIC.MaxStreamReceiveWindow == 0 && profile.QUIC.InitConnReceiveWindow == 0 && profile.QUIC.MaxConnReceiveWindow == 0 && profile.QUIC.MaxIdleTimeout == "" && !profile.QUIC.DisablePathMTUDiscovery {
			profile.QUIC = nil
		}
	}

	if profile.Bandwidth != nil {
		profile.Bandwidth.Up = normalizeBandwidthInput(profile.Bandwidth.Up)
		profile.Bandwidth.Down = normalizeBandwidthInput(profile.Bandwidth.Down)
		if profile.Bandwidth.Up == "" && profile.Bandwidth.Down == "" {
			profile.Bandwidth = nil
		}
	}

	if profile.Socks5 != nil {
		profile.Socks5.Listen = strings.TrimSpace(profile.Socks5.Listen)
		profile.Socks5.Username = strings.TrimSpace(profile.Socks5.Username)
		profile.Socks5.Password = strings.TrimSpace(profile.Socks5.Password)
		if profile.Socks5.Listen == "" && profile.Socks5.Username == "" && profile.Socks5.Password == "" && !profile.Socks5.DisableUDP {
			profile.Socks5 = nil
		}
	}

	if profile.HTTP != nil {
		profile.HTTP.Listen = strings.TrimSpace(profile.HTTP.Listen)
		profile.HTTP.Username = strings.TrimSpace(profile.HTTP.Username)
		profile.HTTP.Password = strings.TrimSpace(profile.HTTP.Password)
		profile.HTTP.Realm = strings.TrimSpace(profile.HTTP.Realm)
		if profile.HTTP.Listen == "" && profile.HTTP.Username == "" && profile.HTTP.Password == "" && profile.HTTP.Realm == "" {
			profile.HTTP = nil
		}
	}

	return profile
}

type hy2ResolvedServerMode string

const (
	hy2ResolvedServerPlain hy2ResolvedServerMode = "plain"
	hy2ResolvedServerURI   hy2ResolvedServerMode = "uri"
)

type hy2ResolvedClientProfile struct {
	Name       string
	ServerMode hy2ResolvedServerMode
	Server     string
	Host       string
	PortUnion  string
	Auth       string
	AuthType   string
	TLS        Hy2ClientTLS
	Obfs       *Hy2ClientObfs
	QUIC       *Hy2ClientQUIC
	Bandwidth  *Hy2ClientBandwidth
	Socks5     *Hy2ClientSocks5Mode
	HTTP       *Hy2ClientHTTPMode
}

func validateClientProfile(profile Hy2ClientProfile) Hy2ClientValidation {
	_, validation := resolveClientProfile(profile)
	return validation
}

func resolveClientProfile(profile Hy2ClientProfile) (hy2ResolvedClientProfile, Hy2ClientValidation) {
	normalized := normalizeClientProfile(profile)
	v := Hy2ClientValidation{Errors: []string{}, Warnings: []string{}}
	resolved := hy2ResolvedClientProfile{
		Name:      normalized.Name,
		Server:    normalized.Server,
		AuthType:  normalized.AuthType,
		TLS:       normalized.TLS,
		Obfs:      normalized.Obfs,
		QUIC:      normalized.QUIC,
		Bandwidth: normalized.Bandwidth,
		Socks5:    normalized.Socks5,
		HTTP:      normalized.HTTP,
	}


	if isHy2ServerURI(normalized.Server) {
		uriResolved, err := parseClientServerURI(normalized.Server)
		if err != nil {
			v.Errors = append(v.Errors, "profile.server URI is invalid: "+strings.TrimSpace(err.Error()))
		} else {
			resolved.ServerMode = hy2ResolvedServerURI
			resolved.Server = uriResolved.Server
			resolved.Host = uriResolved.Host
			resolved.PortUnion = uriResolved.PortUnion
			resolved.Auth = uriResolved.Auth
			resolved.TLS.SNI = uriResolved.TLS.SNI
			resolved.TLS.Insecure = uriResolved.TLS.Insecure
			resolved.TLS.PinSHA256 = uriResolved.TLS.PinSHA256
			resolved.Obfs = uriResolved.Obfs
		}

		if strings.TrimSpace(normalized.Auth) != "" {
			v.Errors = append(v.Errors, "profile.auth must be omitted when profile.server is a hysteria2:// URI")
		}
		if strings.TrimSpace(normalized.TLS.SNI) != "" || normalized.TLS.Insecure || strings.TrimSpace(normalized.TLS.PinSHA256) != "" {
			v.Errors = append(v.Errors, "profile.tls.sni/insecure/pinSHA256 must be omitted when profile.server is a hysteria2:// URI")
		}
		if normalized.Obfs != nil {
			v.Errors = append(v.Errors, "profile.obfs must be omitted when profile.server is a hysteria2:// URI")
		}
	} else {
		host, ports := splitServerForClient(normalized.Server)
		if host == "" {
			v.Errors = append(v.Errors, "profile.server is required")
		} else {
			resolved.Host = host
			resolved.PortUnion = ports
			resolved.Server = joinServerValue(host, ports)
			resolved.ServerMode = hy2ResolvedServerPlain
		}
		if ports != "" && !validPortUnion(ports) {
			v.Errors = append(v.Errors, "profile.server port section must be a valid port union")
		}
		resolved.Auth = normalized.Auth
	}

	if resolved.AuthType == "" {
		if strings.Contains(strings.TrimSpace(resolved.Auth), ":") {
			resolved.AuthType = "userpass"
		} else {
			resolved.AuthType = "password"
		}
	}

	authForValidation := strings.TrimSpace(resolved.Auth)
	switch resolved.AuthType {
	case "userpass":
		parts := strings.SplitN(authForValidation, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			v.Errors = append(v.Errors, "profile.auth must use username:password format when authType=userpass")
		}
	case "password", "":
		if authForValidation == "" {
			v.Errors = append(v.Errors, "profile.auth is required")
		}
	default:
		v.Errors = append(v.Errors, "profile.authType must be userpass or password")
	}

	if resolved.TLS.SNI != "" && NormalizeHost(resolved.TLS.SNI) == "" {
		v.Errors = append(v.Errors, "profile.tls.sni must be a valid host")
	}
	if resolved.TLS.PinSHA256 != "" && !isValidPinSHA256(resolved.TLS.PinSHA256) {
		v.Errors = append(v.Errors, "profile.tls.pinSHA256 must be a valid SHA-256 certificate fingerprint")
	}
	if (resolved.TLS.ClientCertificate == "") != (resolved.TLS.ClientKey == "") {
		v.Errors = append(v.Errors, "profile.tls.clientCertificate and profile.tls.clientKey must be set together")
	}

	if resolved.Obfs != nil {
		if resolved.Obfs.Type != "salamander" {
			v.Errors = append(v.Errors, "profile.obfs.type must be salamander")
		} else if resolved.Obfs.Salamander == nil || strings.TrimSpace(resolved.Obfs.Salamander.Password) == "" {
			v.Errors = append(v.Errors, "profile.obfs.salamander.password is required when obfs.type=salamander")
		}
	}

	if resolved.Bandwidth != nil {
		if resolved.Bandwidth.Up != "" && !validBandwidthValue(resolved.Bandwidth.Up) {
			v.Errors = append(v.Errors, "profile.bandwidth.up must be a positive value (for example, 100 or 100 mbps)")
		}
		if resolved.Bandwidth.Down != "" && !validBandwidthValue(resolved.Bandwidth.Down) {
			v.Errors = append(v.Errors, "profile.bandwidth.down must be a positive value (for example, 200 or 200 mbps)")
		}
	}

	if resolved.QUIC != nil {
		hasCustomValue := false
		if resolved.QUIC.InitStreamReceiveWindow > 0 {
			hasCustomValue = true
		}
		if resolved.QUIC.MaxStreamReceiveWindow > 0 {
			hasCustomValue = true
		}
		if resolved.QUIC.InitConnReceiveWindow > 0 {
			hasCustomValue = true
		}
		if resolved.QUIC.MaxConnReceiveWindow > 0 {
			hasCustomValue = true
		}
		if resolved.QUIC.MaxIdleTimeout != "" {
			if _, err := time.ParseDuration(resolved.QUIC.MaxIdleTimeout); err != nil {
				v.Errors = append(v.Errors, "profile.quic.maxIdleTimeout must be a valid duration")
			} else {
				hasCustomValue = true
			}
		}
		if resolved.QUIC.DisablePathMTUDiscovery {
			hasCustomValue = true
		}
		if !hasCustomValue {
			v.Errors = append(v.Errors, "profile.quic is present but no valid values are set")
		}
	}

	if resolved.Socks5 != nil {
		if !validListenAddress(resolved.Socks5.Listen) {
			v.Errors = append(v.Errors, "profile.socks5.listen must be a valid host:port or :port")
		}
		if (resolved.Socks5.Username == "") != (resolved.Socks5.Password == "") {
			v.Errors = append(v.Errors, "profile.socks5.username and profile.socks5.password must be set together")
		}
	}
	if resolved.HTTP != nil {
		if !validListenAddress(resolved.HTTP.Listen) {
			v.Errors = append(v.Errors, "profile.http.listen must be a valid host:port or :port")
		}
		if (resolved.HTTP.Username == "") != (resolved.HTTP.Password == "") {
			v.Errors = append(v.Errors, "profile.http.username and profile.http.password must be set together")
		}
	}
	if resolved.Socks5 == nil && resolved.HTTP == nil {
		v.Errors = append(v.Errors, "at least one client mode must be configured (socks5 or http)")
	}

	v.Valid = len(v.Errors) == 0
	return resolved, v
}

func isHy2ServerURI(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(value, "hysteria2://") || strings.HasPrefix(value, "hy2://")
}

func joinServerValue(host string, ports string) string {
	server := strings.TrimSpace(host)
	if strings.TrimSpace(ports) != "" {
		server = server + ":" + strings.TrimSpace(ports)
	}
	return ensureBracketedIPv6(server)
}

func parseClientServerURI(raw string) (hy2ResolvedClientProfile, error) {
	value := strings.TrimSpace(raw)
	u, err := url.Parse(value)
	if err != nil {
		return hy2ResolvedClientProfile{}, err
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "hysteria2" && scheme != "hy2" {
		return hy2ResolvedClientProfile{}, fmt.Errorf("unsupported URI scheme")
	}

	host, ports := splitServerForClient(strings.TrimSpace(u.Host))
	if strings.TrimSpace(host) == "" {
		return hy2ResolvedClientProfile{}, fmt.Errorf("missing host")
	}
	u.Scheme = "hysteria2"
	u.Host = joinServerValue(host, ports)
	u.Path = "/"
	u.Fragment = ""

	q := u.Query()
	tls := Hy2ClientTLS{}
	if sni := strings.TrimSpace(q.Get("sni")); sni != "" {
		tls.SNI = sni
	}
	if insecureRaw := strings.TrimSpace(q.Get("insecure")); insecureRaw != "" {
		insecure, parseErr := parseURIBool(insecureRaw)
		if parseErr != nil {
			return hy2ResolvedClientProfile{}, fmt.Errorf("invalid insecure value: %w", parseErr)
		}
		tls.Insecure = insecure
	}
	if pin := strings.TrimSpace(q.Get("pinSHA256")); pin != "" {
		tls.PinSHA256 = pin
	}

	var obfs *Hy2ClientObfs
	if obfsType := strings.ToLower(strings.TrimSpace(q.Get("obfs"))); obfsType != "" {
		if obfsType != "salamander" {
			return hy2ResolvedClientProfile{}, fmt.Errorf("unsupported URI obfs type")
		}
		password := strings.TrimSpace(q.Get("obfs-password"))
		if password == "" {
			return hy2ResolvedClientProfile{}, fmt.Errorf("URI obfs-password is required when obfs=salamander")
		}
		obfs = &Hy2ClientObfs{Type: "salamander", Salamander: &Hy2ClientSalamander{Password: password}}
	}

	u.RawQuery = q.Encode()

	auth := ""
	if u.User != nil {
		user := strings.TrimSpace(u.User.Username())
		if pass, hasPass := u.User.Password(); hasPass {
			auth = user + ":" + pass
		} else {
			auth = user
		}
	}

	return hy2ResolvedClientProfile{
		ServerMode: hy2ResolvedServerURI,
		Server:     u.String(),
		Host:       host,
		PortUnion:  ports,
		Auth:       auth,
		TLS:        tls,
		Obfs:       obfs,
	}, nil
}

func parseURIBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("expected 0/1 or boolean")
	}
}

func buildClientURI(profile Hy2ClientProfile) (string, string) {
	resolved, validation := resolveClientProfile(profile)
	if !validation.Valid {
		return "", ""
	}

	if resolved.ServerMode == hy2ResolvedServerURI {
		u, err := url.Parse(resolved.Server)
		if err != nil {
			return "", ""
		}
		u.Scheme = "hysteria2"
		u.Path = "/"
		if resolved.Name != "" {
			u.Fragment = resolved.Name
		}
		uri := u.String()
		return uri, strings.Replace(uri, "hysteria2://", "hy2://", 1)
	}

	u := url.URL{Scheme: "hysteria2", Host: joinServerValue(resolved.Host, resolved.PortUnion), Path: "/"}
	if strings.TrimSpace(resolved.Auth) != "" {
		u.User = buildURIUserInfo(resolved.Auth)
	}

	query := url.Values{}
	if resolved.TLS.SNI != "" {
		query.Set("sni", resolved.TLS.SNI)
	}
	if resolved.TLS.Insecure {
		query.Set("insecure", "1")
	}
	if pin := normalizeCertHash(resolved.TLS.PinSHA256); pin != "" {
		query.Set("pinSHA256", pin)
	}
	if resolved.Obfs != nil && strings.EqualFold(resolved.Obfs.Type, "salamander") {
		query.Set("obfs", "salamander")
		if resolved.Obfs.Salamander != nil && strings.TrimSpace(resolved.Obfs.Salamander.Password) != "" {
			query.Set("obfs-password", strings.TrimSpace(resolved.Obfs.Salamander.Password))
		}
	}
	u.RawQuery = query.Encode()
	if resolved.Name != "" {
		u.Fragment = resolved.Name
	}

	uri := u.String()
	return uri, strings.Replace(uri, "hysteria2://", "hy2://", 1)
}

func buildClientYAML(profile Hy2ClientProfile) (string, error) {
	resolved, validation := resolveClientProfile(profile)
	if !validation.Valid {
		return "", errors.New(strings.Join(validation.Errors, "; "))
	}

	cfg := map[string]any{"server": resolved.Server}
	if resolved.ServerMode == hy2ResolvedServerPlain {
		cfg["auth"] = resolved.Auth
		if tlsMap := buildClientTLSMap(resolved.TLS, true); len(tlsMap) > 0 {
			cfg["tls"] = tlsMap
		}
		if resolved.Obfs != nil {
			obfs := map[string]any{"type": resolved.Obfs.Type}
			if resolved.Obfs.Salamander != nil {
				obfs["salamander"] = map[string]any{"password": resolved.Obfs.Salamander.Password}
			}
			cfg["obfs"] = obfs
		}
	} else if tlsMap := buildClientTLSMap(resolved.TLS, false); len(tlsMap) > 0 {
		cfg["tls"] = tlsMap
	}

	if resolved.Bandwidth != nil {
		bandwidth := map[string]any{}
		if resolved.Bandwidth.Up != "" {
			bandwidth["up"] = resolved.Bandwidth.Up
		}
		if resolved.Bandwidth.Down != "" {
			bandwidth["down"] = resolved.Bandwidth.Down
		}
		if len(bandwidth) > 0 {
			cfg["bandwidth"] = bandwidth
		}
	}

	if resolved.QUIC != nil {
		quic := map[string]any{}
		if resolved.QUIC.InitStreamReceiveWindow > 0 {
			quic["initStreamReceiveWindow"] = resolved.QUIC.InitStreamReceiveWindow
		}
		if resolved.QUIC.MaxStreamReceiveWindow > 0 {
			quic["maxStreamReceiveWindow"] = resolved.QUIC.MaxStreamReceiveWindow
		}
		if resolved.QUIC.InitConnReceiveWindow > 0 {
			quic["initConnReceiveWindow"] = resolved.QUIC.InitConnReceiveWindow
		}
		if resolved.QUIC.MaxConnReceiveWindow > 0 {
			quic["maxConnReceiveWindow"] = resolved.QUIC.MaxConnReceiveWindow
		}
		if resolved.QUIC.MaxIdleTimeout != "" {
			quic["maxIdleTimeout"] = resolved.QUIC.MaxIdleTimeout
		}
		if resolved.QUIC.DisablePathMTUDiscovery {
			quic["disablePathMTUDiscovery"] = true
		}
		if len(quic) > 0 {
			cfg["quic"] = quic
		}
	}

	if resolved.Socks5 != nil {
		socks5 := map[string]any{"listen": resolved.Socks5.Listen}
		if resolved.Socks5.Username != "" {
			socks5["username"] = resolved.Socks5.Username
			socks5["password"] = resolved.Socks5.Password
		}
		if resolved.Socks5.DisableUDP {
			socks5["disableUDP"] = true
		}
		cfg["socks5"] = socks5
	}
	if resolved.HTTP != nil {
		httpMap := map[string]any{"listen": resolved.HTTP.Listen}
		if resolved.HTTP.Username != "" {
			httpMap["username"] = resolved.HTTP.Username
			httpMap["password"] = resolved.HTTP.Password
		}
		if resolved.HTTP.Realm != "" {
			httpMap["realm"] = resolved.HTTP.Realm
		}
		cfg["http"] = httpMap
	}
	if resolved.Socks5 == nil && resolved.HTTP == nil {
		return "", errors.New("at least one client mode must be configured")
	}

	normalized, ok := normalizeYAMLValue(cfg).(map[string]any)
	if !ok {
		return "", errors.New("failed to render client config")
	}
	return marshalYAMLMap(normalized)
}

func buildClientTLSMap(tls Hy2ClientTLS, includeConnectionFields bool) map[string]any {
	out := map[string]any{}
	if includeConnectionFields {
		if tls.SNI != "" {
			out["sni"] = tls.SNI
		}
		if tls.Insecure {
			out["insecure"] = true
		}
		if tls.PinSHA256 != "" {
			out["pinSHA256"] = tls.PinSHA256
		}
	}
	if tls.CA != "" {
		out["ca"] = tls.CA
	}
	if tls.ClientCertificate != "" {
		out["clientCertificate"] = tls.ClientCertificate
	}
	if tls.ClientKey != "" {
		out["clientKey"] = tls.ClientKey
	}
	return out
}

func mergeKnownValues(current any, desired any, schema *schemaNode) any {
	if schema == nil {
		return desired
	}
	if schema.AnyMap {
		return desired
	}

	if desiredMap, ok := toStringAnyMap(desired); ok {
		currentMap, _ := toStringAnyMap(current)
		out := make(map[string]any)
		for k, v := range currentMap {
			if schema.Fields == nil {
				out[k] = v
				continue
			}
			if _, known := schema.Fields[k]; !known {
				out[k] = v
			}
		}
		if schema.Fields == nil {
			for k, v := range desiredMap {
				out[k] = normalizeYAMLValue(v)
			}
			return out
		}
		for key, child := range schema.Fields {
			dv, exists := desiredMap[key]
			if !exists {
				continue
			}
			cv, _ := currentMap[key]
			out[key] = mergeKnownValues(cv, dv, child)
		}
		return out
	}

	if desiredSlice, ok := toAnySlice(desired); ok {
		if schema.ListItem == nil {
			return normalizeYAMLValue(desiredSlice)
		}
		currentSlice, _ := toAnySlice(current)
		out := make([]any, 0, len(desiredSlice))
		for i, item := range desiredSlice {
			var currentItem any
			if i < len(currentSlice) {
				currentItem = currentSlice[i]
			}
			out = append(out, mergeKnownValues(currentItem, item, schema.ListItem))
		}
		return out
	}
	return normalizeYAMLValue(desired)
}

func collectUnknown(root map[string]any, schema *schemaNode) []string {
	paths := make([]string, 0, 8)
	collectUnknownFromValue(root, schema, "", &paths)
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	uniq := make([]string, 0, len(paths))
	for _, item := range paths {
		if len(uniq) == 0 || uniq[len(uniq)-1] != item {
			uniq = append(uniq, item)
		}
	}
	return uniq
}

func collectUnknownFromValue(value any, schema *schemaNode, path string, out *[]string) {
	if schema == nil || schema.AnyMap {
		return
	}
	if m, ok := toStringAnyMap(value); ok {
		for key, childValue := range m {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			if schema.Fields == nil {
				*out = append(*out, childPath)
				continue
			}
			childSchema, known := schema.Fields[key]
			if !known {
				*out = append(*out, childPath)
				continue
			}
			collectUnknownFromValue(childValue, childSchema, childPath, out)
		}
		return
	}
	if list, ok := toAnySlice(value); ok {
		if schema.ListItem == nil {
			return
		}
		for i, item := range list {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			collectUnknownFromValue(item, schema.ListItem, childPath, out)
		}
	}
}

func parseYAMLMap(content string) (map[string]any, error) {
	if strings.TrimSpace(content) == "" {
		return map[string]any{}, nil
	}
	var raw any
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		return nil, err
	}
	normalized := normalizeYAMLValue(raw)
	if normalized == nil {
		return map[string]any{}, nil
	}
	m, ok := normalized.(map[string]any)
	if !ok {
		return nil, errors.New("top-level YAML value must be a mapping")
	}
	return m, nil
}

func marshalYAMLMap(root map[string]any) (string, error) {
	if root == nil {
		root = map[string]any{}
	}
	buf, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func normalizeYAMLValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = normalizeYAMLValue(item)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[fmt.Sprintf("%v", key)] = normalizeYAMLValue(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeYAMLValue(item))
		}
		return out

	default:
		return value
	}
}

func toStringAnyMap(value any) (map[string]any, bool) {
	if value == nil {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[fmt.Sprintf("%v", key)] = item
		}
		return out, true
	default:
		return nil, false
	}
}

func toAnySlice(value any) ([]any, bool) {
	if value == nil {
		return nil, false
	}
	switch typed := value.(type) {
	case []any:
		return typed, true

	default:
		return nil, false
	}
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		return fmt.Sprintf("%v", typed)
	default:
		return ""
	}
}

func toBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func toInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func toStringSlice(value any) []string {
	if value == nil {
		return nil
	}
	if typed, ok := toAnySlice(value); ok {
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s := strings.TrimSpace(toString(item))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	s := strings.TrimSpace(toString(value))
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func toStringStringMap(value any) map[string]string {
	if value == nil {
		return nil
	}
	m, ok := toStringAnyMap(value)
	if !ok {
		return nil
	}
	out := make(map[string]string)
	for key, item := range m {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		out[k] = strings.TrimSpace(toString(item))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func trimStringSlice(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	out := make([]string, 0, len(input))
	for _, item := range input {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func trimStringStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string)
	for key, val := range input {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		out[k] = strings.TrimSpace(val)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeListenInput(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, ":") {
		return value
	}
	if validPortUnion(value) {
		return ":" + value
	}
	return value
}

func parseListen(listen string) (string, string, bool) {
	value := strings.TrimSpace(listen)
	if value == "" {
		return "", "", false
	}
	if strings.HasPrefix(value, ":") {
		ports := strings.TrimSpace(strings.TrimPrefix(value, ":"))
		return "", ports, ports != ""
	}
	if strings.HasPrefix(value, "[") {
		idx := strings.LastIndex(value, "]:")
		if idx < 0 {
			return "", "", false
		}
		host := strings.TrimSpace(value[1:idx])
		ports := strings.TrimSpace(value[idx+2:])
		return host, ports, ports != ""
	}
	idx := strings.LastIndex(value, ":")
	if idx <= 0 || idx >= len(value)-1 {
		return "", "", false
	}
	host := strings.TrimSpace(value[:idx])
	ports := strings.TrimSpace(value[idx+1:])
	if ports == "" {
		return "", "", false
	}
	return host, ports, true
}

func validListenAddress(listen string) bool {
	_, ports, ok := parseListen(listen)
	if !ok {
		return false
	}
	return validPortUnion(ports)
}

func validPortUnion(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	segments := strings.Split(value, ",")
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return false
		}
		if strings.Contains(segment, "-") {
			parts := strings.SplitN(segment, "-", 2)
			if len(parts) != 2 {
				return false
			}
			start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 != nil || err2 != nil || !validPort(start) || !validPort(end) || start > end {
				return false
			}
			continue
		}
		port, err := strconv.Atoi(segment)
		if err != nil || !validPort(port) {
			return false
		}
	}
	return true
}

func firstPortFromUnion(value string) int {
	segments := strings.Split(strings.TrimSpace(value), ",")
	if len(segments) == 0 {
		return 0
	}
	first := strings.TrimSpace(segments[0])
	if first == "" {
		return 0
	}
	if strings.Contains(first, "-") {
		parts := strings.SplitN(first, "-", 2)
		if len(parts) != 2 {
			return 0
		}
		port, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		return port
	}
	port, _ := strconv.Atoi(first)
	return port
}

func splitServerForClient(server string) (string, string) {
	value := strings.TrimSpace(server)
	if value == "" {
		return "", ""
	}
	if strings.Contains(value, "://") {
		if u, err := url.Parse(value); err == nil {
			host, ports := splitServerAuthority(strings.TrimSpace(u.Host))
			if host != "" {
				return host, ports
			}
		}
	}
	return splitServerAuthority(value)
}

func splitServerAuthority(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	if strings.HasPrefix(value, "[") {
		idx := strings.LastIndex(value, "]:")
		if idx < 0 {
			return strings.Trim(value, "[]"), ""
		}
		host := strings.TrimSpace(value[:idx+1])
		ports := strings.TrimSpace(value[idx+2:])
		if !validPortUnion(ports) {
			return value, ""
		}
		return host, ports
	}
	if strings.Count(value, ":") == 0 {
		return value, ""
	}
	idx := strings.LastIndex(value, ":")
	if idx <= 0 || idx >= len(value)-1 {
		return value, ""
	}
	host := strings.TrimSpace(value[:idx])
	ports := strings.TrimSpace(value[idx+1:])
	if !validPortUnion(ports) {
		return value, ""
	}
	return host, ports
}

func ensureBracketedIPv6(authority string) string {
	if strings.HasPrefix(authority, "[") {
		return authority
	}
	host, ports := splitServerForClient(authority)
	if strings.Count(host, ":") > 1 {
		if ports != "" {
			return "[" + strings.Trim(host, "[]") + "]:" + ports
		}
		return "[" + strings.Trim(host, "[]") + "]"
	}
	return authority
}

func sanitizePublicHost(raw string) string {
	host := NormalizeHost(raw)
	switch host {
	case "", "0.0.0.0", "::":
		return ""
	default:
		return host
	}
}

func validPort(value int) bool {
	return value >= 1 && value <= 65535
}

func normalizeCertHash(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, ":", "")
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func buildURIUserInfo(auth string) *url.Userinfo {
	credential := strings.TrimSpace(auth)
	if credential == "" {
		return nil
	}
	username, password, hasPassword := strings.Cut(credential, ":")
	if hasPassword {
		return url.UserPassword(username, password)
	}
	return url.User(credential)
}

func isValidPinSHA256(raw string) bool {
	hash := normalizeCertHash(raw)
	if len(hash) != 64 {
		return false
	}
	for _, ch := range hash {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

func validBandwidthValue(raw string) bool {
	value := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(raw), " ", ""))
	if value == "" {
		return false
	}
	idx := 0
	dotSeen := false
	for idx < len(value) {
		ch := value[idx]
		if ch >= '0' && ch <= '9' {
			idx++
			continue
		}
		if ch == '.' && !dotSeen {
			dotSeen = true
			idx++
			continue
		}
		break
	}
	if idx == 0 || idx >= len(value) {
		return false
	}
	number := value[:idx]
	unit := value[idx:]
	parsed, err := strconv.ParseFloat(number, 64)
	if err != nil || parsed <= 0 {
		return false
	}
	switch unit {
	case "bps", "kbps", "mbps", "gbps", "tbps":
		return true
	default:
		return false
	}
}

func normalizeBandwidthInput(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	compact := strings.ToLower(strings.ReplaceAll(value, " ", ""))
	idx := 0
	dotSeen := false
	for idx < len(compact) {
		ch := compact[idx]
		if ch >= '0' && ch <= '9' {
			idx++
			continue
		}
		if ch == '.' && !dotSeen {
			dotSeen = true
			idx++
			continue
		}
		break
	}
	if idx == 0 {
		return value
	}

	number := compact[:idx]
	unit := compact[idx:]
	if unit == "" {
		return number + " mbps"
	}

	switch unit {
	case "b", "kb", "mb", "gb", "tb":
		return number + " " + unit + "ps"
	case "bps", "kbps", "mbps", "gbps", "tbps":
		return number + " " + unit
	default:
		return value
	}
}

func isValidAbsURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return parsed.Scheme != "" && parsed.Host != ""
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func positiveIntOrZero(value int) int {
	if value > 0 {
		return value
	}
	return 0
}

func normalizedObfsType(obfs *Hy2ServerObfs) string {
	if obfs == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(obfs.Type))
}

func normalizedMasqueradeType(masquerade *Hy2ServerMasquerade) string {
	if masquerade == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(masquerade.Type))
}

func configSummaryFromSettings(settings Hy2Settings, rawOnlyPathsCount int) Hy2ConfigSummary {
	bandwidthUp := ""
	bandwidthDown := ""
	if settings.Bandwidth != nil {
		bandwidthUp = strings.TrimSpace(settings.Bandwidth.Up)
		bandwidthDown = strings.TrimSpace(settings.Bandwidth.Down)
	}
	return Hy2ConfigSummary{
		Listen:                settings.Listen,
		TLSEnabled:            settings.TLSEnabled,
		TLSMode:               settings.TLSMode,
		AuthType:              strings.ToLower(strings.TrimSpace(settings.Auth.Type)),
		ObfsType:              normalizedObfsType(settings.Obfs),
		MasqueradeType:        normalizedMasqueradeType(settings.Masquerade),
		QUICEnabled:           settings.QUICEnabled,
		BandwidthUp:           bandwidthUp,
		BandwidthDown:         bandwidthDown,
		IgnoreClientBandwidth: settings.IgnoreClientBandwidth,
		SpeedTest:             settings.SpeedTest,
		DisableUDP:            settings.DisableUDP,
		UDPIdleTimeout:        strings.TrimSpace(settings.UDPIdleTimeout),
		RawOnlyPathsCount:     rawOnlyPathsCount,
	}
}
func randomHex(bytesN int) (string, error) {
	if bytesN <= 0 {
		return "", errors.New("bytes count must be positive")
	}
	buf := make([]byte, bytesN)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func buildServerSchema() *schemaNode {
	return &schemaNode{Fields: map[string]*schemaNode{
		"listen": emptySchema,
		"tls": {
			Fields: map[string]*schemaNode{
				"cert": emptySchema,
				"key":  emptySchema,
			},
		},
		"acme": {
			Fields: map[string]*schemaNode{
				"domains": emptySchema,
				"email":   emptySchema,
			},
		},
		"quic": {
			Fields: map[string]*schemaNode{
				"initStreamReceiveWindow": emptySchema,
				"maxStreamReceiveWindow":  emptySchema,
				"initConnReceiveWindow":   emptySchema,
				"maxConnReceiveWindow":    emptySchema,
				"maxIdleTimeout":          emptySchema,
				"maxIncomingStreams":      emptySchema,
				"disablePathMTUDiscovery": emptySchema,
			},
		},
		"auth": {
			Fields: map[string]*schemaNode{
				"type":     emptySchema,
				"password": emptySchema,
				"userpass": {AnyMap: true},
				"command": emptySchema,
				"http": {
					Fields: map[string]*schemaNode{
						"url":      emptySchema,
						"insecure": emptySchema,
					},
				},
			},
		},
		"obfs": {
			Fields: map[string]*schemaNode{
				"type": emptySchema,
				"salamander": {
					Fields: map[string]*schemaNode{
						"password": emptySchema,
					},
				},
			},
		},
		"masquerade": {
			Fields: map[string]*schemaNode{
				"type": emptySchema,
				"file": {
					Fields: map[string]*schemaNode{
						"dir": emptySchema,
					},
				},
				"proxy": {
					Fields: map[string]*schemaNode{
						"url":         emptySchema,
						"rewriteHost": emptySchema,
						"insecure":    emptySchema,
					},
				},
				"string": {
					Fields: map[string]*schemaNode{
						"content":    emptySchema,
						"headers":    {AnyMap: true},
						"statusCode": emptySchema,
					},
				},
				"listenHTTP":  emptySchema,
				"listenHTTPS": emptySchema,
				"forceHTTPS":  emptySchema,
			},
		},
		"bandwidth": {
			Fields: map[string]*schemaNode{
				"up":   emptySchema,
				"down": emptySchema,
			},
		},
		"clientTLSInsecure":     emptySchema,
		"ignoreClientBandwidth": emptySchema,
		"speedTest":             emptySchema,
		"disableUDP":            emptySchema,
		"udpIdleTimeout":        emptySchema,
	}}
}
