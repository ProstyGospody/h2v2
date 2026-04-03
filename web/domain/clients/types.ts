export type Protocol = "hy2" | "vless";

export type ClientAccess = {
  id: string;
  user_id: string;
  inbound_id: string;
  enabled: boolean;
  protocol: Protocol;
  vless_uuid?: string;
  hysteria2_password?: string;
};

export type Client = {
  id: string;
  username: string;
  username_normalized: string;
  password: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
  last_seen_at?: string | null;
  last_tx_bytes: number;
  last_rx_bytes: number;
  online_count: number;
  download_bps: number;
  upload_bps: number;
  traffic_limit_bytes: number;
  expire_at?: string | null;
  protocols: Protocol[];
  preferred_protocol: Protocol;
  access: ClientAccess[];
};

export type ClientArtifactsView = {
  access_url: string;
  subscription_url: string;
  access_qr_url: string;
  subscription_qr_url: string;
  vless_qr_url: string;
  hy2_qr_url: string;
  profile_url: string;
  uris_url: string;
  all_uris: string[];
  vless_uris: string[];
  hy2_uris: string[];
};

export type UserPayload = {
  user: Client;
  artifacts: ClientArtifactsView | null;
};

export type ClientCreateRequest = {
  username: string;
  auth_secret?: string;
  protocol?: Protocol;
  uuid?: string;
  traffic_limit_bytes?: number;
  expire_at?: string | null;
};

export type ClientUpdateRequest = {
  username: string;
  auth_secret?: string;
  protocol?: Protocol;
  uuid?: string;
  traffic_limit_bytes?: number;
  expire_at?: string | null;
};

export type ClientStateBatchResponse = {
  ok: boolean;
  enabled: boolean;
  updated: number;
};

export type ClientDeleteBatchResponse = {
  ok: boolean;
  deleted: number;
};
