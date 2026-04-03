package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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

type Service struct {
	cfg            config.Config
	logger         *slog.Logger
	store          *Store
	serviceManager *services.ServiceManager
	rateLimiter    *subscriptionRateLimiter
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
		rateLimiter:    newSubscriptionRateLimiter(60, time.Minute),
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

type subscriptionRateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func newSubscriptionRateLimiter(limit int, window time.Duration) *subscriptionRateLimiter {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	return &subscriptionRateLimiter{
		limit:  limit,
		window: window,
		hits:   make(map[string][]time.Time),
	}
}

func (l *subscriptionRateLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	now := time.Now().UTC()
	threshold := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	entries := l.hits[key]
	filtered := entries[:0]
	for _, item := range entries {
		if item.After(threshold) {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) >= l.limit {
		l.hits[key] = filtered
		return false
	}
	filtered = append(filtered, now)
	l.hits[key] = filtered
	return true
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
		out.SingBoxBinaryPath = "/usr/local/bin/sing-box"
	}
	if strings.TrimSpace(out.SingBoxConfigPath) == "" {
		out.SingBoxConfigPath = s.cfg.SingBoxConfigPath
	}
	if strings.TrimSpace(out.SingBoxConfigPath) == "" {
		out.SingBoxConfigPath = "/etc/h2v2/sing-box/config.json"
	}
	if strings.TrimSpace(out.SingBoxServiceName) == "" {
		out.SingBoxServiceName = s.cfg.SingBoxServiceName
	}
	if strings.TrimSpace(out.SingBoxServiceName) == "" {
		out.SingBoxServiceName = "sing-box"
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
			handshake = "www.cloudflare.com"
		}
		value.TLSServerName = handshake
	}
	// Default handshake server if not set.
	if strings.TrimSpace(value.RealityHandshakeServer) == "" {
		value.RealityHandshakeServer = "www.cloudflare.com"
	}
	if value.RealityHandshakeServerPort <= 0 {
		value.RealityHandshakeServerPort = 443
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
	return s.store.UpsertServer(ctx, s.defaultServer(server))
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
	if inbound.Protocol == InboundProtocolVLESS {
		if inbound.VLESS == nil {
			inbound.VLESS = &VLESSInboundSettings{}
		}
		if strings.TrimSpace(inbound.VLESS.TransportType) == "" {
			inbound.VLESS.TransportType = "tcp"
		}
		if err := normalizeRealitySettings(inbound.VLESS); err != nil {
			return Inbound{}, err
		}
	} else if inbound.Protocol == InboundProtocolHysteria2 {
		if inbound.Hysteria2 == nil {
			inbound.Hysteria2 = &Hysteria2InboundSettings{}
		}
	}
	return s.store.UpsertInbound(ctx, inbound)
}

func (s *Service) DeleteInbound(ctx context.Context, id string) error {
	return s.store.DeleteInbound(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.store.ListUsers(ctx)
}

func (s *Service) GetUser(ctx context.Context, id string) (User, error) {
	return s.store.GetUser(ctx, id)
}

func (s *Service) UpsertUser(ctx context.Context, user User) (User, error) {
	return s.store.UpsertUser(ctx, user)
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
			access.Hysteria2Password = defaultHY2Password(user.Username)
		}
		access.VLESSUUID = ""
		access.VLESSFlowOverride = ""
	}
	return s.store.UpsertUserAccess(ctx, access)
}

func (s *Service) DeleteUserAccess(ctx context.Context, id string) error {
	return s.store.DeleteUserAccess(ctx, id)
}

func (s *Service) EnsureSubscriptionForUser(ctx context.Context, userID string) (Subscription, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return Subscription{}, err
	}
	return s.store.EnsureSubscriptionForUser(ctx, userID, "default")
}

func (s *Service) ListSubscriptionTokensByUser(ctx context.Context, userID string) (Subscription, []SubscriptionToken, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return Subscription{}, nil, err
	}
	subscription, err := s.store.EnsureSubscriptionForUser(ctx, userID, "default")
	if err != nil {
		return Subscription{}, nil, err
	}
	tokens, err := s.store.ListSubscriptionTokens(ctx, subscription.ID)
	if err != nil {
		return Subscription{}, nil, err
	}
	return subscription, tokens, nil
}

func (s *Service) RotateSubscriptionTokenByUser(ctx context.Context, userID string, expiresAt *time.Time) (Subscription, IssuedSubscriptionToken, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	subscription, err := s.store.EnsureSubscriptionForUser(ctx, userID, "default")
	if err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	token, err := s.store.RotateSubscriptionToken(ctx, subscription.ID, expiresAt)
	if err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	return subscription, token, nil
}

func (s *Service) IssueAdditionalSubscriptionTokenByUser(ctx context.Context, userID string, expiresAt *time.Time) (Subscription, IssuedSubscriptionToken, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	subscription, err := s.store.EnsureSubscriptionForUser(ctx, userID, "default")
	if err != nil {
		return Subscription{}, IssuedSubscriptionToken{}, err
	}
	token, err := s.store.IssueSubscriptionToken(ctx, subscription.ID, expiresAt)
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
	subscription, err := s.store.EnsureSubscriptionForUser(ctx, user.ID, "default")
	if err != nil {
		return UserArtifacts{}, err
	}
	issued, err := s.store.IssueSubscriptionToken(ctx, subscription.ID, nil)
	if err != nil {
		return UserArtifacts{}, err
	}
	plainToken := issued.PlaintextToken

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

	return UserArtifacts{
		UserID:                 user.ID,
		SubscriptionID:         subscription.ID,
		SubscriptionImportURL:  importURL,
		SubscriptionProfileURL: profileURL,
		SubscriptionURIsURL:    urisURL,
		SubscriptionQRURL:      qrURL,
		VLESSURIs:              vlessURIs,
		Hysteria2URIs:          hy2URIs,
		AllURIs:                uris,
		SingBoxProfileJSON:     string(profileJSON),
	}, nil
}

func (s *Service) RenderSubscriptionContentByToken(ctx context.Context, plainToken string, kind string, clientIP string, ifNoneMatch string) (SubscriptionContent, error) {
	if !s.rateLimiter.Allow(strings.TrimSpace(clientIP) + "|" + tokenPrefix(plainToken)) {
		return SubscriptionContent{}, ErrRateLimited
	}
	tokenCtx, err := s.store.ResolveSubscriptionToken(ctx, plainToken, clientIP)
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
		fileName string
		body []byte
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
	default:
		contentType = "application/json; charset=utf-8"
		fileName = tokenCtx.User.Username + "-profile.singbox.json"
		body, err = buildSingBoxProfileJSON(outbounds)
		if err != nil {
			return SubscriptionContent{}, err
		}
	}
	hash := sha256.Sum256(body)
	etag := "\"" + hex.EncodeToString(hash[:]) + "\""
	if strings.TrimSpace(ifNoneMatch) == etag {
		return SubscriptionContent{StatusCode: 304, ETag: etag, Headers: map[string]string{
			"Cache-Control": "private, max-age=60",
		}}, nil
	}
	return SubscriptionContent{
		StatusCode:  200,
		ContentType: contentType,
		FileName:    fileName,
		ETag:        etag,
		Body:        body,
		Headers: map[string]string{
			"Cache-Control":            "private, max-age=60",
			"Profile-Title":            tokenCtx.User.Username,
			"Profile-Update-Interval":  "60",
			"Subscription-Userinfo":    formatTrafficUserInfo(tokenCtx.User),
			"X-Subscription-Import-URL": importURL,
		},
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
		if err != nil || !inbound.Enabled {
			continue
		}
		server, err := s.store.GetServer(ctx, inbound.ServerID)
		if err != nil {
			continue
		}
		host := normalizeClientEndpointHost(server.PublicHost)
		if host == "" {
			host = normalizeClientEndpointHost(server.PanelPublicURL)
		}
		if host == "" {
			continue
		}
		switch inbound.Protocol {
		case InboundProtocolVLESS:
			uri, outbound, err := buildVLESSClientArtifacts(user, access, inbound, host)
			if err != nil {
				continue
			}
			uris = append(uris, uri)
			outbounds = append(outbounds, outbound)
		case InboundProtocolHysteria2:
			uri, outbound, err := buildHysteria2ClientArtifacts(user, access, inbound, host)
			if err != nil {
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
	} else if inbound.VLESS.TLSEnabled {
		query.Set("security", "tls")
		if strings.TrimSpace(inbound.VLESS.TLSServerName) != "" {
			query.Set("sni", strings.TrimSpace(inbound.VLESS.TLSServerName))
		}
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
		Fragment: url.QueryEscape(user.Username + "-" + inbound.Tag),
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
		password = defaultHY2Password(user.Username)
	}
	query := url.Values{}
	if strings.TrimSpace(inbound.Hysteria2.TLSServerName) != "" {
		query.Set("sni", strings.TrimSpace(inbound.Hysteria2.TLSServerName))
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
		Fragment: url.QueryEscape(user.Username + "-" + inbound.Tag),
	}

	outbound := map[string]any{
		"type":        "hysteria2",
		"tag":         "hysteria2-" + inbound.Tag,
		"server":      host,
		"server_port": inbound.ListenPort,
		"password":    password,
	}
	tls := map[string]any{"enabled": true}
	if strings.TrimSpace(inbound.Hysteria2.TLSServerName) != "" {
		tls["server_name"] = strings.TrimSpace(inbound.Hysteria2.TLSServerName)
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
	if inbound.Hysteria2.IgnoreClientBandwidth {
		outbound["ignore_client_bandwidth"] = true
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
		return ConfigRevision{}, err
	}
	if err := fsutil.WriteFileAtomic(server.SingBoxConfigPath, payload, 0o660); err != nil {
		return ConfigRevision{}, err
	}
	if s.serviceManager != nil {
		if err := s.serviceManager.Reload(ctx, server.SingBoxServiceName); err != nil {
			if restartErr := s.serviceManager.Restart(ctx, server.SingBoxServiceName); restartErr != nil {
				return ConfigRevision{}, fmt.Errorf("reload failed: %w; restart failed: %v", err, restartErr)
			}
		}
	}
	if err := s.store.MarkConfigRevisionApplied(ctx, revision.ID); err != nil {
		return ConfigRevision{}, err
	}
	return s.store.GetConfigRevision(ctx, revision.ID)
}

func (s *Service) ListServerConfigRevisions(ctx context.Context, serverID string, limit int) ([]ConfigRevision, error) {
	return s.store.ListConfigRevisions(ctx, serverID, limit)
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
		binary = "/usr/local/bin/sing-box"
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
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, binary, "check", "-c", tmpPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box check failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *Service) buildServerConfigJSON(ctx context.Context, server Server) ([]byte, error) {
	inbounds, err := s.store.ListEnabledInbounds(ctx, server.ID)
	if err != nil {
		return nil, err
	}
	renderedInbounds := make([]map[string]any, 0, len(inbounds))
	for _, inbound := range inbounds {
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
			case InboundProtocolHysteria2:
				password := strings.TrimSpace(access.Hysteria2Password)
				if password == "" {
					password = defaultHY2Password(user.Username)
				}
				entry := map[string]any{"name": user.Username, "password": password}
				userEntries = append(userEntries, entry)
			}
		}
		if len(userEntries) == 0 {
			continue
		}
		rendered, err := renderInboundForServer(inbound, userEntries)
		if err != nil {
			return nil, err
		}
		renderedInbounds = append(renderedInbounds, rendered)
	}
	payload := map[string]any{
		"log": map[string]any{"level": "warn"},
		"inbounds": renderedInbounds,
		"outbounds": []map[string]any{
			{"type": "direct", "tag": "direct"},
			{"type": "block", "tag": "block"},
		},
		"route": map[string]any{
			"final": "direct",
		},
	}
	return json.MarshalIndent(payload, "", "  ")
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
