import type { Client, ClientFormValues, Protocol } from "./types";

export function formDefaults(): ClientFormValues {
  return { username: "", traffic_limit_gb: "", expire_at: "" };
}

export function formFromClient(client: Client): ClientFormValues {
  const limitGB =
    client.traffic_limit_bytes > 0
      ? String(+(client.traffic_limit_bytes / 1_073_741_824).toFixed(2))
      : "";
  return {
    username: client.username,
    traffic_limit_gb: limitGB,
    expire_at: client.expire_at ? client.expire_at.slice(0, 16) : "",
  };
}

const MIN_TRAFFIC_LIMIT_BYTES = 1_048_576; // 1 MB minimum to prevent accidental "unlimited" from rounding

export function trafficLimitToBytes(gb: string): number {
  const n = parseFloat(gb.trim());
  if (!Number.isFinite(n) || n <= 0) return 0;
  const bytes = Math.round(n * 1_073_741_824);
  // If a positive value was entered but rounds below the minimum, clamp to minimum.
  return bytes < MIN_TRAFFIC_LIMIT_BYTES ? MIN_TRAFFIC_LIMIT_BYTES : bytes;
}

export function expireToISO(value: string): string | null {
  const trimmed = value.trim();
  if (!trimmed) return null;
  const d = new Date(trimmed);
  if (Number.isNaN(d.getTime())) return null;
  // Reject dates in the past to prevent accidental immediate expiry.
  if (d.getTime() <= Date.now()) return null;
  return d.toISOString();
}

export function uniqueProtocols(access: { protocol: Protocol }[]): Protocol[] {
  return Array.from(new Set(access.map((a) => a.protocol)));
}
