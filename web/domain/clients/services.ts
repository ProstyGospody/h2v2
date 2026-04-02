import { apiFetch, APIError } from "@/services/api";

import {
  ClientOverrides,
  Credential,
  HysteriaClient,
  HysteriaClientCreateRequest,
  HysteriaClientDefaults,
  HysteriaClientDeleteBatchResponse,
  HysteriaClientStateBatchResponse,
  HysteriaUserArtifacts,
  HysteriaUserPayload,
  Protocol,
  UserArtifacts,
} from "@/domain/clients/types";

const CLIENT_FETCH_LIMIT = 500;
const HYSTERIA_MUTATION_TIMEOUT_MS = 120_000;

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

function normalizeCredential(raw: unknown): Credential | null {
  const record = asRecord(raw);
  if (!record) {
    return null;
  }
  const protocolRaw = readString(record.protocol).toLowerCase();
  if (protocolRaw !== "hy2" && protocolRaw !== "vless") {
    return null;
  }
  return {
    protocol: protocolRaw as Protocol,
    identity: readString(record.identity),
    secret: readOptionalString(record.secret),
    data_json: readOptionalString(record.data_json),
  };
}

function protocolsFromCredentials(credentials: Credential[]): Protocol[] {
  const hasHY2 = credentials.some((item) => item.protocol === "hy2");
  const hasVLESS = credentials.some((item) => item.protocol === "vless");
  const result: Protocol[] = [];
  if (hasHY2) result.push("hy2");
  if (hasVLESS) result.push("vless");
  if (result.length === 0) {
    result.push("hy2");
  }
  return result;
}

function preferredProtocol(credentials: Credential[]): Protocol {
  for (const credential of credentials) {
    if (credential.protocol === "hy2") {
      return "hy2";
    }
  }
  for (const credential of credentials) {
    if (credential.protocol === "vless") {
      return "vless";
    }
  }
  return "hy2";
}

function selectPrimaryCredential(credentials: Credential[], protocol: Protocol): Credential | null {
  for (const item of credentials) {
    if (item.protocol === protocol) {
      return item;
    }
  }
  return null;
}

function mapUnifiedUser(raw: unknown, index: number): HysteriaClient {
  const payload = asRecord(raw);
  const userRecord = asRecord(payload?.user) ?? payload;
  const credentialsRaw = Array.isArray(userRecord?.credentials) ? userRecord.credentials : [];
  const credentials = credentialsRaw.map((item) => normalizeCredential(item)).filter((item): item is Credential => item !== null);
  const protocols = protocolsFromCredentials(credentials);
  const preferred = preferredProtocol(credentials);
  const primary = selectPrimaryCredential(credentials, preferred);

  const id = readString(userRecord?.id, `user-${index + 1}`);
  const username = readString(userRecord?.name, readString(userRecord?.username));
  const normalized = readString(userRecord?.name_normalized, readString(userRecord?.username_normalized, username.toLowerCase()));
  const now = new Date().toISOString();

  let overrides: ClientOverrides | null = null;
  if (primary?.data_json) {
    try {
      overrides = normalizeClientOverrides(JSON.parse(primary.data_json));
    } catch {
      overrides = null;
    }
  }

  return {
    id,
    username,
    username_normalized: normalized,
    password: primary?.secret || primary?.identity || "",
    enabled: readBoolean(userRecord?.enabled, false),
    note: typeof userRecord?.note === "string" || userRecord?.note === null ? (userRecord.note as string | null) : null,
    created_at: readString(userRecord?.created_at, now),
    updated_at: readString(userRecord?.updated_at, now),
    last_seen_at: typeof userRecord?.last_seen_at === "string" || userRecord?.last_seen_at === null ? (userRecord.last_seen_at as string | null) : null,
    last_tx_bytes: readNumber(userRecord?.traffic_used_tx_bytes, readNumber(userRecord?.last_tx_bytes, 0)),
    last_rx_bytes: readNumber(userRecord?.traffic_used_rx_bytes, readNumber(userRecord?.last_rx_bytes, 0)),
    online_count: readNumber(userRecord?.online_count, 0),
    download_bps: readNumber(userRecord?.download_bps, 0),
    upload_bps: readNumber(userRecord?.upload_bps, 0),
    traffic_limit_bytes: readNumber(userRecord?.traffic_limit_bytes, 0),
    expire_at: typeof userRecord?.expire_at === "string" || userRecord?.expire_at === null ? (userRecord.expire_at as string | null) : null,
    credentials,
    protocols,
    preferred_protocol: preferred,
    client_overrides: overrides,
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

function normalizeUnifiedArtifacts(raw: unknown): Record<string, UserArtifacts> {
  const record = asRecord(raw);
  if (!record) {
    return {};
  }
  const result: Record<string, UserArtifacts> = {};
  for (const [key, value] of Object.entries(record)) {
    const item = asRecord(value);
    if (!item) {
      continue;
    }
    result[key] = {
      protocol: (readString(item.protocol) || key) as Protocol,
      access_uri: readOptionalString(item.access_uri),
      config: readOptionalString(item.config),
      subscription: readOptionalString(item.subscription),
      clash_node: readOptionalString(item.clash_node),
      singbox_node: (asRecord(item.singbox_node) || undefined) as Record<string, unknown> | undefined,
    };
  }
  return result;
}

function mapUnifiedPayload(raw: unknown): HysteriaUserPayload {
  const record = asRecord(raw);
  const user = mapUnifiedUser(record, 0);
  const artifacts = normalizeUnifiedArtifacts(record?.artifacts);
  const subscriptionURL = readString(record?.subscription_url, "");

  const preferred = user.preferred_protocol;
  const preferredArtifact = artifacts[preferred] || artifacts.hy2 || artifacts.vless;

  const payload: HysteriaUserPayload = {
    user,
    artifacts: {
      uri: preferredArtifact?.access_uri || "",
      uri_hy2: artifacts.hy2?.access_uri || "",
      subscription_url: subscriptionURL || preferredArtifact?.subscription || "",
      client_config: preferredArtifact?.config || "",
      client_params: {
        server: "",
        port: 0,
        insecure: false,
      },
      server_defaults: {
        server: "",
        port: 0,
        insecure: false,
      },
      server_options: {
        tls_enabled: true,
        tls_mode: "managed",
        ignore_client_bandwidth: false,
      },
      singbox_outbound: preferredArtifact?.singbox_node || {},
      unified: artifacts,
    },
  };

  return payload;
}

function mapUnifiedCredentials(input: HysteriaClientCreateRequest): Credential[] {
  const protocol = (input.protocol || "hy2") as Protocol;
  if (protocol === "vless") {
    return [{ protocol: "vless", identity: (input.uuid || "").trim() }];
  }
  return [{
    protocol: "hy2",
    identity: input.username.trim(),
    secret: (input.auth_secret || "").trim(),
    data_json: input.client_overrides ? JSON.stringify(input.client_overrides) : undefined,
  }];
}

async function listUnifiedClients(): Promise<{ items: HysteriaClient[]; limited: boolean }> {
  const payload = await apiFetch<{ items: unknown[] }>(`/api/users?limit=${CLIENT_FETCH_LIMIT}`, { method: "GET" });
  const rawItems = Array.isArray(payload?.items) ? payload.items : [];
  const items = rawItems.map((item, index) => mapUnifiedUser(item, index));
  return { items, limited: items.length >= CLIENT_FETCH_LIMIT };
}

export async function listClients(): Promise<{ items: HysteriaClient[]; limited: boolean }> {
  try {
    return await listUnifiedClients();
  } catch (error) {
    if (!(error instanceof APIError) || error.status === 404) {
      const payload = await apiFetch<{ items: unknown[] }>(`/api/hysteria/users?limit=${CLIENT_FETCH_LIMIT}`, { method: "GET" });
      const rawItems = Array.isArray(payload?.items) ? payload.items : [];
      const items = rawItems.map((item, index) => mapUnifiedUser({ user: item }, index));
      return { items, limited: items.length >= CLIENT_FETCH_LIMIT };
    }
    throw error;
  }
}

export async function getClientDefaults(): Promise<HysteriaClientDefaults> {
  const payload = await apiFetch<unknown>("/api/hysteria/client-defaults", { method: "GET" });
  return normalizeDefaults(payload);
}

export async function getClientArtifacts(clientID: string): Promise<HysteriaUserPayload> {
  try {
    const payload = await apiFetch<unknown>(`/api/users/${clientID}`, { method: "GET" });
    return mapUnifiedPayload(payload);
  } catch (error) {
    if (error instanceof APIError && error.status !== 404) {
      throw error;
    }
    return apiFetch<HysteriaUserPayload>(`/api/hysteria/users/${clientID}/artifacts`, { method: "GET" });
  }
}

export function createClient(input: HysteriaClientCreateRequest): Promise<HysteriaUserPayload> {
  return apiFetch<unknown>("/api/users", {
    method: "POST",
    body: JSON.stringify({
      name: input.username,
      note: input.note?.trim() || undefined,
      enabled: true,
      traffic_limit_bytes: input.traffic_limit_bytes || 0,
      expire_at: input.expire_at || undefined,
      credentials: mapUnifiedCredentials(input),
    }),
    timeoutMs: HYSTERIA_MUTATION_TIMEOUT_MS,
  }).then((payload) => mapUnifiedPayload(payload));
}

export function updateClient(clientID: string, input: HysteriaClientCreateRequest): Promise<HysteriaUserPayload> {
  return apiFetch<unknown>(`/api/users/${clientID}`, {
    method: "PATCH",
    body: JSON.stringify({
      name: input.username,
      note: input.note?.trim() || undefined,
      traffic_limit_bytes: input.traffic_limit_bytes || 0,
      expire_at: input.expire_at || undefined,
      credentials: mapUnifiedCredentials(input),
    }),
    timeoutMs: HYSTERIA_MUTATION_TIMEOUT_MS,
  }).then((payload) => mapUnifiedPayload(payload));
}

export function deleteClient(clientID: string): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>(`/api/users/${clientID}`, {
    method: "DELETE",
    body: JSON.stringify({}),
    timeoutMs: HYSTERIA_MUTATION_TIMEOUT_MS,
  });
}

export function setClientEnabled(clientID: string, enabled: boolean): Promise<{ ok: boolean; enabled: boolean }> {
  return apiFetch<{ ok: boolean; enabled: boolean }>("/api/users/state", {
    method: "POST",
    body: JSON.stringify({ ids: [clientID], enabled }),
    timeoutMs: HYSTERIA_MUTATION_TIMEOUT_MS,
  });
}

export function setClientsEnabledBulk(clientIDs: string[], enabled: boolean): Promise<HysteriaClientStateBatchResponse> {
  const ids = clientIDs.map((id) => id.trim()).filter((id) => id.length > 0);
  return apiFetch<HysteriaClientStateBatchResponse>("/api/users/state", {
    method: "POST",
    body: JSON.stringify({ ids, enabled }),
    timeoutMs: HYSTERIA_MUTATION_TIMEOUT_MS,
  });
}

export function deleteClientsBulk(clientIDs: string[]): Promise<HysteriaClientDeleteBatchResponse> {
  const ids = clientIDs.map((id) => id.trim()).filter((id) => id.length > 0);
  return apiFetch<HysteriaClientDeleteBatchResponse>("/api/users/delete", {
    method: "POST",
    body: JSON.stringify({ ids }),
    timeoutMs: HYSTERIA_MUTATION_TIMEOUT_MS,
  });
}

export function qrURL(clientID: string, size = 360, kind: "access" | "subscription" = "access"): string {
  return `/api/users/${clientID}/qr?size=${size}&kind=${kind === "subscription" ? "subscription" : "access"}`;
}
