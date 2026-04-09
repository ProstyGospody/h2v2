package core

import "time"

type InboundProtocol string

const (
	InboundProtocolVLESS     InboundProtocol = "vless"
	InboundProtocolHysteria2 InboundProtocol = "hysteria2"
)

// currentSchemaVersion is bumped with every structural migration.
const currentSchemaVersion = 13

// rendererVersion is bumped when the config renderer output changes significantly.
const rendererVersion = "2.0.0"

type Server struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	PublicHost          string    `json:"public_host"`
	PanelPublicURL      string    `json:"panel_public_url"`
	SubscriptionBaseURL string    `json:"subscription_base_url"`
	SingBoxBinaryPath   string    `json:"singbox_binary_path"`
	SingBoxConfigPath   string    `json:"singbox_config_path"`
	SingBoxServiceName  string    `json:"singbox_service_name"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type VLESSInboundSettings struct {
	TLSEnabled                 bool     `json:"tls_enabled"`
	TLSServerName              string   `json:"tls_server_name,omitempty"`
	TLSALPN                    []string `json:"tls_alpn,omitempty"`
	TLSCertificatePath         string   `json:"tls_certificate_path,omitempty"`
	TLSKeyPath                 string   `json:"tls_key_path,omitempty"`
	RealityEnabled             bool     `json:"reality_enabled"`
	RealityPublicKey           string   `json:"reality_public_key,omitempty"`
	RealityPrivateKey          string   `json:"reality_private_key,omitempty"`
	RealityShortID             string   `json:"reality_short_id,omitempty"`
	RealityHandshakeServer     string   `json:"reality_handshake_server,omitempty"`
	RealityHandshakeServerPort int      `json:"reality_handshake_server_port,omitempty"`
	Flow                       string   `json:"flow,omitempty"`
	TransportType              string   `json:"transport_type"`
	TransportHost              string   `json:"transport_host,omitempty"`
	TransportPath              string   `json:"transport_path,omitempty"`
	MultiplexEnabled           bool     `json:"multiplex_enabled"`
	MultiplexProtocol          string   `json:"multiplex_protocol,omitempty"`
	MultiplexMaxConnections    int      `json:"multiplex_max_connections,omitempty"`
	MultiplexMinStreams        int      `json:"multiplex_min_streams,omitempty"`
	MultiplexMaxStreams        int      `json:"multiplex_max_streams,omitempty"`
	// Profile references (take precedence over inline settings when set).
	TLSProfileID       string `json:"tls_profile_id,omitempty"`
	RealityProfileID   string `json:"reality_profile_id,omitempty"`
	TransportProfileID string `json:"transport_profile_id,omitempty"`
	MultiplexProfileID string `json:"multiplex_profile_id,omitempty"`
	// PacketEncoding default for user-access artifacts.
	PacketEncodingDefault string `json:"packet_encoding_default,omitempty"`
}

type Hysteria2InboundSettings struct {
	TLSEnabled            bool     `json:"tls_enabled"`
	TLSServerName         string   `json:"tls_server_name,omitempty"`
	TLSALPN               []string `json:"tls_alpn,omitempty"`
	TLSCertificatePath    string   `json:"tls_certificate_path,omitempty"`
	TLSKeyPath            string   `json:"tls_key_path,omitempty"`
	AllowInsecure         bool     `json:"allow_insecure"`
	UpMbps                *int     `json:"up_mbps,omitempty"`
	DownMbps              *int     `json:"down_mbps,omitempty"`
	IgnoreClientBandwidth bool     `json:"ignore_client_bandwidth"`
	ObfsType              string   `json:"obfs_type,omitempty"`
	ObfsPassword          string   `json:"obfs_password,omitempty"`
	MasqueradeJSON        string   `json:"masquerade_json,omitempty"`
	BBRProfile            string   `json:"bbr_profile,omitempty"`
	BrutalDebug           bool     `json:"brutal_debug"`
	// Port hopping: comma-separated ports or ranges, e.g. "443,8000-9000".
	ServerPorts string `json:"server_ports,omitempty"`
	HopInterval int    `json:"hop_interval,omitempty"` // seconds
	// Network constraint: "tcp", "udp", or "" (both).
	Network string `json:"network,omitempty"`
	// BandwidthProfileMode is a user-facing preset: "auto", "poor", "fast".
	BandwidthProfileMode string `json:"bandwidth_profile_mode,omitempty"`
	// Profile references.
	TLSProfileID        string `json:"tls_profile_id,omitempty"`
	MasqueradeProfileID string `json:"masquerade_profile_id,omitempty"`
}

type Inbound struct {
	ID          string                    `json:"id"`
	ServerID    string                    `json:"server_id"`
	Name        string                    `json:"name"`
	Tag         string                    `json:"tag"`
	Protocol    InboundProtocol           `json:"protocol"`
	Listen      string                    `json:"listen"`
	ListenPort  int                       `json:"listen_port"`
	Enabled     bool                      `json:"enabled"`
	TemplateKey string                    `json:"template_key"`
	VLESS       *VLESSInboundSettings     `json:"vless,omitempty"`
	Hysteria2   *Hysteria2InboundSettings `json:"hysteria2,omitempty"`
	// Extended lifecycle metadata.
	Notes       string   `json:"notes,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	SortOrder   int      `json:"sort_order,omitempty"`
	LogProfileID string  `json:"log_profile_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ---------- Policy entities ----------

// Outbound is a first-class sing-box outbound policy stored per server.
type Outbound struct {
	ID                 string    `json:"id"`
	ServerID           string    `json:"server_id"`
	Tag                string    `json:"tag"`
	Type               string    `json:"type"` // direct, block, socks, http, shadowsocks, etc.
	Enabled            bool      `json:"enabled"`
	Priority           int       `json:"priority"`
	SettingsJSON       string    `json:"settings_json,omitempty"` // protocol-specific JSON blob
	HealthcheckEnabled bool      `json:"healthcheck_enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// RouteRule is a first-class sing-box route rule stored per server.
type RouteRule struct {
	ID             string   `json:"id"`
	ServerID       string   `json:"server_id"`
	Enabled        bool     `json:"enabled"`
	Priority       int      `json:"priority"`
	InboundTags    []string `json:"inbound_tags,omitempty"`
	Protocols      []string `json:"protocols,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	DomainSuffixes []string `json:"domain_suffixes,omitempty"`
	DomainKeywords []string `json:"domain_keywords,omitempty"`
	IPCIDRs        []string `json:"ip_cidrs,omitempty"`
	Ports          []int    `json:"ports,omitempty"`
	Network        string   `json:"network,omitempty"` // tcp, udp, or ""
	GeoIPCodes     []string `json:"geoip_codes,omitempty"`
	GeositeCodes   []string `json:"geosite_codes,omitempty"`
	OutboundTag    string   `json:"outbound_tag"`
	Action         string   `json:"action,omitempty"` // route (default), block
	Invert         bool     `json:"invert,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// DNSProfile is a per-server DNS policy profile.
type DNSProfile struct {
	ID           string    `json:"id"`
	ServerID     string    `json:"server_id"`
	Name         string    `json:"name"`
	Enabled      bool      `json:"enabled"`
	Strategy     string    `json:"strategy,omitempty"` // prefer_ipv4, prefer_ipv6, ipv4_only, ipv6_only
	DisableCache bool      `json:"disable_cache"`
	FinalServer  string    `json:"final_server,omitempty"`
	ServersJSON  string    `json:"servers_json,omitempty"` // []map[string]any
	RulesJSON    string    `json:"rules_json,omitempty"`   // []map[string]any
	FakeIPEnabled bool     `json:"fakeip_enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// LogProfile is a per-server logging policy.
type LogProfile struct {
	ID               string    `json:"id"`
	ServerID         string    `json:"server_id"`
	Name             string    `json:"name"`
	Enabled          bool      `json:"enabled"`
	Level            string    `json:"level"` // trace, debug, info, warn, error
	Output           string    `json:"output,omitempty"` // stderr or file path
	Timestamp        bool      `json:"timestamp"`
	AccessLogEnabled bool      `json:"access_log_enabled"`
	DebugMode        bool      `json:"debug_mode"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ---------- Security & transport profiles ----------

// RealityProfile is a reusable Reality security configuration.
type RealityProfile struct {
	ID                  string    `json:"id"`
	ServerID            string    `json:"server_id"`
	Name                string    `json:"name"`
	Enabled             bool      `json:"enabled"`
	ServerName          string    `json:"server_name,omitempty"`
	HandshakeServer     string    `json:"handshake_server"`
	HandshakeServerPort int       `json:"handshake_server_port"`
	PrivateKey          string    `json:"private_key,omitempty"` // never sent to client
	PublicKey           string    `json:"public_key"`
	ShortIDs            []string  `json:"short_ids,omitempty"`
	ShortIDRotationMode string    `json:"short_id_rotation_mode,omitempty"` // manual, auto
	KeyRotationMode     string    `json:"key_rotation_mode,omitempty"`      // manual, auto
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// TransportProfile is a reusable sing-box transport configuration (ws, grpc, http).
type TransportProfile struct {
	ID          string    `json:"id"`
	ServerID    string    `json:"server_id"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Type        string    `json:"type"` // tcp, ws, grpc, http
	Host        string    `json:"host,omitempty"`
	Path        string    `json:"path,omitempty"`
	ServiceName string    `json:"service_name,omitempty"` // grpc service name
	HeadersJSON string    `json:"headers_json,omitempty"` // map[string]string
	IdleTimeout int       `json:"idle_timeout,omitempty"` // seconds
	PingTimeout int       `json:"ping_timeout,omitempty"` // seconds
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MultiplexProfile is a reusable sing-box multiplex configuration.
type MultiplexProfile struct {
	ID             string    `json:"id"`
	ServerID       string    `json:"server_id"`
	Name           string    `json:"name"`
	Enabled        bool      `json:"enabled"`
	Protocol       string    `json:"protocol,omitempty"` // smux, yamux, h2mux
	MaxConnections int       `json:"max_connections,omitempty"`
	MinStreams      int       `json:"min_streams,omitempty"`
	MaxStreams      int       `json:"max_streams,omitempty"`
	Padding        bool      `json:"padding"`
	Brutal         bool      `json:"brutal"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// HY2MasqueradeProfile is a reusable Hysteria2 masquerade configuration.
type HY2MasqueradeProfile struct {
	ID          string    `json:"id"`
	ServerID    string    `json:"server_id"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Type        string    `json:"type"` // off, string, file, proxy
	URL         string    `json:"url,omitempty"`
	RewriteHost bool      `json:"rewrite_host"`
	Directory   string    `json:"directory,omitempty"`
	StatusCode  int       `json:"status_code,omitempty"`
	HeadersJSON string    `json:"headers_json,omitempty"` // map[string]string
	Content     string    `json:"content,omitempty"`      // for type=string
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TLSProfile struct {
	ID              string    `json:"id"`
	ServerID        string    `json:"server_id"`
	Name            string    `json:"name"`
	Enabled         bool      `json:"enabled"`
	ServerName      string    `json:"server_name,omitempty"`
	ALPN            []string  `json:"alpn,omitempty"`
	CertificatePath string    `json:"certificate_path,omitempty"`
	KeyPath         string    `json:"key_path,omitempty"`
	AllowInsecure   bool      `json:"allow_insecure"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ---------- User-facing connection profiles ----------

// ClientProfile defines a user-facing connection mode that abstracts low-level
// protocol settings into a named preset (e.g. "Poor Network", "Port Hopping").
type ClientProfile struct {
	ID           string          `json:"id"`
	ServerID     string          `json:"server_id"`
	Name         string          `json:"name"`
	Protocol     InboundProtocol `json:"protocol"`
	// Mode is the human-readable preset name.
	// For HY2: standard, obfuscated, poor_network, port_hopping
	// For VLESS: standard, multiplex, udp_compat, transport, compat
	Mode         string    `json:"mode"`
	Description  string    `json:"description,omitempty"`
	// SettingsJSON holds mode-specific overrides applied during artifact generation.
	SettingsJSON string    `json:"settings_json,omitempty"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type User struct {
	ID                   string     `json:"id"`
	Username             string     `json:"username"`
	Enabled              bool       `json:"enabled"`
	TrafficLimitBytes    int64      `json:"traffic_limit_bytes"`
	TrafficUsedUpBytes   int64      `json:"traffic_used_up_bytes"`
	TrafficUsedDownBytes int64      `json:"traffic_used_down_bytes"`
	ExpireAt             *time.Time `json:"expire_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type UserAccess struct {
	ID                        string     `json:"id"`
	UserID                    string     `json:"user_id"`
	InboundID                 string     `json:"inbound_id"`
	Enabled                   bool       `json:"enabled"`
	VLESSUUID                 string     `json:"vless_uuid,omitempty"`
	VLESSFlowOverride         string     `json:"vless_flow_override,omitempty"`
	Hysteria2Password         string     `json:"hysteria2_password,omitempty"`
	TrafficLimitBytesOverride *int64     `json:"traffic_limit_bytes_override,omitempty"`
	ExpireAtOverride          *time.Time `json:"expire_at_override,omitempty"`
	// Extended lifecycle fields.
	DisplayName      string     `json:"display_name,omitempty"`
	Description      string     `json:"description,omitempty"`
	CredentialStatus string     `json:"credential_status,omitempty"` // active, revoked
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty"`
	LastClientIP     *string    `json:"last_client_ip,omitempty"`
	ClientProfileID  string     `json:"client_profile_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Subscription struct {
	ID                        string     `json:"id"`
	UserID                    string     `json:"user_id"`
	ProfileName               string     `json:"profile_name"`
	Enabled                   bool       `json:"enabled"`
	PrimaryTokenID            string     `json:"primary_token_id,omitempty"`
	ArtifactVersion           int        `json:"artifact_version"`
	ArtifactsNeedRefresh      bool       `json:"artifacts_need_refresh"`
	LastArtifactRenderedAt    *time.Time `json:"last_artifact_rendered_at,omitempty"`
	LastArtifactRefreshReason *string    `json:"last_artifact_refresh_reason,omitempty"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type SubscriptionToken struct {
	ID             string     `json:"id"`
	SubscriptionID string     `json:"subscription_id"`
	TokenPrefix    string     `json:"token_prefix"`
	IsPrimary      bool       `json:"is_primary"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	LastUsedIP     *string    `json:"last_used_ip,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type IssuedSubscriptionToken struct {
	PlaintextToken string            `json:"plaintext_token"`
	Token          SubscriptionToken `json:"token"`
}

// ApplyStatus values for ConfigRevision.ApplyStatus.
const (
	ApplyStatusSucceeded = "succeeded"
	ApplyStatusFailed    = "failed"
)

type ConfigRevision struct {
	ID                     string     `json:"id"`
	ServerID               string     `json:"server_id"`
	RevisionNo             int        `json:"revision_no"`
	ConfigHash             string     `json:"config_hash"`
	RenderedJSON           string     `json:"rendered_json"`
	CheckOK                bool       `json:"check_ok"`
	CheckError             *string    `json:"check_error,omitempty"`
	AppliedAt              *time.Time `json:"applied_at,omitempty"`
	RollbackFromRevisionID *string    `json:"rollback_from_revision_id,omitempty"`
	// Apply outcome tracking вЂ” set after an Apply attempt.
	ApplyStatus *string `json:"apply_status,omitempty"`
	ApplyError  *string `json:"apply_error,omitempty"`
	// Extended revision metadata.
	SchemaVersion   int       `json:"schema_version"`
	RendererVersion string    `json:"renderer_version,omitempty"`
	CreatedBy       string    `json:"created_by,omitempty"`
	IsCurrent       bool      `json:"is_current"`
	CreatedAt       time.Time `json:"created_at"`
}

// BulkPreviewResult summarises the domain impact of a bulk user deletion.
type BulkPreviewResult struct {
	UserCount               int      `json:"user_count"`
	AccessCount             int      `json:"access_count"`
	AffectedInboundIDs      []string `json:"affected_inbound_ids"`
	AffectedInboundCount    int      `json:"affected_inbounds"`
	AffectedSubscriptions   int      `json:"affected_subscriptions"`
	AffectedArtifacts       int      `json:"affected_artifacts"`
	RuntimeChangeExpected   bool     `json:"runtime_change_expected"`
	ArtifactRefreshExpected bool     `json:"artifact_refresh_expected"`
	RestartRequired         bool     `json:"restart_required"`
}

type UserArtifacts struct {
	UserID                    string     `json:"user_id"`
	SubscriptionID            string     `json:"subscription_id"`
	PrimaryTokenPrefix        string     `json:"primary_token_prefix,omitempty"`
	ArtifactVersion           int        `json:"artifact_version"`
	ArtifactsNeedRefresh      bool       `json:"artifacts_need_refresh"`
	LastArtifactRenderedAt    *time.Time `json:"last_artifact_rendered_at,omitempty"`
	LastArtifactRefreshReason *string    `json:"last_artifact_refresh_reason,omitempty"`
	SubscriptionImportURL     string     `json:"subscription_import_url"`
	SubscriptionProfileURL    string     `json:"subscription_profile_url"`
	SubscriptionURIsURL       string     `json:"subscription_uris_url"`
	SubscriptionQRURL         string     `json:"subscription_qr_url"`
	SubscriptionClashURL      string     `json:"subscription_clash_url"`
	SubscriptionBase64URL     string     `json:"subscription_base64_url"`
	VLESSURIs                 []string   `json:"vless_uris"`
	Hysteria2URIs             []string   `json:"hysteria2_uris"`
	AllURIs                   []string   `json:"all_uris"`
	SingBoxProfileJSON        string     `json:"singbox_profile_json"`
}

type RenderResult struct {
	Server   Server         `json:"server"`
	Revision ConfigRevision `json:"revision"`
}

type ChangeImpact struct {
	AffectedUsers           int      `json:"affected_users"`
	AffectedAccess          int      `json:"affected_access"`
	AffectedInbounds        int      `json:"affected_inbounds"`
	AffectedSubscriptions   int      `json:"affected_subscriptions"`
	AffectedArtifacts       int      `json:"affected_artifacts"`
	RequiresRuntimeApply    bool     `json:"requires_runtime_apply"`
	RequiresArtifactRefresh bool     `json:"requires_artifact_refresh"`
	ServerIDs               []string `json:"server_ids,omitempty"`
	InboundIDs              []string `json:"inbound_ids,omitempty"`
}

type DraftRevisionState struct {
	ServerID          string  `json:"server_id"`
	CurrentRevisionID string  `json:"current_revision_id,omitempty"`
	CurrentRevisionNo int     `json:"current_revision_no,omitempty"`
	DraftRevisionID   string  `json:"draft_revision_id,omitempty"`
	DraftRevisionNo   int     `json:"draft_revision_no,omitempty"`
	PendingChanges    bool    `json:"pending_changes"`
	CheckOK           bool    `json:"check_ok"`
	CheckError        *string `json:"check_error,omitempty"`
	ApplyStatus       *string `json:"apply_status,omitempty"`
	ApplyError        *string `json:"apply_error,omitempty"`
}

type PolicyUsage struct {
	Kind                  string `json:"kind"`
	ID                    string `json:"id"`
	UsedByUsers           int    `json:"used_by_users"`
	UsedByAccess          int    `json:"used_by_access"`
	UsedByInbounds        int    `json:"used_by_inbounds"`
	UsedByRouteRules      int    `json:"used_by_route_rules"`
	UsedByOutbounds       int    `json:"used_by_outbounds"`
	AffectedSubscriptions int    `json:"affected_subscriptions"`
	AffectedArtifacts     int    `json:"affected_artifacts"`
	RequiresRuntimeApply  bool   `json:"requires_runtime_apply"`
	UnsafeDelete          bool   `json:"unsafe_delete"`
}

type BulkMutationResult struct {
	Updated     int                  `json:"updated"`
	Deleted     int                  `json:"deleted"`
	Rotated     int                  `json:"rotated"`
	Regenerated int                  `json:"regenerated"`
	Impact      ChangeImpact         `json:"impact"`
	Drafts      []DraftRevisionState `json:"drafts,omitempty"`
}

type SubscriptionContent struct {
	StatusCode  int
	ContentType string
	FileName    string
	ETag        string
	Body        []byte
	Headers     map[string]string
}

type TokenContext struct {
	Token        SubscriptionToken `json:"token"`
	Subscription Subscription      `json:"subscription"`
	User         User              `json:"user"`
}








