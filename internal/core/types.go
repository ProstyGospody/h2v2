package core

import "time"

type InboundProtocol string

const (
	InboundProtocolVLESS     InboundProtocol = "vless"
	InboundProtocolHysteria2 InboundProtocol = "hysteria2"
)

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
}

type Hysteria2InboundSettings struct {
	TLSEnabled            bool   `json:"tls_enabled"`
	TLSServerName         string `json:"tls_server_name,omitempty"`
	TLSCertificatePath    string `json:"tls_certificate_path,omitempty"`
	TLSKeyPath            string `json:"tls_key_path,omitempty"`
	UpMbps                *int   `json:"up_mbps,omitempty"`
	DownMbps              *int   `json:"down_mbps,omitempty"`
	IgnoreClientBandwidth bool   `json:"ignore_client_bandwidth"`
	ObfsType              string `json:"obfs_type,omitempty"`
	ObfsPassword          string `json:"obfs_password,omitempty"`
	MasqueradeJSON        string `json:"masquerade_json,omitempty"`
	BBRProfile            string `json:"bbr_profile,omitempty"`
	BrutalDebug           bool   `json:"brutal_debug"`
}

type Inbound struct {
	ID          string                   `json:"id"`
	ServerID    string                   `json:"server_id"`
	Name        string                   `json:"name"`
	Tag         string                   `json:"tag"`
	Protocol    InboundProtocol          `json:"protocol"`
	Listen      string                   `json:"listen"`
	ListenPort  int                      `json:"listen_port"`
	Enabled     bool                     `json:"enabled"`
	TemplateKey string                   `json:"template_key"`
	VLESS       *VLESSInboundSettings    `json:"vless,omitempty"`
	Hysteria2   *Hysteria2InboundSettings `json:"hysteria2,omitempty"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
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
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type Subscription struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	ProfileName string    `json:"profile_name"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SubscriptionToken struct {
	ID             string     `json:"id"`
	SubscriptionID string     `json:"subscription_id"`
	TokenPrefix    string     `json:"token_prefix"`
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
	CreatedAt              time.Time  `json:"created_at"`
}

type UserArtifacts struct {
	UserID                 string   `json:"user_id"`
	SubscriptionID         string   `json:"subscription_id"`
	SubscriptionImportURL  string   `json:"subscription_import_url"`
	SubscriptionProfileURL string   `json:"subscription_profile_url"`
	SubscriptionURIsURL    string   `json:"subscription_uris_url"`
	SubscriptionQRURL      string   `json:"subscription_qr_url"`
	VLESSURIs              []string `json:"vless_uris"`
	Hysteria2URIs          []string `json:"hysteria2_uris"`
	AllURIs                []string `json:"all_uris"`
	SingBoxProfileJSON     string   `json:"singbox_profile_json"`
}

type RenderResult struct {
	Server   Server         `json:"server"`
	Revision ConfigRevision `json:"revision"`
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
