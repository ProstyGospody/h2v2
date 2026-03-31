import { apiFetch } from "@/services/api";

import {
  ClientOverrides,
  HysteriaClient,
  HysteriaClientCreateRequest,
  HysteriaClientDefaults,
  HysteriaClientListResponse,
  HysteriaClientUpdateRequest,
  HysteriaUserPayload,
} from "@/domain/clients/types";

const CLIENT_FETCH_LIMIT = 500;

type UnknownRecord = Record<string, unknown>;

function asRecord(value: unknown): UnknownRecord | null {
  if (typeof value !== "object" || value === null) {
    return null;
  }
  return value as UnknownRecord;
}

function readString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function readOptionalString(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function readNumber(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function readBoolean(value: unknown, fallback = false): boolean {
  return typeof value === "boolean" ? value : fallback;
}

function normalizeClientOverrides(input: unknown): ClientOverrides | null {
  const record = asRecord(input);
  if (!record) {
    return null;
  }
  return {
    sni: readOptionalString(record.sni),
    insecure: readBoolean(record.insecure, false),
    pinSHA256: readOptionalString(record.pinSHA256),
    obfsType: readOptionalString(record.obfsType),
    obfsPassword: readOptionalString(record.obfsPassword),
  };
}

function normalizeClient(raw: unknown, index: number): HysteriaClient {
  const record = asRecord(raw);
  const note = record?.note;
  const lastSeenAt = record?.last_seen_at;
  const id = readString(record?.id, `client-${index + 1}`);
  const username = readString(record?.username);
  const now = new Date().toISOString();

  return {
    id,
    username,
    username_normalized: readString(record?.username_normalized, username.toLowerCase()),
    password: readString(record?.password),
    enabled: readBoolean(record?.enabled, false),
    note: typeof note === "string" || note === null ? note : null,
    created_at: readString(record?.created_at, now),
    updated_at: readString(record?.updated_at, now),
    last_seen_at: typeof lastSeenAt === "string" || lastSeenAt === null ? lastSeenAt : null,
    last_tx_bytes: readNumber(record?.last_tx_bytes, 0),
    last_rx_bytes: readNumber(record?.last_rx_bytes, 0),
    online_count: readNumber(record?.online_count, 0),
    download_bps: readNumber(record?.download_bps, 0),
    upload_bps: readNumber(record?.upload_bps, 0),
    client_overrides: normalizeClientOverrides(record?.client_overrides),
  };
}

function normalizeDefaults(raw: unknown): HysteriaClientDefaults {
  const payload = asRecord(raw);
  const clientParams = asRecord(payload?.client_params);
  const serverOptions = asRecord(payload?.server_options);

  return {
    client_params: {
      server: readString(clientParams?.server),
      port: readNumber(clientParams?.port, 443),
      portUnion: readOptionalString(clientParams?.portUnion),
      sni: readOptionalString(clientParams?.sni),
      insecure: readBoolean(clientParams?.insecure, false),
      pinSHA256: readOptionalString(clientParams?.pinSHA256),
      obfsType: readOptionalString(clientParams?.obfsType),
      obfsPassword: readOptionalString(clientParams?.obfsPassword),
    },
    server_options: {
      tls_enabled: readBoolean(serverOptions?.tls_enabled, true),
      tls_mode: readString(serverOptions?.tls_mode, "acme"),
      obfs_type: readOptionalString(serverOptions?.obfs_type),
      masquerade_type: readOptionalString(serverOptions?.masquerade_type),
      bandwidth_up: readOptionalString(serverOptions?.bandwidth_up),
      bandwidth_down: readOptionalString(serverOptions?.bandwidth_down),
      ignore_client_bandwidth: readBoolean(serverOptions?.ignore_client_bandwidth, false),
    },
  };
}

export async function listClients(): Promise<{ items: HysteriaClient[]; limited: boolean }> {
  const payload = await apiFetch<HysteriaClientListResponse>(`/api/hysteria/users?limit=${CLIENT_FETCH_LIMIT}`, { method: "GET" });
  const rawItems = Array.isArray(payload?.items) ? payload.items : [];
  const items = rawItems.map((item, index) => normalizeClient(item, index));
  return { items, limited: items.length >= CLIENT_FETCH_LIMIT };
}

export async function getClientDefaults(): Promise<HysteriaClientDefaults> {
  const payload = await apiFetch<unknown>("/api/hysteria/client-defaults", { method: "GET" });
  return normalizeDefaults(payload);
}

export function getClientArtifacts(clientID: string): Promise<HysteriaUserPayload> {
  return apiFetch<HysteriaUserPayload>(`/api/hysteria/users/${clientID}/artifacts`, { method: "GET" });
}

export function createClient(input: HysteriaClientCreateRequest): Promise<HysteriaUserPayload> {
  return apiFetch<HysteriaUserPayload>("/api/hysteria/users", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateClient(clientID: string, input: HysteriaClientUpdateRequest): Promise<HysteriaUserPayload> {
  return apiFetch<HysteriaUserPayload>(`/api/hysteria/users/${clientID}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deleteClient(clientID: string): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>(`/api/hysteria/users/${clientID}`, {
    method: "DELETE",
    body: JSON.stringify({}),
  });
}

export function setClientEnabled(clientID: string, enabled: boolean): Promise<{ ok: boolean; enabled: boolean }> {
  return apiFetch<{ ok: boolean; enabled: boolean }>(`/api/hysteria/users/${clientID}/${enabled ? "enable" : "disable"}`, {
    method: "POST",
    body: JSON.stringify({}),
  });
}

export function qrURL(clientID: string, size = 360, kind: "access" | "subscription" = "access"): string {
  return `/api/hysteria/users/${clientID}/qr?size=${size}&kind=${kind === "subscription" ? "subscription" : "access"}`;
}
