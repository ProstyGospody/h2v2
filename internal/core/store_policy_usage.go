package core

import (
	"context"
	"fmt"
	"strings"
)

func (s *Store) GetPolicyUsage(ctx context.Context, kind string, id string) (PolicyUsage, error) {
	kind = strings.TrimSpace(strings.ToLower(kind))
	id = normalizeString(id)
	usage := PolicyUsage{Kind: kind, ID: id}
	if id == "" {
		return usage, fmt.Errorf("id is required")
	}
	switch kind {
	case "client-profile":
		if _, err := s.GetClientProfile(ctx, id); err != nil {
			return PolicyUsage{}, err
		}
		usage.UsedByAccess = s.countQuery(ctx, `SELECT COUNT(*) FROM core_user_access WHERE client_profile_id = ?`, id)
		usage.UsedByUsers = s.countQuery(ctx, `SELECT COUNT(DISTINCT user_id) FROM core_user_access WHERE client_profile_id = ?`, id)
		usage.AffectedSubscriptions = s.countQuery(ctx, `SELECT COUNT(DISTINCT s.id) FROM core_subscriptions s JOIN core_user_access ua ON ua.user_id = s.user_id WHERE ua.client_profile_id = ?`, id)
		usage.AffectedArtifacts = usage.AffectedSubscriptions
		usage.RequiresRuntimeApply = false
	case "reality-profile":
		if _, err := s.GetRealityProfile(ctx, id); err != nil {
			return PolicyUsage{}, err
		}
		usage.UsedByInbounds = s.countQuery(ctx, `SELECT COUNT(*) FROM core_inbound_vless_settings WHERE reality_profile_id = ?`, id)
		usage.UsedByAccess = s.countQuery(ctx, `SELECT COUNT(*) FROM core_user_access ua JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id WHERE vs.reality_profile_id = ?`, id)
		usage.UsedByUsers = s.countQuery(ctx, `SELECT COUNT(DISTINCT ua.user_id) FROM core_user_access ua JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id WHERE vs.reality_profile_id = ?`, id)
		usage.AffectedSubscriptions = s.countQuery(ctx, `SELECT COUNT(DISTINCT s.id) FROM core_subscriptions s JOIN core_user_access ua ON ua.user_id = s.user_id JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id WHERE vs.reality_profile_id = ?`, id)
		usage.AffectedArtifacts = usage.AffectedSubscriptions
		usage.RequiresRuntimeApply = usage.UsedByInbounds > 0
	case "transport-profile":
		if _, err := s.GetTransportProfile(ctx, id); err != nil {
			return PolicyUsage{}, err
		}
		usage.UsedByInbounds = s.countQuery(ctx, `SELECT COUNT(*) FROM core_inbound_vless_settings WHERE transport_profile_id = ?`, id)
		usage.UsedByAccess = s.countQuery(ctx, `SELECT COUNT(*) FROM core_user_access ua JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id WHERE vs.transport_profile_id = ?`, id)
		usage.UsedByUsers = s.countQuery(ctx, `SELECT COUNT(DISTINCT ua.user_id) FROM core_user_access ua JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id WHERE vs.transport_profile_id = ?`, id)
		usage.AffectedSubscriptions = s.countQuery(ctx, `SELECT COUNT(DISTINCT s.id) FROM core_subscriptions s JOIN core_user_access ua ON ua.user_id = s.user_id JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id WHERE vs.transport_profile_id = ?`, id)
		usage.AffectedArtifacts = usage.AffectedSubscriptions
		usage.RequiresRuntimeApply = usage.UsedByInbounds > 0
	case "multiplex-profile":
		if _, err := s.GetMultiplexProfile(ctx, id); err != nil {
			return PolicyUsage{}, err
		}
		usage.UsedByInbounds = s.countQuery(ctx, `SELECT COUNT(*) FROM core_inbound_vless_settings WHERE multiplex_profile_id = ?`, id)
		usage.UsedByAccess = s.countQuery(ctx, `SELECT COUNT(*) FROM core_user_access ua JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id WHERE vs.multiplex_profile_id = ?`, id)
		usage.UsedByUsers = s.countQuery(ctx, `SELECT COUNT(DISTINCT ua.user_id) FROM core_user_access ua JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id WHERE vs.multiplex_profile_id = ?`, id)
		usage.AffectedSubscriptions = s.countQuery(ctx, `SELECT COUNT(DISTINCT s.id) FROM core_subscriptions s JOIN core_user_access ua ON ua.user_id = s.user_id JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id WHERE vs.multiplex_profile_id = ?`, id)
		usage.AffectedArtifacts = usage.AffectedSubscriptions
		usage.RequiresRuntimeApply = usage.UsedByInbounds > 0
	case "hy2-masquerade-profile":
		if _, err := s.GetHY2MasqueradeProfile(ctx, id); err != nil {
			return PolicyUsage{}, err
		}
		usage.UsedByInbounds = s.countQuery(ctx, `SELECT COUNT(*) FROM core_inbound_hysteria2_settings WHERE masquerade_profile_id = ?`, id)
		usage.UsedByAccess = s.countQuery(ctx, `SELECT COUNT(*) FROM core_user_access ua JOIN core_inbound_hysteria2_settings hs ON hs.inbound_id = ua.inbound_id WHERE hs.masquerade_profile_id = ?`, id)
		usage.UsedByUsers = s.countQuery(ctx, `SELECT COUNT(DISTINCT ua.user_id) FROM core_user_access ua JOIN core_inbound_hysteria2_settings hs ON hs.inbound_id = ua.inbound_id WHERE hs.masquerade_profile_id = ?`, id)
		usage.AffectedSubscriptions = s.countQuery(ctx, `SELECT COUNT(DISTINCT s.id) FROM core_subscriptions s JOIN core_user_access ua ON ua.user_id = s.user_id JOIN core_inbound_hysteria2_settings hs ON hs.inbound_id = ua.inbound_id WHERE hs.masquerade_profile_id = ?`, id)
		usage.AffectedArtifacts = usage.AffectedSubscriptions
		usage.RequiresRuntimeApply = usage.UsedByInbounds > 0
	case "tls-profile":
		if _, err := s.GetTLSProfile(ctx, id); err != nil {
			return PolicyUsage{}, err
		}
		usage.UsedByInbounds = s.countQuery(ctx, `SELECT COUNT(DISTINCT inbound_id) FROM (
			SELECT inbound_id FROM core_inbound_vless_settings WHERE tls_profile_id = ?
			UNION ALL
			SELECT inbound_id FROM core_inbound_hysteria2_settings WHERE tls_profile_id = ?
		)`, id, id)
		usage.UsedByAccess = s.countQuery(ctx, `SELECT COUNT(*) FROM core_user_access ua LEFT JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id LEFT JOIN core_inbound_hysteria2_settings hs ON hs.inbound_id = ua.inbound_id WHERE vs.tls_profile_id = ? OR hs.tls_profile_id = ?`, id, id)
		usage.UsedByUsers = s.countQuery(ctx, `SELECT COUNT(DISTINCT ua.user_id) FROM core_user_access ua LEFT JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id LEFT JOIN core_inbound_hysteria2_settings hs ON hs.inbound_id = ua.inbound_id WHERE vs.tls_profile_id = ? OR hs.tls_profile_id = ?`, id, id)
		usage.AffectedSubscriptions = s.countQuery(ctx, `SELECT COUNT(DISTINCT s.id) FROM core_subscriptions s JOIN core_user_access ua ON ua.user_id = s.user_id LEFT JOIN core_inbound_vless_settings vs ON vs.inbound_id = ua.inbound_id LEFT JOIN core_inbound_hysteria2_settings hs ON hs.inbound_id = ua.inbound_id WHERE vs.tls_profile_id = ? OR hs.tls_profile_id = ?`, id, id)
		usage.AffectedArtifacts = usage.AffectedSubscriptions
		usage.RequiresRuntimeApply = usage.UsedByInbounds > 0
	case "log-profile":
		if _, err := s.GetLogProfile(ctx, id); err != nil {
			return PolicyUsage{}, err
		}
		usage.UsedByInbounds = s.countQuery(ctx, `SELECT COUNT(*) FROM core_inbounds WHERE log_profile_id = ?`, id)
		usage.UsedByAccess = s.countQuery(ctx, `SELECT COUNT(*) FROM core_user_access ua JOIN core_inbounds ib ON ib.id = ua.inbound_id WHERE ib.log_profile_id = ?`, id)
		usage.UsedByUsers = s.countQuery(ctx, `SELECT COUNT(DISTINCT ua.user_id) FROM core_user_access ua JOIN core_inbounds ib ON ib.id = ua.inbound_id WHERE ib.log_profile_id = ?`, id)
		usage.RequiresRuntimeApply = usage.UsedByInbounds > 0
	case "dns-profile":
		if _, err := s.GetDNSProfile(ctx, id); err != nil {
			return PolicyUsage{}, err
		}
		usage.RequiresRuntimeApply = true
	case "outbound":
		outbound, err := s.GetOutbound(ctx, id)
		if err != nil {
			return PolicyUsage{}, err
		}
		usage.UsedByRouteRules = s.countQuery(ctx, `SELECT COUNT(*) FROM core_route_rules WHERE outbound_tag = ?`, outbound.Tag)
		usage.RequiresRuntimeApply = true
	case "route-rule":
		if _, err := s.GetRouteRule(ctx, id); err != nil {
			return PolicyUsage{}, err
		}
		usage.RequiresRuntimeApply = true
	default:
		return PolicyUsage{}, fmt.Errorf("unsupported policy kind")
	}
	usage.UnsafeDelete = usage.UsedByUsers > 0 || usage.UsedByAccess > 0 || usage.UsedByInbounds > 0 || usage.UsedByRouteRules > 0 || usage.UsedByOutbounds > 0
	return usage, nil
}

func (s *Store) countQuery(ctx context.Context, query string, args ...any) int {
	var count int
	if err := s.db.QueryRowContext(resolveCtx(ctx), query, args...).Scan(&count); err != nil {
		return 0
	}
	return count
}
