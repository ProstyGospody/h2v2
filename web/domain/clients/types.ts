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
  enabled: boolean;
  traffic_limit_bytes: number;
  traffic_used_up_bytes: number;
  traffic_used_down_bytes: number;
  expire_at: string | null;
  created_at: string;
  updated_at: string;
  protocols: Protocol[];
  access: ClientAccess[];
};

export type ClientArtifacts = {
  subscription_import_url: string;
  subscription_profile_url: string;
  subscription_uris_url: string;
  subscription_qr_url: string;
  subscription_clash_url: string;
  subscription_base64_url: string;
  vless_uris: string[];
  hy2_uris: string[];
  all_uris: string[];
  singbox_profile_json: string;
};

export type ClientFormValues = {
  username: string;
  traffic_limit_gb: string;
  expire_at: string;
};
