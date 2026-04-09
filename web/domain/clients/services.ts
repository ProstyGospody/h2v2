import { apiFetch } from "@/services/api";
import type { ChangeImpact, DraftRevisionState } from "@/types/common";
import { uniqueProtocols } from "./adapters";
import type {
  BulkAccessPatch,
  BulkMutationResult,
  BulkUserPatch,
  Client,
  ClientAccess,
  ClientArtifacts,
  Protocol,
} from "./types";

const TIMEOUT = 120_000;

function str(v: unknown, fb = ""): string {
  return typeof v === "string" ? v : fb;
}
function num(v: unknown, fb = 0): number {
  return typeof v === "number" && Number.isFinite(v) ? v : fb;
}
function bool(v: unknown, fb = false): boolean {
  return typeof v === "boolean" ? v : fb;
}
function arr(v: unknown): unknown[] {
  return Array.isArray(v) ? v : [];
}
function rec(v: unknown): Record<string, unknown> | null {
  return typeof v === "object" && v !== null ? (v as Record<string, unknown>) : null;
}
function strs(v: unknown): string[] {
  return arr(v)
    .map((x) => (typeof x === "string" ? x.trim() : ""))
    .filter(Boolean);
}

function normalizeProtocol(v: unknown): Protocol | null {
  const s = str(v).toLowerCase();
  if (s === "vless") return "vless";
  if (s === "hy2" || s === "hysteria2") return "hy2";
  return null;
}

function mapAccess(raw: unknown): ClientAccess | null {
  const r = rec(raw);
  if (!r) return null;
  const protocol = normalizeProtocol(r.protocol);
  if (!protocol) return null;
  return {
    id: str(r.id),
    user_id: str(r.user_id),
    inbound_id: str(r.inbound_id),
    enabled: bool(r.enabled, true),
    protocol,
    vless_uuid: typeof r.vless_uuid === "string" ? r.vless_uuid : undefined,
    vless_flow_override: typeof r.vless_flow_override === "string" ? r.vless_flow_override : undefined,
    hysteria2_password: typeof r.hysteria2_password === "string" ? r.hysteria2_password : undefined,
    traffic_limit_bytes_override:
      typeof r.traffic_limit_bytes_override === "number" ? r.traffic_limit_bytes_override : null,
    expire_at_override: typeof r.expire_at_override === "string" ? r.expire_at_override : null,
    display_name: typeof r.display_name === "string" ? r.display_name : undefined,
    description: typeof r.description === "string" ? r.description : undefined,
    credential_status: typeof r.credential_status === "string" ? r.credential_status : undefined,
    last_seen_at: typeof r.last_seen_at === "string" ? r.last_seen_at : null,
    last_client_ip: typeof r.last_client_ip === "string" ? r.last_client_ip : null,
    client_profile_id: typeof r.client_profile_id === "string" ? r.client_profile_id : undefined,
  };
}

function mapClient(raw: unknown, accessItems: ClientAccess[]): Client {
  const r = rec(raw);
  const now = new Date().toISOString();
  return {
    id: str(r?.id),
    username: str(r?.username),
    enabled: bool(r?.enabled, false),
    has_subscription: bool(r?.has_subscription),
    artifacts_need_refresh: bool(r?.artifacts_need_refresh),
    last_artifact_rendered_at:
      typeof r?.last_artifact_rendered_at === "string" ? r.last_artifact_rendered_at : null,
    last_artifact_refresh_reason:
      typeof r?.last_artifact_refresh_reason === "string" ? r.last_artifact_refresh_reason : null,
    traffic_limit_bytes: num(r?.traffic_limit_bytes),
    traffic_used_up_bytes: num(r?.traffic_used_up_bytes),
    traffic_used_down_bytes: num(r?.traffic_used_down_bytes),
    expire_at: typeof r?.expire_at === "string" ? r.expire_at : null,
    created_at: str(r?.created_at, now),
    updated_at: str(r?.updated_at, now),
    protocols: uniqueProtocols(accessItems),
    access: accessItems,
  };
}

function mapEntry(raw: unknown): Client {
  const entry = rec(raw);
  const userRaw = {
    ...(rec(entry?.user) ?? rec(raw) ?? {}),
    has_subscription: entry?.has_subscription,
    artifacts_need_refresh: entry?.artifacts_need_refresh,
    last_artifact_rendered_at: entry?.last_artifact_rendered_at,
    last_artifact_refresh_reason: entry?.last_artifact_refresh_reason,
  };
  const access = arr(entry?.access)
    .map(mapAccess)
    .filter((a): a is ClientAccess => a !== null);
  return mapClient(userRaw, access);
}

function mapImpact(raw: unknown): ChangeImpact {
  const r = rec(raw) ?? {};
  return {
    affected_users: num(r.affected_users),
    affected_access: num(r.affected_access),
    affected_inbounds: num(r.affected_inbounds),
    affected_subscriptions: num(r.affected_subscriptions),
    affected_artifacts: num(r.affected_artifacts),
    requires_runtime_apply: bool(r.requires_runtime_apply),
    requires_artifact_refresh: bool(r.requires_artifact_refresh),
    server_ids: strs(r.server_ids),
    inbound_ids: strs(r.inbound_ids),
  };
}

function mapDraftState(raw: unknown): DraftRevisionState {
  const r = rec(raw) ?? {};
  return {
    server_id: str(r.server_id),
    current_revision_id: str(r.current_revision_id) || undefined,
    current_revision_no: typeof r.current_revision_no === "number" ? r.current_revision_no : undefined,
    draft_revision_id: str(r.draft_revision_id) || undefined,
    draft_revision_no: typeof r.draft_revision_no === "number" ? r.draft_revision_no : undefined,
    pending_changes: bool(r.pending_changes),
    check_ok: bool(r.check_ok),
    check_error: typeof r.check_error === "string" ? r.check_error : null,
    apply_status: typeof r.apply_status === "string" ? r.apply_status : null,
    apply_error: typeof r.apply_error === "string" ? r.apply_error : null,
  };
}

function mapBulkMutationResult(raw: unknown): BulkMutationResult {
  const r = rec(raw) ?? {};
  return {
    updated: num(r.updated),
    deleted: num(r.deleted),
    rotated: num(r.rotated),
    regenerated: num(r.regenerated),
    impact: mapImpact(r.impact),
    drafts: arr(r.drafts).map(mapDraftState),
  };
}

export async function listClients(): Promise<Client[]> {
  const res = await apiFetch<{ items: unknown[] }>("/api/v1/users", { method: "GET" });
  return arr(res?.items).map(mapEntry);
}

export async function getClientArtifacts(clientID: string): Promise<ClientArtifacts> {
  const raw = await apiFetch<Record<string, unknown>>(`/api/v1/users/${clientID}/artifacts`, { method: "GET" });
  return {
    primary_token_prefix: str(raw?.primary_token_prefix) || undefined,
    artifact_version: num(raw?.artifact_version, 1),
    artifacts_need_refresh: bool(raw?.artifacts_need_refresh),
    last_artifact_rendered_at:
      typeof raw?.last_artifact_rendered_at === "string" ? raw.last_artifact_rendered_at : null,
    last_artifact_refresh_reason:
      typeof raw?.last_artifact_refresh_reason === "string" ? raw.last_artifact_refresh_reason : null,
    subscription_import_url: str(raw?.subscription_import_url),
    subscription_profile_url: str(raw?.subscription_profile_url),
    subscription_uris_url: str(raw?.subscription_uris_url),
    subscription_qr_url: str(raw?.subscription_qr_url),
    subscription_clash_url: str(raw?.subscription_clash_url),
    subscription_base64_url: str(raw?.subscription_base64_url),
    vless_uris: strs(raw?.vless_uris),
    hy2_uris: strs(raw?.hysteria2_uris),
    all_uris: strs(raw?.all_uris),
    singbox_profile_json: str(raw?.singbox_profile_json),
  };
}

export async function refreshClientArtifacts(clientID: string): Promise<ClientArtifacts> {
  const raw = await apiFetch<Record<string, unknown>>(`/api/v1/users/${clientID}/artifacts/refresh`, {
    method: "POST",
    body: JSON.stringify({}),
    timeoutMs: TIMEOUT,
  });
  return {
    primary_token_prefix: str(raw?.primary_token_prefix) || undefined,
    artifact_version: num(raw?.artifact_version, 1),
    artifacts_need_refresh: bool(raw?.artifacts_need_refresh),
    last_artifact_rendered_at:
      typeof raw?.last_artifact_rendered_at === "string" ? raw.last_artifact_rendered_at : null,
    last_artifact_refresh_reason:
      typeof raw?.last_artifact_refresh_reason === "string" ? raw.last_artifact_refresh_reason : null,
    subscription_import_url: str(raw?.subscription_import_url),
    subscription_profile_url: str(raw?.subscription_profile_url),
    subscription_uris_url: str(raw?.subscription_uris_url),
    subscription_qr_url: str(raw?.subscription_qr_url),
    subscription_clash_url: str(raw?.subscription_clash_url),
    subscription_base64_url: str(raw?.subscription_base64_url),
    vless_uris: strs(raw?.vless_uris),
    hy2_uris: strs(raw?.hysteria2_uris),
    all_uris: strs(raw?.all_uris),
    singbox_profile_json: str(raw?.singbox_profile_json),
  };
}

export async function createClient(input: {
  username: string;
  traffic_limit_bytes: number;
  expire_at: string | null;
}): Promise<void> {
  await apiFetch("/api/v1/users/provision", {
    method: "POST",
    body: JSON.stringify({
      username: input.username,
      traffic_limit_bytes: input.traffic_limit_bytes || 0,
      expire_at: input.expire_at || undefined,
    }),
    timeoutMs: TIMEOUT,
  });
}

export async function updateClient(
  clientID: string,
  input: { username: string; traffic_limit_bytes: number; expire_at: string | null },
): Promise<void> {
  await apiFetch(`/api/v1/users/${clientID}`, {
    method: "PATCH",
    body: JSON.stringify({
      username: input.username,
      traffic_limit_bytes: input.traffic_limit_bytes,
      expire_at: input.expire_at || undefined,
    }),
    timeoutMs: TIMEOUT,
  });
}

export async function deleteClient(clientID: string): Promise<void> {
  await apiFetch(`/api/v1/users/${clientID}`, {
    method: "DELETE",
    body: JSON.stringify({}),
    timeoutMs: TIMEOUT,
  });
}

export async function setClientEnabled(clientID: string, enabled: boolean): Promise<void> {
  await apiFetch(`/api/v1/users/${clientID}`, {
    method: "PATCH",
    body: JSON.stringify({ enabled }),
    timeoutMs: TIMEOUT,
  });
}

export interface BulkPreviewResult {
  user_count: number;
  access_count: number;
  affected_inbound_ids: string[];
  affected_subscriptions: number;
  runtime_change_expected: boolean;
  restart_required: boolean;
}

export async function previewClientsBulkDelete(ids: string[]): Promise<BulkPreviewResult> {
  const res = await apiFetch<BulkPreviewResult>("/api/v1/users/bulk/preview", {
    method: "POST",
    body: JSON.stringify({ ids }),
    timeoutMs: TIMEOUT,
  });
  return res as BulkPreviewResult;
}

export async function deleteClientsBulk(ids: string[]): Promise<number> {
  const res = await apiFetch<{ deleted: number }>("/api/v1/users/bulk/delete", {
    method: "POST",
    body: JSON.stringify({ ids }),
    timeoutMs: TIMEOUT,
  });
  return (res as { deleted: number }).deleted ?? 0;
}

export async function setClientsEnabledBulk(ids: string[], enabled: boolean): Promise<number> {
  const res = await apiFetch<{ updated: number }>("/api/v1/users/bulk/enable-disable", {
    method: "POST",
    body: JSON.stringify({ ids, enabled }),
    timeoutMs: TIMEOUT,
  });
  return (res as { updated: number }).updated ?? 0;
}

export async function previewClientsBulkPatch(patch: BulkUserPatch): Promise<ChangeImpact> {
  const raw = await apiFetch<unknown>("/api/v1/users/bulk/impact", {
    method: "POST",
    body: JSON.stringify(patch),
    timeoutMs: TIMEOUT,
  });
  return mapImpact(raw);
}

export async function applyClientsBulkPatch(patch: BulkUserPatch): Promise<BulkMutationResult> {
  const raw = await apiFetch<unknown>("/api/v1/users/bulk/apply", {
    method: "POST",
    body: JSON.stringify(patch),
    timeoutMs: TIMEOUT,
  });
  return mapBulkMutationResult(raw);
}

export async function previewAccessBulkPatch(patch: BulkAccessPatch): Promise<ChangeImpact> {
  const raw = await apiFetch<unknown>("/api/v1/access/bulk/impact", {
    method: "POST",
    body: JSON.stringify(patch),
    timeoutMs: TIMEOUT,
  });
  return mapImpact(raw);
}

export async function applyAccessBulkPatch(patch: BulkAccessPatch): Promise<BulkMutationResult> {
  const raw = await apiFetch<unknown>("/api/v1/access/bulk/apply", {
    method: "POST",
    body: JSON.stringify(patch),
    timeoutMs: TIMEOUT,
  });
  return mapBulkMutationResult(raw);
}

export function qrURL(clientID: string, value: string, size = 320): string {
  if (!value || !clientID) return "";
  return `/api/v1/users/${clientID}/artifacts/qr.png?value=${encodeURIComponent(value)}&size=${size}`;
}

export function subscriptionQRURL(clientID: string, size = 320): string {
  return `/api/v1/users/${clientID}/artifacts/qr.png?kind=subscription&size=${size}`;
}
