// ─── Policy entities ──────────────────────────────────────────────────────────

export type Protocol = "vless" | "hysteria2";

export type Outbound = {
  id: string;
  server_id: string;
  tag: string;
  type: string;
  enabled: boolean;
  priority: number;
  settings_json?: string;
  healthcheck_enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type RouteRule = {
  id: string;
  server_id: string;
  enabled: boolean;
  priority: number;
  inbound_tags?: string[];
  protocols?: string[];
  domains?: string[];
  domain_suffixes?: string[];
  domain_keywords?: string[];
  ip_cidrs?: string[];
  ports?: number[];
  network?: string;
  geoip_codes?: string[];
  geosite_codes?: string[];
  outbound_tag: string;
  action?: string;
  invert?: boolean;
  created_at: string;
  updated_at: string;
};

export type DNSProfile = {
  id: string;
  server_id: string;
  name: string;
  enabled: boolean;
  strategy?: string;
  disable_cache: boolean;
  final_server?: string;
  servers_json?: string;
  rules_json?: string;
  fakeip_enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type LogProfile = {
  id: string;
  server_id: string;
  name: string;
  enabled: boolean;
  level: string;
  output?: string;
  timestamp: boolean;
  access_log_enabled: boolean;
  debug_mode: boolean;
  created_at: string;
  updated_at: string;
};

export type RealityProfile = {
  id: string;
  server_id: string;
  name: string;
  enabled: boolean;
  server_name?: string;
  handshake_server: string;
  handshake_server_port: number;
  // private_key is never returned from API
  public_key: string;
  short_ids?: string[];
  short_id_rotation_mode?: string;
  key_rotation_mode?: string;
  created_at: string;
  updated_at: string;
};

export type TransportProfile = {
  id: string;
  server_id: string;
  name: string;
  enabled: boolean;
  type: string;
  host?: string;
  path?: string;
  service_name?: string;
  headers_json?: string;
  idle_timeout?: number;
  ping_timeout?: number;
  created_at: string;
  updated_at: string;
};

export type MultiplexProfile = {
  id: string;
  server_id: string;
  name: string;
  enabled: boolean;
  protocol?: string;
  max_connections?: number;
  min_streams?: number;
  max_streams?: number;
  padding: boolean;
  brutal: boolean;
  created_at: string;
  updated_at: string;
};

export type HY2MasqueradeProfile = {
  id: string;
  server_id: string;
  name: string;
  enabled: boolean;
  type: "off" | "string" | "file" | "proxy";
  url?: string;
  rewrite_host: boolean;
  directory?: string;
  status_code?: number;
  headers_json?: string;
  content?: string;
  created_at: string;
  updated_at: string;
};

/** User-facing connection mode preset. */
export type ClientProfile = {
  id: string;
  server_id: string;
  name: string;
  protocol: Protocol;
  /**
   * HY2 modes: standard | obfuscated | poor_network | port_hopping
   * VLESS modes: standard | multiplex | udp_compat | transport | compat
   */
  mode: string;
  description?: string;
  settings_json?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type DomainValidationResult = {
  valid: boolean;
  errors: string[];
};

export type SystemLiveResponse = {
  collected_at: string;
  system: {
    cpu_usage_percent: number;
    memory_used_bytes: number;
    memory_total_bytes: number;
    memory_used_percent: number;
    uptime_seconds: number;
    network_rx_bps: number;
    network_tx_bps: number;
    network_rx_bytes: number;
    network_tx_bytes: number;
    tcp_sockets: number;
    udp_sockets: number;
    tcp_packets: number;
    udp_packets: number;
    tcp_packets_per_sec: number;
    udp_packets_per_sec: number;
    packets_collected_at: string;
    packets_source: string;
    packets_is_stale: boolean;
    collected_at: string;
    source: string;
    is_stale: boolean;
  };
  runtime: {
    enabled_users: number;
    total_tx_bytes: number;
    total_rx_bytes: number;
    online_count: number;
    connections_tcp: number;
    connections_udp: number;
    connections_breakdown_available: boolean;
    collected_at: string;
    source: string;
    is_stale: boolean;
  };
  services: Array<{
    service_name: string;
    status: string;
    last_check_at: string;
    source: string;
    is_stale: boolean;
    error?: string;
  }>;
  errors: string[];
};

export type SystemHistorySample = {
  timestamp: string;
  cpu_usage_percent: number;
  memory_used_percent: number;
  network_rx_bps: number;
  network_tx_bps: number;
  tcp_sockets?: number;
  udp_sockets?: number;
};

export type SystemHistoryResponse = {
  items: SystemHistorySample[];
};

export type TLSProfile = {
  id: string;
  server_id: string;
  name: string;
  enabled: boolean;
  server_name?: string;
  alpn?: string[];
  certificate_path?: string;
  key_path?: string;
  allow_insecure: boolean;
  created_at: string;
  updated_at: string;
};

export type ChangeImpact = {
  affected_users: number;
  affected_access: number;
  affected_inbounds: number;
  affected_subscriptions: number;
  affected_artifacts: number;
  requires_runtime_apply: boolean;
  requires_artifact_refresh: boolean;
  server_ids?: string[];
  inbound_ids?: string[];
};

export type DraftRevisionState = {
  server_id: string;
  current_revision_id?: string;
  current_revision_no?: number;
  draft_revision_id?: string;
  draft_revision_no?: number;
  pending_changes: boolean;
  check_ok: boolean;
  check_error?: string | null;
  apply_status?: string | null;
  apply_error?: string | null;
};

export type PolicyUsage = {
  kind: string;
  id: string;
  used_by_users: number;
  used_by_access: number;
  used_by_inbounds: number;
  used_by_route_rules: number;
  used_by_outbounds: number;
  affected_subscriptions: number;
  affected_artifacts: number;
  requires_runtime_apply: boolean;
  unsafe_delete: boolean;
};
