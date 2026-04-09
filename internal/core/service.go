package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/curve25519"

	"h2v2/internal/config"
	"h2v2/internal/fsutil"
	"h2v2/internal/services"
)

const (
	subscriptionRateLimit  = 60
	subscriptionRateWindow = time.Minute
	singBoxCheckTimeout    = 20 * time.Second
	defaultRealityHost     = "www.cloudflare.com"
	defaultRealityPort     = 443
	defaultSingBoxBinary   = "/usr/local/bin/sing-box"
	defaultSingBoxConfig   = "/etc/h2v2/sing-box/config.json"
	defaultSingBoxService  = "sing-box"
)

// ApplyError carries a machine-readable stage name alongside the underlying
// error so that HTTP handlers can return per-stage diagnostics to the UI.
type ApplyError struct {
	Stage string // e.g. "config_validation_failed", "runtime_restart_failed"
	Cause error
}

func (e *ApplyError) Error() string {
	if e.Cause != nil {
		return e.Stage + ": " + e.Cause.Error()
	}
	return e.Stage
}

func (e *ApplyError) Unwrap() error { return e.Cause }

// IsApplyError reports whether err is an *ApplyError and returns it.
func IsApplyError(err error) (*ApplyError, bool) {
	var ae *ApplyError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

type Service struct {
	cfg            config.Config
	logger         *slog.Logger
	store          *Store
	serviceManager *services.ServiceManager
	capabilityMu   sync.Mutex
	capabilityTTL  time.Duration
	v2rayAPIChecks map[string]capabilityCheck
}

type capabilityCheck struct {
	CheckedAt time.Time
	Supported bool
}

func NewService(cfg config.Config, logger *slog.Logger, serviceManager *services.ServiceManager) (*Service, error) {
	store, err := NewStore(cfg.SQLitePath)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:            cfg,
		logger:         logger,
		store:          store,
		serviceManager: serviceManager,
		capabilityTTL:  10 * time.Minute,
		v2rayAPIChecks: make(map[string]capabilityCheck),
	}, nil
}

func (s *Service) Store() *Store {
	if s == nil {
		return nil
	}
	return s.store
}

func (s *Service) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}


func (s *Service) defaultServer(input Server) Server {
	out := input
	if strings.TrimSpace(out.PublicHost) == "" {
		out.PublicHost = s.cfg.PanelPublicHost
	}
	if strings.TrimSpace(out.PanelPublicURL) == "" {
		out.PanelPublicURL = s.cfg.PublicPanelURL
	}
	if strings.TrimSpace(out.SubscriptionBaseURL) == "" {
		out.SubscriptionBaseURL = s.cfg.SubscriptionPublicURL
	}
	if strings.TrimSpace(out.SubscriptionBaseURL) == "" {
		out.SubscriptionBaseURL = out.PanelPublicURL
	}
	if strings.TrimSpace(out.SingBoxBinaryPath) == "" {
		out.SingBoxBinaryPath = s.cfg.SingBoxBinaryPath
	}
	if strings.TrimSpace(out.SingBoxBinaryPath) == "" {
		out.SingBoxBinaryPath = defaultSingBoxBinary
	}
	if strings.TrimSpace(out.SingBoxConfigPath) == "" {
		out.SingBoxConfigPath = s.cfg.SingBoxConfigPath
	}
	if strings.TrimSpace(out.SingBoxConfigPath) == "" {
		out.SingBoxConfigPath = defaultSingBoxConfig
	}
	if strings.TrimSpace(out.SingBoxServiceName) == "" {
		out.SingBoxServiceName = s.cfg.SingBoxServiceName
	}
	if strings.TrimSpace(out.SingBoxServiceName) == "" {
		out.SingBoxServiceName = defaultSingBoxService
	}
	return out
}

func (s *Service) checkUserActive(user User, access UserAccess) bool {
	if !user.Enabled || !access.Enabled {
		return false
	}
	now := time.Now().UTC()
	expire := user.ExpireAt
	if access.ExpireAtOverride != nil {
		expire = access.ExpireAtOverride
	}
	if expire != nil && !expire.After(now) {
		return false
	}
	limit := user.TrafficLimitBytes
	if access.TrafficLimitBytesOverride != nil {
		limit = *access.TrafficLimitBytesOverride
	}
	if limit > 0 && (user.TrafficUsedUpBytes+user.TrafficUsedDownBytes) >= limit {
		return false
	}
	return true
}

func sanitizeVLESSUUID(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = uuid.NewString()
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid vless uuid")
	}
	return strings.ToLower(parsed.String()), nil
}

func decodeRealityKey(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("key is empty")
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
	return nil, fmt.Errorf("key must decode to 32 bytes")
}

func encodeRealityKey(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func deriveRealityPublicKey(privateKey string) (string, error) {
	privateDecoded, err := decodeRealityKey(privateKey)
	if err != nil {
		return "", err
	}
	derived, err := curve25519.X25519(privateDecoded, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return encodeRealityKey(derived), nil
}

// GenerateRealityKeyPair generates a new X25519 keypair and returns (privateKey, publicKey) as base64url strings.
func GenerateRealityKeyPair() (string, string, error) {
	return generateRealityKeyPair()
}

// generateRealityKeyPair generates a new X25519 keypair and returns (privateKey, publicKey) as base64url strings.
func generateRealityKeyPair() (string, string, error) {
	privateKey := make([]byte, 32)
	if _, err := rand.Read(privateKey); err != nil {
		return "", "", fmt.Errorf("failed to generate reality private key: %w", err)
	}
	pub, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to derive reality public key: %w", err)
	}
	return encodeRealityKey(privateKey), encodeRealityKey(pub), nil
}

func normalizeRealitySettings(value *VLESSInboundSettings) error {
	if value == nil || !value.RealityEnabled {
		return nil
	}
	// Auto-generate Reality key pair when private key is not provided.
	if strings.TrimSpace(value.RealityPrivateKey) == "" {
		privateKey := make([]byte, 32)
		if _, err := rand.Read(privateKey); err != nil {
			return fmt.Errorf("failed to generate reality private key: %w", err)
		}
		value.RealityPrivateKey = encodeRealityKey(privateKey)
	}
	privateDecoded, err := decodeRealityKey(value.RealityPrivateKey)
	if err != nil {
		return fmt.Errorf("reality private key is invalid")
	}
	value.RealityPrivateKey = encodeRealityKey(privateDecoded)

	publicTrimmed := strings.TrimSpace(value.RealityPublicKey)
	if publicTrimmed == "" {
		derived, err := deriveRealityPublicKey(value.RealityPrivateKey)
		if err != nil {
			return fmt.Errorf("reality public key is invalid")
		}
		value.RealityPublicKey = derived
	} else {
		publicDecoded, err := decodeRealityKey(publicTrimmed)
		if err != nil {
			return fmt.Errorf("reality public key is invalid")
		}
		value.RealityPublicKey = encodeRealityKey(publicDecoded)
	}

	shortID := strings.ToLower(strings.TrimSpace(value.RealityShortID))
	if shortID == "" {
		// Auto-generate 8-byte (16 hex chars) short ID.
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			return fmt.Errorf("failed to generate reality short id: %w", err)
		}
		shortID = hex.EncodeToString(buf)
	}
	if len(shortID) > 32 || len(shortID)%2 != 0 {
		return fmt.Errorf("reality short id is invalid")
	}
	if _, err := hex.DecodeString(shortID); err != nil {
		return fmt.Errorf("reality short id is invalid")
	}
	value.RealityShortID = shortID
	value.TLSEnabled = true

	// Default TLS server name to the Reality handshake server for correct SNI.
	if strings.TrimSpace(value.TLSServerName) == "" {
		handshake := strings.TrimSpace(value.RealityHandshakeServer)
		if handshake == "" {
			handshake = defaultRealityHost
		}
		value.TLSServerName = handshake
	}
	// Default handshake server if not set.
	if strings.TrimSpace(value.RealityHandshakeServer) == "" {
		value.RealityHandshakeServer = defaultRealityHost
	}
	if value.RealityHandshakeServerPort <= 0 {
		value.RealityHandshakeServerPort = defaultRealityPort
	}
	return nil
}

func normalizeClientEndpointHost(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil {
			value = parsed.Host
		}
	}
	if idx := strings.IndexAny(value, "/?#"); idx >= 0 {
		value = value[:idx]
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.TrimSpace(strings.Trim(value, "[]"))
	normalized := strings.ToLower(value)
	switch normalized {
	case "", "localhost", "127.0.0.1", "::1", "0.0.0.0", "::":
		return ""
	}
	if ip := net.ParseIP(normalized); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() {
			return ""
		}
	}
	return value
}

func defaultHY2Password(username string) string {
	randPart, err := randomHex(8)
	if err != nil {
		randPart = "password"
	}
	baseUser := strings.TrimSpace(username)
	if baseUser == "" {
		baseUser = "user"
	}
	return baseUser + ":" + randPart
}
func (s *Service) ListServers(ctx context.Context) ([]Server, error) {
	return s.store.ListServers(ctx)
}

func (s *Service) GetServer(ctx context.Context, id string) (Server, error) {
	return s.store.GetServer(ctx, id)
}

func (s *Service) UpsertServer(ctx context.Context, server Server) (Server, error) {
	current, currentErr := s.store.GetServer(ctx, server.ID)
	saved, err := s.store.UpsertServer(ctx, s.defaultServer(server))
	if err != nil {
		return Server{}, err
	}
	if currentErr == nil {
		s.invalidateServerCapabilities(current)
	}
	s.invalidateServerCapabilities(saved)
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ID, "server_updated")
	return saved, nil
}

func (s *Service) DeleteServer(ctx context.Context, id string) error {
	return s.store.DeleteServer(ctx, id)
}

func (s *Service) ListInbounds(ctx context.Context, serverID string) ([]Inbound, error) {
	return s.store.ListInbounds(ctx, serverID)
}

func (s *Service) GetInbound(ctx context.Context, id string) (Inbound, error) {
	return s.store.GetInbound(ctx, id)
}

func (s *Service) UpsertInbound(ctx context.Context, inbound Inbound) (Inbound, error) {
	s.applyInboundDefaults(&inbound)
	if inbound.Protocol == InboundProtocolVLESS {
		if err := normalizeRealitySettings(inbound.VLESS); err != nil {
			return Inbound{}, err
		}
	}
	saved, err := s.store.UpsertInbound(ctx, inbound)
	if err != nil {
		return Inbound{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ServerID, "inbound_updated")
	return saved, nil
}

// applyInboundDefaults fills missing fields with stable working defaults so
// that operators do not need to configure every knob manually. It is safe to
// call on partial input; existing non-empty fields are preserved.
func (s *Service) applyInboundDefaults(inbound *Inbound) {
	if inbound == nil {
		return
	}
	if inbound.ListenPort <= 0 {
		inbound.ListenPort = 443
	}
	if strings.TrimSpace(inbound.Listen) == "" {
		inbound.Listen = "::"
	}
	switch inbound.Protocol {
	case InboundProtocolVLESS:
		if inbound.VLESS == nil {
			inbound.VLESS = &VLESSInboundSettings{}
		}
		v := inbound.VLESS
		if strings.TrimSpace(v.TransportType) == "" {
			v.TransportType = "tcp"
		}
		// Enable Reality by default for plain TCP transport when no TLS cert
		// paths are configured — this is the most stable censorship-resistant
		// profile that works without external certificates.
		if v.TransportType == "tcp" && !v.RealityEnabled &&
			strings.TrimSpace(v.TLSCertificatePath) == "" &&
			strings.TrimSpace(v.TLSKeyPath) == "" {
			v.RealityEnabled = true
			v.TLSEnabled = true
		}
		if v.RealityEnabled && strings.TrimSpace(v.Flow) == "" {
			v.Flow = "xtls-rprx-vision"
		}
		if v.RealityEnabled && strings.TrimSpace(v.RealityHandshakeServer) == "" {
			v.RealityHandshakeServer = defaultRealityHost
		}
		if v.RealityEnabled && v.RealityHandshakeServerPort <= 0 {
			v.RealityHandshakeServerPort = defaultRealityPort
		}
		// ALPN is only meaningful for classic TLS (not Reality, which spoofs
		// the handshake of the real target server).
		if v.TLSEnabled && !v.RealityEnabled && len(v.TLSALPN) == 0 {
			v.TLSALPN = []string{"h2", "http/1.1"}
		}
	case InboundProtocolHysteria2:
		if inbound.Hysteria2 == nil {
			inbound.Hysteria2 = &Hysteria2InboundSettings{}
		}
		h := inbound.Hysteria2
		h.TLSEnabled = true
		if strings.TrimSpace(h.TLSServerName) == "" && strings.TrimSpace(s.cfg.HY2Domain) != "" {
			h.TLSServerName = strings.TrimSpace(s.cfg.HY2Domain)
		}
		if strings.TrimSpace(h.TLSCertificatePath) == "" && strings.TrimSpace(s.cfg.HY2CertPath) != "" {
			h.TLSCertificatePath = s.cfg.HY2CertPath
		}
		if strings.TrimSpace(h.TLSKeyPath) == "" && strings.TrimSpace(s.cfg.HY2KeyPath) != "" {
			h.TLSKeyPath = s.cfg.HY2KeyPath
		}
		// The panel-managed cert path is owned by cert-sync, which may leave a
		// self-signed placeholder there while Caddy is provisioning or renewing
		// the ACME cert. Force AllowInsecure so clients keep working across that
		// window — it is a no-op once a real cert is in place.
		if strings.TrimSpace(s.cfg.HY2CertPath) != "" && h.TLSCertificatePath == s.cfg.HY2CertPath {
			h.AllowInsecure = true
		}
		// If the operator hasn't pinned bandwidth, trust the client's
		// advertised speeds — this is the stable "just works" default.
		if h.UpMbps == nil && h.DownMbps == nil && !h.IgnoreClientBandwidth {
			h.IgnoreClientBandwidth = true
		}
	}
}

func (s *Service) DeleteInbound(ctx context.Context, id string) error {
	inbound, err := s.store.GetInbound(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteInbound(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, inbound.ServerID, "inbound_deleted")
	return nil
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.store.ListUsers(ctx)
}

func (s *Service) GetUser(ctx context.Context, id string) (User, error) {
	return s.store.GetUser(ctx, id)
}

func (s *Service) UpsertUser(ctx context.Context, user User) (User, error) {
	saved, err := s.store.UpsertUser(ctx, user)
	if err != nil {
		return User{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByUserIDs(ctx, []string{saved.ID}, "user_updated")
	return saved, nil
}

func (s *Service) DeleteUser(ctx context.Context, id string) error {
	return s.store.DeleteUser(ctx, id)
}

func (s *Service) ListUserAccess(ctx context.Context, userID string) ([]UserAccess, error) {
	return s.store.ListUserAccessByUser(ctx, userID)
}

func (s *Service) GetUserAccess(ctx context.Context, id string) (UserAccess, error) {
	return s.store.GetUserAccess(ctx, id)
}

func (s *Service) UpsertUserAccess(ctx context.Context, access UserAccess) (UserAccess, error) {
	inbound, err := s.store.GetInbound(ctx, access.InboundID)
	if err != nil {
		return UserAccess{}, err
	}
	user, err := s.store.GetUser(ctx, access.UserID)
	if err != nil {
		return UserAccess{}, err
	}
	if inbound.Protocol == InboundProtocolVLESS {
		uuidValue, err := sanitizeVLESSUUID(access.VLESSUUID)
		if err != nil {
			return UserAccess{}, err
		}
		access.VLESSUUID = uuidValue
		access.Hysteria2Password = ""
		if strings.TrimSpace(access.VLESSFlowOverride) == "" && inbound.VLESS != nil {
			access.VLESSFlowOverride = inbound.VLESS.Flow
		}
	} else {
		if strings.TrimSpace(access.Hysteria2Password) == "" {
			existing, getErr := s.store.GetUserAccessByPair(ctx, access.UserID, access.InboundID)
			if getErr == nil && strings.TrimSpace(existing.Hysteria2Password) != "" {
				access.Hysteria2Password = existing.Hysteria2Password
			} else {
				access.Hysteria2Password = defaultHY2Password(user.Username)
			}
		}
		access.VLESSUUID = ""
		access.VLESSFlowOverride = ""
	}
	saved, err := s.store.UpsertUserAccess(ctx, access)
	if err != nil {
		return UserAccess{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByUserIDs(ctx, []string{saved.UserID}, "access_updated")
	return saved, nil
}

func (s *Service) DeleteUserAccess(ctx context.Context, id string) error {
	access, err := s.store.GetUserAccess(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteUserAccess(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByUserIDs(ctx, []string{access.UserID}, "access_deleted")
	return nil
}

// EnsureDefaultInbounds makes sure a server record exists and that both a
// VLESS-Reality and a Hysteria2 inbound are provisioned with stable defaults.
// It is idempotent and safe to call from the installer, from ProvisionUser, or
// from the CLI bootstrap command.
func (s *Service) EnsureDefaultInbounds(ctx context.Context) (string, *Inbound, *Inbound, error) {
	servers, err := s.store.ListServers(ctx)
	if err != nil {
		return "", nil, nil, fmt.Errorf("list servers: %w", err)
	}
	var serverID string
	if len(servers) > 0 {
		serverID = servers[0].ID
	} else {
		server, err := s.UpsertServer(ctx, Server{ID: "default", Name: "default"})
		if err != nil {
			return "", nil, nil, fmt.Errorf("create default server: %w", err)
		}
		serverID = server.ID
	}

	inbounds, err := s.store.ListInbounds(ctx, serverID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("list inbounds: %w", err)
	}
	var vlessInbound, hy2Inbound *Inbound
	for i := range inbounds {
		if inbounds[i].Protocol == InboundProtocolVLESS && vlessInbound == nil {
			cp := inbounds[i]
			vlessInbound = &cp
		}
		if inbounds[i].Protocol == InboundProtocolHysteria2 && hy2Inbound == nil {
			cp := inbounds[i]
			hy2Inbound = &cp
		}
	}
	if vlessInbound == nil {
		created, err := s.UpsertInbound(ctx, Inbound{
			ServerID:    serverID,
			Name:        "VLESS Reality",
			Tag:         "vless-in",
			Protocol:    InboundProtocolVLESS,
			ListenPort:  443,
			Enabled:     true,
			TemplateKey: "vless-reality",
		})
		if err != nil {
			return "", nil, nil, fmt.Errorf("create VLESS inbound: %w", err)
		}
		vlessInbound = &created
	}
	if hy2Inbound == nil {
		port := s.cfg.HY2Port
		if port <= 0 {
			port = 443
		}
		created, err := s.UpsertInbound(ctx, Inbound{
			ServerID:    serverID,
			Name:        "Hysteria2",
			Tag:         "hy2-in",
			Protocol:    InboundProtocolHysteria2,
			ListenPort:  port,
			Enabled:     true,
			TemplateKey: "hysteria2-default",
		})
		if err != nil {
			return "", nil, nil, fmt.Errorf("create HY2 inbound: %w", err)
		}
		hy2Inbound = &created
	}
	return serverID, vlessInbound, hy2Inbound, nil
}

// RefreshInbounds re-normalizes every stored inbound through applyInboundDefaults
// and normalizeRealitySettings, then renders and applies the sing-box config for
// each server (reloading the service if a ServiceManager is wired in). This is
// the remediation path after upgrading defaults or fixing a broken install —
// Reality keys are auto-regenerated only when missing, so existing clients keep
// working.
func (s *Service) RefreshInbounds(ctx context.Context) error {
	servers, err := s.store.ListServers(ctx)
	if err != nil {
		return fmt.Errorf("list servers: %w", err)
	}
	for _, server := range servers {
		inbounds, err := s.store.ListInbounds(ctx, server.ID)
		if err != nil {
			return fmt.Errorf("list inbounds for %s: %w", server.ID, err)
		}
		for _, inbound := range inbounds {
			if _, err := s.UpsertInbound(ctx, inbound); err != nil {
				return fmt.Errorf("refresh inbound %s: %w", inbound.ID, err)
			}
		}
		rendered, err := s.RenderServerConfig(ctx, server.ID, nil)
		if err != nil {
			return fmt.Errorf("render config for %s: %w", server.ID, err)
		}
		if _, err := s.ApplyServerConfig(ctx, server.ID, rendered.Revision.ID); err != nil {
			return fmt.Errorf("apply config for %s: %w", server.ID, err)
		}
	}
	return nil
}

// ProvisionUser ensures a server and both VLESS+HY2 inbounds exist, then creates
// the user and access entries for every available protocol inbound on the server.
// Returns the created user, access entries, and the server ID so the caller can
// apply the updated config once.
func (s *Service) ProvisionUser(ctx context.Context, username string, trafficLimitBytes int64, expireAt *time.Time) (User, []UserAccess, string, error) {
	serverID, vlessInbound, hy2Inbound, err := s.EnsureDefaultInbounds(ctx)
	if err != nil {
		return User{}, nil, "", err
	}

	// Create user.
	user, err := s.UpsertUser(ctx, User{
		Username:          username,
		Enabled:           true,
		TrafficLimitBytes: trafficLimitBytes,
		ExpireAt:          expireAt,
	})
	if err != nil {
		return User{}, nil, "", err
	}

	// Create access entries; roll back user on failure.
	var accessItems []UserAccess
	for _, ib := range []Inbound{*vlessInbound, *hy2Inbound} {
		a, err := s.UpsertUserAccess(ctx, UserAccess{
			UserID:    user.ID,
			InboundID: ib.ID,
			Enabled:   true,
		})
		if err != nil {
			_ = s.store.DeleteUser(ctx, user.ID)
			return User{}, nil, "", fmt.Errorf("create access for inbound %s: %w", ib.ID, err)
		}
		accessItems = append(accessItems, a)
	}

	return user, accessItems, serverID, nil
}

func (s *Service) EnsureSubscriptionForUser(ctx context.Context, userID string) (Subscription, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return Subscription{}, err
	}
	if _, err := s.store.EnsureSubscriptionForUser(ctx, userID, "default"); err != nil {
		return Subscription{}, err
	}
	return s.store.GetSubscriptionStateByUser(ctx, userID)
}

func (s *Service) ListSubscriptionTokensByUser(ctx context.Context, userID string) (Subscription, []SubscriptionToken, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return Subscription{}, nil, err
	}
	subscription, err := s.EnsureSubscriptionForUser(ctx, userID)
	if err != nil {
		return Subscription{}, nil, err
	}
	tokens, err := s.store.ListSubscriptionTokensState(ctx, subscription.ID)
	if err != nil {
		return Subscription{}, nil, err
	}
	return subscription, tokens, nil
}

func (s *Service) RotateSubscriptionTokenByUser(ctx context.Context, userID string, expiresAt *time.Time) (Subscription, IssuedSubscriptionToken, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	subscription, err := s.EnsureSubscriptionForUser(ctx, userID)
	if err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	if err := s.store.RevokeSubscriptionTokens(ctx, subscription.ID); err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	token, err := s.issueManagedSubscriptionToken(ctx, subscription.ID, expiresAt, true)
	if err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	if err := s.store.MarkSubscriptionArtifactsDirty(ctx, subscription.ID, "token_rotated"); err != nil && !IsNotFound(err) {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	subscription, err = s.store.GetSubscriptionState(ctx, subscription.ID)
	if err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	return subscription, token, nil
}

func (s *Service) IssueAdditionalSubscriptionTokenByUser(ctx context.Context, userID string, expiresAt *time.Time) (Subscription, IssuedSubscriptionToken, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	subscription, err := s.EnsureSubscriptionForUser(ctx, userID)
	if err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	token, err := s.issueManagedSubscriptionToken(ctx, subscription.ID, expiresAt, false)
	if err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	subscription, err = s.store.GetSubscriptionState(ctx, subscription.ID)
	if err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	return subscription, token, nil
}

func (s *Service) RevokeSubscriptionTokensByUser(ctx context.Context, userID string) (Subscription, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return Subscription{}, err
	}
	subscription, err := s.store.EnsureSubscriptionForUser(ctx, userID, "default")
	if err != nil {
		return Subscription{}, err
	}
	if err := s.store.RevokeSubscriptionTokens(ctx, subscription.ID); err != nil {
		return Subscription{}, err
	}
	return subscription, nil
}
func (s *Service) BuildUserArtifacts(ctx context.Context, userID string) (UserArtifacts, error) {
	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return UserArtifacts{}, err
	}
	subscription, err := s.EnsureSubscriptionForUser(ctx, user.ID)
	if err != nil {
		return UserArtifacts{}, err
	}
	primaryToken, err := s.ensurePrimarySubscriptionToken(ctx, subscription)
	if err != nil {
		return UserArtifacts{}, err
	}
	plainToken := primaryToken.PlaintextToken

	uris, outbounds, err := s.buildUserURIsAndOutbounds(ctx, user)
	if err != nil {
		return UserArtifacts{}, err
	}
	profileJSON, err := buildSingBoxProfileJSON(outbounds)
	if err != nil {
		return UserArtifacts{}, err
	}
	base := strings.TrimRight(s.subscriptionBaseURLForUser(user), "/")
	profileURL := base + "/sub/" + plainToken + "/profile.singbox.json"
	urisURL := base + "/sub/" + plainToken + "/uris.txt"
	qrURL := base + "/sub/" + plainToken + "/qr.png"
	clashURL := base + "/sub/" + plainToken + "/profile.clash.yaml"
	base64URL := base + "/sub/" + plainToken + "/profile.base64.txt"
	importURL := "sing-box://import-remote-profile?url=" + url.QueryEscape(profileURL) + "&name=" + url.QueryEscape(user.Username)

	vlessURIs := make([]string, 0)
	hy2URIs := make([]string, 0)
	for _, item := range uris {
		if strings.HasPrefix(item, "vless://") {
			vlessURIs = append(vlessURIs, item)
		} else if strings.HasPrefix(item, "hysteria2://") {
			hy2URIs = append(hy2URIs, item)
		}
	}

	if err := s.store.MarkSubscriptionArtifactsRendered(ctx, subscription.ID); err == nil {
		now := time.Now().UTC()
		subscription.ArtifactsNeedRefresh = false
		subscription.LastArtifactRenderedAt = &now
		subscription.LastArtifactRefreshReason = nil
	}

	return UserArtifacts{
		UserID:                    user.ID,
		SubscriptionID:            subscription.ID,
		PrimaryTokenPrefix:        primaryToken.Token.TokenPrefix,
		ArtifactVersion:           subscription.ArtifactVersion,
		ArtifactsNeedRefresh:      subscription.ArtifactsNeedRefresh,
		LastArtifactRenderedAt:    subscription.LastArtifactRenderedAt,
		LastArtifactRefreshReason: subscription.LastArtifactRefreshReason,
		SubscriptionImportURL:     importURL,
		SubscriptionProfileURL:    profileURL,
		SubscriptionURIsURL:       urisURL,
		SubscriptionQRURL:         qrURL,
		SubscriptionClashURL:      clashURL,
		SubscriptionBase64URL:     base64URL,
		VLESSURIs:                 vlessURIs,
		Hysteria2URIs:             hy2URIs,
		AllURIs:                   uris,
		SingBoxProfileJSON:        string(profileJSON),
	}, nil
}

func (s *Service) RenderSubscriptionContentByToken(ctx context.Context, plainToken string, kind string, clientIP string, ifNoneMatch string) (SubscriptionContent, error) {
	rateKey := strings.TrimSpace(clientIP) + "|" + tokenPrefix(plainToken)
	allowed, err := s.store.AllowSubscriptionRateHit(ctx, rateKey, subscriptionRateLimit, subscriptionRateWindow)
	if err != nil {
		s.logger.Warn("subscription rate limiter error, allowing request", "err", err)
	} else if !allowed {
		return SubscriptionContent{}, ErrRateLimited
	}
	tokenCtx, err := s.store.ResolveSubscriptionTokenState(ctx, plainToken, clientIP)
	if err != nil {
		return SubscriptionContent{}, err
	}
	if !tokenCtx.Subscription.Enabled || !tokenCtx.User.Enabled {
		return SubscriptionContent{}, ErrNotFound
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "profile"
	}

	uris, outbounds, err := s.buildUserURIsAndOutbounds(ctx, tokenCtx.User)
	if err != nil {
		return SubscriptionContent{}, err
	}
	base := strings.TrimRight(s.subscriptionBaseURLForUser(tokenCtx.User), "/")
	profileURL := base + "/sub/" + plainToken + "/profile.singbox.json"
	importURL := "sing-box://import-remote-profile?url=" + url.QueryEscape(profileURL) + "&name=" + url.QueryEscape(tokenCtx.User.Username)

	var (
		contentType string
		fileName    string
		body        []byte
	)
	switch kind {
	case "uris":
		contentType = "text/plain; charset=utf-8"
		fileName = tokenCtx.User.Username + "-uris.txt"
		body = []byte(strings.Join(uris, "\n") + "\n")
	case "qr":
		contentType = "image/png"
		fileName = tokenCtx.User.Username + "-subscription.png"
		body, err = qrcode.Encode(importURL, qrcode.Medium, 320)
		if err != nil {
			return SubscriptionContent{}, err
		}
	case "clash":
		contentType = "text/yaml; charset=utf-8"
		fileName = tokenCtx.User.Username + "-profile.yaml"
		body, err = buildClashYAML(tokenCtx.User, uris, outbounds)
		if err != nil {
			return SubscriptionContent{}, err
		}
	case "base64":
		contentType = "text/plain; charset=utf-8"
		fileName = tokenCtx.User.Username + "-shadowrocket.txt"
		body = buildShadowrocketBase64(uris)
	default:
		contentType = "application/json; charset=utf-8"
		fileName = tokenCtx.User.Username + "-profile.singbox.json"
		body, err = buildSingBoxProfileJSON(outbounds)
		if err != nil {
			return SubscriptionContent{}, err
		}
	}
	hash := sha256.Sum256(body)
	version := tokenCtx.Subscription.ArtifactVersion
	if version <= 0 {
		version = 1
	}
	etag := fmt.Sprintf("\"v%d-%s-%s\"", version, kind, hex.EncodeToString(hash[:]))
	headers := map[string]string{
		"Cache-Control":             "private, max-age=60",
		"Profile-Title":             tokenCtx.User.Username,
		"Profile-Update-Interval":   "60",
		"Subscription-Userinfo":     formatTrafficUserInfo(tokenCtx.User),
		"X-Subscription-Import-URL": importURL,
		"X-Artifact-Version":        fmt.Sprintf("%d", version),
	}
	if strings.TrimSpace(ifNoneMatch) == etag {
		return SubscriptionContent{StatusCode: 304, ETag: etag, Headers: headers}, nil
	}
	_ = s.store.MarkSubscriptionArtifactsRendered(ctx, tokenCtx.Subscription.ID)
	return SubscriptionContent{
		StatusCode:  200,
		ContentType: contentType,
		FileName:    fileName,
		ETag:        etag,
		Body:        body,
		Headers:     headers,
	}, nil
}

func (s *Service) buildUserURIsAndOutbounds(ctx context.Context, user User) ([]string, []map[string]any, error) {
	accesses, err := s.store.ListUserAccessByUser(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	uris := make([]string, 0)
	outbounds := make([]map[string]any, 0)
	for _, access := range accesses {
		if !s.checkUserActive(user, access) {
			continue
		}
		inbound, err := s.store.GetInbound(ctx, access.InboundID)
		if err != nil {
			s.logger.Warn("buildUserURIsAndOutbounds: failed to get inbound", "user_id", user.ID, "inbound_id", access.InboundID, "err", err)
			continue
		}
		if !inbound.Enabled {
			continue
		}
		access, inbound, err = s.materializeAccessForArtifacts(ctx, access, inbound)
		if err != nil {
			s.logger.Warn("buildUserURIsAndOutbounds: failed to materialize access", "user_id", user.ID, "inbound_id", inbound.ID, "access_id", access.ID, "err", err)
			continue
		}
		server, err := s.store.GetServer(ctx, inbound.ServerID)
		if err != nil {
			s.logger.Warn("buildUserURIsAndOutbounds: failed to get server", "user_id", user.ID, "inbound_id", inbound.ID, "server_id", inbound.ServerID, "err", err)
			continue
		}
		host := normalizeClientEndpointHost(server.PublicHost)
		if host == "" {
			host = normalizeClientEndpointHost(server.PanelPublicURL)
		}
		if host == "" {
			s.logger.Warn("buildUserURIsAndOutbounds: server has no usable public host, skipping inbound", "user_id", user.ID, "server_id", server.ID, "inbound_id", inbound.ID)
			continue
		}
		switch inbound.Protocol {
		case InboundProtocolVLESS:
			uri, outbound, err := buildVLESSClientArtifacts(user, access, inbound, host)
			if err != nil {
				s.logger.Warn("buildUserURIsAndOutbounds: failed to build VLESS artifacts", "user_id", user.ID, "inbound_id", inbound.ID, "err", err)
				continue
			}
			uris = append(uris, uri)
			outbounds = append(outbounds, outbound)
		case InboundProtocolHysteria2:
			uri, outbound, err := buildHysteria2ClientArtifacts(user, access, inbound, host)
			if err != nil {
				s.logger.Warn("buildUserURIsAndOutbounds: failed to build Hysteria2 artifacts", "user_id", user.ID, "inbound_id", inbound.ID, "err", err)
				continue
			}
			uris = append(uris, uri)
			outbounds = append(outbounds, outbound)
		}
	}
	sort.Strings(uris)
	return uris, outbounds, nil
}

func buildSingBoxProfileJSON(proxyOutbounds []map[string]any) ([]byte, error) {
	outbounds := make([]map[string]any, 0, len(proxyOutbounds)+3)
	tags := make([]string, 0, len(proxyOutbounds))
	for _, item := range proxyOutbounds {
		tag, _ := item["tag"].(string)
		if strings.TrimSpace(tag) != "" {
			tags = append(tags, tag)
		}
		outbounds = append(outbounds, item)
	}
	if len(tags) == 0 {
		tags = append(tags, "direct")
	}
	outbounds = append(outbounds,
		map[string]any{
			"type":     "selector",
			"tag":      "proxy",
			"outbounds": tags,
		},
		map[string]any{"type": "direct", "tag": "direct"},
		map[string]any{"type": "block", "tag": "block"},
	)
	payload := map[string]any{
		"log": map[string]any{"level": "warn"},
		"outbounds": outbounds,
		"route": map[string]any{
			"auto_detect_interface": true,
			"final":                 "proxy",
		},
	}
	return json.MarshalIndent(payload, "", "  ")
}

// DetectSubscriptionKind returns the appropriate subscription format based on the User-Agent.
func DetectSubscriptionKind(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "shadowrocket"):
		return "base64"
	case strings.Contains(ua, "clash") || strings.Contains(ua, "stash") || strings.Contains(ua, "surge") || strings.Contains(ua, "loon"):
		return "clash"
	case strings.Contains(ua, "sing-box") || strings.Contains(ua, "singbox") ||
		strings.Contains(ua, "hiddify") || strings.Contains(ua, "nekobox") ||
		strings.Contains(ua, "nekoray") || strings.Contains(ua, "sfi") ||
		strings.Contains(ua, "sfa") || strings.Contains(ua, "sfm") ||
		strings.Contains(ua, "karing"):
		return "profile"
	default:
		return "profile"
	}
}

func buildClashYAML(user User, uris []string, outbounds []map[string]any) ([]byte, error) {
	proxies := make([]map[string]any, 0, len(outbounds))
	proxyNames := make([]string, 0, len(outbounds))

	for _, ob := range outbounds {
		obType, _ := ob["type"].(string)
		tag, _ := ob["tag"].(string)
		server, _ := ob["server"].(string)
		port, _ := ob["server_port"].(int)
		if server == "" || port == 0 || tag == "" {
			continue
		}
		proxy := map[string]any{
			"name":   tag,
			"server": server,
			"port":   port,
		}
		switch obType {
		case "vless":
			uuid, _ := ob["uuid"].(string)
			proxy["type"] = "vless"
			proxy["uuid"] = uuid
			proxy["udp"] = true
			if tlsRaw, ok := ob["tls"]; ok {
				if tls, ok := tlsRaw.(map[string]any); ok {
					if sn, _ := tls["server_name"].(string); sn != "" {
						proxy["servername"] = sn
					}
					if realityRaw, ok := tls["reality"]; ok {
						if reality, ok := realityRaw.(map[string]any); ok {
							proxy["tls"] = true
							proxy["client-fingerprint"] = "chrome"
							proxy["reality-opts"] = map[string]any{
								"public-key": reality["public_key"],
								"short-id":   reality["short_id"],
							}
							if flow, _ := ob["flow"].(string); flow != "" {
								proxy["flow"] = flow
							}
						}
					} else if enabled, _ := tls["enabled"].(bool); enabled {
						proxy["tls"] = true
						if utlsRaw, ok := tls["utls"]; ok {
							if utls, ok := utlsRaw.(map[string]any); ok {
								if fp, _ := utls["fingerprint"].(string); fp != "" {
									proxy["client-fingerprint"] = fp
								}
							}
						}
						if flow, _ := ob["flow"].(string); flow != "" {
							proxy["flow"] = flow
						}
					}
				}
			}
			if transportRaw, ok := ob["transport"]; ok {
				if transport, ok := transportRaw.(map[string]any); ok {
					ttype, _ := transport["type"].(string)
					proxy["network"] = ttype
					switch ttype {
					case "ws":
						wsOpts := map[string]any{}
						if path, _ := transport["path"].(string); path != "" {
							wsOpts["path"] = path
						}
						if host, _ := transport["host"].(string); host != "" {
							wsOpts["headers"] = map[string]any{"Host": host}
						}
						proxy["ws-opts"] = wsOpts
					case "grpc":
						grpcOpts := map[string]any{}
						if svcName, _ := transport["service_name"].(string); svcName != "" {
							grpcOpts["grpc-service-name"] = svcName
						}
						proxy["grpc-opts"] = grpcOpts
					}
				}
			}
		case "hysteria2":
			password, _ := ob["password"].(string)
			proxy["type"] = "hysteria2"
			proxy["password"] = password
			proxy["udp"] = true
			if tlsRaw, ok := ob["tls"]; ok {
				if tls, ok := tlsRaw.(map[string]any); ok {
					if sn, _ := tls["server_name"].(string); sn != "" {
						proxy["sni"] = sn
					}
					if insecure, _ := tls["insecure"].(bool); insecure {
						proxy["skip-cert-verify"] = true
					}
				}
			}
			if obfsRaw, ok := ob["obfs"]; ok {
				if obfs, ok := obfsRaw.(map[string]any); ok {
					proxy["obfs"] = obfs["type"]
					proxy["obfs-password"] = obfs["password"]
				}
			}
		default:
			continue
		}
		proxies = append(proxies, proxy)
		proxyNames = append(proxyNames, tag)
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no proxies to include in clash profile")
	}

	var b strings.Builder
	b.WriteString("# Clash Meta profile — generated by h2v2\n")
	b.WriteString("# Profile-Title: " + user.Username + "\n\n")
	b.WriteString("mixed-port: 7890\nallow-lan: false\nmode: rule\nlog-level: warning\n\n")
	b.WriteString("proxies:\n")
	for _, p := range proxies {
		b.WriteString("  - ")
		writeClashProxy(&b, p, "    ")
	}
	b.WriteString("\nproxy-groups:\n")
	b.WriteString("  - name: Proxy\n    type: select\n    proxies:\n")
	for _, name := range proxyNames {
		b.WriteString("      - " + name + "\n")
	}
	b.WriteString("      - DIRECT\n")
	b.WriteString("\nrules:\n  - MATCH,Proxy\n")
	return []byte(b.String()), nil
}

func writeClashProxy(b *strings.Builder, proxy map[string]any, indent string) {
	first := true
	for k, v := range proxy {
		if first {
			first = false
		} else {
			b.WriteString(indent)
		}
		switch val := v.(type) {
		case map[string]any:
			b.WriteString(k + ":\n")
			for mk, mv := range val {
				b.WriteString(indent + "  " + mk + ": " + fmt.Sprintf("%v", mv) + "\n")
			}
		case []string:
			b.WriteString(k + ":\n")
			for _, item := range val {
				b.WriteString(indent + "  - " + item + "\n")
			}
		case bool:
			b.WriteString(fmt.Sprintf("%s: %t\n", k, val))
		default:
			b.WriteString(fmt.Sprintf("%s: %v\n", k, val))
		}
	}
}

func buildShadowrocketBase64(uris []string) []byte {
	joined := strings.Join(uris, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(joined))
	return []byte(encoded)
}

func buildVLESSClientArtifacts(user User, access UserAccess, inbound Inbound, host string) (string, map[string]any, error) {
	if inbound.VLESS == nil {
		return "", nil, fmt.Errorf("vless settings are missing")
	}
	if err := normalizeRealitySettings(inbound.VLESS); err != nil {
		return "", nil, err
	}
	uuidValue, err := sanitizeVLESSUUID(access.VLESSUUID)
	if err != nil {
		return "", nil, err
	}
	flow := strings.TrimSpace(access.VLESSFlowOverride)
	if flow == "" {
		flow = strings.TrimSpace(inbound.VLESS.Flow)
	}
	query := url.Values{}
	query.Set("encryption", "none")
	transport := strings.TrimSpace(inbound.VLESS.TransportType)
	if transport == "" {
		transport = "tcp"
	}
	query.Set("type", transport)
	if inbound.VLESS.RealityEnabled {
		query.Set("security", "reality")
		if strings.TrimSpace(inbound.VLESS.TLSServerName) != "" {
			query.Set("sni", strings.TrimSpace(inbound.VLESS.TLSServerName))
		}
		query.Set("pbk", strings.TrimSpace(inbound.VLESS.RealityPublicKey))
		if strings.TrimSpace(inbound.VLESS.RealityShortID) != "" {
			query.Set("sid", strings.TrimSpace(inbound.VLESS.RealityShortID))
		}
		query.Set("fp", "chrome")
		query.Set("spx", "/")
	} else if inbound.VLESS.TLSEnabled {
		query.Set("security", "tls")
		if strings.TrimSpace(inbound.VLESS.TLSServerName) != "" {
			query.Set("sni", strings.TrimSpace(inbound.VLESS.TLSServerName))
		}
		if len(inbound.VLESS.TLSALPN) > 0 {
			query.Set("alpn", strings.Join(inbound.VLESS.TLSALPN, ","))
		} else {
			query.Set("alpn", "h2,http/1.1")
		}
		query.Set("fp", "chrome")
	}
	if flow != "" {
		query.Set("flow", flow)
	}
	if strings.TrimSpace(inbound.VLESS.TransportHost) != "" {
		query.Set("host", strings.TrimSpace(inbound.VLESS.TransportHost))
	}
	if strings.TrimSpace(inbound.VLESS.TransportPath) != "" {
		query.Set("path", strings.TrimSpace(inbound.VLESS.TransportPath))
	}

	uri := &url.URL{
		Scheme:   "vless",
		User:     url.User(uuidValue),
		Host:     fmt.Sprintf("%s:%d", host, inbound.ListenPort),
		RawQuery: query.Encode(),
		Fragment: user.Username + "-" + inbound.Tag,
	}

	outbound := map[string]any{
		"type":        "vless",
		"tag":         "vless-" + inbound.Tag,
		"server":      host,
		"server_port": inbound.ListenPort,
		"uuid":        uuidValue,
	}
	if flow != "" {
		outbound["flow"] = flow
	}
	if inbound.VLESS.TLSEnabled || inbound.VLESS.RealityEnabled {
		tls := map[string]any{"enabled": true}
		if strings.TrimSpace(inbound.VLESS.TLSServerName) != "" {
			tls["server_name"] = strings.TrimSpace(inbound.VLESS.TLSServerName)
		}
		if inbound.VLESS.RealityEnabled {
			reality := map[string]any{"enabled": true}
			reality["public_key"] = strings.TrimSpace(inbound.VLESS.RealityPublicKey)
			if strings.TrimSpace(inbound.VLESS.RealityShortID) != "" {
				reality["short_id"] = strings.TrimSpace(inbound.VLESS.RealityShortID)
			}
			tls["reality"] = reality
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": "chrome"}
		}
		outbound["tls"] = tls
	}
	if transport != "tcp" {
		transportMap := map[string]any{"type": transport}
		if strings.TrimSpace(inbound.VLESS.TransportHost) != "" {
			transportMap["host"] = strings.TrimSpace(inbound.VLESS.TransportHost)
		}
		if strings.TrimSpace(inbound.VLESS.TransportPath) != "" {
			transportMap["path"] = strings.TrimSpace(inbound.VLESS.TransportPath)
		}
		outbound["transport"] = transportMap
	}
	if inbound.VLESS.MultiplexEnabled {
		mux := map[string]any{"enabled": true}
		if strings.TrimSpace(inbound.VLESS.MultiplexProtocol) != "" {
			mux["protocol"] = strings.TrimSpace(inbound.VLESS.MultiplexProtocol)
		}
		if inbound.VLESS.MultiplexMaxConnections > 0 {
			mux["max_connections"] = inbound.VLESS.MultiplexMaxConnections
		}
		if inbound.VLESS.MultiplexMinStreams > 0 {
			mux["min_streams"] = inbound.VLESS.MultiplexMinStreams
		}
		if inbound.VLESS.MultiplexMaxStreams > 0 {
			mux["max_streams"] = inbound.VLESS.MultiplexMaxStreams
		}
		outbound["multiplex"] = mux
	}
	return uri.String(), outbound, nil
}

func buildHysteria2ClientArtifacts(user User, access UserAccess, inbound Inbound, host string) (string, map[string]any, error) {
	if inbound.Hysteria2 == nil {
		return "", nil, fmt.Errorf("hysteria2 settings are missing")
	}
	password := strings.TrimSpace(access.Hysteria2Password)
	if password == "" {
		return "", nil, fmt.Errorf("hysteria2 password is not set for user %s", user.ID)
	}
	query := url.Values{}
	sni := strings.TrimSpace(inbound.Hysteria2.TLSServerName)
	if sni == "" {
		// Fall back to the endpoint host so the client sends a valid SNI even
		// when the operator hasn't set one explicitly.
		sni = host
	}
	if sni != "" {
		query.Set("sni", sni)
	}
	if inbound.Hysteria2.AllowInsecure {
		query.Set("insecure", "1")
	}
	if strings.TrimSpace(inbound.Hysteria2.ObfsType) != "" {
		query.Set("obfs", strings.TrimSpace(inbound.Hysteria2.ObfsType))
		if strings.TrimSpace(inbound.Hysteria2.ObfsPassword) != "" {
			query.Set("obfs-password", strings.TrimSpace(inbound.Hysteria2.ObfsPassword))
		}
	}
	if inbound.Hysteria2.UpMbps != nil {
		query.Set("upmbps", fmt.Sprintf("%d", *inbound.Hysteria2.UpMbps))
	}
	if inbound.Hysteria2.DownMbps != nil {
		query.Set("downmbps", fmt.Sprintf("%d", *inbound.Hysteria2.DownMbps))
	}

	uri := &url.URL{
		Scheme:   "hysteria2",
		User:     url.User(password),
		Host:     fmt.Sprintf("%s:%d", host, inbound.ListenPort),
		RawQuery: query.Encode(),
		Fragment: user.Username + "-" + inbound.Tag,
	}

	outbound := map[string]any{
		"type":        "hysteria2",
		"tag":         "hysteria2-" + inbound.Tag,
		"server":      host,
		"server_port": inbound.ListenPort,
		"password":    password,
	}
	tls := map[string]any{"enabled": true}
	if sni != "" {
		tls["server_name"] = sni
	}
	if inbound.Hysteria2.AllowInsecure {
		tls["insecure"] = true
	}
	outbound["tls"] = tls
	if strings.TrimSpace(inbound.Hysteria2.ObfsType) != "" {
		obfs := map[string]any{"type": strings.TrimSpace(inbound.Hysteria2.ObfsType)}
		if strings.TrimSpace(inbound.Hysteria2.ObfsPassword) != "" {
			obfs["password"] = strings.TrimSpace(inbound.Hysteria2.ObfsPassword)
		}
		outbound["obfs"] = obfs
	}
	if inbound.Hysteria2.UpMbps != nil {
		outbound["up_mbps"] = *inbound.Hysteria2.UpMbps
	}
	if inbound.Hysteria2.DownMbps != nil {
		outbound["down_mbps"] = *inbound.Hysteria2.DownMbps
	}
	return uri.String(), outbound, nil
}

func (s *Service) subscriptionBaseURLForUser(user User) string {
	servers, err := s.store.ListServers(context.Background())
	if err == nil {
		for _, server := range servers {
			if strings.TrimSpace(server.SubscriptionBaseURL) != "" {
				return strings.TrimSpace(server.SubscriptionBaseURL)
			}
		}
	}
	if strings.TrimSpace(s.cfg.SubscriptionPublicURL) != "" {
		return strings.TrimSpace(s.cfg.SubscriptionPublicURL)
	}
	return strings.TrimSpace(s.cfg.PublicPanelURL)
}
func (s *Service) RenderServerConfig(ctx context.Context, serverID string, rollbackFromRevisionID *string) (RenderResult, error) {
	server, err := s.store.GetServer(ctx, serverID)
	if err != nil {
		return RenderResult{}, err
	}
	configBytes, err := s.buildServerConfigJSON(ctx, server)
	if err != nil {
		return RenderResult{}, err
	}
	hash := sha256.Sum256(configBytes)
	hashValue := hex.EncodeToString(hash[:])
	checkErr := s.checkConfig(ctx, server, configBytes)
	checkOK := checkErr == nil
	var checkMessage *string
	if checkErr != nil {
		message := checkErr.Error()
		checkMessage = &message
	}
	revision, err := s.store.CreateConfigRevision(ctx, server.ID, string(configBytes), hashValue, checkOK, checkMessage, rollbackFromRevisionID)
	if err != nil {
		return RenderResult{}, err
	}
	return RenderResult{Server: server, Revision: revision}, nil
}

func (s *Service) ValidateServerConfig(ctx context.Context, serverID string, revisionID string) (ConfigRevision, error) {
	server, err := s.store.GetServer(ctx, serverID)
	if err != nil {
		return ConfigRevision{}, err
	}
	var revision ConfigRevision
	if strings.TrimSpace(revisionID) == "" {
		revision, err = s.store.GetLatestConfigRevision(ctx, serverID)
	} else {
		revision, err = s.store.GetConfigRevision(ctx, revisionID)
	}
	if err != nil {
		return ConfigRevision{}, err
	}
	if err := s.checkConfig(ctx, server, []byte(revision.RenderedJSON)); err != nil {
		return ConfigRevision{}, err
	}
	return revision, nil
}

// ApplyServerConfig writes the rendered config to disk and restarts/reloads
// the sing-box service. The service action is capability-aware: reload is only
// attempted when the service declares SupportsReload; otherwise restart is used
// directly. On any service-action failure the revision is marked apply_failed
// while the previous current revision remains unchanged.
func (s *Service) ApplyServerConfig(ctx context.Context, serverID string, revisionID string) (ConfigRevision, error) {
	server, err := s.store.GetServer(ctx, serverID)
	if err != nil {
		return ConfigRevision{}, err
	}
	var revision ConfigRevision
	if strings.TrimSpace(revisionID) == "" {
		revision, err = s.store.GetLatestConfigRevision(ctx, serverID)
	} else {
		revision, err = s.store.GetConfigRevision(ctx, revisionID)
	}
	if err != nil {
		return ConfigRevision{}, err
	}
	payload := []byte(revision.RenderedJSON)
	if err := s.checkConfig(ctx, server, payload); err != nil {
		_ = s.store.MarkConfigRevisionApplyFailed(ctx, revision.ID, "config_validation_failed: "+err.Error())
		return ConfigRevision{}, &ApplyError{Stage: "config_validation_failed", Cause: err}
	}
	if err := fsutil.WriteFileAtomic(server.SingBoxConfigPath, payload, 0o660); err != nil {
		_ = s.store.MarkConfigRevisionApplyFailed(ctx, revision.ID, "write_failed: "+err.Error())
		return ConfigRevision{}, &ApplyError{Stage: "write_failed", Cause: err}
	}
	if s.serviceManager != nil {
		svcErr := s.applyServiceAction(ctx, server.SingBoxServiceName)
		if svcErr != nil {
			_ = s.store.MarkConfigRevisionApplyFailed(ctx, revision.ID, svcErr.Error())
			return ConfigRevision{}, svcErr
		}
	}
	if err := s.store.MarkConfigRevisionApplied(ctx, revision.ID); err != nil {
		return ConfigRevision{}, err
	}
	return s.store.GetConfigRevision(ctx, revision.ID)
}

// applyServiceAction restarts (or reloads) the named service using capability
// flags. sing-box does not declare ExecReload, so it is always restarted.
func (s *Service) applyServiceAction(ctx context.Context, serviceName string) error {
	caps := s.serviceManager.Capabilities(serviceName)
	if caps.SupportsReload {
		if err := s.serviceManager.Reload(ctx, serviceName); err == nil {
			return nil
		}
		// Reload supported but failed — fall through to restart.
		s.logger.Warn("service reload failed, attempting restart", "service", serviceName)
	}
	if err := s.serviceManager.Restart(ctx, serviceName); err != nil {
		return &ApplyError{Stage: "runtime_restart_failed", Cause: err}
	}
	return nil
}

func (s *Service) ListServerConfigRevisions(ctx context.Context, serverID string, limit int) ([]ConfigRevision, error) {
	return s.store.ListConfigRevisions(ctx, serverID, limit)
}

// BulkPreviewUsers returns the impact of deleting the given user IDs without
// modifying any state. The result can be shown to the operator before confirm.
func (s *Service) BulkPreviewUsers(ctx context.Context, ids []string) (BulkPreviewResult, error) {
	if len(ids) == 0 {
		return BulkPreviewResult{}, nil
	}
	usersByID, err := s.store.ListUsersByIDs(ctx, ids)
	if err != nil {
		return BulkPreviewResult{}, err
	}
	// Collect access entries for each user.
	inboundSeen := make(map[string]struct{})
	accessCount := 0
	subCount := 0
	for id := range usersByID {
		accesses, err := s.store.ListUserAccessByUser(ctx, id)
		if err != nil {
			return BulkPreviewResult{}, err
		}
		accessCount += len(accesses)
		for _, a := range accesses {
			inboundSeen[a.InboundID] = struct{}{}
		}
		if _, subErr := s.store.GetSubscriptionByUser(ctx, id); subErr == nil {
			subCount++
		}
	}
	affectedIDs := make([]string, 0, len(inboundSeen))
	for id := range inboundSeen {
		affectedIDs = append(affectedIDs, id)
	}
	runtimeChange := len(affectedIDs) > 0
	return BulkPreviewResult{
		UserCount:             len(usersByID),
		AccessCount:           accessCount,
		AffectedInboundIDs:    affectedIDs,
		AffectedSubscriptions: subCount,
		RuntimeChangeExpected: runtimeChange,
		RestartRequired:       runtimeChange,
	}, nil
}

// BulkDeleteUsers deletes users and their associated data from the database
// without triggering a runtime apply. The caller is responsible for rendering
// and applying a new config revision when ready.
func (s *Service) BulkDeleteUsers(ctx context.Context, ids []string) (int, error) {
	return s.store.BulkDeleteUsers(ctx, ids)
}

// BulkSetUsersEnabled enables or disables the given users without triggering
// a runtime apply.
func (s *Service) BulkSetUsersEnabled(ctx context.Context, ids []string, enabled bool) (int, error) {
	return s.store.BulkSetUsersEnabled(ctx, ids, enabled)
}

func (s *Service) RollbackServerConfig(ctx context.Context, serverID string, revisionID string) (ConfigRevision, error) {
	baseRevision, err := s.store.GetConfigRevision(ctx, revisionID)
	if err != nil {
		return ConfigRevision{}, err
	}
	if baseRevision.ServerID != serverID {
		return ConfigRevision{}, ErrNotFound
	}
	rendered, err := s.RenderServerConfig(ctx, serverID, &baseRevision.ID)
	if err != nil {
		return ConfigRevision{}, err
	}
	return s.ApplyServerConfig(ctx, serverID, rendered.Revision.ID)
}

func (s *Service) checkConfig(ctx context.Context, server Server, content []byte) error {
	binary := strings.TrimSpace(server.SingBoxBinaryPath)
	if binary == "" {
		binary = defaultSingBoxBinary
	}
	// Skip validation gracefully if the binary doesn't exist yet.
	if _, statErr := os.Stat(binary); os.IsNotExist(statErr) {
		s.logger.Warn("sing-box binary not found, skipping config validation", "path", binary)
		return nil
	}
	tmpDir := strings.TrimSpace(s.cfg.RuntimeDir)
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	if err := os.MkdirAll(tmpDir, 0o750); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(tmpDir, "singbox-check-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	checkCtx, cancel := context.WithTimeout(ctx, singBoxCheckTimeout)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, binary, "check", "-c", tmpPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box check failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// PreviewServerConfig generates the sing-box config JSON for a server without
// saving a revision or requiring the binary to be present. It optionally tries
// a sing-box check and returns the warning message (empty string if check
// passed or was skipped). The first error value is a hard failure (e.g. DB
// read, JSON marshal); the second is a soft check warning.
func (s *Service) PreviewServerConfig(ctx context.Context, serverID string) ([]byte, string, error) {
	server, err := s.store.GetServer(ctx, serverID)
	if err != nil {
		return nil, "", err
	}
	server = s.defaultServer(server)
	configBytes, err := s.buildServerConfigJSON(ctx, server)
	if err != nil {
		return nil, "", err
	}
	var checkWarning string
	if checkErr := s.checkConfig(ctx, server, configBytes); checkErr != nil {
		checkWarning = checkErr.Error()
	}
	return configBytes, checkWarning, nil
}

func (s *Service) buildServerConfigJSON(ctx context.Context, server Server) ([]byte, error) {
	inbounds, err := s.store.ListEnabledInbounds(ctx, server.ID)
	if err != nil {
		return nil, err
	}

	renderedInbounds := make([]map[string]any, 0, len(inbounds))
	statsUsers := make([]string, 0)
	for _, inbound := range inbounds {
		// Resolve profile references into inline settings before rendering.
		if err := s.resolveInboundProfiles(ctx, &inbound); err != nil {
			s.logger.Warn("buildServerConfigJSON: failed to resolve profiles, using inline settings",
				"inbound_id", inbound.ID, "err", err)
		}

		accesses, usersByID, err := s.store.ListInboundActiveUserAccess(ctx, inbound.ID)
		if err != nil {
			return nil, err
		}
		userEntries := make([]map[string]any, 0)
		for _, access := range accesses {
			user, ok := usersByID[access.UserID]
			if !ok {
				continue
			}
			if !s.checkUserActive(user, access) {
				continue
			}
			if access.CredentialStatus == "revoked" {
				continue
			}
			switch inbound.Protocol {
			case InboundProtocolVLESS:
				uuidValue, err := sanitizeVLESSUUID(access.VLESSUUID)
				if err != nil {
					continue
				}
				flow := strings.TrimSpace(access.VLESSFlowOverride)
				if flow == "" && inbound.VLESS != nil {
					flow = strings.TrimSpace(inbound.VLESS.Flow)
				}
				entry := map[string]any{"name": user.Username, "uuid": uuidValue}
				if flow != "" {
					entry["flow"] = flow
				}
				userEntries = append(userEntries, entry)
				statsUsers = append(statsUsers, user.Username)
			case InboundProtocolHysteria2:
				password := strings.TrimSpace(access.Hysteria2Password)
				if password == "" {
					s.logger.Warn("buildServerConfigJSON: user has no hysteria2 password, skipping", "user_id", user.ID)
					continue
				}
				entry := map[string]any{"name": user.Username, "password": password}
				userEntries = append(userEntries, entry)
				statsUsers = append(statsUsers, user.Username)
			}
		}
		if len(userEntries) == 0 {
			continue
		}
		rendered, err := s.renderInboundForServer(ctx, inbound, userEntries)
		if err != nil {
			return nil, err
		}
		renderedInbounds = append(renderedInbounds, rendered)
	}

	// Build log section from active log profile or default.
	logSection := s.buildLogSection(ctx, server.ID)

	// Build outbounds section from DB + defaults.
	outboundsSection := s.buildOutboundsSection(ctx, server.ID)

	// Build route section from DB rules.
	routeSection := s.buildRouteSection(ctx, server.ID)

	payload := map[string]any{
		"log":       logSection,
		"inbounds":  renderedInbounds,
		"outbounds": outboundsSection,
		"route":     routeSection,
	}

	// Attach DNS section if any active DNS profile exists.
	if dnsSection := s.buildDNSSection(ctx, server.ID); dnsSection != nil {
		payload["dns"] = dnsSection
	}
	if experimentalSection := s.buildExperimentalSection(ctx, server, statsUsers); experimentalSection != nil {
		payload["experimental"] = experimentalSection
	}

	return json.MarshalIndent(payload, "", "  ")
}

// resolveInboundProfiles loads profile data into an inbound's inline settings,
// so the renderer always works with fully-resolved settings.
func (s *Service) resolveInboundProfiles(ctx context.Context, inbound *Inbound) error {
	if inbound.VLESS != nil {
		v := inbound.VLESS
		if pid := strings.TrimSpace(v.TLSProfileID); pid != "" {
			p, err := s.store.GetTLSProfile(ctx, pid)
			if err == nil && p.Enabled {
				v.TLSEnabled = true
				if p.ServerName != "" {
					v.TLSServerName = p.ServerName
				}
				if len(p.ALPN) > 0 {
					v.TLSALPN = append([]string(nil), p.ALPN...)
				}
				if p.CertificatePath != "" {
					v.TLSCertificatePath = p.CertificatePath
				}
				if p.KeyPath != "" {
					v.TLSKeyPath = p.KeyPath
				}
			}
		}
		if pid := strings.TrimSpace(v.RealityProfileID); pid != "" {
			p, err := s.store.GetRealityProfile(ctx, pid)
			if err == nil && p.Enabled {
				v.RealityEnabled = true
				v.RealityPrivateKey = p.PrivateKey
				v.RealityPublicKey = p.PublicKey
				v.RealityHandshakeServer = p.HandshakeServer
				v.RealityHandshakeServerPort = p.HandshakeServerPort
				if p.ServerName != "" {
					v.TLSServerName = p.ServerName
				}
				if len(p.ShortIDs) > 0 {
					v.RealityShortID = p.ShortIDs[0]
				}
			}
		}
		if pid := strings.TrimSpace(v.TransportProfileID); pid != "" {
			p, err := s.store.GetTransportProfile(ctx, pid)
			if err == nil && p.Enabled {
				v.TransportType = p.Type
				v.TransportHost = p.Host
				v.TransportPath = p.Path
			}
		}
		if pid := strings.TrimSpace(v.MultiplexProfileID); pid != "" {
			p, err := s.store.GetMultiplexProfile(ctx, pid)
			if err == nil && p.Enabled {
				v.MultiplexEnabled = true
				v.MultiplexProtocol = p.Protocol
				v.MultiplexMaxConnections = p.MaxConnections
				v.MultiplexMinStreams = p.MinStreams
				v.MultiplexMaxStreams = p.MaxStreams
			}
		}
	}
	if inbound.Hysteria2 != nil {
		h := inbound.Hysteria2
		if pid := strings.TrimSpace(h.TLSProfileID); pid != "" {
			p, err := s.store.GetTLSProfile(ctx, pid)
			if err == nil && p.Enabled {
				h.TLSEnabled = true
				if p.ServerName != "" {
					h.TLSServerName = p.ServerName
				}
				if len(p.ALPN) > 0 {
					h.TLSALPN = append([]string(nil), p.ALPN...)
				}
				if p.CertificatePath != "" {
					h.TLSCertificatePath = p.CertificatePath
				}
				if p.KeyPath != "" {
					h.TLSKeyPath = p.KeyPath
				}
				if p.AllowInsecure {
					h.AllowInsecure = true
				}
			}
		}
		if pid := strings.TrimSpace(h.MasqueradeProfileID); pid != "" {
			p, err := s.store.GetHY2MasqueradeProfile(ctx, pid)
			if err == nil && p.Enabled {
				masqJSON, err := masqueradeProfileToJSON(p)
				if err == nil {
					h.MasqueradeJSON = masqJSON
				}
			}
		}
	}
	return nil
}

// buildLogSection returns the sing-box "log" object from the first enabled log profile,
// or a default "warn" level config.
func (s *Service) buildLogSection(ctx context.Context, serverID string) map[string]any {
	profiles, err := s.store.ListLogProfiles(ctx, serverID)
	if err == nil {
		for _, p := range profiles {
			if p.Enabled {
				log := map[string]any{"level": p.Level}
				if strings.TrimSpace(p.Output) != "" {
					log["output"] = p.Output
				}
				if p.Timestamp {
					log["timestamp"] = true
				}
				return log
			}
		}
	}
	return map[string]any{"level": "warn"}
}

// buildOutboundsSection builds the outbounds array from DB + always-present defaults.
func (s *Service) buildOutboundsSection(ctx context.Context, serverID string) []map[string]any {
	result := make([]map[string]any, 0)
	outbounds, err := s.store.ListEnabledOutbounds(ctx, serverID)
	if err == nil {
		for _, ob := range outbounds {
			entry := map[string]any{"type": ob.Type, "tag": ob.Tag}
			if strings.TrimSpace(ob.SettingsJSON) != "" {
				var extra map[string]any
				if json.Unmarshal([]byte(ob.SettingsJSON), &extra) == nil {
					for k, v := range extra {
						entry[k] = v
					}
				}
			}
			result = append(result, entry)
		}
	}
	// Ensure direct and block always exist (sing-box requires them).
	hasDirect, hasBlock := false, false
	for _, ob := range result {
		if ob["tag"] == "direct" {
			hasDirect = true
		}
		if ob["tag"] == "block" {
			hasBlock = true
		}
	}
	if !hasDirect {
		result = append(result, map[string]any{"type": "direct", "tag": "direct"})
	}
	if !hasBlock {
		result = append(result, map[string]any{"type": "block", "tag": "block"})
	}
	return result
}

// buildRouteSection builds the sing-box "route" object from DB rules.
func (s *Service) buildRouteSection(ctx context.Context, serverID string) map[string]any {
	rules, err := s.store.ListEnabledRouteRules(ctx, serverID)
	if err != nil || len(rules) == 0 {
		return map[string]any{"final": "direct"}
	}
	rendered := make([]map[string]any, 0, len(rules))
	for _, r := range rules {
		entry := map[string]any{"outbound": r.OutboundTag}
		if len(r.InboundTags) > 0 {
			entry["inbound"] = r.InboundTags
		}
		if len(r.Protocols) > 0 {
			entry["protocol"] = r.Protocols
		}
		if len(r.Domains) > 0 {
			entry["domain"] = r.Domains
		}
		if len(r.DomainSuffixes) > 0 {
			entry["domain_suffix"] = r.DomainSuffixes
		}
		if len(r.DomainKeywords) > 0 {
			entry["domain_keyword"] = r.DomainKeywords
		}
		if len(r.IPCIDRs) > 0 {
			entry["ip_cidr"] = r.IPCIDRs
		}
		if len(r.Ports) > 0 {
			entry["port"] = r.Ports
		}
		if strings.TrimSpace(r.Network) != "" {
			entry["network"] = r.Network
		}
		if len(r.GeoIPCodes) > 0 {
			entry["geoip"] = r.GeoIPCodes
		}
		if len(r.GeositeCodes) > 0 {
			entry["geosite"] = r.GeositeCodes
		}
		if r.Action == "block" {
			entry["outbound"] = "block"
		}
		if r.Invert {
			entry["invert"] = true
		}
		rendered = append(rendered, entry)
	}
	return map[string]any{
		"rules": rendered,
		"final": "direct",
	}
}

// buildDNSSection builds the sing-box "dns" object from the first enabled DNS profile.
// Returns nil if no DNS profile is configured.
func (s *Service) buildDNSSection(ctx context.Context, serverID string) map[string]any {
	profiles, err := s.store.ListDNSProfiles(ctx, serverID)
	if err != nil {
		return nil
	}
	for _, p := range profiles {
		if !p.Enabled {
			continue
		}
		dns := map[string]any{}
		if strings.TrimSpace(p.Strategy) != "" {
			dns["strategy"] = p.Strategy
		}
		if p.DisableCache {
			dns["disable_cache"] = true
		}
		if strings.TrimSpace(p.FinalServer) != "" {
			dns["final"] = p.FinalServer
		}
		if strings.TrimSpace(p.ServersJSON) != "" {
			var servers []any
			if json.Unmarshal([]byte(p.ServersJSON), &servers) == nil {
				dns["servers"] = servers
			}
		}
		if strings.TrimSpace(p.RulesJSON) != "" {
			var rules []any
			if json.Unmarshal([]byte(p.RulesJSON), &rules) == nil {
				dns["rules"] = rules
			}
		}
		if p.FakeIPEnabled {
			dns["fakeip"] = map[string]any{"enabled": true}
		}
		return dns
	}
	return nil
}

// renderInboundForServer is the service-level inbound renderer (replaces standalone func).
func (s *Service) renderInboundForServer(ctx context.Context, inbound Inbound, users []map[string]any) (map[string]any, error) {
	return renderInboundForServer(inbound, users)
}

func renderInboundForServer(inbound Inbound, users []map[string]any) (map[string]any, error) {
	base := map[string]any{
		"type":       string(inbound.Protocol),
		"tag":        inbound.Tag,
		"listen":     inbound.Listen,
		"listen_port": inbound.ListenPort,
		"users":      users,
	}
	switch inbound.Protocol {
	case InboundProtocolVLESS:
		if inbound.VLESS == nil {
			return nil, fmt.Errorf("vless settings are missing")
		}
		if err := normalizeRealitySettings(inbound.VLESS); err != nil {
			return nil, err
		}
		transportType := strings.TrimSpace(inbound.VLESS.TransportType)
		if transportType == "" {
			transportType = "tcp"
		}
		if transportType != "tcp" {
			transport := map[string]any{"type": transportType}
			if strings.TrimSpace(inbound.VLESS.TransportHost) != "" {
				transport["host"] = strings.TrimSpace(inbound.VLESS.TransportHost)
			}
			if strings.TrimSpace(inbound.VLESS.TransportPath) != "" {
				transport["path"] = strings.TrimSpace(inbound.VLESS.TransportPath)
			}
			base["transport"] = transport
		}
		if inbound.VLESS.TLSEnabled || inbound.VLESS.RealityEnabled {
			tls := map[string]any{"enabled": true}
			if strings.TrimSpace(inbound.VLESS.TLSServerName) != "" {
				tls["server_name"] = strings.TrimSpace(inbound.VLESS.TLSServerName)
			}
			if len(inbound.VLESS.TLSALPN) > 0 {
				tls["alpn"] = inbound.VLESS.TLSALPN
			}
			if strings.TrimSpace(inbound.VLESS.TLSCertificatePath) != "" {
				tls["certificate_path"] = strings.TrimSpace(inbound.VLESS.TLSCertificatePath)
			}
			if strings.TrimSpace(inbound.VLESS.TLSKeyPath) != "" {
				tls["key_path"] = strings.TrimSpace(inbound.VLESS.TLSKeyPath)
			}
			if inbound.VLESS.RealityEnabled {
				reality := map[string]any{"enabled": true}
				if strings.TrimSpace(inbound.VLESS.RealityPrivateKey) != "" {
					reality["private_key"] = strings.TrimSpace(inbound.VLESS.RealityPrivateKey)
				}
				if strings.TrimSpace(inbound.VLESS.RealityShortID) != "" {
					reality["short_id"] = []string{strings.TrimSpace(inbound.VLESS.RealityShortID)}
				}
				handshakeServer := strings.TrimSpace(inbound.VLESS.RealityHandshakeServer)
				if handshakeServer == "" {
					handshakeServer = "www.cloudflare.com"
				}
				handshakePort := inbound.VLESS.RealityHandshakeServerPort
				if handshakePort <= 0 {
					handshakePort = 443
				}
				reality["handshake"] = map[string]any{"server": handshakeServer, "server_port": handshakePort}
				tls["reality"] = reality
			}
			base["tls"] = tls
		}
		if inbound.VLESS.MultiplexEnabled {
			mux := map[string]any{"enabled": true}
			if strings.TrimSpace(inbound.VLESS.MultiplexProtocol) != "" {
				mux["protocol"] = strings.TrimSpace(inbound.VLESS.MultiplexProtocol)
			}
			if inbound.VLESS.MultiplexMaxConnections > 0 {
				mux["max_connections"] = inbound.VLESS.MultiplexMaxConnections
			}
			if inbound.VLESS.MultiplexMinStreams > 0 {
				mux["min_streams"] = inbound.VLESS.MultiplexMinStreams
			}
			if inbound.VLESS.MultiplexMaxStreams > 0 {
				mux["max_streams"] = inbound.VLESS.MultiplexMaxStreams
			}
			base["multiplex"] = mux
		}
	case InboundProtocolHysteria2:
		if inbound.Hysteria2 == nil {
			return nil, fmt.Errorf("hysteria2 settings are missing")
		}
		tls := map[string]any{"enabled": true}
		if strings.TrimSpace(inbound.Hysteria2.TLSServerName) != "" {
			tls["server_name"] = strings.TrimSpace(inbound.Hysteria2.TLSServerName)
		}
		if strings.TrimSpace(inbound.Hysteria2.TLSCertificatePath) != "" {
			tls["certificate_path"] = strings.TrimSpace(inbound.Hysteria2.TLSCertificatePath)
		}
		if strings.TrimSpace(inbound.Hysteria2.TLSKeyPath) != "" {
			tls["key_path"] = strings.TrimSpace(inbound.Hysteria2.TLSKeyPath)
		}
		base["tls"] = tls
		if inbound.Hysteria2.UpMbps != nil {
			base["up_mbps"] = *inbound.Hysteria2.UpMbps
		}
		if inbound.Hysteria2.DownMbps != nil {
			base["down_mbps"] = *inbound.Hysteria2.DownMbps
		}
		if inbound.Hysteria2.IgnoreClientBandwidth {
			base["ignore_client_bandwidth"] = true
		}
		if strings.TrimSpace(inbound.Hysteria2.ObfsType) != "" {
			obfs := map[string]any{"type": strings.TrimSpace(inbound.Hysteria2.ObfsType)}
			if strings.TrimSpace(inbound.Hysteria2.ObfsPassword) != "" {
				obfs["password"] = strings.TrimSpace(inbound.Hysteria2.ObfsPassword)
			}
			base["obfs"] = obfs
		}
		if strings.TrimSpace(inbound.Hysteria2.MasqueradeJSON) != "" {
			var masquerade any
			if err := json.Unmarshal([]byte(inbound.Hysteria2.MasqueradeJSON), &masquerade); err == nil {
				base["masquerade"] = masquerade
			}
		}
		if strings.TrimSpace(inbound.Hysteria2.BBRProfile) != "" {
			base["bbr_profile"] = strings.TrimSpace(inbound.Hysteria2.BBRProfile)
		}
	default:
		return nil, fmt.Errorf("unsupported protocol")
	}
	return base, nil
}

func (s *Service) RenderConfigToPath(ctx context.Context, serverID string, outPath string) error {
	render, err := s.RenderServerConfig(ctx, serverID, nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(outPath) == "" {
		outPath = render.Server.SingBoxConfigPath
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(outPath, []byte(render.Revision.RenderedJSON), 0o660)
}



