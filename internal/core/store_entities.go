package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)
func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx), `SELECT id, name, public_host, panel_public_url, subscription_base_url, singbox_binary_path, singbox_config_path, singbox_service_name, created_at_ns, updated_at_ns FROM core_servers ORDER BY created_at_ns DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Server, 0)
	for rows.Next() {
		item, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetServer(ctx context.Context, id string) (Server, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT id, name, public_host, panel_public_url, subscription_base_url, singbox_binary_path, singbox_config_path, singbox_service_name, created_at_ns, updated_at_ns FROM core_servers WHERE id = ? LIMIT 1`, normalizeString(id))
	item, err := scanServer(row)
	if err != nil {
		return Server{}, err
	}
	return item, nil
}

func (s *Store) UpsertServer(ctx context.Context, server Server) (Server, error) {
	now := time.Now().UTC()
	server.ID = normalizeString(server.ID)
	if server.ID == "" {
		server.ID = generateID()
	}
	server.Name = normalizeString(server.Name)
	if server.Name == "" {
		return Server{}, fmt.Errorf("server name is required")
	}
	server.PublicHost = normalizeString(server.PublicHost)
	if server.PublicHost == "" {
		return Server{}, fmt.Errorf("server public_host is required")
	}
	if normalizeClientEndpointHost(server.PublicHost) == "" {
		return Server{}, fmt.Errorf("server public_host must be a public host")
	}
	server.PanelPublicURL = normalizeString(server.PanelPublicURL)
	if server.PanelPublicURL == "" {
		return Server{}, fmt.Errorf("server panel_public_url is required")
	}
	server.SubscriptionBaseURL = normalizeString(server.SubscriptionBaseURL)
	if server.SubscriptionBaseURL == "" {
		return Server{}, fmt.Errorf("server subscription_base_url is required")
	}
	server.SingBoxBinaryPath = normalizeString(server.SingBoxBinaryPath)
	if server.SingBoxBinaryPath == "" {
		return Server{}, fmt.Errorf("server singbox_binary_path is required")
	}
	server.SingBoxConfigPath = normalizeString(server.SingBoxConfigPath)
	if server.SingBoxConfigPath == "" {
		return Server{}, fmt.Errorf("server singbox_config_path is required")
	}
	server.SingBoxServiceName = normalizeString(server.SingBoxServiceName)
	if server.SingBoxServiceName == "" {
		return Server{}, fmt.Errorf("server singbox_service_name is required")
	}

	if server.CreatedAt.IsZero() {
		server.CreatedAt = now
	}
	server.UpdatedAt = now

	_, err := s.db.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO core_servers(id, name, public_host, panel_public_url, subscription_base_url, singbox_binary_path, singbox_config_path, singbox_service_name, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			public_host = excluded.public_host,
			panel_public_url = excluded.panel_public_url,
			subscription_base_url = excluded.subscription_base_url,
			singbox_binary_path = excluded.singbox_binary_path,
			singbox_config_path = excluded.singbox_config_path,
			singbox_service_name = excluded.singbox_service_name,
			updated_at_ns = excluded.updated_at_ns`,
		server.ID,
		server.Name,
		server.PublicHost,
		server.PanelPublicURL,
		server.SubscriptionBaseURL,
		server.SingBoxBinaryPath,
		server.SingBoxConfigPath,
		server.SingBoxServiceName,
		toUnixNano(server.CreatedAt),
		toUnixNano(server.UpdatedAt),
	)
	if err != nil {
		return Server{}, parseUnique(err)
	}
	return s.GetServer(ctx, server.ID)
}

func (s *Store) DeleteServer(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(resolveCtx(ctx), `DELETE FROM core_servers WHERE id = ?`, normalizeString(id))
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

func (s *Store) ListInbounds(ctx context.Context, serverID string) ([]Inbound, error) {
	query := `SELECT id, server_id, name, tag, protocol, listen, listen_port, enabled, template_key, COALESCE(notes,''), COALESCE(labels_json,''), COALESCE(sort_order,0), COALESCE(log_profile_id,''), created_at_ns, updated_at_ns FROM core_inbounds`
	args := make([]any, 0, 1)
	if normalizeString(serverID) != "" {
		query += ` WHERE server_id = ?`
		args = append(args, normalizeString(serverID))
	}
	query += ` ORDER BY COALESCE(sort_order,0) ASC, created_at_ns DESC`

	rows, err := s.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Inbound, 0)
	for rows.Next() {
		item, err := scanInboundBase(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range items {
		if err := s.populateInboundSettings(ctx, &items[index]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) GetInbound(ctx context.Context, id string) (Inbound, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT id, server_id, name, tag, protocol, listen, listen_port, enabled, template_key, COALESCE(notes,''), COALESCE(labels_json,''), COALESCE(sort_order,0), COALESCE(log_profile_id,''), created_at_ns, updated_at_ns FROM core_inbounds WHERE id = ? LIMIT 1`, normalizeString(id))
	item, err := scanInboundBase(row)
	if err != nil {
		return Inbound{}, err
	}
	if err := s.populateInboundSettings(ctx, &item); err != nil {
		return Inbound{}, err
	}
	return item, nil
}
func (s *Store) UpsertInbound(ctx context.Context, inbound Inbound) (Inbound, error) {
	now := time.Now().UTC()
	inbound.ID = normalizeString(inbound.ID)
	if inbound.ID == "" {
		inbound.ID = generateID()
	}
	inbound.ServerID = normalizeString(inbound.ServerID)
	if inbound.ServerID == "" {
		return Inbound{}, fmt.Errorf("inbound server_id is required")
	}
	inbound.Name = normalizeString(inbound.Name)
	if inbound.Name == "" {
		return Inbound{}, fmt.Errorf("inbound name is required")
	}
	inbound.Tag = normalizeString(inbound.Tag)
	if inbound.Tag == "" {
		inbound.Tag = strings.ToLower(strings.ReplaceAll(inbound.Name, " ", "-"))
	}
	if inbound.Protocol != InboundProtocolVLESS && inbound.Protocol != InboundProtocolHysteria2 {
		return Inbound{}, fmt.Errorf("unsupported inbound protocol")
	}
	inbound.Listen = normalizeString(inbound.Listen)
	if inbound.Listen == "" {
		inbound.Listen = "::"
	}
	if inbound.ListenPort <= 0 {
		inbound.ListenPort = 443
	}
	if strings.TrimSpace(inbound.TemplateKey) == "" {
		if inbound.Protocol == InboundProtocolVLESS {
			inbound.TemplateKey = "vless-reality-vision"
		} else {
			inbound.TemplateKey = "hysteria2-default"
		}
	}
	if inbound.CreatedAt.IsZero() {
		inbound.CreatedAt = now
	}
	inbound.UpdatedAt = now

	tx, err := s.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return Inbound{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO core_inbounds(id, server_id, name, tag, protocol, listen, listen_port, enabled, template_key, notes, labels_json, sort_order, log_profile_id, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			server_id = excluded.server_id,
			name = excluded.name,
			tag = excluded.tag,
			protocol = excluded.protocol,
			listen = excluded.listen,
			listen_port = excluded.listen_port,
			enabled = excluded.enabled,
			template_key = excluded.template_key,
			notes = excluded.notes,
			labels_json = excluded.labels_json,
			sort_order = excluded.sort_order,
			log_profile_id = excluded.log_profile_id,
			updated_at_ns = excluded.updated_at_ns`,
		inbound.ID,
		inbound.ServerID,
		inbound.Name,
		inbound.Tag,
		string(inbound.Protocol),
		inbound.Listen,
		inbound.ListenPort,
		boolToInt(inbound.Enabled),
		inbound.TemplateKey,
		nullIfEmpty(inbound.Notes),
		nullIfEmpty(stringsToJSON(inbound.Labels)),
		inbound.SortOrder,
		nullIfEmpty(inbound.LogProfileID),
		toUnixNano(inbound.CreatedAt),
		toUnixNano(inbound.UpdatedAt),
	)
	if err != nil {
		return Inbound{}, parseUnique(err)
	}

	if inbound.Protocol == InboundProtocolVLESS {
		if inbound.VLESS == nil {
			inbound.VLESS = &VLESSInboundSettings{}
		}
		if strings.TrimSpace(inbound.VLESS.TransportType) == "" {
			inbound.VLESS.TransportType = "tcp"
		}
		if inbound.VLESS.RealityEnabled {
			if strings.TrimSpace(inbound.VLESS.RealityHandshakeServer) == "" {
				inbound.VLESS.RealityHandshakeServer = "www.cloudflare.com"
			}
			if inbound.VLESS.RealityHandshakeServerPort <= 0 {
				inbound.VLESS.RealityHandshakeServerPort = 443
			}
		}
		_, err = tx.ExecContext(
			resolveCtx(ctx),
			`INSERT INTO core_inbound_vless_settings(
				inbound_id, tls_enabled, tls_server_name, tls_alpn_csv, tls_certificate_path, tls_key_path,
				reality_enabled, reality_public_key, reality_private_key_enc, reality_short_id, reality_handshake_server, reality_handshake_server_port,
				flow, transport_type, transport_host, transport_path,
				multiplex_enabled, multiplex_protocol, multiplex_max_connections, multiplex_min_streams, multiplex_max_streams,
				reality_profile_id, transport_profile_id, multiplex_profile_id, packet_encoding_default
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(inbound_id) DO UPDATE SET
				tls_enabled = excluded.tls_enabled,
				tls_server_name = excluded.tls_server_name,
				tls_alpn_csv = excluded.tls_alpn_csv,
				tls_certificate_path = excluded.tls_certificate_path,
				tls_key_path = excluded.tls_key_path,
				reality_enabled = excluded.reality_enabled,
				reality_public_key = excluded.reality_public_key,
				reality_private_key_enc = excluded.reality_private_key_enc,
				reality_short_id = excluded.reality_short_id,
				reality_handshake_server = excluded.reality_handshake_server,
				reality_handshake_server_port = excluded.reality_handshake_server_port,
				flow = excluded.flow,
				transport_type = excluded.transport_type,
				transport_host = excluded.transport_host,
				transport_path = excluded.transport_path,
				multiplex_enabled = excluded.multiplex_enabled,
				multiplex_protocol = excluded.multiplex_protocol,
				multiplex_max_connections = excluded.multiplex_max_connections,
				multiplex_min_streams = excluded.multiplex_min_streams,
				multiplex_max_streams = excluded.multiplex_max_streams,
				reality_profile_id = excluded.reality_profile_id,
				transport_profile_id = excluded.transport_profile_id,
				multiplex_profile_id = excluded.multiplex_profile_id,
				packet_encoding_default = excluded.packet_encoding_default`,
			inbound.ID,
			boolToInt(inbound.VLESS.TLSEnabled),
			nullIfEmpty(inbound.VLESS.TLSServerName),
			nullIfEmpty(joinCSV(inbound.VLESS.TLSALPN)),
			nullIfEmpty(inbound.VLESS.TLSCertificatePath),
			nullIfEmpty(inbound.VLESS.TLSKeyPath),
			boolToInt(inbound.VLESS.RealityEnabled),
			nullIfEmpty(inbound.VLESS.RealityPublicKey),
			nullIfEmpty(inbound.VLESS.RealityPrivateKey),
			nullIfEmpty(inbound.VLESS.RealityShortID),
			nullIfEmpty(inbound.VLESS.RealityHandshakeServer),
			inbound.VLESS.RealityHandshakeServerPort,
			nullIfEmpty(inbound.VLESS.Flow),
			inbound.VLESS.TransportType,
			nullIfEmpty(inbound.VLESS.TransportHost),
			nullIfEmpty(inbound.VLESS.TransportPath),
			boolToInt(inbound.VLESS.MultiplexEnabled),
			nullIfEmpty(inbound.VLESS.MultiplexProtocol),
			inbound.VLESS.MultiplexMaxConnections,
			inbound.VLESS.MultiplexMinStreams,
			inbound.VLESS.MultiplexMaxStreams,
			nullIfEmpty(inbound.VLESS.RealityProfileID),
			nullIfEmpty(inbound.VLESS.TransportProfileID),
			nullIfEmpty(inbound.VLESS.MultiplexProfileID),
			nullIfEmpty(inbound.VLESS.PacketEncodingDefault),
		)
		if err != nil {
			return Inbound{}, err
		}
		if _, err = tx.ExecContext(resolveCtx(ctx), `DELETE FROM core_inbound_hysteria2_settings WHERE inbound_id = ?`, inbound.ID); err != nil {
			return Inbound{}, err
		}
	} else {
		if inbound.Hysteria2 == nil {
			inbound.Hysteria2 = &Hysteria2InboundSettings{}
		}
		_, err = tx.ExecContext(
			resolveCtx(ctx),
			`INSERT INTO core_inbound_hysteria2_settings(
				inbound_id, tls_enabled, tls_server_name, tls_certificate_path, tls_key_path,
				allow_insecure, up_mbps, down_mbps, ignore_client_bandwidth, obfs_type, obfs_password_enc, masquerade_json, bbr_profile, brutal_debug,
				masquerade_profile_id, server_ports, hop_interval, network, tls_alpn_csv, bandwidth_profile_mode
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(inbound_id) DO UPDATE SET
				tls_enabled = excluded.tls_enabled,
				tls_server_name = excluded.tls_server_name,
				tls_certificate_path = excluded.tls_certificate_path,
				tls_key_path = excluded.tls_key_path,
				allow_insecure = excluded.allow_insecure,
				up_mbps = excluded.up_mbps,
				down_mbps = excluded.down_mbps,
				ignore_client_bandwidth = excluded.ignore_client_bandwidth,
				obfs_type = excluded.obfs_type,
				obfs_password_enc = excluded.obfs_password_enc,
				masquerade_json = excluded.masquerade_json,
				bbr_profile = excluded.bbr_profile,
				brutal_debug = excluded.brutal_debug,
				masquerade_profile_id = excluded.masquerade_profile_id,
				server_ports = excluded.server_ports,
				hop_interval = excluded.hop_interval,
				network = excluded.network,
				tls_alpn_csv = excluded.tls_alpn_csv,
				bandwidth_profile_mode = excluded.bandwidth_profile_mode`,
			inbound.ID,
			boolToInt(inbound.Hysteria2.TLSEnabled),
			nullIfEmpty(inbound.Hysteria2.TLSServerName),
			nullIfEmpty(inbound.Hysteria2.TLSCertificatePath),
			nullIfEmpty(inbound.Hysteria2.TLSKeyPath),
			boolToInt(inbound.Hysteria2.AllowInsecure),
			nullableInt(inbound.Hysteria2.UpMbps),
			nullableInt(inbound.Hysteria2.DownMbps),
			boolToInt(inbound.Hysteria2.IgnoreClientBandwidth),
			nullIfEmpty(inbound.Hysteria2.ObfsType),
			nullIfEmpty(inbound.Hysteria2.ObfsPassword),
			nullIfEmpty(inbound.Hysteria2.MasqueradeJSON),
			nullIfEmpty(inbound.Hysteria2.BBRProfile),
			boolToInt(inbound.Hysteria2.BrutalDebug),
			nullIfEmpty(inbound.Hysteria2.MasqueradeProfileID),
			nullIfEmpty(inbound.Hysteria2.ServerPorts),
			nullableIntVal(inbound.Hysteria2.HopInterval),
			nullIfEmpty(inbound.Hysteria2.Network),
			nullIfEmpty(joinCSV(inbound.Hysteria2.TLSALPN)),
			nullIfEmpty(inbound.Hysteria2.BandwidthProfileMode),
		)
		if err != nil {
			return Inbound{}, err
		}
		if _, err = tx.ExecContext(resolveCtx(ctx), `DELETE FROM core_inbound_vless_settings WHERE inbound_id = ?`, inbound.ID); err != nil {
			return Inbound{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return Inbound{}, err
	}
	return s.GetInbound(ctx, inbound.ID)
}

func (s *Store) DeleteInbound(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(resolveCtx(ctx), `DELETE FROM core_inbounds WHERE id = ?`, normalizeString(id))
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

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx), `SELECT id, username, enabled, traffic_limit_bytes, traffic_used_up_bytes, traffic_used_down_bytes, expire_at_ns, created_at_ns, updated_at_ns FROM core_users ORDER BY created_at_ns DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]User, 0)
	for rows.Next() {
		item, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetUser(ctx context.Context, id string) (User, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT id, username, enabled, traffic_limit_bytes, traffic_used_up_bytes, traffic_used_down_bytes, expire_at_ns, created_at_ns, updated_at_ns FROM core_users WHERE id = ? LIMIT 1`, normalizeString(id))
	item, err := scanUser(row)
	if err != nil {
		return User{}, err
	}
	return item, nil
}

func (s *Store) UpsertUser(ctx context.Context, user User) (User, error) {
	now := time.Now().UTC()
	user.ID = normalizeString(user.ID)
	if user.ID == "" {
		user.ID = generateID()
	}
	user.Username = strings.ToLower(normalizeString(user.Username))
	if user.Username == "" {
		return User{}, fmt.Errorf("username is required")
	}
	if user.TrafficLimitBytes < 0 {
		return User{}, fmt.Errorf("traffic_limit_bytes must be >= 0")
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	_, err := s.db.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO core_users(id, username, enabled, traffic_limit_bytes, traffic_used_up_bytes, traffic_used_down_bytes, expire_at_ns, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			username = excluded.username,
			enabled = excluded.enabled,
			traffic_limit_bytes = excluded.traffic_limit_bytes,
			expire_at_ns = excluded.expire_at_ns,
			updated_at_ns = excluded.updated_at_ns`,
		user.ID,
		user.Username,
		boolToInt(user.Enabled),
		user.TrafficLimitBytes,
		user.TrafficUsedUpBytes,
		user.TrafficUsedDownBytes,
		nullableTime(user.ExpireAt),
		toUnixNano(user.CreatedAt),
		toUnixNano(user.UpdatedAt),
	)
	if err != nil {
		return User{}, parseUnique(err)
	}
	return s.GetUser(ctx, user.ID)
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(resolveCtx(ctx), `DELETE FROM core_users WHERE id = ?`, normalizeString(id))
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
func (s *Store) UpsertUserAccess(ctx context.Context, access UserAccess) (UserAccess, error) {
	now := time.Now().UTC()
	access.ID = normalizeString(access.ID)
	if access.ID == "" {
		access.ID = generateID()
	}
	access.UserID = normalizeString(access.UserID)
	access.InboundID = normalizeString(access.InboundID)
	if access.UserID == "" || access.InboundID == "" {
		return UserAccess{}, fmt.Errorf("user_id and inbound_id are required")
	}
	if access.CreatedAt.IsZero() {
		access.CreatedAt = now
	}
	access.UpdatedAt = now

	if strings.TrimSpace(access.CredentialStatus) == "" {
		access.CredentialStatus = "active"
	}
	_, err := s.db.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO core_user_access(
			id, user_id, inbound_id, enabled, vless_uuid, vless_flow_override, hy2_password_enc,
			traffic_limit_bytes_override, expire_at_ns_override,
			display_name, description, credential_status, last_seen_at_ns, last_client_ip, client_profile_id,
			created_at_ns, updated_at_ns
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, inbound_id) DO UPDATE SET
			enabled = excluded.enabled,
			vless_uuid = excluded.vless_uuid,
			vless_flow_override = excluded.vless_flow_override,
			hy2_password_enc = excluded.hy2_password_enc,
			traffic_limit_bytes_override = excluded.traffic_limit_bytes_override,
			expire_at_ns_override = excluded.expire_at_ns_override,
			display_name = excluded.display_name,
			description = excluded.description,
			credential_status = excluded.credential_status,
			client_profile_id = excluded.client_profile_id,
			updated_at_ns = excluded.updated_at_ns`,
		access.ID,
		access.UserID,
		access.InboundID,
		boolToInt(access.Enabled),
		nullIfEmpty(access.VLESSUUID),
		nullIfEmpty(access.VLESSFlowOverride),
		nullIfEmpty(access.Hysteria2Password),
		nullableInt64(access.TrafficLimitBytesOverride),
		nullableTime(access.ExpireAtOverride),
		nullIfEmpty(access.DisplayName),
		nullIfEmpty(access.Description),
		access.CredentialStatus,
		nullableTime(access.LastSeenAt),
		nullIfEmpty(valueOrEmpty(access.LastClientIP)),
		nullIfEmpty(access.ClientProfileID),
		toUnixNano(access.CreatedAt),
		toUnixNano(access.UpdatedAt),
	)
	if err != nil {
		return UserAccess{}, parseUnique(err)
	}
	return s.GetUserAccessByPair(ctx, access.UserID, access.InboundID)
}

const userAccessSelectCols = `id, user_id, inbound_id, enabled, vless_uuid, vless_flow_override, hy2_password_enc, traffic_limit_bytes_override, expire_at_ns_override, COALESCE(display_name,''), COALESCE(description,''), COALESCE(credential_status,'active'), last_seen_at_ns, last_client_ip, COALESCE(client_profile_id,''), created_at_ns, updated_at_ns`

func (s *Store) GetUserAccess(ctx context.Context, id string) (UserAccess, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT `+userAccessSelectCols+` FROM core_user_access WHERE id = ? LIMIT 1`, normalizeString(id))
	item, err := scanUserAccess(row)
	if err != nil {
		return UserAccess{}, err
	}
	return item, nil
}

func (s *Store) GetUserAccessByPair(ctx context.Context, userID string, inboundID string) (UserAccess, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT `+userAccessSelectCols+` FROM core_user_access WHERE user_id = ? AND inbound_id = ? LIMIT 1`, normalizeString(userID), normalizeString(inboundID))
	item, err := scanUserAccess(row)
	if err != nil {
		return UserAccess{}, err
	}
	return item, nil
}

func (s *Store) ListUserAccessByUser(ctx context.Context, userID string) ([]UserAccess, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx), `SELECT `+userAccessSelectCols+` FROM core_user_access WHERE user_id = ? ORDER BY created_at_ns DESC`, normalizeString(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]UserAccess, 0)
	for rows.Next() {
		item, err := scanUserAccess(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListUserAccessByInbound(ctx context.Context, inboundID string, enabledOnly bool) ([]UserAccess, error) {
	query := `SELECT ` + userAccessSelectCols + ` FROM core_user_access WHERE inbound_id = ?`
	args := []any{normalizeString(inboundID)}
	if enabledOnly {
		query += ` AND enabled = 1`
	}
	query += ` ORDER BY created_at_ns ASC`
	rows, err := s.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]UserAccess, 0)
	for rows.Next() {
		item, err := scanUserAccess(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// UpdateUserAccessLastSeen records the last connection metadata for an access entry.
func (s *Store) UpdateUserAccessLastSeen(ctx context.Context, id string, ip string) error {
	_, err := s.db.ExecContext(resolveCtx(ctx),
		`UPDATE core_user_access SET last_seen_at_ns = ?, last_client_ip = ? WHERE id = ?`,
		nowNano(), nullIfEmpty(ip), normalizeString(id))
	return err
}

func (s *Store) DeleteUserAccess(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(resolveCtx(ctx), `DELETE FROM core_user_access WHERE id = ?`, normalizeString(id))
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

func (s *Store) EnsureSubscriptionForUser(ctx context.Context, userID string, profileName string) (Subscription, error) {
	userID = normalizeString(userID)
	if userID == "" {
		return Subscription{}, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(profileName) == "" {
		profileName = "default"
	}
	now := nowNano()
	subID := generateID()
	_, err := s.db.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO core_subscriptions(id, user_id, profile_name, enabled, created_at_ns, updated_at_ns)
		 VALUES (?, ?, ?, 1, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET profile_name = excluded.profile_name, enabled = 1, updated_at_ns = excluded.updated_at_ns`,
		subID,
		userID,
		normalizeString(profileName),
		now,
		now,
	)
	if err != nil {
		return Subscription{}, parseUnique(err)
	}
	return s.GetSubscriptionByUser(ctx, userID)
}

func (s *Store) GetSubscriptionByUser(ctx context.Context, userID string) (Subscription, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT id, user_id, profile_name, enabled, created_at_ns, updated_at_ns FROM core_subscriptions WHERE user_id = ? LIMIT 1`, normalizeString(userID))
	item, err := scanSubscription(row)
	if err != nil {
		return Subscription{}, err
	}
	return item, nil
}

func (s *Store) GetSubscription(ctx context.Context, id string) (Subscription, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT id, user_id, profile_name, enabled, created_at_ns, updated_at_ns FROM core_subscriptions WHERE id = ? LIMIT 1`, normalizeString(id))
	item, err := scanSubscription(row)
	if err != nil {
		return Subscription{}, err
	}
	return item, nil
}

func (s *Store) ListSubscriptionTokens(ctx context.Context, subscriptionID string) ([]SubscriptionToken, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx), `SELECT id, subscription_id, token_prefix, revoked_at_ns, expires_at_ns, last_used_at_ns, last_used_ip, created_at_ns FROM core_subscription_tokens WHERE subscription_id = ? ORDER BY created_at_ns DESC`, normalizeString(subscriptionID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SubscriptionToken, 0)
	for rows.Next() {
		item, err := scanSubscriptionToken(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) IssueSubscriptionToken(ctx context.Context, subscriptionID string, expiresAt *time.Time) (IssuedSubscriptionToken, error) {
	tokenPlain, err := randomToken(32)
	if err != nil {
		return IssuedSubscriptionToken{}, err
	}
	salt, err := randomHex(16)
	if err != nil {
		return IssuedSubscriptionToken{}, err
	}
	hash := tokenHash(salt, tokenPlain)
	now := time.Now().UTC()
	token := SubscriptionToken{
		ID:             generateID(),
		SubscriptionID: normalizeString(subscriptionID),
		TokenPrefix:    tokenPrefix(tokenPlain),
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
	}
	_, err = s.db.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO core_subscription_tokens(
			id, subscription_id, token_prefix, token_hash, token_salt, revoked_at_ns, expires_at_ns, last_used_at_ns, last_used_ip, created_at_ns
		) VALUES (?, ?, ?, ?, ?, NULL, ?, NULL, NULL, ?)`,
		token.ID,
		token.SubscriptionID,
		token.TokenPrefix,
		hash,
		salt,
		nullableTime(token.ExpiresAt),
		toUnixNano(token.CreatedAt),
	)
	if err != nil {
		return IssuedSubscriptionToken{}, parseUnique(err)
	}
	return IssuedSubscriptionToken{PlaintextToken: tokenPlain, Token: token}, nil
}

func (s *Store) RevokeSubscriptionTokens(ctx context.Context, subscriptionID string) error {
	_, err := s.db.ExecContext(resolveCtx(ctx), `UPDATE core_subscription_tokens SET revoked_at_ns = ? WHERE subscription_id = ? AND revoked_at_ns IS NULL`, nowNano(), normalizeString(subscriptionID))
	return err
}

func (s *Store) RotateSubscriptionToken(ctx context.Context, subscriptionID string, expiresAt *time.Time) (IssuedSubscriptionToken, error) {
	if err := s.RevokeSubscriptionTokens(ctx, subscriptionID); err != nil {
		return IssuedSubscriptionToken{}, err
	}
	return s.IssueSubscriptionToken(ctx, subscriptionID, expiresAt)
}
func (s *Store) ResolveSubscriptionToken(ctx context.Context, plainToken string, ip string) (TokenContext, error) {
	plainToken = normalizeString(plainToken)
	if plainToken == "" {
		return TokenContext{}, ErrInvalidToken
	}
	prefix := tokenPrefix(plainToken)
	rows, err := s.db.QueryContext(
		resolveCtx(ctx),
		`SELECT
			t.id, t.subscription_id, t.token_prefix, t.token_hash, t.token_salt, t.revoked_at_ns, t.expires_at_ns, t.last_used_at_ns, t.last_used_ip, t.created_at_ns,
			s.id, s.user_id, s.profile_name, s.enabled, s.created_at_ns, s.updated_at_ns,
			u.id, u.username, u.enabled, u.traffic_limit_bytes, u.traffic_used_up_bytes, u.traffic_used_down_bytes, u.expire_at_ns, u.created_at_ns, u.updated_at_ns
		FROM core_subscription_tokens t
		JOIN core_subscriptions s ON s.id = t.subscription_id
		JOIN core_users u ON u.id = s.user_id
		WHERE t.token_prefix = ?`,
		prefix,
	)
	if err != nil {
		return TokenContext{}, err
	}
	defer rows.Close()

	type tokenCandidate struct {
		TokenHash string
		TokenSalt string
		Ctx       TokenContext
	}
	candidates := make([]tokenCandidate, 0)
	for rows.Next() {
		var (
			ctxItem TokenContext
			tokenHashValue string
			tokenSaltValue string
			tRevoked sql.NullInt64
			tExpires sql.NullInt64
			tLastUsed sql.NullInt64
			tLastUsedIP sql.NullString
			tCreated int64
			sEnabled int64
			sCreated int64
			sUpdated int64
			uEnabled int64
			uExpire sql.NullInt64
			uCreated int64
			uUpdated int64
		)
		if err := rows.Scan(
			&ctxItem.Token.ID,
			&ctxItem.Token.SubscriptionID,
			&ctxItem.Token.TokenPrefix,
			&tokenHashValue,
			&tokenSaltValue,
			&tRevoked,
			&tExpires,
			&tLastUsed,
			&tLastUsedIP,
			&tCreated,
			&ctxItem.Subscription.ID,
			&ctxItem.Subscription.UserID,
			&ctxItem.Subscription.ProfileName,
			&sEnabled,
			&sCreated,
			&sUpdated,
			&ctxItem.User.ID,
			&ctxItem.User.Username,
			&uEnabled,
			&ctxItem.User.TrafficLimitBytes,
			&ctxItem.User.TrafficUsedUpBytes,
			&ctxItem.User.TrafficUsedDownBytes,
			&uExpire,
			&uCreated,
			&uUpdated,
		); err != nil {
			return TokenContext{}, err
		}
		ctxItem.Token.RevokedAt = optionalTime(tRevoked)
		ctxItem.Token.ExpiresAt = optionalTime(tExpires)
		ctxItem.Token.LastUsedAt = optionalTime(tLastUsed)
		ctxItem.Token.LastUsedIP = optionalString(tLastUsedIP)
		ctxItem.Token.CreatedAt = fromUnixNano(tCreated)
		ctxItem.Subscription.Enabled = intToBool(sEnabled)
		ctxItem.Subscription.CreatedAt = fromUnixNano(sCreated)
		ctxItem.Subscription.UpdatedAt = fromUnixNano(sUpdated)
		ctxItem.User.Enabled = intToBool(uEnabled)
		ctxItem.User.ExpireAt = optionalTime(uExpire)
		ctxItem.User.CreatedAt = fromUnixNano(uCreated)
		ctxItem.User.UpdatedAt = fromUnixNano(uUpdated)
		candidates = append(candidates, tokenCandidate{TokenHash: tokenHashValue, TokenSalt: tokenSaltValue, Ctx: ctxItem})
	}
	if err := rows.Err(); err != nil {
		return TokenContext{}, err
	}
	if len(candidates) == 0 {
		return TokenContext{}, ErrInvalidToken
	}

	computed := make([]tokenCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		candidateHash := tokenHash(candidate.TokenSalt, plainToken)
		if hashEqual(candidate.TokenHash, candidateHash) {
			computed = append(computed, candidate)
		}
	}
	if len(computed) == 0 {
		return TokenContext{}, ErrInvalidToken
	}
	sort.Slice(computed, func(i, j int) bool {
		return computed[i].Ctx.Token.CreatedAt.After(computed[j].Ctx.Token.CreatedAt)
	})
	selected := computed[0].Ctx
	if selected.Token.RevokedAt != nil {
		return TokenContext{}, ErrTokenRevoked
	}
	if selected.Token.ExpiresAt != nil && !selected.Token.ExpiresAt.After(time.Now().UTC()) {
		return TokenContext{}, ErrTokenRevoked
	}
	if _, err := s.db.ExecContext(resolveCtx(ctx), `UPDATE core_subscription_tokens SET last_used_at_ns = ?, last_used_ip = ? WHERE id = ?`, nowNano(), nullIfEmpty(ip), selected.Token.ID); err == nil {
		now := time.Now().UTC()
		selected.Token.LastUsedAt = &now
		trimmedIP := strings.TrimSpace(ip)
		if trimmedIP != "" {
			selected.Token.LastUsedIP = &trimmedIP
		}
	}
	return selected, nil
}

func (s *Store) CreateConfigRevision(ctx context.Context, serverID string, renderedJSON string, configHash string, checkOK bool, checkError *string, rollbackFrom *string) (ConfigRevision, error) {
	return s.CreateConfigRevisionEx(ctx, serverID, renderedJSON, configHash, checkOK, checkError, rollbackFrom, "")
}

func (s *Store) CreateConfigRevisionEx(ctx context.Context, serverID string, renderedJSON string, configHash string, checkOK bool, checkError *string, rollbackFrom *string, createdBy string) (ConfigRevision, error) {
	serverID = normalizeString(serverID)
	if serverID == "" {
		return ConfigRevision{}, fmt.Errorf("server_id is required")
	}
	tx, err := s.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return ConfigRevision{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var revisionNo int
	if err = tx.QueryRowContext(resolveCtx(ctx), `SELECT COALESCE(MAX(revision_no), 0) + 1 FROM core_config_revisions WHERE server_id = ?`, serverID).Scan(&revisionNo); err != nil {
		return ConfigRevision{}, err
	}
	revisionID := generateID()
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		resolveCtx(ctx),
		`INSERT INTO core_config_revisions(id, server_id, revision_no, config_hash, rendered_json, check_ok, check_error, applied_at_ns, rollback_from_revision_id, schema_version, renderer_version, created_by, is_current, created_at_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, 0, ?)`,
		revisionID,
		serverID,
		revisionNo,
		strings.TrimSpace(configHash),
		renderedJSON,
		boolToInt(checkOK),
		nullIfEmpty(valueOrNil(checkError)),
		nullIfEmpty(valueOrNil(rollbackFrom)),
		currentSchemaVersion,
		rendererVersion,
		nullIfEmpty(createdBy),
		toUnixNano(now),
	)
	if err != nil {
		return ConfigRevision{}, parseUnique(err)
	}
	if err = tx.Commit(); err != nil {
		return ConfigRevision{}, err
	}
	return s.GetConfigRevision(ctx, revisionID)
}

const configRevisionSelectCols = `id, server_id, revision_no, config_hash, rendered_json, check_ok, check_error, applied_at_ns, rollback_from_revision_id, COALESCE(schema_version,0), COALESCE(renderer_version,''), COALESCE(created_by,''), COALESCE(is_current,0), created_at_ns, apply_status, apply_error`

func (s *Store) GetConfigRevision(ctx context.Context, revisionID string) (ConfigRevision, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT `+configRevisionSelectCols+` FROM core_config_revisions WHERE id = ? LIMIT 1`, normalizeString(revisionID))
	item, err := scanConfigRevision(row)
	if err != nil {
		return ConfigRevision{}, err
	}
	return item, nil
}

func (s *Store) GetLatestConfigRevision(ctx context.Context, serverID string) (ConfigRevision, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT `+configRevisionSelectCols+` FROM core_config_revisions WHERE server_id = ? ORDER BY revision_no DESC LIMIT 1`, normalizeString(serverID))
	item, err := scanConfigRevision(row)
	if err != nil {
		return ConfigRevision{}, err
	}
	return item, nil
}

func (s *Store) GetCurrentConfigRevision(ctx context.Context, serverID string) (ConfigRevision, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT `+configRevisionSelectCols+` FROM core_config_revisions WHERE server_id = ? AND COALESCE(is_current,0) = 1 ORDER BY revision_no DESC LIMIT 1`, normalizeString(serverID))
	item, err := scanConfigRevision(row)
	if err != nil {
		return ConfigRevision{}, err
	}
	return item, nil
}

func (s *Store) ListConfigRevisions(ctx context.Context, serverID string, limit int) ([]ConfigRevision, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(resolveCtx(ctx), `SELECT `+configRevisionSelectCols+` FROM core_config_revisions WHERE server_id = ? ORDER BY revision_no DESC LIMIT ?`, normalizeString(serverID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ConfigRevision, 0)
	for rows.Next() {
		item, err := scanConfigRevision(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) MarkConfigRevisionApplied(ctx context.Context, revisionID string) error {
	tx, err := s.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Fetch server_id for this revision to unset other is_current flags.
	var serverID string
	if err = tx.QueryRowContext(resolveCtx(ctx), `SELECT server_id FROM core_config_revisions WHERE id = ?`, normalizeString(revisionID)).Scan(&serverID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	// Clear is_current for all revisions of this server.
	if _, err = tx.ExecContext(resolveCtx(ctx), `UPDATE core_config_revisions SET is_current = 0 WHERE server_id = ?`, serverID); err != nil {
		return err
	}
	// Mark the target revision as applied and current.
	result, err := tx.ExecContext(resolveCtx(ctx), `UPDATE core_config_revisions SET applied_at_ns = ?, is_current = 1, apply_status = 'succeeded', apply_error = NULL WHERE id = ?`, nowNano(), normalizeString(revisionID))
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
	return tx.Commit()
}

// MarkConfigRevisionApplyFailed records that an apply attempt failed without
// changing is_current (the previous applied revision remains current).
func (s *Store) MarkConfigRevisionApplyFailed(ctx context.Context, revisionID string, applyErr string) error {
	result, err := s.db.ExecContext(
		resolveCtx(ctx),
		`UPDATE core_config_revisions SET apply_status = 'failed', apply_error = ? WHERE id = ?`,
		strings.TrimSpace(applyErr),
		normalizeString(revisionID),
	)
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

// BulkDeleteUsers deletes multiple users by ID in a single transaction and
// returns the count of rows actually deleted. It does NOT trigger a config
// render or runtime apply — the caller is responsible for that.
func (s *Store) BulkDeleteUsers(ctx context.Context, ids []string) (int, error) {
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		if t := normalizeString(id); t != "" {
			clean = append(clean, t)
		}
	}
	if len(clean) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	total := 0
	for _, id := range clean {
		res, err := tx.ExecContext(resolveCtx(ctx), `DELETE FROM core_users WHERE id = ?`, id)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}
	return total, tx.Commit()
}

// BulkSetUsersEnabled enables or disables multiple users by ID without
// triggering a runtime apply. Returns the count of rows updated.
func (s *Store) BulkSetUsersEnabled(ctx context.Context, ids []string, enabled bool) (int, error) {
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		if t := normalizeString(id); t != "" {
			clean = append(clean, t)
		}
	}
	if len(clean) == 0 {
		return 0, nil
	}
	flag := boolToInt(enabled)
	tx, err := s.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	total := 0
	for _, id := range clean {
		res, err := tx.ExecContext(resolveCtx(ctx), `UPDATE core_users SET enabled = ?, updated_at_ns = ? WHERE id = ?`, flag, nowNano(), id)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}
	return total, tx.Commit()
}

func (s *Store) ListUsersByIDs(ctx context.Context, ids []string) (map[string]User, error) {
	result := make(map[string]User, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		trimmed := normalizeString(id)
		if trimmed == "" {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, trimmed)
	}
	if len(placeholders) == 0 {
		return result, nil
	}
	query := `SELECT id, username, enabled, traffic_limit_bytes, traffic_used_up_bytes, traffic_used_down_bytes, expire_at_ns, created_at_ns, updated_at_ns FROM core_users WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		result[item.ID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) ListInboundActiveUserAccess(ctx context.Context, inboundID string) ([]UserAccess, map[string]User, error) {
	accesses, err := s.ListUserAccessByInbound(ctx, inboundID, true)
	if err != nil {
		return nil, nil, err
	}
	userIDs := make([]string, 0, len(accesses))
	seen := make(map[string]struct{}, len(accesses))
	for _, access := range accesses {
		if _, ok := seen[access.UserID]; ok {
			continue
		}
		seen[access.UserID] = struct{}{}
		userIDs = append(userIDs, access.UserID)
	}
	usersByID, err := s.ListUsersByIDs(ctx, userIDs)
	if err != nil {
		return nil, nil, err
	}
	return accesses, usersByID, nil
}

func (s *Store) ListEnabledInbounds(ctx context.Context, serverID string) ([]Inbound, error) {
	items, err := s.ListInbounds(ctx, serverID)
	if err != nil {
		return nil, err
	}
	result := make([]Inbound, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			result = append(result, item)
		}
	}
	return result, nil
}
func (s *Store) populateInboundSettings(ctx context.Context, inbound *Inbound) error {
	if inbound == nil {
		return nil
	}
	switch inbound.Protocol {
	case InboundProtocolVLESS:
		row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT tls_enabled, tls_server_name, tls_alpn_csv, tls_certificate_path, tls_key_path, reality_enabled, reality_public_key, reality_private_key_enc, reality_short_id, reality_handshake_server, reality_handshake_server_port, flow, transport_type, transport_host, transport_path, multiplex_enabled, multiplex_protocol, multiplex_max_connections, multiplex_min_streams, multiplex_max_streams, COALESCE(reality_profile_id,''), COALESCE(transport_profile_id,''), COALESCE(multiplex_profile_id,''), COALESCE(packet_encoding_default,'') FROM core_inbound_vless_settings WHERE inbound_id = ? LIMIT 1`, inbound.ID)
		var (
			item             VLESSInboundSettings
			tlsServerName    sql.NullString
			tlsALPN          sql.NullString
			tlsCertPath      sql.NullString
			tlsKeyPath       sql.NullString
			realityPub       sql.NullString
			realityPriv      sql.NullString
			realitySID       sql.NullString
			realityHandshake sql.NullString
			realityPort      sql.NullInt64
			flow             sql.NullString
			transportType    string
			transportHost    sql.NullString
			transportPath    sql.NullString
			muxProtocol      sql.NullString
			muxMaxConn       sql.NullInt64
			muxMinStreams     sql.NullInt64
			muxMaxStreams     sql.NullInt64
			tlsEnabled       int64
			realityEnabled   int64
			muxEnabled       int64
		)
		if err := row.Scan(
			&tlsEnabled,
			&tlsServerName,
			&tlsALPN,
			&tlsCertPath,
			&tlsKeyPath,
			&realityEnabled,
			&realityPub,
			&realityPriv,
			&realitySID,
			&realityHandshake,
			&realityPort,
			&flow,
			&transportType,
			&transportHost,
			&transportPath,
			&muxEnabled,
			&muxProtocol,
			&muxMaxConn,
			&muxMinStreams,
			&muxMaxStreams,
			&item.RealityProfileID,
			&item.TransportProfileID,
			&item.MultiplexProfileID,
			&item.PacketEncodingDefault,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				inbound.VLESS = &VLESSInboundSettings{TransportType: "tcp"}
				return nil
			}
			return err
		}
		item.TLSEnabled = intToBool(tlsEnabled)
		item.TLSServerName = valueOrEmpty(optionalString(tlsServerName))
		item.TLSALPN = splitCSV(valueOrEmpty(optionalString(tlsALPN)))
		item.TLSCertificatePath = valueOrEmpty(optionalString(tlsCertPath))
		item.TLSKeyPath = valueOrEmpty(optionalString(tlsKeyPath))
		item.RealityEnabled = intToBool(realityEnabled)
		item.RealityPublicKey = valueOrEmpty(optionalString(realityPub))
		item.RealityPrivateKey = valueOrEmpty(optionalString(realityPriv))
		item.RealityShortID = valueOrEmpty(optionalString(realitySID))
		item.RealityHandshakeServer = valueOrEmpty(optionalString(realityHandshake))
		item.RealityHandshakeServerPort = int(nullableIntToZero(realityPort))
		item.Flow = valueOrEmpty(optionalString(flow))
		item.TransportType = normalizeString(transportType)
		if item.TransportType == "" {
			item.TransportType = "tcp"
		}
		item.TransportHost = valueOrEmpty(optionalString(transportHost))
		item.TransportPath = valueOrEmpty(optionalString(transportPath))
		item.MultiplexEnabled = intToBool(muxEnabled)
		item.MultiplexProtocol = valueOrEmpty(optionalString(muxProtocol))
		item.MultiplexMaxConnections = int(nullableIntToZero(muxMaxConn))
		item.MultiplexMinStreams = int(nullableIntToZero(muxMinStreams))
		item.MultiplexMaxStreams = int(nullableIntToZero(muxMaxStreams))
		inbound.VLESS = &item
		inbound.Hysteria2 = nil
	case InboundProtocolHysteria2:
		row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT tls_enabled, tls_server_name, tls_certificate_path, tls_key_path, COALESCE(allow_insecure, 0), up_mbps, down_mbps, ignore_client_bandwidth, obfs_type, obfs_password_enc, masquerade_json, bbr_profile, brutal_debug, COALESCE(masquerade_profile_id,''), COALESCE(server_ports,''), COALESCE(hop_interval,0), COALESCE(network,''), COALESCE(tls_alpn_csv,''), COALESCE(bandwidth_profile_mode,'') FROM core_inbound_hysteria2_settings WHERE inbound_id = ? LIMIT 1`, inbound.ID)
		var (
			item          Hysteria2InboundSettings
			tlsEnabled    int64
			tlsSNI        sql.NullString
			tlsCert       sql.NullString
			tlsKey        sql.NullString
			allowInsecure int64
			up            sql.NullInt64
			down          sql.NullInt64
			ignoreBW      int64
			obfsType      sql.NullString
			obfsPassword  sql.NullString
			masquerade    sql.NullString
			bbrProfile    sql.NullString
			brutalDebug   int64
			tlsALPNCSV    string
		)
		if err := row.Scan(
			&tlsEnabled,
			&tlsSNI,
			&tlsCert,
			&tlsKey,
			&allowInsecure,
			&up,
			&down,
			&ignoreBW,
			&obfsType,
			&obfsPassword,
			&masquerade,
			&bbrProfile,
			&brutalDebug,
			&item.MasqueradeProfileID,
			&item.ServerPorts,
			&item.HopInterval,
			&item.Network,
			&tlsALPNCSV,
			&item.BandwidthProfileMode,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				inbound.Hysteria2 = &Hysteria2InboundSettings{}
				return nil
			}
			return err
		}
		item.TLSEnabled = intToBool(tlsEnabled)
		item.TLSServerName = valueOrEmpty(optionalString(tlsSNI))
		item.TLSCertificatePath = valueOrEmpty(optionalString(tlsCert))
		item.TLSKeyPath = valueOrEmpty(optionalString(tlsKey))
		item.AllowInsecure = intToBool(allowInsecure)
		item.UpMbps = nullableIntToPointer(up)
		item.DownMbps = nullableIntToPointer(down)
		item.IgnoreClientBandwidth = intToBool(ignoreBW)
		item.ObfsType = valueOrEmpty(optionalString(obfsType))
		item.ObfsPassword = valueOrEmpty(optionalString(obfsPassword))
		item.MasqueradeJSON = valueOrEmpty(optionalString(masquerade))
		item.BBRProfile = valueOrEmpty(optionalString(bbrProfile))
		item.BrutalDebug = intToBool(brutalDebug)
		item.TLSALPN = splitCSV(tlsALPNCSV)
		inbound.Hysteria2 = &item
		inbound.VLESS = nil
	default:
		return fmt.Errorf("unsupported inbound protocol: %s", inbound.Protocol)
	}
	return nil
}

func scanServer(row interface{ Scan(dest ...any) error }) (Server, error) {
	var (
		item Server
		createdAt int64
		updatedAt int64
	)
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.PublicHost,
		&item.PanelPublicURL,
		&item.SubscriptionBaseURL,
		&item.SingBoxBinaryPath,
		&item.SingBoxConfigPath,
		&item.SingBoxServiceName,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Server{}, parseUnique(err)
	}
	item.CreatedAt = fromUnixNano(createdAt)
	item.UpdatedAt = fromUnixNano(updatedAt)
	return item, nil
}

func scanInboundBase(row interface{ Scan(dest ...any) error }) (Inbound, error) {
	var (
		item         Inbound
		protocol     string
		enabled      int64
		labelsJSON   string
		logProfileID string
		createdAt    int64
		updatedAt    int64
	)
	if err := row.Scan(
		&item.ID,
		&item.ServerID,
		&item.Name,
		&item.Tag,
		&protocol,
		&item.Listen,
		&item.ListenPort,
		&enabled,
		&item.TemplateKey,
		&item.Notes,
		&labelsJSON,
		&item.SortOrder,
		&logProfileID,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Inbound{}, parseUnique(err)
	}
	item.Protocol = InboundProtocol(strings.TrimSpace(protocol))
	item.Enabled = intToBool(enabled)
	item.Labels = jsonToStrings(labelsJSON)
	item.LogProfileID = strings.TrimSpace(logProfileID)
	item.CreatedAt = fromUnixNano(createdAt)
	item.UpdatedAt = fromUnixNano(updatedAt)
	return item, nil
}

func scanUser(row interface{ Scan(dest ...any) error }) (User, error) {
	var (
		item User
		enabled int64
		expireAt sql.NullInt64
		createdAt int64
		updatedAt int64
	)
	if err := row.Scan(
		&item.ID,
		&item.Username,
		&enabled,
		&item.TrafficLimitBytes,
		&item.TrafficUsedUpBytes,
		&item.TrafficUsedDownBytes,
		&expireAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return User{}, parseUnique(err)
	}
	item.Enabled = intToBool(enabled)
	item.ExpireAt = optionalTime(expireAt)
	item.CreatedAt = fromUnixNano(createdAt)
	item.UpdatedAt = fromUnixNano(updatedAt)
	return item, nil
}

func scanUserAccess(row interface{ Scan(dest ...any) error }) (UserAccess, error) {
	var (
		item           UserAccess
		enabled        int64
		vlessUUID      sql.NullString
		vlessFlow      sql.NullString
		hy2Password    sql.NullString
		overrideLimit  sql.NullInt64
		overrideExpire sql.NullInt64
		lastSeenAt     sql.NullInt64
		lastClientIP   sql.NullString
		createdAt      int64
		updatedAt      int64
	)
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.InboundID,
		&enabled,
		&vlessUUID,
		&vlessFlow,
		&hy2Password,
		&overrideLimit,
		&overrideExpire,
		&item.DisplayName,
		&item.Description,
		&item.CredentialStatus,
		&lastSeenAt,
		&lastClientIP,
		&item.ClientProfileID,
		&createdAt,
		&updatedAt,
	); err != nil {
		return UserAccess{}, parseUnique(err)
	}
	item.Enabled = intToBool(enabled)
	item.VLESSUUID = valueOrEmpty(optionalString(vlessUUID))
	item.VLESSFlowOverride = valueOrEmpty(optionalString(vlessFlow))
	item.Hysteria2Password = valueOrEmpty(optionalString(hy2Password))
	if overrideLimit.Valid {
		value := overrideLimit.Int64
		item.TrafficLimitBytesOverride = &value
	}
	item.ExpireAtOverride = optionalTime(overrideExpire)
	item.LastSeenAt = optionalTime(lastSeenAt)
	item.LastClientIP = optionalString(lastClientIP)
	item.CreatedAt = fromUnixNano(createdAt)
	item.UpdatedAt = fromUnixNano(updatedAt)
	return item, nil
}

func scanSubscription(row interface{ Scan(dest ...any) error }) (Subscription, error) {
	var (
		item Subscription
		enabled int64
		createdAt int64
		updatedAt int64
	)
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.ProfileName,
		&enabled,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Subscription{}, parseUnique(err)
	}
	item.Enabled = intToBool(enabled)
	item.CreatedAt = fromUnixNano(createdAt)
	item.UpdatedAt = fromUnixNano(updatedAt)
	return item, nil
}

func scanSubscriptionToken(row interface{ Scan(dest ...any) error }) (SubscriptionToken, error) {
	var (
		item SubscriptionToken
		revokedAt sql.NullInt64
		expiresAt sql.NullInt64
		lastUsedAt sql.NullInt64
		lastUsedIP sql.NullString
		createdAt int64
	)
	if err := row.Scan(
		&item.ID,
		&item.SubscriptionID,
		&item.TokenPrefix,
		&revokedAt,
		&expiresAt,
		&lastUsedAt,
		&lastUsedIP,
		&createdAt,
	); err != nil {
		return SubscriptionToken{}, parseUnique(err)
	}
	item.RevokedAt = optionalTime(revokedAt)
	item.ExpiresAt = optionalTime(expiresAt)
	item.LastUsedAt = optionalTime(lastUsedAt)
	item.LastUsedIP = optionalString(lastUsedIP)
	item.CreatedAt = fromUnixNano(createdAt)
	return item, nil
}

func scanConfigRevision(row interface{ Scan(dest ...any) error }) (ConfigRevision, error) {
	var (
		item         ConfigRevision
		checkOK      int64
		checkError   sql.NullString
		appliedAt    sql.NullInt64
		rollbackFrom sql.NullString
		isCurrent    int64
		createdAt    int64
		applyStatus  sql.NullString
		applyError   sql.NullString
	)
	if err := row.Scan(
		&item.ID,
		&item.ServerID,
		&item.RevisionNo,
		&item.ConfigHash,
		&item.RenderedJSON,
		&checkOK,
		&checkError,
		&appliedAt,
		&rollbackFrom,
		&item.SchemaVersion,
		&item.RendererVersion,
		&item.CreatedBy,
		&isCurrent,
		&createdAt,
		&applyStatus,
		&applyError,
	); err != nil {
		return ConfigRevision{}, parseUnique(err)
	}
	item.CheckOK = intToBool(checkOK)
	item.CheckError = optionalString(checkError)
	item.AppliedAt = optionalTime(appliedAt)
	item.RollbackFromRevisionID = optionalString(rollbackFrom)
	item.IsCurrent = intToBool(isCurrent)
	item.CreatedAt = fromUnixNano(createdAt)
	item.ApplyStatus = optionalString(applyStatus)
	item.ApplyError = optionalString(applyError)
	return item, nil
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	ts := value.UTC()
	if ts.IsZero() {
		return nil
	}
	return ts.UnixNano()
}

func nullableIntToPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func nullableIntToZero(value sql.NullInt64) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func valueOrNil(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func nullableIntVal(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

// jsonToStrings parses a JSON string array, returning nil on empty/invalid input.
func jsonToStrings(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" || s == "[]" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// stringsToJSON encodes a string slice as a JSON array; returns "" for empty slices.
func stringsToJSON(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	b, err := json.Marshal(ss)
	if err != nil {
		return ""
	}
	return string(b)
}

// intsToJSON encodes an int slice as a JSON array.
func intsToJSON(ii []int) string {
	if len(ii) == 0 {
		return ""
	}
	b, err := json.Marshal(ii)
	if err != nil {
		return ""
	}
	return string(b)
}

// jsonToInts parses a JSON int array.
func jsonToInts(s string) []int {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" || s == "[]" {
		return nil
	}
	var out []int
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

func formatTrafficUserInfo(user User) string {
	expireUnix := int64(0)
	if user.ExpireAt != nil {
		expireUnix = user.ExpireAt.UTC().Unix()
	}
	return "upload=" + strconv.FormatInt(user.TrafficUsedUpBytes, 10) +
		"; download=" + strconv.FormatInt(user.TrafficUsedDownBytes, 10) +
		"; total=" + strconv.FormatInt(user.TrafficLimitBytes, 10) +
		"; expire=" + strconv.FormatInt(expireUnix, 10)
}
