export type Protocol = "hy2" | "vless";

export type Credential = {
  protocol: Protocol;
  identity: string;
  secret?: string;
  data_json?: string;
};

export type ClientOverrides = {
  sni?: string;
  insecure?: boolean;
  pinSHA256?: string;
  obfsType?: string;
  obfsPassword?: string;
};

export type UserCore = {
  id: string;
  name: string;
  name_normalized: string;
  enabled: boolean;
  traffic_limit_bytes: number;
  traffic_used_tx_bytes: number;
  traffic_used_rx_bytes: number;
  expire_at?: string | null;
  note?: string | null;
  subject: string;
  created_at: string;
  updated_at: string;
  last_seen_at?: string | null;
  online_count?: number;
  download_bps?: number;
  upload_bps?: number;
};

export type UserArtifacts = {
  protocol: Protocol;
  access_uri?: string;
  config?: string;
  subscription?: string;
  clash_node?: string;
  singbox_node?: Record<string, unknown>;
};

export type UnifiedUserPayload = {
  user: UserCore & { credentials: Credential[] };
  artifacts?: Record<string, UserArtifacts>;
  subscription_url?: string;
};

export type Client = {
  id: string;
  username: string;
  username_normalized: string;
  password: string;
  enabled: boolean;
  note?: string | null;
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
  credentials: Credential[];
  protocols: Protocol[];
  preferred_protocol: Protocol;
  client_overrides?: ClientOverrides | null;
};

export type ClientArtifactsView = {
  url: string;
  subscription: string;
  unified?: Record<string, UserArtifacts>;
};

export type UserPayload = {
  user: Client;
  artifacts: ClientArtifactsView | null;
  access_state?: string;
  access_message?: string;
};

export type ClientListResponse = { items: Client[] };

export type ClientCreateRequest = {
  username: string;
  note?: string;
  auth_secret?: string;
  protocol?: Protocol;
  uuid?: string;
  traffic_limit_bytes?: number;
  expire_at?: string | null;
  client_overrides?: ClientOverrides;
};

export type ClientUpdateRequest = {
  username: string;
  note?: string;
  auth_secret?: string;
  protocol?: Protocol;
  uuid?: string;
  traffic_limit_bytes?: number;
  expire_at?: string | null;
  client_overrides?: ClientOverrides;
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
