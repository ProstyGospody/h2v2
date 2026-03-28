import { apiFetch } from "@/services/api";

import { HysteriaSettingsResponse, HysteriaSettingsSaveResponse, Hy2Settings } from "@/domain/settings/types";

export function getHysteriaSettings(): Promise<HysteriaSettingsResponse> {
  return apiFetch<HysteriaSettingsResponse>("/api/hysteria/settings", { method: "GET" });
}

export function validateHysteriaSettings(settings: Hy2Settings): Promise<HysteriaSettingsResponse> {
  return apiFetch<HysteriaSettingsResponse>("/api/hysteria/settings/validate", {
    method: "POST",
    body: JSON.stringify(settings),
  });
}

export function saveHysteriaSettings(settings: Hy2Settings): Promise<HysteriaSettingsSaveResponse> {
  return apiFetch<HysteriaSettingsSaveResponse>("/api/hysteria/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export function applyHysteriaSettings(): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>("/api/hysteria/settings/apply", {
    method: "POST",
    body: JSON.stringify({}),
  });
}
