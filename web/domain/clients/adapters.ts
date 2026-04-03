import {
  Client,
  ClientCreateRequest,
  ClientUpdateRequest,
  Protocol,
} from "@/domain/clients/types";

type ClientFormValues = {
  username: string;
  authSecret: string;
  protocol: Protocol;
  uuid: string;
  trafficLimitBytes: string;
  expireAt: string;
};

export function formFromClient(client: Client | null): ClientFormValues {
  return {
    username: client?.username || "",
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

export function toCreateRequest(values: ClientFormValues): ClientCreateRequest {
  const payload: ClientCreateRequest = {
    username: values.username,
    protocol: values.protocol,
  };
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

export function toUpdateRequest(values: ClientFormValues): ClientUpdateRequest {
  return toCreateRequest(values);
}

export type { ClientFormValues };
