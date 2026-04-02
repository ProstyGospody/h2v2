import {
  HysteriaClient,
  HysteriaClientCreateRequest,
  HysteriaClientDefaults,
  HysteriaClientUpdateRequest,
  Protocol,
} from "@/domain/clients/types";

type ClientFormValues = {
  username: string;
  note: string;
  authSecret: string;
  protocol: Protocol;
  uuid: string;
  trafficLimitBytes: string;
  expireAt: string;
};

export function defaultsSummary(defaults: HysteriaClientDefaults | null): string {
  if (!defaults) {
    return "Defaults loading";
  }
  const params = defaults.client_params || ({} as HysteriaClientDefaults["client_params"]);
  const options = defaults.server_options || ({} as HysteriaClientDefaults["server_options"]);
  const parts = [
    params.server ? `${params.server}:${params.port || 443}` : "",
    params.sni ? `SNI ${params.sni}` : "",
    options.obfs_type ? `OBFS ${options.obfs_type}` : "",
  ].filter(Boolean);
  return parts.join(" | ");
}

export function formFromClient(client: HysteriaClient | null): ClientFormValues {
  return {
    username: client?.username || "",
    note: client?.note || "",
    authSecret: client?.preferred_protocol === "hy2" ? client.password || "" : "",
    protocol: client?.preferred_protocol || "hy2",
    uuid: client?.preferred_protocol === "vless" ? client.password || "" : "",
    trafficLimitBytes: client?.traffic_limit_bytes ? String(client.traffic_limit_bytes) : "",
    expireAt: client?.expire_at ? client.expire_at.slice(0, 16) : "",
  };
}

function toNumberOrUndefined(value: string): number | undefined {
  const trimmed = value.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return undefined;
  }
  return Math.floor(parsed);
}

function toDateOrNull(value: string): string | null | undefined {
  const trimmed = value.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = new Date(trimmed);
  if (Number.isNaN(parsed.getTime())) {
    return undefined;
  }
  return parsed.toISOString();
}

export function toCreateRequest(values: ClientFormValues): HysteriaClientCreateRequest {
  const payload: HysteriaClientCreateRequest = {
    username: values.username,
    protocol: values.protocol,
  };
  if (values.note.trim()) {
    payload.note = values.note.trim();
  }
  if (values.protocol === "hy2" && values.authSecret.trim()) {
    payload.auth_secret = values.authSecret.trim();
  }
  if (values.protocol === "vless" && values.uuid.trim()) {
    payload.uuid = values.uuid.trim();
  }
  const limit = toNumberOrUndefined(values.trafficLimitBytes);
  if (limit !== undefined) {
    payload.traffic_limit_bytes = limit;
  }
  const expire = toDateOrNull(values.expireAt);
  if (expire !== undefined) {
    payload.expire_at = expire;
  }
  return payload;
}

export function toUpdateRequest(values: ClientFormValues): HysteriaClientUpdateRequest {
  return toCreateRequest(values);
}

export type { ClientFormValues };
