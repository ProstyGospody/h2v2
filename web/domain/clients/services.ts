import { apiFetch } from "@/services/api";
import { uniqueProtocols } from "./adapters";
import type { Client, ClientAccess, ClientArtifacts, Protocol } from "./types";

const TIMEOUT = 120_000;

// ---------------------------------------------------------------------------
// Helpers
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
// Mapping
// ---------------------------------------------------------------------------

type CoreInbound = { id: string; server_id: string; protocol: string; enabled: boolean };

function mapInbounds(raw: unknown[]): CoreInbound[] {
  return raw
    .map((x) => rec(x))
    .filter((x): x is Record<string, unknown> => x !== null)
    .map((x) => ({
      id: str(x.id),
      server_id: str(x.server_id),
      protocol: str(x.protocol),
      enabled: bool(x.enabled, true),
    }))
    .filter((x) => x.id.length > 0);
}

function protocolMap(inbounds: CoreInbound[]): Record<string, Protocol> {
  const m: Record<string, Protocol> = {};
  for (const ib of inbounds) {
    const p = normalizeProtocol(ib.protocol);
    if (p) m[ib.id] = p;
  }
  return m;
}

function mapAccess(raw: unknown, pmap: Record<string, Protocol>): ClientAccess | null {
  const r = rec(raw);
  if (!r) return null;
  const inboundID = str(r.inbound_id);
  const protocol = normalizeProtocol(r.protocol) || pmap[inboundID] || null;
  if (!protocol) return null;
  return {
    id: str(r.id),
    user_id: str(r.user_id),
    inbound_id: inboundID,
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

// ---------------------------------------------------------------------------
// Server / Inbound auto-provisioning
// ---------------------------------------------------------------------------

type Defaults = { hy2_port: number; hy2_domain: string; hy2_cert_path: string; hy2_key_path: string };

async function fetchDefaults(): Promise<Defaults> {
  try {
    const raw = await apiFetch<Record<string, unknown>>("/api/v1/defaults", { method: "GET" });
    return {
      hy2_port: num(raw?.hy2_port, 443),
      hy2_domain: str(raw?.hy2_domain),
      hy2_cert_path: str(raw?.hy2_cert_path, "/etc/h2v2/hysteria/server.crt"),
      hy2_key_path: str(raw?.hy2_key_path, "/etc/h2v2/hysteria/server.key"),
    };
  } catch {
    return { hy2_port: 443, hy2_domain: "", hy2_cert_path: "/etc/h2v2/hysteria/server.crt", hy2_key_path: "/etc/h2v2/hysteria/server.key" };
  }
}

async function ensureServer(): Promise<string> {
  const res = await apiFetch<{ items: unknown[] }>("/api/v1/servers", { method: "GET" });
  const items = arr(res?.items)
    .map((x) => rec(x))
    .filter((x): x is Record<string, unknown> => x !== null);
  if (items.length > 0) return str(items[0].id);

  const created = await apiFetch<Record<string, unknown>>("/api/v1/servers", {
    method: "POST",
    body: JSON.stringify({ id: "default", name: "default" }),
    timeoutMs: TIMEOUT,
  });
  return str(created?.id);
}

async function ensureInbounds(serverID: string): Promise<{ vless: CoreInbound; hy2: CoreInbound }> {
  const [inboundsRes, defaults] = await Promise.all([
    apiFetch<{ items: unknown[] }>(`/api/v1/inbounds?server_id=${encodeURIComponent(serverID)}`, { method: "GET" }),
    fetchDefaults(),
  ]);
  const existing = mapInbounds(arr(inboundsRes?.items));

  let vless = existing.find((x) => normalizeProtocol(x.protocol) === "vless") || null;
  let hy2 = existing.find((x) => normalizeProtocol(x.protocol) === "hy2") || null;

  if (!vless) {
    const created = await apiFetch<Record<string, unknown>>("/api/v1/inbounds", {
      method: "POST",
      body: JSON.stringify({
        server_id: serverID,
        name: "VLESS Reality",
        tag: "vless-in",
        protocol: "vless",
        listen: "::",
        listen_port: 443,
        enabled: true,
        template_key: "vless-reality",
        vless: {
          tls_enabled: true,
          reality_enabled: true,
          flow: "xtls-rprx-vision",
          transport_type: "tcp",
          multiplex_enabled: false,
        },
      }),
      timeoutMs: TIMEOUT,
    });
    vless = { id: str(created?.id), server_id: serverID, protocol: "vless", enabled: true };
  }

  if (!hy2) {
    const created = await apiFetch<Record<string, unknown>>("/api/v1/inbounds", {
      method: "POST",
      body: JSON.stringify({
        server_id: serverID,
        name: "Hysteria2",
        tag: "hy2-in",
        protocol: "hysteria2",
        listen: "::",
        listen_port: defaults.hy2_port,
        enabled: true,
        template_key: "hysteria2-default",
        hysteria2: {
          tls_enabled: true,
          tls_server_name: defaults.hy2_domain || undefined,
          tls_certificate_path: defaults.hy2_cert_path,
          tls_key_path: defaults.hy2_key_path,
          ignore_client_bandwidth: true,
          brutal_debug: false,
        },
      }),
      timeoutMs: TIMEOUT,
    });
    hy2 = { id: str(created?.id), server_id: serverID, protocol: "hysteria2", enabled: true };
  }

  return { vless, hy2 };
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

export async function listClients(): Promise<Client[]> {
  const [usersRes, inboundsRes] = await Promise.all([
    apiFetch<{ items: unknown[] }>("/api/v1/users", { method: "GET" }),
    apiFetch<{ items: unknown[] }>("/api/v1/inbounds", { method: "GET" }),
  ]);
  const pmap = protocolMap(mapInbounds(arr(inboundsRes?.items)));
  return arr(usersRes?.items).map((raw) => {
    const entry = rec(raw);
    const userRaw = rec(entry?.user) || entry;
    const access = arr(entry?.access)
      .map((a) => mapAccess(a, pmap))
      .filter((a): a is ClientAccess => a !== null);
    return mapClient(userRaw, access);
  });
}

export async function getClientArtifacts(clientID: string): Promise<ClientArtifacts> {
  const raw = await apiFetch<Record<string, unknown>>(`/api/v1/users/${clientID}/artifacts`, { method: "GET" });
  return {
    subscription_import_url: str(raw?.subscription_import_url),
    subscription_profile_url: str(raw?.subscription_profile_url),
    subscription_uris_url: str(raw?.subscription_uris_url),
    subscription_qr_url: str(raw?.subscription_qr_url),
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
}): Promise<Client> {
  const serverID = await ensureServer();
  const { vless, hy2 } = await ensureInbounds(serverID);

  const created = await apiFetch<Record<string, unknown>>("/api/v1/users", {
    method: "POST",
    body: JSON.stringify({
      username: input.username,
      enabled: true,
      traffic_limit_bytes: input.traffic_limit_bytes,
      expire_at: input.expire_at || undefined,
    }),
    timeoutMs: TIMEOUT,
  });
  const userID = str(created?.id);

  // Create access for both protocols
  await Promise.all([
    apiFetch("/api/v1/access", {
      method: "POST",
      body: JSON.stringify({ user_id: userID, inbound_id: vless.id, enabled: true, vless_uuid: "" }),
      timeoutMs: TIMEOUT,
    }),
    apiFetch("/api/v1/access", {
      method: "POST",
      body: JSON.stringify({ user_id: userID, inbound_id: hy2.id, enabled: true, hysteria2_password: "" }),
      timeoutMs: TIMEOUT,
    }),
  ]);

  // Fetch full client with access
  const inbounds = await apiFetch<{ items: unknown[] }>("/api/v1/inbounds", { method: "GET" });
  const pmap = protocolMap(mapInbounds(arr(inbounds?.items)));
  const accessRes = await apiFetch<{ items: unknown[] }>(`/api/v1/users/${userID}/access`, { method: "GET" });
  const access = arr(accessRes?.items)
    .map((a) => mapAccess(a, pmap))
    .filter((a): a is ClientAccess => a !== null);
  const userRes = await apiFetch<Record<string, unknown>>(`/api/v1/users/${userID}`, { method: "GET" });
  return mapClient(userRes, access);
}

export async function updateClient(
  clientID: string,
  input: { username: string; traffic_limit_bytes: number; expire_at: string | null },
): Promise<Client> {
  await apiFetch(`/api/v1/users/${clientID}`, {
    method: "PATCH",
    body: JSON.stringify({
      username: input.username,
      traffic_limit_bytes: input.traffic_limit_bytes,
      expire_at: input.expire_at || undefined,
    }),
    timeoutMs: TIMEOUT,
  });

  const inbounds = await apiFetch<{ items: unknown[] }>("/api/v1/inbounds", { method: "GET" });
  const pmap = protocolMap(mapInbounds(arr(inbounds?.items)));
  const accessRes = await apiFetch<{ items: unknown[] }>(`/api/v1/users/${clientID}/access`, { method: "GET" });
  const access = arr(accessRes?.items)
    .map((a) => mapAccess(a, pmap))
    .filter((a): a is ClientAccess => a !== null);
  const userRes = await apiFetch<Record<string, unknown>>(`/api/v1/users/${clientID}`, { method: "GET" });
  return mapClient(userRes, access);
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

export async function deleteClientsBulk(ids: string[]): Promise<number> {
  let deleted = 0;
  for (const id of ids) {
    await apiFetch(`/api/v1/users/${id}`, { method: "DELETE", body: JSON.stringify({}), timeoutMs: TIMEOUT });
    deleted++;
  }
  return deleted;
}

export async function setClientsEnabledBulk(ids: string[], enabled: boolean): Promise<number> {
  let updated = 0;
  for (const id of ids) {
    await apiFetch(`/api/v1/users/${id}`, { method: "PATCH", body: JSON.stringify({ enabled }), timeoutMs: TIMEOUT });
    updated++;
  }
  return updated;
}

export function qrURL(clientID: string, value: string, size = 320): string {
  if (!value || !clientID) return "";
  return `/api/v1/users/${clientID}/artifacts/qr.png?value=${encodeURIComponent(value)}&size=${size}`;
}

export function subscriptionQRURL(clientID: string, size = 320): string {
  return `/api/v1/users/${clientID}/artifacts/qr.png?kind=subscription&size=${size}`;
}
