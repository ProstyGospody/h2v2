import type { ChangeImpact, DraftRevisionState } from "@/types/common";

export type Protocol = "hy2" | "vless";

export type ClientAccess = {
  id: string;
  user_id: string;
  inbound_id: string;
  enabled: boolean;
  protocol: Protocol;
  vless_uuid?: string;
  vless_flow_override?: string;
  hysteria2_password?: string;
  traffic_limit_bytes_override?: number | null;
  expire_at_override?: string | null;
  display_name?: string;
  description?: string;
  credential_status?: string;
  last_seen_at?: string | null;
  last_client_ip?: string | null;
  client_profile_id?: string;
};

export type Client = {
  id: string;
  username: string;
  enabled: boolean;
  has_subscription?: boolean;
  artifacts_need_refresh?: boolean;
  last_artifact_rendered_at?: string | null;
  last_artifact_refresh_reason?: string | null;
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
  primary_token_prefix?: string;
  artifact_version: number;
  artifacts_need_refresh: boolean;
  last_artifact_rendered_at?: string | null;
  last_artifact_refresh_reason?: string | null;
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

export type BulkDeleteMode = "" | "soft" | "hard";

export type BulkUserPatch = {
  ids: string[];
  enabled?: boolean;
  extend_seconds?: number;
  set_expire_at?: string | null;
  clear_expire?: boolean;
  traffic_limit_bytes?: number;
  client_profile_id?: string;
  inbound_id?: string;
  rotate_tokens?: boolean;
  regenerate_artifacts?: boolean;
  delete_mode?: BulkDeleteMode;
};

export type BulkAccessPatch = {
  ids: string[];
  enabled?: boolean;
  extend_seconds?: number;
  set_expire_at?: string | null;
  clear_expire?: boolean;
  traffic_limit_bytes?: number;
  client_profile_id?: string;
  inbound_id?: string;
  rotate_credentials?: boolean;
  regenerate_artifacts?: boolean;
  delete_mode?: BulkDeleteMode;
};

export type BulkMutationResult = {
  updated: number;
  deleted: number;
  rotated: number;
  regenerated: number;
  impact: ChangeImpact;
  drafts?: DraftRevisionState[];
};
