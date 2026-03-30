import { APIError, apiFetch } from "@/services/api";

import { HysteriaSettingsResponse, HysteriaSettingsSaveResponse, Hy2Settings } from "@/domain/settings/types";

type SQLiteEntityCounts = {
  admins: number;
  sessions: number;
  hysteria_users: number;
  hysteria_snapshots: number;
  system_snapshots: number;
  audit_logs: number;
  service_states: number;
};

type SQLiteRestoreResponse = {
  ok: boolean;
  counts: SQLiteEntityCounts;
};

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

function parseFilename(contentDisposition: string | null): string {
  if (!contentDisposition) {
    return "panel-backup.db";
  }
  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]);
    } catch {
      return utf8Match[1];
    }
  }
  const plainMatch = contentDisposition.match(/filename=\"?([^\";]+)\"?/i);
  if (plainMatch?.[1]) {
    return plainMatch[1];
  }
  return "panel-backup.db";
}

export async function downloadSQLiteBackup(): Promise<string> {
  const response = await fetch("/api/storage/sqlite/backup", {
    method: "GET",
    credentials: "include",
    cache: "no-store",
  });

  if (!response.ok) {
    const raw = await response.text();
    let payload: unknown = null;
    if (raw) {
      try {
        payload = JSON.parse(raw);
      } catch {
        payload = raw;
      }
    }
    const message =
      typeof payload === "object" && payload !== null && "error" in payload
        ? String((payload as Record<string, unknown>).error || "Request failed")
        : `${response.status} ${response.statusText}`;
    const details = typeof payload === "object" && payload !== null && "details" in payload ? (payload as Record<string, unknown>).details : null;
    throw new APIError(message, response.status, details);
  }

  const blob = await response.blob();
  const fileName = parseFilename(response.headers.get("Content-Disposition"));
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  setTimeout(() => URL.revokeObjectURL(url), 1000);
  return fileName;
}

export function restoreSQLiteBackup(file: File): Promise<SQLiteRestoreResponse> {
  const formData = new FormData();
  formData.append("file", file);
  return apiFetch<SQLiteRestoreResponse>("/api/storage/sqlite/restore", {
    method: "POST",
    body: formData,
  });
}
