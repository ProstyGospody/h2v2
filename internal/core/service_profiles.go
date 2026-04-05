package core

import (
	"context"
	"fmt"
	"strings"
)

// ─── Outbounds ────────────────────────────────────────────────────────────────

func (s *Service) ListOutbounds(ctx context.Context, serverID string) ([]Outbound, error) {
	return s.store.ListOutbounds(ctx, serverID)
}

func (s *Service) GetOutbound(ctx context.Context, id string) (Outbound, error) {
	return s.store.GetOutbound(ctx, id)
}

func (s *Service) UpsertOutbound(ctx context.Context, ob Outbound) (Outbound, error) {
	return s.store.UpsertOutbound(ctx, ob)
}

func (s *Service) DeleteOutbound(ctx context.Context, id string) error {
	return s.store.DeleteOutbound(ctx, id)
}

// ─── Route Rules ──────────────────────────────────────────────────────────────

func (s *Service) ListRouteRules(ctx context.Context, serverID string) ([]RouteRule, error) {
	return s.store.ListRouteRules(ctx, serverID)
}

func (s *Service) GetRouteRule(ctx context.Context, id string) (RouteRule, error) {
	return s.store.GetRouteRule(ctx, id)
}

func (s *Service) UpsertRouteRule(ctx context.Context, r RouteRule) (RouteRule, error) {
	return s.store.UpsertRouteRule(ctx, r)
}

func (s *Service) DeleteRouteRule(ctx context.Context, id string) error {
	return s.store.DeleteRouteRule(ctx, id)
}

// ─── DNS Profiles ─────────────────────────────────────────────────────────────

func (s *Service) ListDNSProfiles(ctx context.Context, serverID string) ([]DNSProfile, error) {
	return s.store.ListDNSProfiles(ctx, serverID)
}

func (s *Service) GetDNSProfile(ctx context.Context, id string) (DNSProfile, error) {
	return s.store.GetDNSProfile(ctx, id)
}

func (s *Service) UpsertDNSProfile(ctx context.Context, p DNSProfile) (DNSProfile, error) {
	return s.store.UpsertDNSProfile(ctx, p)
}

func (s *Service) DeleteDNSProfile(ctx context.Context, id string) error {
	return s.store.DeleteDNSProfile(ctx, id)
}

// ─── Log Profiles ─────────────────────────────────────────────────────────────

func (s *Service) ListLogProfiles(ctx context.Context, serverID string) ([]LogProfile, error) {
	return s.store.ListLogProfiles(ctx, serverID)
}

func (s *Service) GetLogProfile(ctx context.Context, id string) (LogProfile, error) {
	return s.store.GetLogProfile(ctx, id)
}

func (s *Service) UpsertLogProfile(ctx context.Context, p LogProfile) (LogProfile, error) {
	return s.store.UpsertLogProfile(ctx, p)
}

func (s *Service) DeleteLogProfile(ctx context.Context, id string) error {
	return s.store.DeleteLogProfile(ctx, id)
}

// ─── Reality Profiles ─────────────────────────────────────────────────────────

func (s *Service) ListRealityProfiles(ctx context.Context, serverID string) ([]RealityProfile, error) {
	return s.store.ListRealityProfiles(ctx, serverID)
}

func (s *Service) GetRealityProfile(ctx context.Context, id string) (RealityProfile, error) {
	return s.store.GetRealityProfile(ctx, id)
}

// UpsertRealityProfile creates or updates a Reality profile.
// If PrivateKey is empty, auto-generates a new keypair.
func (s *Service) UpsertRealityProfile(ctx context.Context, p RealityProfile) (RealityProfile, error) {
	// Auto-generate keypair when private key is missing.
	if strings.TrimSpace(p.PrivateKey) == "" {
		priv, pub, err := generateRealityKeyPair()
		if err != nil {
			return RealityProfile{}, err
		}
		p.PrivateKey = priv
		p.PublicKey = pub
	}
	// Auto-generate short ID when none provided.
	if len(p.ShortIDs) == 0 {
		sid, err := randomHex(8)
		if err != nil {
			return RealityProfile{}, err
		}
		p.ShortIDs = []string{sid}
	}
	if strings.TrimSpace(p.HandshakeServer) == "" {
		p.HandshakeServer = defaultRealityHost
	}
	if p.HandshakeServerPort <= 0 {
		p.HandshakeServerPort = defaultRealityPort
	}
	return s.store.UpsertRealityProfile(ctx, p)
}

func (s *Service) DeleteRealityProfile(ctx context.Context, id string) error {
	return s.store.DeleteRealityProfile(ctx, id)
}

// ─── Transport Profiles ───────────────────────────────────────────────────────

func (s *Service) ListTransportProfiles(ctx context.Context, serverID string) ([]TransportProfile, error) {
	return s.store.ListTransportProfiles(ctx, serverID)
}

func (s *Service) GetTransportProfile(ctx context.Context, id string) (TransportProfile, error) {
	return s.store.GetTransportProfile(ctx, id)
}

func (s *Service) UpsertTransportProfile(ctx context.Context, p TransportProfile) (TransportProfile, error) {
	return s.store.UpsertTransportProfile(ctx, p)
}

func (s *Service) DeleteTransportProfile(ctx context.Context, id string) error {
	return s.store.DeleteTransportProfile(ctx, id)
}

// ─── Multiplex Profiles ───────────────────────────────────────────────────────

func (s *Service) ListMultiplexProfiles(ctx context.Context, serverID string) ([]MultiplexProfile, error) {
	return s.store.ListMultiplexProfiles(ctx, serverID)
}

func (s *Service) GetMultiplexProfile(ctx context.Context, id string) (MultiplexProfile, error) {
	return s.store.GetMultiplexProfile(ctx, id)
}

func (s *Service) UpsertMultiplexProfile(ctx context.Context, p MultiplexProfile) (MultiplexProfile, error) {
	return s.store.UpsertMultiplexProfile(ctx, p)
}

func (s *Service) DeleteMultiplexProfile(ctx context.Context, id string) error {
	return s.store.DeleteMultiplexProfile(ctx, id)
}

// ─── HY2 Masquerade Profiles ──────────────────────────────────────────────────

func (s *Service) ListHY2MasqueradeProfiles(ctx context.Context, serverID string) ([]HY2MasqueradeProfile, error) {
	return s.store.ListHY2MasqueradeProfiles(ctx, serverID)
}

func (s *Service) GetHY2MasqueradeProfile(ctx context.Context, id string) (HY2MasqueradeProfile, error) {
	return s.store.GetHY2MasqueradeProfile(ctx, id)
}

func (s *Service) UpsertHY2MasqueradeProfile(ctx context.Context, p HY2MasqueradeProfile) (HY2MasqueradeProfile, error) {
	return s.store.UpsertHY2MasqueradeProfile(ctx, p)
}

func (s *Service) DeleteHY2MasqueradeProfile(ctx context.Context, id string) error {
	return s.store.DeleteHY2MasqueradeProfile(ctx, id)
}

// ─── Client Profiles ──────────────────────────────────────────────────────────

func (s *Service) ListClientProfiles(ctx context.Context, serverID string) ([]ClientProfile, error) {
	return s.store.ListClientProfiles(ctx, serverID)
}

func (s *Service) GetClientProfile(ctx context.Context, id string) (ClientProfile, error) {
	return s.store.GetClientProfile(ctx, id)
}

func (s *Service) UpsertClientProfile(ctx context.Context, p ClientProfile) (ClientProfile, error) {
	return s.store.UpsertClientProfile(ctx, p)
}

func (s *Service) DeleteClientProfile(ctx context.Context, id string) error {
	return s.store.DeleteClientProfile(ctx, id)
}

// ─── Domain Validation ───────────────────────────────────────────────────────

// ValidateDomainModel runs domain-level validation for a server's configuration
// before rendering. Returns a list of human-readable error strings.
func (s *Service) ValidateDomainModel(ctx context.Context, serverID string) []string {
	var errs []string

	inbounds, err := s.store.ListEnabledInbounds(ctx, serverID)
	if err != nil {
		errs = append(errs, "failed to load inbounds: "+err.Error())
		return errs
	}

	portsSeen := make(map[int]string)
	for _, ib := range inbounds {
		// Port conflict detection.
		if tag, ok := portsSeen[ib.ListenPort]; ok {
			errs = append(errs, "port conflict: inbound '"+ib.Tag+"' and '"+tag+"' share port "+itoa(ib.ListenPort))
		} else {
			portsSeen[ib.ListenPort] = ib.Tag
		}

		// Required fields.
		if strings.TrimSpace(ib.Tag) == "" {
			errs = append(errs, "inbound id="+ib.ID+": tag is required")
		}
		if ib.ListenPort <= 0 || ib.ListenPort > 65535 {
			errs = append(errs, "inbound '"+ib.Tag+"': invalid listen_port "+itoa(ib.ListenPort))
		}

		// Profile reference validation.
		if ib.VLESS != nil {
			if pid := strings.TrimSpace(ib.VLESS.RealityProfileID); pid != "" {
				if _, err := s.store.GetRealityProfile(ctx, pid); err != nil {
					errs = append(errs, "inbound '"+ib.Tag+"': reality_profile_id '"+pid+"' not found")
				}
			}
			if pid := strings.TrimSpace(ib.VLESS.TransportProfileID); pid != "" {
				if _, err := s.store.GetTransportProfile(ctx, pid); err != nil {
					errs = append(errs, "inbound '"+ib.Tag+"': transport_profile_id '"+pid+"' not found")
				}
			}
			if pid := strings.TrimSpace(ib.VLESS.MultiplexProfileID); pid != "" {
				if _, err := s.store.GetMultiplexProfile(ctx, pid); err != nil {
					errs = append(errs, "inbound '"+ib.Tag+"': multiplex_profile_id '"+pid+"' not found")
				}
			}
		}
		if ib.Hysteria2 != nil {
			if pid := strings.TrimSpace(ib.Hysteria2.MasqueradeProfileID); pid != "" {
				if _, err := s.store.GetHY2MasqueradeProfile(ctx, pid); err != nil {
					errs = append(errs, "inbound '"+ib.Tag+"': masquerade_profile_id '"+pid+"' not found")
				}
			}
		}
	}

	// Validate route rules reference valid outbound tags.
	rules, err := s.store.ListEnabledRouteRules(ctx, serverID)
	if err != nil {
		errs = append(errs, "failed to load route rules: "+err.Error())
	} else {
		outbounds, _ := s.store.ListEnabledOutbounds(ctx, serverID)
		validTags := map[string]bool{"direct": true, "block": true, "dns-out": true}
		for _, ob := range outbounds {
			validTags[ob.Tag] = true
		}
		for _, r := range rules {
			if !validTags[r.OutboundTag] {
				errs = append(errs, "route rule id="+r.ID+": outbound_tag '"+r.OutboundTag+"' references unknown outbound")
			}
		}
	}

	return errs
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
