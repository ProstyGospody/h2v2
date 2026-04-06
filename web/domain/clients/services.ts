import { apiFetch } from "@/services/api";
import { uniqueProtocols } from "./adapters";
import type { Client, ClientAccess, ClientArtifacts, Protocol } from "./types";

const TIMEOUT = 120_000;

// ---------------------------------------------------------------------------
// Normalisation helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Mapping — the /api/v1/users endpoint already embeds protocol per access entry
// ---------------------------------------------------------------------------

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
    hysteria2_password: typeof r.hysteria2_password === "string" ? r.hysteria2_password : undefined,
  };
}

function mapClient(raw: unknown, accessItems: ClientAccess[]): Client {
  const r = rec(raw);
  const now = new Date().toISOString();
  return {
    id: str(r?.id),
    username: str(r?.username),
    enabled: bool(r?.enabled, false),
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
  const userRaw = rec(entry?.user) || entry;
  const access = arr(entry?.access)
    .map(mapAccess)
    .filter((a): a is ClientAccess => a !== null);
  return mapClient(userRaw, access);
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

export async function listClients(): Promise<Client[]> {
  const res = await apiFetch<{ items: unknown[] }>("/api/v1/users", { method: "GET" });
  return arr(res?.items).map(mapEntry);
}

export async function getClientArtifacts(clientID: string): Promise<ClientArtifacts> {
  const raw = await apiFetch<Record<string, unknown>>(`/api/v1/users/${clientID}/artifacts`, { method: "GET" });
  return {
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

// QR helpers — value= passes the URI to render server-side; kind=subscription uses the
// subscription import URL from the user's artifacts.
export function qrURL(clientID: string, value: string, size = 320): string {
  if (!value || !clientID) return "";
  return `/api/v1/users/${clientID}/artifacts/qr.png?value=${encodeURIComponent(value)}&size=${size}`;
}

export function subscriptionQRURL(clientID: string, size = 320): string {
  return `/api/v1/users/${clientID}/artifacts/qr.png?kind=subscription&size=${size}`;
}
