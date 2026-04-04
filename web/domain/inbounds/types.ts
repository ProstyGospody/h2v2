export type VLESSInboundSettings = {
  tls_enabled: boolean;
  tls_server_name: string;
  tls_alpn: string[];
  tls_certificate_path: string;
  tls_key_path: string;
  reality_enabled: boolean;
  reality_public_key: string;
  reality_private_key: string;
  reality_short_id: string;
  reality_handshake_server: string;
  reality_handshake_server_port: number;
  flow: string;
  transport_type: string;
  transport_host: string;
  transport_path: string;
  multiplex_enabled: boolean;
  multiplex_protocol: string;
  multiplex_max_connections: number;
  multiplex_min_streams: number;
  multiplex_max_streams: number;
};

export type Hysteria2InboundSettings = {
  tls_enabled: boolean;
  tls_server_name: string;
  tls_certificate_path: string;
  tls_key_path: string;
  up_mbps: number | null;
  down_mbps: number | null;
  ignore_client_bandwidth: boolean;
  obfs_type: string;
  obfs_password: string;
  masquerade_json: string;
  bbr_profile: string;
  brutal_debug: boolean;
};

export type Inbound = {
  id: string;
  server_id: string;
  name: string;
  tag: string;
  protocol: "vless" | "hysteria2";
  listen: string;
  listen_port: number;
  enabled: boolean;
  template_key: string;
  vless?: VLESSInboundSettings;
  hysteria2?: Hysteria2InboundSettings;
  created_at: string;
  updated_at: string;
};

export type Server = {
  id: string;
  name: string;
  public_host: string;
  panel_public_url: string;
};

export type VLESSFormValues = {
  name: string;
  tag: string;
  listen_port: string;
  enabled: boolean;
  tls_server_name: string;
  reality_enabled: boolean;
  reality_public_key: string;
  reality_private_key: string;
  reality_short_id: string;
  reality_handshake_server: string;
  reality_handshake_server_port: string;
  flow: string;
  transport_type: string;
  transport_host: string;
  transport_path: string;
};

export type HY2FormValues = {
  name: string;
  tag: string;
  listen_port: string;
  enabled: boolean;
  tls_server_name: string;
  tls_certificate_path: string;
  tls_key_path: string;
  up_mbps: string;
  down_mbps: string;
  ignore_client_bandwidth: boolean;
  obfs_type: string;
  obfs_password: string;
};
