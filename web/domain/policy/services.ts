/**
 * Policy entity services — outbounds, route rules, DNS/log profiles, security
 * profiles, and user-facing client profiles.
 */
import { apiFetch } from "@/services/api";
import type {
  Outbound,
  RouteRule,
  DNSProfile,
  LogProfile,
  RealityProfile,
  TransportProfile,
  MultiplexProfile,
  HY2MasqueradeProfile,
  ClientProfile,
  DomainValidationResult,
} from "@/types/common";

const TIMEOUT = 30_000;
const BASE = "/api/v1";

// ─── Generic helpers ──────────────────────────────────────────────────────────

async function listItems<T>(path: string, serverID?: string): Promise<T[]> {
  const url = serverID ? `${BASE}${path}?server_id=${encodeURIComponent(serverID)}` : `${BASE}${path}`;
  const data = await apiFetch(url, { timeoutMs: TIMEOUT });
  return Array.isArray(data) ? (data as T[]) : [];
}

async function getItem<T>(path: string): Promise<T> {
  return apiFetch(`${BASE}${path}`, { timeoutMs: TIMEOUT }) as Promise<T>;
}

async function upsertItem<T>(path: string, body: Partial<T>, id?: string): Promise<T> {
  const url = id ? `${BASE}${path}/${id}` : `${BASE}${path}`;
  const method = id ? "PATCH" : "POST";
  return apiFetch(url, {
    method,
    body: JSON.stringify(body),
    timeoutMs: TIMEOUT,
  }) as Promise<T>;
}

async function deleteItem(path: string, id: string): Promise<void> {
  await apiFetch(`${BASE}${path}/${id}`, { method: "DELETE", timeoutMs: TIMEOUT });
}

// ─── Outbounds ────────────────────────────────────────────────────────────────

export const listOutbounds = (serverID?: string) => listItems<Outbound>("/outbounds", serverID);
export const getOutbound = (id: string) => getItem<Outbound>(`/outbounds/${id}`);
export const upsertOutbound = (body: Partial<Outbound>, id?: string) => upsertItem<Outbound>("/outbounds", body, id);
export const deleteOutbound = (id: string) => deleteItem("/outbounds", id);

// ─── Route Rules ──────────────────────────────────────────────────────────────

export const listRouteRules = (serverID?: string) => listItems<RouteRule>("/route-rules", serverID);
export const getRouteRule = (id: string) => getItem<RouteRule>(`/route-rules/${id}`);
export const upsertRouteRule = (body: Partial<RouteRule>, id?: string) => upsertItem<RouteRule>("/route-rules", body, id);
export const deleteRouteRule = (id: string) => deleteItem("/route-rules", id);

// ─── DNS Profiles ─────────────────────────────────────────────────────────────

export const listDNSProfiles = (serverID?: string) => listItems<DNSProfile>("/dns-profiles", serverID);
export const getDNSProfile = (id: string) => getItem<DNSProfile>(`/dns-profiles/${id}`);
export const upsertDNSProfile = (body: Partial<DNSProfile>, id?: string) => upsertItem<DNSProfile>("/dns-profiles", body, id);
export const deleteDNSProfile = (id: string) => deleteItem("/dns-profiles", id);

// ─── Log Profiles ─────────────────────────────────────────────────────────────

export const listLogProfiles = (serverID?: string) => listItems<LogProfile>("/log-profiles", serverID);
export const getLogProfile = (id: string) => getItem<LogProfile>(`/log-profiles/${id}`);
export const upsertLogProfile = (body: Partial<LogProfile>, id?: string) => upsertItem<LogProfile>("/log-profiles", body, id);
export const deleteLogProfile = (id: string) => deleteItem("/log-profiles", id);

// ─── Reality Profiles ─────────────────────────────────────────────────────────

export const listRealityProfiles = (serverID?: string) => listItems<RealityProfile>("/reality-profiles", serverID);
export const getRealityProfile = (id: string) => getItem<RealityProfile>(`/reality-profiles/${id}`);
export const upsertRealityProfile = (body: Partial<RealityProfile>, id?: string) => upsertItem<RealityProfile>("/reality-profiles", body, id);
export const deleteRealityProfile = (id: string) => deleteItem("/reality-profiles", id);

// ─── Transport Profiles ───────────────────────────────────────────────────────

export const listTransportProfiles = (serverID?: string) => listItems<TransportProfile>("/transport-profiles", serverID);
export const getTransportProfile = (id: string) => getItem<TransportProfile>(`/transport-profiles/${id}`);
export const upsertTransportProfile = (body: Partial<TransportProfile>, id?: string) => upsertItem<TransportProfile>("/transport-profiles", body, id);
export const deleteTransportProfile = (id: string) => deleteItem("/transport-profiles", id);

// ─── Multiplex Profiles ───────────────────────────────────────────────────────

export const listMultiplexProfiles = (serverID?: string) => listItems<MultiplexProfile>("/multiplex-profiles", serverID);
export const getMultiplexProfile = (id: string) => getItem<MultiplexProfile>(`/multiplex-profiles/${id}`);
export const upsertMultiplexProfile = (body: Partial<MultiplexProfile>, id?: string) => upsertItem<MultiplexProfile>("/multiplex-profiles", body, id);
export const deleteMultiplexProfile = (id: string) => deleteItem("/multiplex-profiles", id);

// ─── HY2 Masquerade Profiles ──────────────────────────────────────────────────

export const listHY2MasqueradeProfiles = (serverID?: string) => listItems<HY2MasqueradeProfile>("/hy2-masquerade-profiles", serverID);
export const getHY2MasqueradeProfile = (id: string) => getItem<HY2MasqueradeProfile>(`/hy2-masquerade-profiles/${id}`);
export const upsertHY2MasqueradeProfile = (body: Partial<HY2MasqueradeProfile>, id?: string) => upsertItem<HY2MasqueradeProfile>("/hy2-masquerade-profiles", body, id);
export const deleteHY2MasqueradeProfile = (id: string) => deleteItem("/hy2-masquerade-profiles", id);

// ─── Client Profiles ──────────────────────────────────────────────────────────

export const listClientProfiles = (serverID?: string) => listItems<ClientProfile>("/client-profiles", serverID);
export const getClientProfile = (id: string) => getItem<ClientProfile>(`/client-profiles/${id}`);
export const upsertClientProfile = (body: Partial<ClientProfile>, id?: string) => upsertItem<ClientProfile>("/client-profiles", body, id);
export const deleteClientProfile = (id: string) => deleteItem("/client-profiles", id);

// ─── Domain Validation ────────────────────────────────────────────────────────

export const validateServerDomain = (serverID: string): Promise<DomainValidationResult> =>
  apiFetch(`${BASE}/servers/${serverID}/validate/domain`, { timeoutMs: TIMEOUT }) as Promise<DomainValidationResult>;

// ─── HY2 connection mode presets ─────────────────────────────────────────────

export type HY2ConnectionMode = "standard" | "obfuscated" | "poor_network" | "port_hopping";
export type VLESSConnectionMode = "standard" | "multiplex" | "udp_compat" | "transport" | "compat";

/** Human-readable labels for HY2 connection modes. */
export const HY2_MODE_LABELS: Record<HY2ConnectionMode, string> = {
  standard: "Standard",
  obfuscated: "Obfuscated",
  poor_network: "Poor Network",
  port_hopping: "Port Hopping",
};

/** Human-readable labels for VLESS connection modes. */
export const VLESS_MODE_LABELS: Record<VLESSConnectionMode, string> = {
  standard: "Standard",
  multiplex: "Multiplex",
  udp_compat: "UDP Compatibility",
  transport: "Transport Optimized",
  compat: "Compatibility Mode",
};
