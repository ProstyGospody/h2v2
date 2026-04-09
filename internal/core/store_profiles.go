package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// в”Ђв”Ђв”Ђ Outbounds в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Store) ListOutbounds(ctx context.Context, serverID string) ([]Outbound, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, tag, type, enabled, priority, COALESCE(settings_json,''), healthcheck_enabled, created_at_ns, updated_at_ns
		 FROM core_outbounds WHERE server_id = ? ORDER BY priority ASC, created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Outbound, 0)
	for rows.Next() {
		item, err := scanOutbound(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListEnabledOutbounds(ctx context.Context, serverID string) ([]Outbound, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, tag, type, enabled, priority, COALESCE(settings_json,''), healthcheck_enabled, created_at_ns, updated_at_ns
		 FROM core_outbounds WHERE server_id = ? AND enabled = 1 ORDER BY priority ASC, created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Outbound, 0)
	for rows.Next() {
		item, err := scanOutbound(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetOutbound(ctx context.Context, id string) (Outbound, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx),
		`SELECT id, server_id, tag, type, enabled, priority, COALESCE(settings_json,''), healthcheck_enabled, created_at_ns, updated_at_ns
		 FROM core_outbounds WHERE id = ? LIMIT 1`,
		normalizeString(id))
	return scanOutbound(row)
}

func (s *Store) UpsertOutbound(ctx context.Context, ob Outbound) (Outbound, error) {
	now := time.Now().UTC()
	ob.ID = normalizeString(ob.ID)
	if ob.ID == "" {
		ob.ID = generateID()
	}
	ob.ServerID = normalizeString(ob.ServerID)
	ob.Tag = normalizeString(ob.Tag)
	ob.Type = normalizeString(ob.Type)
	if ob.ServerID == "" || ob.Tag == "" || ob.Type == "" {
		return Outbound{}, fmt.Errorf("server_id, tag, and type are required")
	}
	if ob.CreatedAt.IsZero() {
		ob.CreatedAt = now
	}
	ob.UpdatedAt = now
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`INSERT INTO core_outbounds(id, server_id, tag, type, enabled, priority, settings_json, healthcheck_enabled, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			tag = excluded.tag, type = excluded.type, enabled = excluded.enabled,
			priority = excluded.priority, settings_json = excluded.settings_json,
			healthcheck_enabled = excluded.healthcheck_enabled, updated_at_ns = excluded.updated_at_ns`,
		ob.ID, ob.ServerID, ob.Tag, ob.Type,
		boolToInt(ob.Enabled), ob.Priority,
		nullIfEmpty(ob.SettingsJSON),
		boolToInt(ob.HealthcheckEnabled),
		toUnixNano(ob.CreatedAt), toUnixNano(ob.UpdatedAt))
	if err != nil {
		return Outbound{}, parseUnique(err)
	}
	return s.GetOutbound(ctx, ob.ID)
}

func (s *Store) DeleteOutbound(ctx context.Context, id string) error {
	return deleteByID(s, ctx, "core_outbounds", id)
}

func scanOutbound(row interface{ Scan(dest ...any) error }) (Outbound, error) {
	var (
		ob              Outbound
		enabled         int64
		healthcheck     int64
		createdAt       int64
		updatedAt       int64
	)
	if err := row.Scan(&ob.ID, &ob.ServerID, &ob.Tag, &ob.Type, &enabled, &ob.Priority,
		&ob.SettingsJSON, &healthcheck, &createdAt, &updatedAt); err != nil {
		return Outbound{}, parseUnique(err)
	}
	ob.Enabled = intToBool(enabled)
	ob.HealthcheckEnabled = intToBool(healthcheck)
	ob.CreatedAt = fromUnixNano(createdAt)
	ob.UpdatedAt = fromUnixNano(updatedAt)
	return ob, nil
}

// в”Ђв”Ђв”Ђ Route Rules в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Store) ListRouteRules(ctx context.Context, serverID string) ([]RouteRule, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, enabled, priority,
			COALESCE(inbound_tags_json,''), COALESCE(protocols_json,''), COALESCE(domains_json,''),
			COALESCE(domain_suffixes_json,''), COALESCE(domain_keywords_json,''), COALESCE(ip_cidrs_json,''),
			COALESCE(ports_json,''), COALESCE(network,''), COALESCE(geoip_codes_json,''), COALESCE(geosite_codes_json,''),
			outbound_tag, COALESCE(action,'route'), COALESCE(invert,0), created_at_ns, updated_at_ns
		 FROM core_route_rules WHERE server_id = ? ORDER BY priority ASC, created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RouteRule, 0)
	for rows.Next() {
		item, err := scanRouteRule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListEnabledRouteRules(ctx context.Context, serverID string) ([]RouteRule, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, enabled, priority,
			COALESCE(inbound_tags_json,''), COALESCE(protocols_json,''), COALESCE(domains_json,''),
			COALESCE(domain_suffixes_json,''), COALESCE(domain_keywords_json,''), COALESCE(ip_cidrs_json,''),
			COALESCE(ports_json,''), COALESCE(network,''), COALESCE(geoip_codes_json,''), COALESCE(geosite_codes_json,''),
			outbound_tag, COALESCE(action,'route'), COALESCE(invert,0), created_at_ns, updated_at_ns
		 FROM core_route_rules WHERE server_id = ? AND enabled = 1 ORDER BY priority ASC, created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RouteRule, 0)
	for rows.Next() {
		item, err := scanRouteRule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetRouteRule(ctx context.Context, id string) (RouteRule, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx),
		`SELECT id, server_id, enabled, priority,
			COALESCE(inbound_tags_json,''), COALESCE(protocols_json,''), COALESCE(domains_json,''),
			COALESCE(domain_suffixes_json,''), COALESCE(domain_keywords_json,''), COALESCE(ip_cidrs_json,''),
			COALESCE(ports_json,''), COALESCE(network,''), COALESCE(geoip_codes_json,''), COALESCE(geosite_codes_json,''),
			outbound_tag, COALESCE(action,'route'), COALESCE(invert,0), created_at_ns, updated_at_ns
		 FROM core_route_rules WHERE id = ? LIMIT 1`,
		normalizeString(id))
	return scanRouteRule(row)
}

func (s *Store) UpsertRouteRule(ctx context.Context, r RouteRule) (RouteRule, error) {
	now := time.Now().UTC()
	r.ID = normalizeString(r.ID)
	if r.ID == "" {
		r.ID = generateID()
	}
	r.ServerID = normalizeString(r.ServerID)
	r.OutboundTag = normalizeString(r.OutboundTag)
	if r.ServerID == "" || r.OutboundTag == "" {
		return RouteRule{}, fmt.Errorf("server_id and outbound_tag are required")
	}
	if strings.TrimSpace(r.Action) == "" {
		r.Action = "route"
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`INSERT INTO core_route_rules(
			id, server_id, enabled, priority,
			inbound_tags_json, protocols_json, domains_json, domain_suffixes_json, domain_keywords_json,
			ip_cidrs_json, ports_json, network, geoip_codes_json, geosite_codes_json,
			outbound_tag, action, invert, created_at_ns, updated_at_ns
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			enabled = excluded.enabled, priority = excluded.priority,
			inbound_tags_json = excluded.inbound_tags_json, protocols_json = excluded.protocols_json,
			domains_json = excluded.domains_json, domain_suffixes_json = excluded.domain_suffixes_json,
			domain_keywords_json = excluded.domain_keywords_json, ip_cidrs_json = excluded.ip_cidrs_json,
			ports_json = excluded.ports_json, network = excluded.network,
			geoip_codes_json = excluded.geoip_codes_json, geosite_codes_json = excluded.geosite_codes_json,
			outbound_tag = excluded.outbound_tag, action = excluded.action,
			invert = excluded.invert, updated_at_ns = excluded.updated_at_ns`,
		r.ID, r.ServerID, boolToInt(r.Enabled), r.Priority,
		nullIfEmpty(stringsToJSON(r.InboundTags)), nullIfEmpty(stringsToJSON(r.Protocols)),
		nullIfEmpty(stringsToJSON(r.Domains)), nullIfEmpty(stringsToJSON(r.DomainSuffixes)),
		nullIfEmpty(stringsToJSON(r.DomainKeywords)), nullIfEmpty(stringsToJSON(r.IPCIDRs)),
		nullIfEmpty(intsToJSON(r.Ports)), nullIfEmpty(r.Network),
		nullIfEmpty(stringsToJSON(r.GeoIPCodes)), nullIfEmpty(stringsToJSON(r.GeositeCodes)),
		r.OutboundTag, r.Action, boolToInt(r.Invert),
		toUnixNano(r.CreatedAt), toUnixNano(r.UpdatedAt))
	if err != nil {
		return RouteRule{}, parseUnique(err)
	}
	return s.GetRouteRule(ctx, r.ID)
}

func (s *Store) DeleteRouteRule(ctx context.Context, id string) error {
	return deleteByID(s, ctx, "core_route_rules", id)
}

func scanRouteRule(row interface{ Scan(dest ...any) error }) (RouteRule, error) {
	var (
		r                                       RouteRule
		enabled, invert                         int64
		inboundTagsJ, protocolsJ, domainsJ      string
		domainSuffixesJ, domainKwJ, ipCIDRsJ    string
		portsJ, network, geoipJ, geositeJ       string
		createdAt, updatedAt                    int64
	)
	if err := row.Scan(
		&r.ID, &r.ServerID, &enabled, &r.Priority,
		&inboundTagsJ, &protocolsJ, &domainsJ, &domainSuffixesJ, &domainKwJ, &ipCIDRsJ,
		&portsJ, &network, &geoipJ, &geositeJ,
		&r.OutboundTag, &r.Action, &invert,
		&createdAt, &updatedAt,
	); err != nil {
		return RouteRule{}, parseUnique(err)
	}
	r.Enabled = intToBool(enabled)
	r.Invert = intToBool(invert)
	r.Network = network
	r.InboundTags = jsonToStrings(inboundTagsJ)
	r.Protocols = jsonToStrings(protocolsJ)
	r.Domains = jsonToStrings(domainsJ)
	r.DomainSuffixes = jsonToStrings(domainSuffixesJ)
	r.DomainKeywords = jsonToStrings(domainKwJ)
	r.IPCIDRs = jsonToStrings(ipCIDRsJ)
	r.Ports = jsonToInts(portsJ)
	r.GeoIPCodes = jsonToStrings(geoipJ)
	r.GeositeCodes = jsonToStrings(geositeJ)
	r.CreatedAt = fromUnixNano(createdAt)
	r.UpdatedAt = fromUnixNano(updatedAt)
	return r, nil
}

// в”Ђв”Ђв”Ђ DNS Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Store) ListDNSProfiles(ctx context.Context, serverID string) ([]DNSProfile, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(strategy,''), disable_cache, COALESCE(final_server,''),
			COALESCE(servers_json,''), COALESCE(rules_json,''), fakeip_enabled, created_at_ns, updated_at_ns
		 FROM core_dns_profiles WHERE server_id = ? ORDER BY created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]DNSProfile, 0)
	for rows.Next() {
		item, err := scanDNSProfile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetDNSProfile(ctx context.Context, id string) (DNSProfile, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(strategy,''), disable_cache, COALESCE(final_server,''),
			COALESCE(servers_json,''), COALESCE(rules_json,''), fakeip_enabled, created_at_ns, updated_at_ns
		 FROM core_dns_profiles WHERE id = ? LIMIT 1`,
		normalizeString(id))
	return scanDNSProfile(row)
}

func (s *Store) UpsertDNSProfile(ctx context.Context, p DNSProfile) (DNSProfile, error) {
	now := time.Now().UTC()
	p.ID = normalizeString(p.ID)
	if p.ID == "" {
		p.ID = generateID()
	}
	p.ServerID = normalizeString(p.ServerID)
	p.Name = normalizeString(p.Name)
	if p.ServerID == "" || p.Name == "" {
		return DNSProfile{}, fmt.Errorf("server_id and name are required")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`INSERT INTO core_dns_profiles(id, server_id, name, enabled, strategy, disable_cache, final_server, servers_json, rules_json, fakeip_enabled, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, enabled = excluded.enabled, strategy = excluded.strategy,
			disable_cache = excluded.disable_cache, final_server = excluded.final_server,
			servers_json = excluded.servers_json, rules_json = excluded.rules_json,
			fakeip_enabled = excluded.fakeip_enabled, updated_at_ns = excluded.updated_at_ns`,
		p.ID, p.ServerID, p.Name, boolToInt(p.Enabled), nullIfEmpty(p.Strategy),
		boolToInt(p.DisableCache), nullIfEmpty(p.FinalServer),
		nullIfEmpty(p.ServersJSON), nullIfEmpty(p.RulesJSON),
		boolToInt(p.FakeIPEnabled),
		toUnixNano(p.CreatedAt), toUnixNano(p.UpdatedAt))
	if err != nil {
		return DNSProfile{}, parseUnique(err)
	}
	return s.GetDNSProfile(ctx, p.ID)
}

func (s *Store) DeleteDNSProfile(ctx context.Context, id string) error {
	return deleteByID(s, ctx, "core_dns_profiles", id)
}

func scanDNSProfile(row interface{ Scan(dest ...any) error }) (DNSProfile, error) {
	var (
		p                    DNSProfile
		enabled, disableCache, fakeIP int64
		createdAt, updatedAt          int64
	)
	if err := row.Scan(&p.ID, &p.ServerID, &p.Name, &enabled, &p.Strategy, &disableCache, &p.FinalServer,
		&p.ServersJSON, &p.RulesJSON, &fakeIP, &createdAt, &updatedAt); err != nil {
		return DNSProfile{}, parseUnique(err)
	}
	p.Enabled = intToBool(enabled)
	p.DisableCache = intToBool(disableCache)
	p.FakeIPEnabled = intToBool(fakeIP)
	p.CreatedAt = fromUnixNano(createdAt)
	p.UpdatedAt = fromUnixNano(updatedAt)
	return p, nil
}

// в”Ђв”Ђв”Ђ Log Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Store) ListLogProfiles(ctx context.Context, serverID string) ([]LogProfile, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(level,'warn'), COALESCE(output,''),
			timestamp, access_log_enabled, debug_mode, created_at_ns, updated_at_ns
		 FROM core_log_profiles WHERE server_id = ? ORDER BY created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]LogProfile, 0)
	for rows.Next() {
		item, err := scanLogProfile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetLogProfile(ctx context.Context, id string) (LogProfile, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(level,'warn'), COALESCE(output,''),
			timestamp, access_log_enabled, debug_mode, created_at_ns, updated_at_ns
		 FROM core_log_profiles WHERE id = ? LIMIT 1`,
		normalizeString(id))
	return scanLogProfile(row)
}

func (s *Store) UpsertLogProfile(ctx context.Context, p LogProfile) (LogProfile, error) {
	now := time.Now().UTC()
	p.ID = normalizeString(p.ID)
	if p.ID == "" {
		p.ID = generateID()
	}
	p.ServerID = normalizeString(p.ServerID)
	p.Name = normalizeString(p.Name)
	if p.ServerID == "" || p.Name == "" {
		return LogProfile{}, fmt.Errorf("server_id and name are required")
	}
	if strings.TrimSpace(p.Level) == "" {
		p.Level = "warn"
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`INSERT INTO core_log_profiles(id, server_id, name, enabled, level, output, timestamp, access_log_enabled, debug_mode, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, enabled = excluded.enabled, level = excluded.level,
			output = excluded.output, timestamp = excluded.timestamp,
			access_log_enabled = excluded.access_log_enabled, debug_mode = excluded.debug_mode,
			updated_at_ns = excluded.updated_at_ns`,
		p.ID, p.ServerID, p.Name, boolToInt(p.Enabled), p.Level, nullIfEmpty(p.Output),
		boolToInt(p.Timestamp), boolToInt(p.AccessLogEnabled), boolToInt(p.DebugMode),
		toUnixNano(p.CreatedAt), toUnixNano(p.UpdatedAt))
	if err != nil {
		return LogProfile{}, parseUnique(err)
	}
	return s.GetLogProfile(ctx, p.ID)
}

func (s *Store) DeleteLogProfile(ctx context.Context, id string) error {
	return deleteByID(s, ctx, "core_log_profiles", id)
}

func scanLogProfile(row interface{ Scan(dest ...any) error }) (LogProfile, error) {
	var (
		p                                     LogProfile
		enabled, ts, accessLog, debug, createdAt, updatedAt int64
	)
	if err := row.Scan(&p.ID, &p.ServerID, &p.Name, &enabled, &p.Level, &p.Output,
		&ts, &accessLog, &debug, &createdAt, &updatedAt); err != nil {
		return LogProfile{}, parseUnique(err)
	}
	p.Enabled = intToBool(enabled)
	p.Timestamp = intToBool(ts)
	p.AccessLogEnabled = intToBool(accessLog)
	p.DebugMode = intToBool(debug)
	p.CreatedAt = fromUnixNano(createdAt)
	p.UpdatedAt = fromUnixNano(updatedAt)
	return p, nil
}

// в”Ђв”Ђв”Ђ Reality Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Store) ListRealityProfiles(ctx context.Context, serverID string) ([]RealityProfile, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(server_name,''), handshake_server, handshake_server_port,
			COALESCE(private_key_enc,''), COALESCE(public_key,''), COALESCE(short_ids_json,''),
			COALESCE(short_id_rotation_mode,''), COALESCE(key_rotation_mode,''), created_at_ns, updated_at_ns
		 FROM core_reality_profiles WHERE server_id = ? ORDER BY created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RealityProfile, 0)
	for rows.Next() {
		item, err := scanRealityProfile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetRealityProfile(ctx context.Context, id string) (RealityProfile, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(server_name,''), handshake_server, handshake_server_port,
			COALESCE(private_key_enc,''), COALESCE(public_key,''), COALESCE(short_ids_json,''),
			COALESCE(short_id_rotation_mode,''), COALESCE(key_rotation_mode,''), created_at_ns, updated_at_ns
		 FROM core_reality_profiles WHERE id = ? LIMIT 1`,
		normalizeString(id))
	return scanRealityProfile(row)
}

func (s *Store) UpsertRealityProfile(ctx context.Context, p RealityProfile) (RealityProfile, error) {
	now := time.Now().UTC()
	p.ID = normalizeString(p.ID)
	if p.ID == "" {
		p.ID = generateID()
	}
	p.ServerID = normalizeString(p.ServerID)
	p.Name = normalizeString(p.Name)
	p.HandshakeServer = normalizeString(p.HandshakeServer)
	if p.ServerID == "" || p.Name == "" || p.HandshakeServer == "" {
		return RealityProfile{}, fmt.Errorf("server_id, name, and handshake_server are required")
	}
	if p.HandshakeServerPort <= 0 {
		p.HandshakeServerPort = 443
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`INSERT INTO core_reality_profiles(id, server_id, name, enabled, server_name, handshake_server, handshake_server_port, private_key_enc, public_key, short_ids_json, short_id_rotation_mode, key_rotation_mode, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, enabled = excluded.enabled, server_name = excluded.server_name,
			handshake_server = excluded.handshake_server, handshake_server_port = excluded.handshake_server_port,
			private_key_enc = excluded.private_key_enc, public_key = excluded.public_key,
			short_ids_json = excluded.short_ids_json,
			short_id_rotation_mode = excluded.short_id_rotation_mode,
			key_rotation_mode = excluded.key_rotation_mode, updated_at_ns = excluded.updated_at_ns`,
		p.ID, p.ServerID, p.Name, boolToInt(p.Enabled), nullIfEmpty(p.ServerName),
		p.HandshakeServer, p.HandshakeServerPort,
		nullIfEmpty(p.PrivateKey), nullIfEmpty(p.PublicKey),
		nullIfEmpty(stringsToJSON(p.ShortIDs)),
		nullIfEmpty(p.ShortIDRotationMode), nullIfEmpty(p.KeyRotationMode),
		toUnixNano(p.CreatedAt), toUnixNano(p.UpdatedAt))
	if err != nil {
		return RealityProfile{}, parseUnique(err)
	}
	return s.GetRealityProfile(ctx, p.ID)
}

func (s *Store) DeleteRealityProfile(ctx context.Context, id string) error {
	return deleteByID(s, ctx, "core_reality_profiles", id)
}

func scanRealityProfile(row interface{ Scan(dest ...any) error }) (RealityProfile, error) {
	var (
		p                    RealityProfile
		enabled              int64
		shortIDsJ            string
		createdAt, updatedAt int64
	)
	if err := row.Scan(&p.ID, &p.ServerID, &p.Name, &enabled, &p.ServerName,
		&p.HandshakeServer, &p.HandshakeServerPort, &p.PrivateKey, &p.PublicKey,
		&shortIDsJ, &p.ShortIDRotationMode, &p.KeyRotationMode,
		&createdAt, &updatedAt); err != nil {
		return RealityProfile{}, parseUnique(err)
	}
	p.Enabled = intToBool(enabled)
	p.ShortIDs = jsonToStrings(shortIDsJ)
	p.CreatedAt = fromUnixNano(createdAt)
	p.UpdatedAt = fromUnixNano(updatedAt)
	return p, nil
}

// в”Ђв”Ђв”Ђ Transport Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Store) ListTransportProfiles(ctx context.Context, serverID string) ([]TransportProfile, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(type,'tcp'), COALESCE(host,''), COALESCE(path,''),
			COALESCE(service_name,''), COALESCE(headers_json,''), COALESCE(idle_timeout,0), COALESCE(ping_timeout,0),
			created_at_ns, updated_at_ns
		 FROM core_transport_profiles WHERE server_id = ? ORDER BY created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TransportProfile, 0)
	for rows.Next() {
		item, err := scanTransportProfile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetTransportProfile(ctx context.Context, id string) (TransportProfile, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(type,'tcp'), COALESCE(host,''), COALESCE(path,''),
			COALESCE(service_name,''), COALESCE(headers_json,''), COALESCE(idle_timeout,0), COALESCE(ping_timeout,0),
			created_at_ns, updated_at_ns
		 FROM core_transport_profiles WHERE id = ? LIMIT 1`,
		normalizeString(id))
	return scanTransportProfile(row)
}

func (s *Store) UpsertTransportProfile(ctx context.Context, p TransportProfile) (TransportProfile, error) {
	now := time.Now().UTC()
	p.ID = normalizeString(p.ID)
	if p.ID == "" {
		p.ID = generateID()
	}
	p.ServerID = normalizeString(p.ServerID)
	p.Name = normalizeString(p.Name)
	if p.ServerID == "" || p.Name == "" {
		return TransportProfile{}, fmt.Errorf("server_id and name are required")
	}
	if strings.TrimSpace(p.Type) == "" {
		p.Type = "tcp"
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`INSERT INTO core_transport_profiles(id, server_id, name, enabled, type, host, path, service_name, headers_json, idle_timeout, ping_timeout, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, enabled = excluded.enabled, type = excluded.type, host = excluded.host,
			path = excluded.path, service_name = excluded.service_name, headers_json = excluded.headers_json,
			idle_timeout = excluded.idle_timeout, ping_timeout = excluded.ping_timeout,
			updated_at_ns = excluded.updated_at_ns`,
		p.ID, p.ServerID, p.Name, boolToInt(p.Enabled), p.Type,
		nullIfEmpty(p.Host), nullIfEmpty(p.Path), nullIfEmpty(p.ServiceName),
		nullIfEmpty(p.HeadersJSON), nullableIntVal(p.IdleTimeout), nullableIntVal(p.PingTimeout),
		toUnixNano(p.CreatedAt), toUnixNano(p.UpdatedAt))
	if err != nil {
		return TransportProfile{}, parseUnique(err)
	}
	return s.GetTransportProfile(ctx, p.ID)
}

func (s *Store) DeleteTransportProfile(ctx context.Context, id string) error {
	return deleteByID(s, ctx, "core_transport_profiles", id)
}

func scanTransportProfile(row interface{ Scan(dest ...any) error }) (TransportProfile, error) {
	var (
		p                    TransportProfile
		enabled              int64
		createdAt, updatedAt int64
	)
	if err := row.Scan(&p.ID, &p.ServerID, &p.Name, &enabled, &p.Type, &p.Host, &p.Path,
		&p.ServiceName, &p.HeadersJSON, &p.IdleTimeout, &p.PingTimeout,
		&createdAt, &updatedAt); err != nil {
		return TransportProfile{}, parseUnique(err)
	}
	p.Enabled = intToBool(enabled)
	p.CreatedAt = fromUnixNano(createdAt)
	p.UpdatedAt = fromUnixNano(updatedAt)
	return p, nil
}

// в”Ђв”Ђв”Ђ Multiplex Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Store) ListMultiplexProfiles(ctx context.Context, serverID string) ([]MultiplexProfile, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(protocol,''), COALESCE(max_connections,0),
			COALESCE(min_streams,0), COALESCE(max_streams,0), padding, brutal, created_at_ns, updated_at_ns
		 FROM core_multiplex_profiles WHERE server_id = ? ORDER BY created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MultiplexProfile, 0)
	for rows.Next() {
		item, err := scanMultiplexProfile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetMultiplexProfile(ctx context.Context, id string) (MultiplexProfile, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(protocol,''), COALESCE(max_connections,0),
			COALESCE(min_streams,0), COALESCE(max_streams,0), padding, brutal, created_at_ns, updated_at_ns
		 FROM core_multiplex_profiles WHERE id = ? LIMIT 1`,
		normalizeString(id))
	return scanMultiplexProfile(row)
}

func (s *Store) UpsertMultiplexProfile(ctx context.Context, p MultiplexProfile) (MultiplexProfile, error) {
	now := time.Now().UTC()
	p.ID = normalizeString(p.ID)
	if p.ID == "" {
		p.ID = generateID()
	}
	p.ServerID = normalizeString(p.ServerID)
	p.Name = normalizeString(p.Name)
	if p.ServerID == "" || p.Name == "" {
		return MultiplexProfile{}, fmt.Errorf("server_id and name are required")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`INSERT INTO core_multiplex_profiles(id, server_id, name, enabled, protocol, max_connections, min_streams, max_streams, padding, brutal, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, enabled = excluded.enabled, protocol = excluded.protocol,
			max_connections = excluded.max_connections, min_streams = excluded.min_streams,
			max_streams = excluded.max_streams, padding = excluded.padding, brutal = excluded.brutal,
			updated_at_ns = excluded.updated_at_ns`,
		p.ID, p.ServerID, p.Name, boolToInt(p.Enabled), nullIfEmpty(p.Protocol),
		nullableIntVal(p.MaxConnections), nullableIntVal(p.MinStreams), nullableIntVal(p.MaxStreams),
		boolToInt(p.Padding), boolToInt(p.Brutal),
		toUnixNano(p.CreatedAt), toUnixNano(p.UpdatedAt))
	if err != nil {
		return MultiplexProfile{}, parseUnique(err)
	}
	return s.GetMultiplexProfile(ctx, p.ID)
}

func (s *Store) DeleteMultiplexProfile(ctx context.Context, id string) error {
	return deleteByID(s, ctx, "core_multiplex_profiles", id)
}

func scanMultiplexProfile(row interface{ Scan(dest ...any) error }) (MultiplexProfile, error) {
	var (
		p                    MultiplexProfile
		enabled, padding, brutal int64
		createdAt, updatedAt int64
	)
	if err := row.Scan(&p.ID, &p.ServerID, &p.Name, &enabled, &p.Protocol,
		&p.MaxConnections, &p.MinStreams, &p.MaxStreams, &padding, &brutal,
		&createdAt, &updatedAt); err != nil {
		return MultiplexProfile{}, parseUnique(err)
	}
	p.Enabled = intToBool(enabled)
	p.Padding = intToBool(padding)
	p.Brutal = intToBool(brutal)
	p.CreatedAt = fromUnixNano(createdAt)
	p.UpdatedAt = fromUnixNano(updatedAt)
	return p, nil
}

// в”Ђв”Ђв”Ђ HY2 Masquerade Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Store) ListHY2MasqueradeProfiles(ctx context.Context, serverID string) ([]HY2MasqueradeProfile, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(type,'off'), COALESCE(url,''),
			COALESCE(rewrite_host,0), COALESCE(directory,''), COALESCE(status_code,0),
			COALESCE(headers_json,''), COALESCE(content,''), created_at_ns, updated_at_ns
		 FROM core_hy2_masquerade_profiles WHERE server_id = ? ORDER BY created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HY2MasqueradeProfile, 0)
	for rows.Next() {
		item, err := scanHY2MasqueradeProfile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetHY2MasqueradeProfile(ctx context.Context, id string) (HY2MasqueradeProfile, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(type,'off'), COALESCE(url,''),
			COALESCE(rewrite_host,0), COALESCE(directory,''), COALESCE(status_code,0),
			COALESCE(headers_json,''), COALESCE(content,''), created_at_ns, updated_at_ns
		 FROM core_hy2_masquerade_profiles WHERE id = ? LIMIT 1`,
		normalizeString(id))
	return scanHY2MasqueradeProfile(row)
}

func (s *Store) UpsertHY2MasqueradeProfile(ctx context.Context, p HY2MasqueradeProfile) (HY2MasqueradeProfile, error) {
	now := time.Now().UTC()
	p.ID = normalizeString(p.ID)
	if p.ID == "" {
		p.ID = generateID()
	}
	p.ServerID = normalizeString(p.ServerID)
	p.Name = normalizeString(p.Name)
	if p.ServerID == "" || p.Name == "" {
		return HY2MasqueradeProfile{}, fmt.Errorf("server_id and name are required")
	}
	if strings.TrimSpace(p.Type) == "" {
		p.Type = "off"
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`INSERT INTO core_hy2_masquerade_profiles(id, server_id, name, enabled, type, url, rewrite_host, directory, status_code, headers_json, content, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, enabled = excluded.enabled, type = excluded.type, url = excluded.url,
			rewrite_host = excluded.rewrite_host, directory = excluded.directory,
			status_code = excluded.status_code, headers_json = excluded.headers_json,
			content = excluded.content, updated_at_ns = excluded.updated_at_ns`,
		p.ID, p.ServerID, p.Name, boolToInt(p.Enabled), p.Type,
		nullIfEmpty(p.URL), boolToInt(p.RewriteHost), nullIfEmpty(p.Directory),
		nullableIntVal(p.StatusCode), nullIfEmpty(p.HeadersJSON), nullIfEmpty(p.Content),
		toUnixNano(p.CreatedAt), toUnixNano(p.UpdatedAt))
	if err != nil {
		return HY2MasqueradeProfile{}, parseUnique(err)
	}
	return s.GetHY2MasqueradeProfile(ctx, p.ID)
}

func (s *Store) DeleteHY2MasqueradeProfile(ctx context.Context, id string) error {
	return deleteByID(s, ctx, "core_hy2_masquerade_profiles", id)
}

func scanHY2MasqueradeProfile(row interface{ Scan(dest ...any) error }) (HY2MasqueradeProfile, error) {
	var (
		p                    HY2MasqueradeProfile
		enabled, rewrite     int64
		createdAt, updatedAt int64
	)
	if err := row.Scan(&p.ID, &p.ServerID, &p.Name, &enabled, &p.Type, &p.URL,
		&rewrite, &p.Directory, &p.StatusCode, &p.HeadersJSON, &p.Content,
		&createdAt, &updatedAt); err != nil {
		return HY2MasqueradeProfile{}, parseUnique(err)
	}
	p.Enabled = intToBool(enabled)
	p.RewriteHost = intToBool(rewrite)
	p.CreatedAt = fromUnixNano(createdAt)
	p.UpdatedAt = fromUnixNano(updatedAt)
	return p, nil
}

// в”Ђв”Ђв”Ђ TLS Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Store) ListTLSProfiles(ctx context.Context, serverID string) ([]TLSProfile, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(server_name,''), COALESCE(alpn_json,''),
			COALESCE(certificate_path,''), COALESCE(key_path,''), allow_insecure, created_at_ns, updated_at_ns
		 FROM core_tls_profiles WHERE server_id = ? ORDER BY created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TLSProfile, 0)
	for rows.Next() {
		item, err := scanTLSProfile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetTLSProfile(ctx context.Context, id string) (TLSProfile, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx),
		`SELECT id, server_id, name, enabled, COALESCE(server_name,''), COALESCE(alpn_json,''),
			COALESCE(certificate_path,''), COALESCE(key_path,''), allow_insecure, created_at_ns, updated_at_ns
		 FROM core_tls_profiles WHERE id = ? LIMIT 1`,
		normalizeString(id))
	return scanTLSProfile(row)
}

func (s *Store) UpsertTLSProfile(ctx context.Context, p TLSProfile) (TLSProfile, error) {
	now := time.Now().UTC()
	p.ID = normalizeString(p.ID)
	if p.ID == "" {
		p.ID = generateID()
	}
	p.ServerID = normalizeString(p.ServerID)
	p.Name = normalizeString(p.Name)
	if p.ServerID == "" || p.Name == "" {
		return TLSProfile{}, fmt.Errorf("server_id and name are required")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`INSERT INTO core_tls_profiles(id, server_id, name, enabled, server_name, alpn_json, certificate_path, key_path, allow_insecure, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, enabled = excluded.enabled, server_name = excluded.server_name,
			alpn_json = excluded.alpn_json, certificate_path = excluded.certificate_path,
			key_path = excluded.key_path, allow_insecure = excluded.allow_insecure,
			updated_at_ns = excluded.updated_at_ns`,
		p.ID, p.ServerID, p.Name, boolToInt(p.Enabled), nullIfEmpty(p.ServerName),
		nullIfEmpty(stringsToJSON(p.ALPN)), nullIfEmpty(p.CertificatePath), nullIfEmpty(p.KeyPath),
		boolToInt(p.AllowInsecure), toUnixNano(p.CreatedAt), toUnixNano(p.UpdatedAt))
	if err != nil {
		return TLSProfile{}, parseUnique(err)
	}
	return s.GetTLSProfile(ctx, p.ID)
}

func (s *Store) DeleteTLSProfile(ctx context.Context, id string) error {
	return deleteByID(s, ctx, "core_tls_profiles", id)
}

func scanTLSProfile(row interface{ Scan(dest ...any) error }) (TLSProfile, error) {
	var (
		p                    TLSProfile
		enabled, allowInsecure int64
		alpnJSON             string
		createdAt, updatedAt int64
	)
	if err := row.Scan(&p.ID, &p.ServerID, &p.Name, &enabled, &p.ServerName, &alpnJSON,
		&p.CertificatePath, &p.KeyPath, &allowInsecure, &createdAt, &updatedAt); err != nil {
		return TLSProfile{}, parseUnique(err)
	}
	p.Enabled = intToBool(enabled)
	p.AllowInsecure = intToBool(allowInsecure)
	p.ALPN = jsonToStrings(alpnJSON)
	p.CreatedAt = fromUnixNano(createdAt)
	p.UpdatedAt = fromUnixNano(updatedAt)
	return p, nil
}
// в”Ђв”Ђв”Ђ Client Profiles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func (s *Store) ListClientProfiles(ctx context.Context, serverID string) ([]ClientProfile, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx),
		`SELECT id, server_id, name, protocol, mode, COALESCE(description,''), COALESCE(settings_json,''),
			enabled, created_at_ns, updated_at_ns
		 FROM core_client_profiles WHERE server_id = ? ORDER BY created_at_ns ASC`,
		normalizeString(serverID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ClientProfile, 0)
	for rows.Next() {
		item, err := scanClientProfile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetClientProfile(ctx context.Context, id string) (ClientProfile, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx),
		`SELECT id, server_id, name, protocol, mode, COALESCE(description,''), COALESCE(settings_json,''),
			enabled, created_at_ns, updated_at_ns
		 FROM core_client_profiles WHERE id = ? LIMIT 1`,
		normalizeString(id))
	return scanClientProfile(row)
}

func (s *Store) UpsertClientProfile(ctx context.Context, p ClientProfile) (ClientProfile, error) {
	now := time.Now().UTC()
	p.ID = normalizeString(p.ID)
	if p.ID == "" {
		p.ID = generateID()
	}
	p.ServerID = normalizeString(p.ServerID)
	p.Name = normalizeString(p.Name)
	p.Mode = normalizeString(p.Mode)
	if p.ServerID == "" || p.Name == "" || p.Mode == "" {
		return ClientProfile{}, fmt.Errorf("server_id, name, and mode are required")
	}
	if p.Protocol != InboundProtocolVLESS && p.Protocol != InboundProtocolHysteria2 {
		return ClientProfile{}, fmt.Errorf("unsupported protocol")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`INSERT INTO core_client_profiles(id, server_id, name, protocol, mode, description, settings_json, enabled, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, protocol = excluded.protocol, mode = excluded.mode,
			description = excluded.description, settings_json = excluded.settings_json,
			enabled = excluded.enabled, updated_at_ns = excluded.updated_at_ns`,
		p.ID, p.ServerID, p.Name, string(p.Protocol), p.Mode,
		nullIfEmpty(p.Description), nullIfEmpty(p.SettingsJSON), boolToInt(p.Enabled),
		toUnixNano(p.CreatedAt), toUnixNano(p.UpdatedAt))
	if err != nil {
		return ClientProfile{}, parseUnique(err)
	}
	return s.GetClientProfile(ctx, p.ID)
}

func (s *Store) DeleteClientProfile(ctx context.Context, id string) error {
	return deleteByID(s, ctx, "core_client_profiles", id)
}

func scanClientProfile(row interface{ Scan(dest ...any) error }) (ClientProfile, error) {
	var (
		p                    ClientProfile
		protocol             string
		enabled              int64
		createdAt, updatedAt int64
	)
	if err := row.Scan(&p.ID, &p.ServerID, &p.Name, &protocol, &p.Mode, &p.Description,
		&p.SettingsJSON, &enabled, &createdAt, &updatedAt); err != nil {
		return ClientProfile{}, parseUnique(err)
	}
	p.Protocol = InboundProtocol(protocol)
	p.Enabled = intToBool(enabled)
	p.CreatedAt = fromUnixNano(createdAt)
	p.UpdatedAt = fromUnixNano(updatedAt)
	return p, nil
}

// в”Ђв”Ђв”Ђ shared helpers в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

// deleteByID deletes a row from tableName by id and returns ErrNotFound when missing.
func deleteByID(s *Store, ctx context.Context, tableName string, id string) error {
	result, err := s.db.ExecContext(resolveCtx(ctx),
		`DELETE FROM `+tableName+` WHERE id = ?`, normalizeString(id))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// masqueradeProfileToJSON converts a HY2MasqueradeProfile into the JSON fragment
// expected by the sing-box masquerade field.
func masqueradeProfileToJSON(p HY2MasqueradeProfile) (string, error) {
	if !p.Enabled || p.Type == "off" || p.Type == "" {
		return "", nil
	}
	m := map[string]any{"type": p.Type}
	switch p.Type {
	case "proxy":
		if strings.TrimSpace(p.URL) != "" {
			m["url"] = strings.TrimSpace(p.URL)
			m["rewrite_host"] = p.RewriteHost
		}
	case "file":
		if strings.TrimSpace(p.Directory) != "" {
			m["dir"] = strings.TrimSpace(p.Directory)
		}
	case "string":
		if strings.TrimSpace(p.Content) != "" {
			m["content"] = strings.TrimSpace(p.Content)
		}
		if p.StatusCode > 0 {
			m["status_code"] = p.StatusCode
		}
		if strings.TrimSpace(p.HeadersJSON) != "" {
			var hdrs map[string]string
			if err := json.Unmarshal([]byte(p.HeadersJSON), &hdrs); err == nil {
				m["headers"] = hdrs
			}
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}


