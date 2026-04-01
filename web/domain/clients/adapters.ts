import {
  HysteriaClient,
  HysteriaClientCreateRequest,
  HysteriaClientDefaults,
  HysteriaClientUpdateRequest,
} from "@/domain/clients/types";

type ClientFormValues = {
  username: string;
  note: string;
  authSecret: string;
};

export function defaultsSummary(defaults: HysteriaClientDefaults | null): string {
  if (!defaults) {
    return "Server defaults are loading";
  }
  const params = defaults.client_params || ({} as HysteriaClientDefaults["client_params"]);
  const options = defaults.server_options || ({} as HysteriaClientDefaults["server_options"]);
  const parts = [
    params.server ? `${params.server}:${params.port || 443}` : "",
    params.sni ? `SNI ${params.sni}` : "",
    params.insecure ? "TLS insecure" : "",
    options.obfs_type ? `OBFS ${options.obfs_type}` : "",
    options.masquerade_type ? `Masquerade ${options.masquerade_type}` : "",
  ].filter(Boolean);
  return parts.join(" | ");
}

export function formFromClient(client: HysteriaClient | null): ClientFormValues {
  return {
    username: client?.username || "",
    note: client?.note || "",
    authSecret: "",
  };
}

export function toCreateRequest(values: ClientFormValues): HysteriaClientCreateRequest {
  const payload: HysteriaClientCreateRequest = {
    username: values.username,
  };
  if (values.note.trim()) {
    payload.note = values.note.trim();
  }
  if (values.authSecret.trim()) {
    payload.auth_secret = values.authSecret.trim();
  }
  return payload;
}

export function toUpdateRequest(values: ClientFormValues): HysteriaClientUpdateRequest {
  const payload: HysteriaClientUpdateRequest = {
    username: values.username,
  };
  if (values.note.trim()) {
    payload.note = values.note.trim();
  }
  if (values.authSecret.trim()) {
    payload.auth_secret = values.authSecret.trim();
  }
  return payload;
}

export type { ClientFormValues };
