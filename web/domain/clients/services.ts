import { apiFetch } from "@/services/api";

import {
  HysteriaClient,
  HysteriaClientCreateRequest,
  HysteriaClientDefaults,
  HysteriaClientListResponse,
  HysteriaClientUpdateRequest,
  HysteriaUserPayload,
} from "@/domain/clients/types";

const CLIENT_FETCH_LIMIT = 500;

export async function listClients(): Promise<{ items: HysteriaClient[]; limited: boolean }> {
  const payload = await apiFetch<HysteriaClientListResponse>(`/api/hysteria/users?limit=${CLIENT_FETCH_LIMIT}`, { method: "GET" });
  const items = payload.items || [];
  return { items, limited: items.length >= CLIENT_FETCH_LIMIT };
}

export function getClientDefaults(): Promise<HysteriaClientDefaults> {
  return apiFetch<HysteriaClientDefaults>("/api/hysteria/client-defaults", { method: "GET" });
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
