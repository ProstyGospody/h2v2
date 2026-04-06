import { apiFetch } from "@/services/api";
import type { Inbound, Server, VLESSInboundSettings, Hysteria2InboundSettings } from "./types";

const TIMEOUT = 30_000;

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
  return arr(v).map((x) => (typeof x === "string" ? x.trim() : "")).filter(Boolean);
}

function mapVLESS(raw: unknown): VLESSInboundSettings | undefined {
  const r = rec(raw);
  if (!r) return undefined;
  return {
    tls_enabled: bool(r.tls_enabled),
    tls_server_name: str(r.tls_server_name),
    tls_alpn: strs(r.tls_alpn),
    tls_certificate_path: str(r.tls_certificate_path),
    tls_key_path: str(r.tls_key_path),
    reality_enabled: bool(r.reality_enabled),
    reality_public_key: str(r.reality_public_key),
    reality_private_key: str(r.reality_private_key),
    reality_short_id: str(r.reality_short_id),
    reality_handshake_server: str(r.reality_handshake_server),
    reality_handshake_server_port: num(r.reality_handshake_server_port, 443),
    flow: str(r.flow),
    transport_type: str(r.transport_type, "tcp"),
    transport_host: str(r.transport_host),
    transport_path: str(r.transport_path),
    multiplex_enabled: bool(r.multiplex_enabled),
    multiplex_protocol: str(r.multiplex_protocol),
    multiplex_max_connections: num(r.multiplex_max_connections),
    multiplex_min_streams: num(r.multiplex_min_streams),
    multiplex_max_streams: num(r.multiplex_max_streams),
  };
}

function mapHY2(raw: unknown): Hysteria2InboundSettings | undefined {
  const r = rec(raw);
  if (!r) return undefined;
  return {
    tls_enabled: bool(r.tls_enabled),
    tls_server_name: str(r.tls_server_name),
    tls_certificate_path: str(r.tls_certificate_path),
    tls_key_path: str(r.tls_key_path),
    allow_insecure: bool(r.allow_insecure),
    up_mbps: typeof r.up_mbps === "number" ? r.up_mbps : null,
    down_mbps: typeof r.down_mbps === "number" ? r.down_mbps : null,
    ignore_client_bandwidth: bool(r.ignore_client_bandwidth),
    obfs_type: str(r.obfs_type),
    obfs_password: str(r.obfs_password),
    masquerade_json: str(r.masquerade_json),
    bbr_profile: str(r.bbr_profile),
    brutal_debug: bool(r.brutal_debug),
  };
}

function mapInbound(raw: unknown): Inbound {
  const r = rec(raw) ?? {};
  const protocol = str(r.protocol) as "vless" | "hysteria2";
  return {
    id: str(r.id),
    server_id: str(r.server_id),
    name: str(r.name),
    tag: str(r.tag),
    protocol,
    listen: str(r.listen),
    listen_port: num(r.listen_port, 443),
    enabled: bool(r.enabled, true),
    template_key: str(r.template_key),
    vless: protocol === "vless" ? mapVLESS(r.vless) : undefined,
    hysteria2: protocol === "hysteria2" ? mapHY2(r.hysteria2) : undefined,
    created_at: str(r.created_at),
    updated_at: str(r.updated_at),
  };
}

function mapServer(raw: unknown): Server {
  const r = rec(raw) ?? {};
  return {
    id: str(r.id),
    name: str(r.name),
    public_host: str(r.public_host),
    panel_public_url: str(r.panel_public_url),
    subscription_base_url: str(r.subscription_base_url),
    singbox_binary_path: str(r.singbox_binary_path),
    singbox_config_path: str(r.singbox_config_path),
    singbox_service_name: str(r.singbox_service_name),
  };
}

// ---------------------------------------------------------------------------
// Server API
// ---------------------------------------------------------------------------

export async function listServers(): Promise<Server[]> {
  const res = await apiFetch<{ items: unknown[] }>("/api/v1/servers", { method: "GET" });
  return arr(res?.items).map(mapServer);
}

export async function updateServer(id: string, patch: Partial<{
  name: string;
  public_host: string;
  panel_public_url: string;
  subscription_base_url: string;
  singbox_binary_path: string;
  singbox_config_path: string;
  singbox_service_name: string;
}>): Promise<Server> {
  const res = await apiFetch<unknown>(`/api/v1/servers/${id}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
    timeoutMs: TIMEOUT,
  });
  return mapServer(res);
}

// ---------------------------------------------------------------------------
// Inbound API
// ---------------------------------------------------------------------------

export async function previewServerConfig(serverID: string): Promise<{ config_json: string; check_warning?: string }> {
  const res = await apiFetch<{ config_json: string; check_warning?: string }>(`/api/v1/servers/${serverID}/config/preview`, { method: "GET" });
  return { config_json: res?.config_json ?? "{}", check_warning: res?.check_warning };
}

export async function listInbounds(serverID?: string): Promise<Inbound[]> {
  const qs = serverID ? `?server_id=${serverID}` : "";
  const res = await apiFetch<{ items: unknown[] }>(`/api/v1/inbounds${qs}`, { method: "GET" });
  return arr(res?.items).map(mapInbound);
}

export async function generateRealityKeypair(): Promise<{ private_key: string; public_key: string }> {
  const res = await apiFetch<{ private_key: string; public_key: string }>("/api/v1/utils/reality-keypair", {
    method: "POST",
    body: JSON.stringify({}),
    timeoutMs: TIMEOUT,
  });
  return { private_key: res?.private_key ?? "", public_key: res?.public_key ?? "" };
}

export async function updateInbound(
  id: string,
  patch: Partial<{
    name: string;
    listen_port: number;
    enabled: boolean;
    vless: Partial<VLESSInboundSettings>;
    hysteria2: Partial<Hysteria2InboundSettings>;
  }>,
): Promise<Inbound> {
  const res = await apiFetch<unknown>(`/api/v1/inbounds/${id}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
    timeoutMs: TIMEOUT,
  });
  return mapInbound(res);
}
