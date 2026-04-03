import { APIError, apiFetch } from "@/services/api";

import {
  Client,
  ClientAccess,
  ClientCreateRequest,
  ClientDeleteBatchResponse,
  ClientStateBatchResponse,
  Protocol,
  UserPayload,
} from "@/domain/clients/types";

const CLIENT_MUTATION_TIMEOUT_MS = 120_000;
const DEFAULT_SERVER_ID = "default";
const DEFAULT_SERVER_NAME = "default";
const DEFAULT_VLESS_INBOUND_ID = "vless-default";
const DEFAULT_HY2_INBOUND_ID = "hy2-default";
const DEFAULT_HY2_CERT_PATH = "/etc/h2v2/hysteria/server.crt";
const DEFAULT_HY2_KEY_PATH = "/etc/h2v2/hysteria/server.key";

type UnknownRecord = Record<string, unknown>;

type CoreServer = {
  id: string;
  name: string;
};

type CoreInbound = {
  id: string;
  server_id: string;
  protocol: string;
  enabled: boolean;
};

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

function readStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => (typeof item === "string" ? item.trim() : ""))
    .filter((item) => item.length > 0);
}

function uniqueProtocols(items: Protocol[]): Protocol[] {
  return Array.from(new Set(items));
}

function normalizeProtocol(value: unknown): Protocol | null {
  const raw = readString(value).toLowerCase().trim();
  if (raw === "vless") {
    return "vless";
  }
  if (raw === "hy2" || raw === "hysteria2") {
    return "hy2";
  }
  return null;
}

function preferredProtocol(access: ClientAccess[]): Protocol {
  const enabled = access.filter((item) => item.enabled);
  const source = enabled.length > 0 ? enabled : access;
  for (const item of source) {
    if (item.protocol === "hy2") {
      return "hy2";
    }
  }
  for (const item of source) {
    if (item.protocol === "vless") {
      return "vless";
    }
  }
  return "hy2";
}

function mapCoreAccess(raw: unknown, protocolByInboundID: Record<string, Protocol>): ClientAccess | null {
  const record = asRecord(raw);
  if (!record) {
    return null;
  }
  const inboundID = readString(record.inbound_id);
  const protocol = normalizeProtocol(record.protocol) || protocolByInboundID[inboundID] || null;
  if (!protocol) {
    return null;
  }
  return {
    id: readString(record.id),
    user_id: readString(record.user_id),
    inbound_id: inboundID,
    enabled: readBoolean(record.enabled, true),
    protocol,
    vless_uuid: readOptionalString(record.vless_uuid),
    hysteria2_password: readOptionalString(record.hysteria2_password),
  };
}

function mapClientFromCore(userRaw: unknown, accessItems: ClientAccess[], index: number): Client {
  const record = asRecord(userRaw);
  const now = new Date().toISOString();
  const id = readString(record?.id, `user-${index + 1}`);
  const username = readString(record?.username, readString(record?.name, id));
  const protocols = uniqueProtocols(accessItems.map((item) => item.protocol));
  const preferred = preferredProtocol(accessItems);
  const preferredAccess = accessItems.find((item) => item.protocol === preferred) || null;
  const password = preferred === "vless"
    ? (preferredAccess?.vless_uuid || "")
    : (preferredAccess?.hysteria2_password || "");

  return {
    id,
    username,
    username_normalized: username.toLowerCase(),
    password,
    enabled: readBoolean(record?.enabled, false),
    created_at: readString(record?.created_at, now),
    updated_at: readString(record?.updated_at, now),
    last_seen_at: readOptionalString(record?.last_seen_at) || readOptionalString(record?.updated_at),
    last_tx_bytes: readNumber(record?.traffic_used_up_bytes, readNumber(record?.traffic_used_tx_bytes, 0)),
    last_rx_bytes: readNumber(record?.traffic_used_down_bytes, readNumber(record?.traffic_used_rx_bytes, 0)),
    online_count: 0,
    download_bps: 0,
    upload_bps: 0,
    traffic_limit_bytes: readNumber(record?.traffic_limit_bytes, 0),
    expire_at: (typeof record?.expire_at === "string" || record?.expire_at === null) ? (record.expire_at as string | null) : null,
    protocols: protocols.length > 0 ? protocols : [preferred],
    preferred_protocol: preferred,
    access: accessItems,
  };
}

function mapProtocolByInbound(items: CoreInbound[]): Record<string, Protocol> {
  const result: Record<string, Protocol> = {};
  for (const item of items) {
    const protocol = normalizeProtocol(item.protocol);
    if (!protocol || !item.id) {
      continue;
    }
    result[item.id] = protocol;
  }
  return result;
}

async function listCoreServers(): Promise<CoreServer[]> {
  const payload = await apiFetch<{ items: unknown[] }>("/api/v1/servers", { method: "GET" });
  const rawItems = Array.isArray(payload?.items) ? payload.items : [];
  return rawItems
    .map((item) => asRecord(item))
    .filter((item): item is UnknownRecord => item !== null)
    .map((item) => ({
      id: readString(item.id),
      name: readString(item.name),
    }))
    .filter((item) => item.id.length > 0);
}

async function listCoreInbounds(serverID?: string): Promise<CoreInbound[]> {
  const query = serverID ? `?server_id=${encodeURIComponent(serverID)}` : "";
  const payload = await apiFetch<{ items: unknown[] }>(`/api/v1/inbounds${query}`, { method: "GET" });
  const rawItems = Array.isArray(payload?.items) ? payload.items : [];
  return rawItems
    .map((item) => asRecord(item))
    .filter((item): item is UnknownRecord => item !== null)
    .map((item) => ({
      id: readString(item.id),
      server_id: readString(item.server_id),
      protocol: readString(item.protocol),
      enabled: readBoolean(item.enabled, true),
    }))
    .filter((item) => item.id.length > 0);
}

async function createDefaultCoreServer(): Promise<CoreServer> {
  const payload = await apiFetch<unknown>("/api/v1/servers", {
    method: "POST",
    body: JSON.stringify({
      id: DEFAULT_SERVER_ID,
      name: DEFAULT_SERVER_NAME,
    }),
    timeoutMs: CLIENT_MUTATION_TIMEOUT_MS,
  });
  const record = asRecord(payload);
  const id = readString(record?.id);
  const name = readString(record?.name, DEFAULT_SERVER_NAME);
  if (!id) {
    throw new APIError("Failed to create server", 500, payload, "runtime_setup");
  }
  return { id, name };
}

async function ensureCoreServer(): Promise<CoreServer> {
  const servers = await listCoreServers();
  if (servers.length > 0) {
    return servers[0];
  }
  return createDefaultCoreServer();
}

function defaultInboundPayload(serverID: string, protocol: Protocol): Record<string, unknown> {
  if (protocol === "vless") {
    return {
      id: DEFAULT_VLESS_INBOUND_ID,
      server_id: serverID,
      name: "VLESS",
      tag: DEFAULT_VLESS_INBOUND_ID,
      protocol: "vless",
      listen: "::",
      listen_port: 443,
      enabled: true,
      template_key: "vless-default",
      vless: {
        tls_enabled: false,
        reality_enabled: false,
        transport_type: "tcp",
        multiplex_enabled: false,
      },
    };
  }
  return {
    id: DEFAULT_HY2_INBOUND_ID,
    server_id: serverID,
    name: "HY2",
    tag: DEFAULT_HY2_INBOUND_ID,
    protocol: "hysteria2",
    listen: "::",
    listen_port: 443,
    enabled: true,
    template_key: "hysteria2-default",
    hysteria2: {
      tls_enabled: true,
      tls_certificate_path: DEFAULT_HY2_CERT_PATH,
      tls_key_path: DEFAULT_HY2_KEY_PATH,
      ignore_client_bandwidth: false,
      brutal_debug: false,
    },
  };
}

async function ensureCoreInbound(serverID: string, protocol: Protocol): Promise<CoreInbound> {
  const inbounds = await listCoreInbounds(serverID);
  const selected = inbounds.find((item) => normalizeProtocol(item.protocol) === protocol) || null;
  if (selected) {
    if (!selected.enabled) {
      await apiFetch<unknown>(`/api/v1/inbounds/${selected.id}`, {
        method: "PATCH",
        body: JSON.stringify({ enabled: true }),
        timeoutMs: CLIENT_MUTATION_TIMEOUT_MS,
      });
      return { ...selected, enabled: true };
    }
    return selected;
  }

  const created = await apiFetch<unknown>("/api/v1/inbounds", {
    method: "POST",
    body: JSON.stringify(defaultInboundPayload(serverID, protocol)),
    timeoutMs: CLIENT_MUTATION_TIMEOUT_MS,
  });
  const record = asRecord(created);
  const id = readString(record?.id);
  if (!id) {
    throw new APIError("Failed to create inbound", 500, created, "runtime_setup");
  }
  return {
    id,
    server_id: readString(record?.server_id, serverID),
    protocol: readString(record?.protocol, protocol),
    enabled: readBoolean(record?.enabled, true),
  };
}

async function ensureCoreContext(protocol: Protocol): Promise<{ server: CoreServer; inbound: CoreInbound }> {
  const server = await ensureCoreServer();
  const inbound = await ensureCoreInbound(server.id, protocol);
  return { server, inbound };
}

function accessPayloadForProtocol(userID: string, inboundID: string, protocol: Protocol, input: ClientCreateRequest): Record<string, unknown> {
  if (protocol === "vless") {
    return {
      user_id: userID,
      inbound_id: inboundID,
      enabled: true,
      vless_uuid: (input.uuid || "").trim(),
    };
  }
  return {
    user_id: userID,
    inbound_id: inboundID,
    enabled: true,
    hysteria2_password: (input.auth_secret || "").trim(),
  };
}

async function upsertCoreAccess(payload: Record<string, unknown>): Promise<void> {
  await apiFetch<unknown>("/api/v1/access", {
    method: "POST",
    body: JSON.stringify(payload),
    timeoutMs: CLIENT_MUTATION_TIMEOUT_MS,
  });
}

async function listCoreUserAccess(userID: string, protocolByInboundID: Record<string, Protocol>): Promise<ClientAccess[]> {
  const payload = await apiFetch<{ items: unknown[] }>(`/api/v1/users/${userID}/access`, { method: "GET" });
  const rawItems = Array.isArray(payload?.items) ? payload.items : [];
  return rawItems
    .map((item) => mapCoreAccess(item, protocolByInboundID))
    .filter((item): item is ClientAccess => item !== null);
}

function normalizeIDs(raw: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const value of raw) {
    const id = value.trim();
    if (!id || seen.has(id)) {
      continue;
    }
    seen.add(id);
    out.push(id);
  }
  return out;
}

function resolveAccessURLByProtocol(artifacts: UnknownRecord, protocol: Protocol): string {
  const vless = readStringArray(artifacts.vless_uris);
  const hy2 = readStringArray(artifacts.hysteria2_uris);
  const all = readStringArray(artifacts.all_uris);
  if (protocol === "vless" && vless.length > 0) {
    return vless[0];
  }
  if (protocol === "hy2" && hy2.length > 0) {
    return hy2[0];
  }
  if (vless.length > 0) {
    return vless[0];
  }
  if (hy2.length > 0) {
    return hy2[0];
  }
  return all[0] || "";
}

export async function listClients(): Promise<{ items: Client[]; limited: boolean }> {
  const [usersPayload, inbounds] = await Promise.all([
    apiFetch<{ items: unknown[] }>("/api/v1/users", { method: "GET" }),
    listCoreInbounds(),
  ]);
  const protocolByInboundID = mapProtocolByInbound(inbounds);
  const rawItems = Array.isArray(usersPayload?.items) ? usersPayload.items : [];
  const items = rawItems.map((item, index) => {
    const entry = asRecord(item);
    const userRaw = asRecord(entry?.user) || entry;
    const rawAccess = Array.isArray(entry?.access) ? entry?.access : [];
    const access = rawAccess
      .map((value) => mapCoreAccess(value, protocolByInboundID))
      .filter((value): value is ClientAccess => value !== null);
    return mapClientFromCore(userRaw, access, index);
  });
  return { items, limited: false };
}

export async function getClientArtifacts(clientID: string): Promise<UserPayload> {
  const [userRaw, artifactsRaw, inbounds] = await Promise.all([
    apiFetch<unknown>(`/api/v1/users/${clientID}`, { method: "GET" }),
    apiFetch<unknown>(`/api/v1/users/${clientID}/artifacts`, { method: "GET" }),
    listCoreInbounds(),
  ]);
  const protocolByInboundID = mapProtocolByInbound(inbounds);
  const access = await listCoreUserAccess(clientID, protocolByInboundID);
  const client = mapClientFromCore(userRaw, access, 0);

  const artifacts = asRecord(artifactsRaw);
  const accessURL = resolveAccessURLByProtocol(artifacts || {}, client.preferred_protocol);
  const subscriptionURL = readString(artifacts?.subscription_import_url, readString(artifacts?.subscription_profile_url));
  const vlessURIs = readStringArray(artifacts?.vless_uris);
  const hy2URIs = readStringArray(artifacts?.hysteria2_uris);
  const allURIs = readStringArray(artifacts?.all_uris);

  const vlessQRUrl = vlessURIs.length > 0
    ? `/api/v1/users/${clientID}/artifacts/qr.png?value=${encodeURIComponent(vlessURIs[0])}&size=280`
    : "";
  const hy2QRUrl = hy2URIs.length > 0
    ? `/api/v1/users/${clientID}/artifacts/qr.png?value=${encodeURIComponent(hy2URIs[0])}&size=280`
    : "";

  return {
    user: client,
    artifacts: {
      access_url: accessURL,
      subscription_url: subscriptionURL,
      access_qr_url: `/api/v1/users/${clientID}/artifacts/qr.png?kind=access&size=320`,
      subscription_qr_url: `/api/v1/users/${clientID}/artifacts/qr.png?kind=subscription&size=320`,
      vless_qr_url: vlessQRUrl,
      hy2_qr_url: hy2QRUrl,
      profile_url: readString(artifacts?.subscription_profile_url),
      uris_url: readString(artifacts?.subscription_uris_url),
      all_uris: allURIs,
      vless_uris: vlessURIs,
      hy2_uris: hy2URIs,
    },
  };
}

export async function createClient(input: ClientCreateRequest): Promise<UserPayload> {
  const protocol = input.protocol || "hy2";
  const { inbound } = await ensureCoreContext(protocol);
  const createdRaw = await apiFetch<unknown>("/api/v1/users", {
    method: "POST",
    body: JSON.stringify({
      username: input.username,
      enabled: true,
      traffic_limit_bytes: input.traffic_limit_bytes || 0,
      expire_at: input.expire_at || undefined,
    }),
    timeoutMs: CLIENT_MUTATION_TIMEOUT_MS,
  });
  const created = asRecord(createdRaw);
  const userID = readString(created?.id);
  if (!userID) {
    throw new APIError("Failed to create user", 500, createdRaw, "create_user");
  }

  await upsertCoreAccess(accessPayloadForProtocol(userID, inbound.id, protocol, input));
  return getClientArtifacts(userID);
}

export async function updateClient(clientID: string, input: ClientCreateRequest): Promise<UserPayload> {
  const protocol = input.protocol || "hy2";
  const { inbound } = await ensureCoreContext(protocol);

  await apiFetch<unknown>(`/api/v1/users/${clientID}`, {
    method: "PATCH",
    body: JSON.stringify({
      username: input.username,
      traffic_limit_bytes: input.traffic_limit_bytes || 0,
      expire_at: input.expire_at || undefined,
    }),
    timeoutMs: CLIENT_MUTATION_TIMEOUT_MS,
  });

  await upsertCoreAccess(accessPayloadForProtocol(clientID, inbound.id, protocol, input));

  const inbounds = await listCoreInbounds();
  const protocolByInboundID = mapProtocolByInbound(inbounds);
  const currentAccess = await listCoreUserAccess(clientID, protocolByInboundID);
  for (const item of currentAccess) {
    if (item.inbound_id === inbound.id && item.protocol === protocol) {
      continue;
    }
    if (!item.enabled) {
      continue;
    }
    await upsertCoreAccess({
      id: item.id,
      user_id: clientID,
      inbound_id: item.inbound_id,
      enabled: false,
      vless_uuid: item.vless_uuid,
      hysteria2_password: item.hysteria2_password,
    });
  }

  return getClientArtifacts(clientID);
}

export async function deleteClient(clientID: string): Promise<{ ok: boolean }> {
  await apiFetch<{ ok: boolean }>(`/api/v1/users/${clientID}`, {
    method: "DELETE",
    body: JSON.stringify({}),
    timeoutMs: CLIENT_MUTATION_TIMEOUT_MS,
  });
  return { ok: true };
}

export async function setClientEnabled(clientID: string, enabled: boolean): Promise<{ ok: boolean; enabled: boolean }> {
  await apiFetch<unknown>(`/api/v1/users/${clientID}`, {
    method: "PATCH",
    body: JSON.stringify({ enabled }),
    timeoutMs: CLIENT_MUTATION_TIMEOUT_MS,
  });
  return { ok: true, enabled };
}

export async function setClientsEnabledBulk(clientIDs: string[], enabled: boolean): Promise<ClientStateBatchResponse> {
  const ids = normalizeIDs(clientIDs);
  let updated = 0;
  for (const id of ids) {
    await apiFetch<unknown>(`/api/v1/users/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ enabled }),
      timeoutMs: CLIENT_MUTATION_TIMEOUT_MS,
    });
    updated += 1;
  }
  return { ok: true, enabled, updated };
}

export async function deleteClientsBulk(clientIDs: string[]): Promise<ClientDeleteBatchResponse> {
  const ids = normalizeIDs(clientIDs);
  let deleted = 0;
  for (const id of ids) {
    await apiFetch<unknown>(`/api/v1/users/${id}`, {
      method: "DELETE",
      body: JSON.stringify({}),
      timeoutMs: CLIENT_MUTATION_TIMEOUT_MS,
    });
    deleted += 1;
  }
  return { ok: true, deleted };
}

export function qrURL(clientID: string, size = 360, kind: "access" | "subscription" = "access"): string {
  return `/api/v1/users/${clientID}/artifacts/qr.png?kind=${kind === "subscription" ? "subscription" : "access"}&size=${size}`;
}
