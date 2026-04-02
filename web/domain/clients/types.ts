export type ClientOverrides = {
  sni?: string;
  insecure?: boolean;
  pinSHA256?: string;
  obfsType?: string;
  obfsPassword?: string;
};

export type HysteriaClient = {
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
  client_overrides?: ClientOverrides | null;
};

export type HysteriaClientParams = {
  server: string;
  port: number;
  portUnion?: string;
  sni?: string;
  insecure: boolean;
  pinSHA256?: string;
  obfsType?: string;
  obfsPassword?: string;
};

export type HysteriaServerClientOptions = {
  tls_enabled: boolean;
  tls_mode: string;
  obfs_type?: string;
  masquerade_type?: string;
  bandwidth_up?: string;
  bandwidth_down?: string;
  ignore_client_bandwidth: boolean;
};

export type HysteriaUserArtifacts = {
  uri: string;
  uri_hy2: string;
  subscription_url: string;
  client_config: string;
  client_params: HysteriaClientParams;
  server_defaults: HysteriaClientParams;
  client_overrides?: ClientOverrides | null;
  server_options: HysteriaServerClientOptions;
  singbox_outbound: Record<string, unknown>;
};

export type HysteriaUserPayload = {
  user: HysteriaClient;
  artifacts: HysteriaUserArtifacts | null;
  access_state?: string;
  access_message?: string;
};

export type HysteriaClientDefaults = {
  client_params: HysteriaClientParams;
  server_options: HysteriaServerClientOptions;
};

export type HysteriaClientListResponse = { items: HysteriaClient[] };

export type HysteriaClientCreateRequest = {
  username: string;
  note?: string;
  auth_secret?: string;
  client_overrides?: ClientOverrides;
};

export type HysteriaClientUpdateRequest = {
  username: string;
  note?: string;
  auth_secret?: string;
  client_overrides?: ClientOverrides;
};

export type HysteriaClientStateBatchResponse = {
  ok: boolean;
  enabled: boolean;
  updated: number;
};

export type HysteriaClientDeleteBatchResponse = {
  ok: boolean;
  deleted: number;
};
