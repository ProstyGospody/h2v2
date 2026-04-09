package core

import (
	"context"
	"fmt"
	"strings"
)

// в”Ђв”Ђв”Ђ Outbounds в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Service) ListOutbounds(ctx context.Context, serverID string) ([]Outbound, error) {
	return s.store.ListOutbounds(ctx, serverID)
}

func (s *Service) GetOutbound(ctx context.Context, id string) (Outbound, error) {
	return s.store.GetOutbound(ctx, id)
}

func (s *Service) UpsertOutbound(ctx context.Context, ob Outbound) (Outbound, error) {
	saved, err := s.store.UpsertOutbound(ctx, ob)
	if err != nil {
		return Outbound{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ServerID, "outbound_updated")
	return saved, nil
}

func (s *Service) DeleteOutbound(ctx context.Context, id string) error {
	current, err := s.store.GetOutbound(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteOutbound(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, current.ServerID, "outbound_deleted")
	return nil
}

// в”Ђв”Ђв”Ђ Route Rules в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Service) ListRouteRules(ctx context.Context, serverID string) ([]RouteRule, error) {
	return s.store.ListRouteRules(ctx, serverID)
}

func (s *Service) GetRouteRule(ctx context.Context, id string) (RouteRule, error) {
	return s.store.GetRouteRule(ctx, id)
}

func (s *Service) UpsertRouteRule(ctx context.Context, r RouteRule) (RouteRule, error) {
	saved, err := s.store.UpsertRouteRule(ctx, r)
	if err != nil {
		return RouteRule{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ServerID, "route_rule_updated")
	return saved, nil
}

func (s *Service) DeleteRouteRule(ctx context.Context, id string) error {
	current, err := s.store.GetRouteRule(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteRouteRule(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, current.ServerID, "route_rule_deleted")
	return nil
}

// в”Ђв”Ђв”Ђ DNS Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Service) ListDNSProfiles(ctx context.Context, serverID string) ([]DNSProfile, error) {
	return s.store.ListDNSProfiles(ctx, serverID)
}

func (s *Service) GetDNSProfile(ctx context.Context, id string) (DNSProfile, error) {
	return s.store.GetDNSProfile(ctx, id)
}

func (s *Service) UpsertDNSProfile(ctx context.Context, p DNSProfile) (DNSProfile, error) {
	saved, err := s.store.UpsertDNSProfile(ctx, p)
	if err != nil {
		return DNSProfile{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ServerID, "dns_profile_updated")
	return saved, nil
}

func (s *Service) DeleteDNSProfile(ctx context.Context, id string) error {
	current, err := s.store.GetDNSProfile(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteDNSProfile(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, current.ServerID, "dns_profile_deleted")
	return nil
}

// в”Ђв”Ђв”Ђ Log Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Service) ListLogProfiles(ctx context.Context, serverID string) ([]LogProfile, error) {
	return s.store.ListLogProfiles(ctx, serverID)
}

func (s *Service) GetLogProfile(ctx context.Context, id string) (LogProfile, error) {
	return s.store.GetLogProfile(ctx, id)
}

func (s *Service) UpsertLogProfile(ctx context.Context, p LogProfile) (LogProfile, error) {
	saved, err := s.store.UpsertLogProfile(ctx, p)
	if err != nil {
		return LogProfile{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ServerID, "log_profile_updated")
	return saved, nil
}

func (s *Service) DeleteLogProfile(ctx context.Context, id string) error {
	current, err := s.store.GetLogProfile(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteLogProfile(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, current.ServerID, "log_profile_deleted")
	return nil
}

// в”Ђв”Ђв”Ђ Reality Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

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
	saved, err := s.store.UpsertRealityProfile(ctx, p)
	if err != nil {
		return RealityProfile{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ServerID, "reality_profile_updated")
	return saved, nil
}

func (s *Service) DeleteRealityProfile(ctx context.Context, id string) error {
	current, err := s.store.GetRealityProfile(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteRealityProfile(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, current.ServerID, "reality_profile_deleted")
	return nil
}

// в”Ђв”Ђв”Ђ Transport Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Service) ListTransportProfiles(ctx context.Context, serverID string) ([]TransportProfile, error) {
	return s.store.ListTransportProfiles(ctx, serverID)
}

func (s *Service) GetTransportProfile(ctx context.Context, id string) (TransportProfile, error) {
	return s.store.GetTransportProfile(ctx, id)
}

func (s *Service) UpsertTransportProfile(ctx context.Context, p TransportProfile) (TransportProfile, error) {
	saved, err := s.store.UpsertTransportProfile(ctx, p)
	if err != nil {
		return TransportProfile{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ServerID, "transport_profile_updated")
	return saved, nil
}

func (s *Service) DeleteTransportProfile(ctx context.Context, id string) error {
	current, err := s.store.GetTransportProfile(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteTransportProfile(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, current.ServerID, "transport_profile_deleted")
	return nil
}

// в”Ђв”Ђв”Ђ Multiplex Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Service) ListMultiplexProfiles(ctx context.Context, serverID string) ([]MultiplexProfile, error) {
	return s.store.ListMultiplexProfiles(ctx, serverID)
}

func (s *Service) GetMultiplexProfile(ctx context.Context, id string) (MultiplexProfile, error) {
	return s.store.GetMultiplexProfile(ctx, id)
}

func (s *Service) UpsertMultiplexProfile(ctx context.Context, p MultiplexProfile) (MultiplexProfile, error) {
	saved, err := s.store.UpsertMultiplexProfile(ctx, p)
	if err != nil {
		return MultiplexProfile{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ServerID, "multiplex_profile_updated")
	return saved, nil
}

func (s *Service) DeleteMultiplexProfile(ctx context.Context, id string) error {
	current, err := s.store.GetMultiplexProfile(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteMultiplexProfile(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, current.ServerID, "multiplex_profile_deleted")
	return nil
}

// в”Ђв”Ђв”Ђ HY2 Masquerade Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Service) ListHY2MasqueradeProfiles(ctx context.Context, serverID string) ([]HY2MasqueradeProfile, error) {
	return s.store.ListHY2MasqueradeProfiles(ctx, serverID)
}

func (s *Service) GetHY2MasqueradeProfile(ctx context.Context, id string) (HY2MasqueradeProfile, error) {
	return s.store.GetHY2MasqueradeProfile(ctx, id)
}

func (s *Service) UpsertHY2MasqueradeProfile(ctx context.Context, p HY2MasqueradeProfile) (HY2MasqueradeProfile, error) {
	saved, err := s.store.UpsertHY2MasqueradeProfile(ctx, p)
	if err != nil {
		return HY2MasqueradeProfile{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ServerID, "hy2_masquerade_profile_updated")
	return saved, nil
}

func (s *Service) DeleteHY2MasqueradeProfile(ctx context.Context, id string) error {
	current, err := s.store.GetHY2MasqueradeProfile(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteHY2MasqueradeProfile(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, current.ServerID, "hy2_masquerade_profile_deleted")
	return nil
}

// в”Ђв”Ђв”Ђ TLS Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Service) ListTLSProfiles(ctx context.Context, serverID string) ([]TLSProfile, error) {
	return s.store.ListTLSProfiles(ctx, serverID)
}

func (s *Service) GetTLSProfile(ctx context.Context, id string) (TLSProfile, error) {
	return s.store.GetTLSProfile(ctx, id)
}

func (s *Service) UpsertTLSProfile(ctx context.Context, p TLSProfile) (TLSProfile, error) {
	saved, err := s.store.UpsertTLSProfile(ctx, p)
	if err != nil {
		return TLSProfile{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, saved.ServerID, "tls_profile_updated")
	return saved, nil
}

func (s *Service) DeleteTLSProfile(ctx context.Context, id string) error {
	current, err := s.store.GetTLSProfile(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteTLSProfile(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, current.ServerID, "tls_profile_deleted")
	return nil
}
// в”Ђв”Ђв”Ђ Client Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Service) ListClientProfiles(ctx context.Context, serverID string) ([]ClientProfile, error) {
	return s.store.ListClientProfiles(ctx, serverID)
}

func (s *Service) GetClientProfile(ctx context.Context, id string) (ClientProfile, error) {
	return s.store.GetClientProfile(ctx, id)
}

func (s *Service) UpsertClientProfile(ctx context.Context, p ClientProfile) (ClientProfile, error) {
	saved, err := s.store.UpsertClientProfile(ctx, p)
	if err != nil {
		return ClientProfile{}, err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByClientProfile(ctx, saved.ID, "client_profile_updated")
	return saved, nil
}

func (s *Service) DeleteClientProfile(ctx context.Context, id string) error {
	current, err := s.store.GetClientProfile(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteClientProfile(ctx, id); err != nil {
		return err
	}
	_, _ = s.store.MarkSubscriptionsArtifactsDirtyByServer(ctx, current.ServerID, "client_profile_deleted")
	return nil
}

// в”Ђв”Ђв”Ђ Domain Validation в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

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

