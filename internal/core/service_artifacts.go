package core

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

func (s *Service) subscriptionTokenKey() []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(s.cfg.InternalAuthToken)))
	return sum[:]
}

func (s *Service) encryptSubscriptionTokenPlaintext(plain string) (string, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return "", fmt.Errorf("token is empty")
	}
	block, err := aes.NewCipher(s.subscriptionTokenKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(crand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (s *Service) decryptSubscriptionTokenPlaintext(ciphertext string) (string, error) {
	ciphertext = strings.TrimSpace(ciphertext)
	if ciphertext == "" {
		return "", fmt.Errorf("token ciphertext is empty")
	}
	payload, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.subscriptionTokenKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("token ciphertext is invalid")
	}
	nonce := payload[:gcm.NonceSize()]
	body := payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(plain)), nil
}

func (s *Service) issueManagedSubscriptionToken(ctx context.Context, subscriptionID string, expiresAt *time.Time, primary bool) (IssuedSubscriptionToken, error) {
	issued, err := s.store.IssueSubscriptionToken(ctx, subscriptionID, expiresAt)
	if err != nil {
		return IssuedSubscriptionToken{}, err
	}
	ciphertext, err := s.encryptSubscriptionTokenPlaintext(issued.PlaintextToken)
	if err != nil {
		return IssuedSubscriptionToken{}, err
	}
	if err := s.store.UpdateSubscriptionTokenSecret(ctx, issued.Token.ID, ciphertext); err != nil {
		return IssuedSubscriptionToken{}, err
	}
	if primary {
		if err := s.store.UpdateSubscriptionPrimaryToken(ctx, subscriptionID, issued.Token.ID); err != nil {
			return IssuedSubscriptionToken{}, err
		}
		issued.Token.IsPrimary = true
	}
	return issued, nil
}

func (s *Service) ensurePrimarySubscriptionToken(ctx context.Context, subscription Subscription) (IssuedSubscriptionToken, error) {
	now := time.Now().UTC()
	candidateIDs := make([]string, 0, 2)
	if strings.TrimSpace(subscription.PrimaryTokenID) != "" {
		candidateIDs = append(candidateIDs, strings.TrimSpace(subscription.PrimaryTokenID))
	}
	tokens, err := s.store.ListSubscriptionTokensState(ctx, subscription.ID)
	if err == nil {
		for _, token := range tokens {
			if token.IsPrimary {
				candidateIDs = append(candidateIDs, token.ID)
				break
			}
		}
		for _, token := range tokens {
			candidateIDs = append(candidateIDs, token.ID)
		}
	}
	seen := make(map[string]struct{}, len(candidateIDs))
	for _, id := range candidateIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		token, secret, err := s.store.GetSubscriptionTokenSecret(ctx, id)
		if err != nil {
			continue
		}
		if token.RevokedAt != nil {
			continue
		}
		if token.ExpiresAt != nil && !token.ExpiresAt.After(now) {
			continue
		}
		plain, err := s.decryptSubscriptionTokenPlaintext(secret)
		if err != nil || strings.TrimSpace(plain) == "" {
			continue
		}
		if !token.IsPrimary {
			if err := s.store.UpdateSubscriptionPrimaryToken(ctx, subscription.ID, token.ID); err == nil {
				token.IsPrimary = true
			}
		}
		return IssuedSubscriptionToken{PlaintextToken: plain, Token: token}, nil
	}
	return s.issueManagedSubscriptionToken(ctx, subscription.ID, nil, true)
}

func (s *Service) materializeAccessForArtifacts(ctx context.Context, access UserAccess, inbound Inbound) (UserAccess, Inbound, error) {
	if err := s.resolveInboundProfiles(ctx, &inbound); err != nil {
		return access, inbound, err
	}
	if err := s.applyClientProfileToArtifacts(ctx, &access, &inbound); err != nil {
		return access, inbound, err
	}
	return access, inbound, nil
}

func (s *Service) applyClientProfileToArtifacts(ctx context.Context, access *UserAccess, inbound *Inbound) error {
	if access == nil || inbound == nil {
		return nil
	}
	profileID := strings.TrimSpace(access.ClientProfileID)
	if profileID == "" {
		return nil
	}
	profile, err := s.store.GetClientProfile(ctx, profileID)
	if err != nil {
		return err
	}
	if !profile.Enabled || profile.Protocol != inbound.Protocol {
		return nil
	}
	var settings map[string]any
	if raw := strings.TrimSpace(profile.SettingsJSON); raw != "" {
		if err := json.Unmarshal([]byte(raw), &settings); err != nil {
			return err
		}
	}
	switch inbound.Protocol {
	case InboundProtocolVLESS:
		if inbound.VLESS == nil {
			inbound.VLESS = &VLESSInboundSettings{}
		}
		s.applyVLESSClientProfilePreset(profile, inbound.VLESS, access)
		applyVLESSClientProfileSettings(settings, inbound.VLESS, access)
	case InboundProtocolHysteria2:
		if inbound.Hysteria2 == nil {
			inbound.Hysteria2 = &Hysteria2InboundSettings{}
		}
		s.applyHY2ClientProfilePreset(profile, inbound.Hysteria2)
		applyHY2ClientProfileSettings(settings, inbound.Hysteria2)
	}
	return nil
}

func (s *Service) applyVLESSClientProfilePreset(profile ClientProfile, inbound *VLESSInboundSettings, access *UserAccess) {
	switch strings.TrimSpace(profile.Mode) {
	case "multiplex":
		inbound.MultiplexEnabled = true
		if strings.TrimSpace(inbound.MultiplexProtocol) == "" {
			inbound.MultiplexProtocol = "smux"
		}
	case "udp_compat":
		if strings.TrimSpace(inbound.PacketEncodingDefault) == "" {
			inbound.PacketEncodingDefault = "packetaddr"
		}
	case "compat":
		access.VLESSFlowOverride = ""
	}
}

func (s *Service) applyHY2ClientProfilePreset(profile ClientProfile, inbound *Hysteria2InboundSettings) {
	switch strings.TrimSpace(profile.Mode) {
	case "obfuscated":
		if strings.TrimSpace(inbound.ObfsType) == "" {
			inbound.ObfsType = "salamander"
		}
		if strings.TrimSpace(inbound.ObfsPassword) == "" {
			if generated, err := randomHex(8); err == nil {
				inbound.ObfsPassword = generated
			}
		}
	case "poor_network":
		inbound.IgnoreClientBandwidth = true
	case "port_hopping":
		if inbound.HopInterval <= 0 {
			inbound.HopInterval = 30
		}
	}
}

func applyVLESSClientProfileSettings(settings map[string]any, inbound *VLESSInboundSettings, access *UserAccess) {
	if inbound == nil || len(settings) == 0 {
		return
	}
	if value := strings.TrimSpace(jsonString(settings["flow"])); value != "" {
		access.VLESSFlowOverride = value
	}
	if value := strings.TrimSpace(jsonString(settings["transport_type"])); value != "" {
		inbound.TransportType = value
	}
	if value := strings.TrimSpace(jsonString(settings["transport_host"])); value != "" {
		inbound.TransportHost = value
	}
	if value := strings.TrimSpace(jsonString(settings["transport_path"])); value != "" {
		inbound.TransportPath = value
	}
	if value := strings.TrimSpace(jsonString(settings["tls_server_name"])); value != "" {
		inbound.TLSServerName = value
	}
	if values := jsonStringSlice(settings["tls_alpn"]); len(values) > 0 {
		inbound.TLSALPN = values
	}
	if value, ok := jsonBool(settings["multiplex_enabled"]); ok {
		inbound.MultiplexEnabled = value
	}
	if value := strings.TrimSpace(jsonString(settings["multiplex_protocol"])); value != "" {
		inbound.MultiplexProtocol = value
	}
	if value, ok := jsonInt(settings["multiplex_max_connections"]); ok {
		inbound.MultiplexMaxConnections = value
	}
	if value, ok := jsonInt(settings["multiplex_min_streams"]); ok {
		inbound.MultiplexMinStreams = value
	}
	if value, ok := jsonInt(settings["multiplex_max_streams"]); ok {
		inbound.MultiplexMaxStreams = value
	}
	if value := strings.TrimSpace(jsonString(settings["packet_encoding_default"])); value != "" {
		inbound.PacketEncodingDefault = value
	}
}

func applyHY2ClientProfileSettings(settings map[string]any, inbound *Hysteria2InboundSettings) {
	if inbound == nil || len(settings) == 0 {
		return
	}
	if value, ok := jsonBool(settings["allow_insecure"]); ok {
		inbound.AllowInsecure = value
	}
	if value, ok := jsonBool(settings["ignore_client_bandwidth"]); ok {
		inbound.IgnoreClientBandwidth = value
	}
	if value := strings.TrimSpace(jsonString(settings["obfs_type"])); value != "" {
		inbound.ObfsType = value
	}
	if value := strings.TrimSpace(jsonString(settings["obfs_password"])); value != "" {
		inbound.ObfsPassword = value
	}
	if value, ok := jsonIntPointer(settings["up_mbps"]); ok {
		inbound.UpMbps = value
	}
	if value, ok := jsonIntPointer(settings["down_mbps"]); ok {
		inbound.DownMbps = value
	}
	if value := strings.TrimSpace(jsonString(settings["server_ports"])); value != "" {
		inbound.ServerPorts = value
	}
	if value, ok := jsonInt(settings["hop_interval"]); ok {
		inbound.HopInterval = value
	}
	if value := strings.TrimSpace(jsonString(settings["network"])); value != "" {
		inbound.Network = value
	}
	if value := strings.TrimSpace(jsonString(settings["bandwidth_profile_mode"])); value != "" {
		inbound.BandwidthProfileMode = value
	}
	if values := jsonStringSlice(settings["tls_alpn"]); len(values) > 0 {
		inbound.TLSALPN = values
	}
}

func jsonString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func jsonStringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return typed
		}
		return nil
	}
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			items = append(items, strings.TrimSpace(s))
		}
	}
	return items
}

func jsonBool(value any) (bool, bool) {
	v, ok := value.(bool)
	return v, ok
}

func jsonInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
}

func jsonIntPointer(value any) (*int, bool) {
	v, ok := jsonInt(value)
	if !ok {
		return nil, false
	}
	return &v, true
}

