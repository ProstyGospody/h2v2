import { type Client } from "@/domain/clients/types";

export type ClientFilter = "all" | "online" | "enabled" | "disabled";
export type SortField = "username" | "traffic" | "last_seen";
export type SortDir = "asc" | "desc";
export type SortState = { field: SortField; dir: SortDir };

export function asText(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

export function initials(value: string): string {
  const clean = asText(value).trim();
  return clean ? clean.slice(0, 1).toUpperCase() : "?";
}

export function resolveStatus(client: Client): "online" | "offline" | "disabled" {
  if (!client.enabled) {
    return "disabled";
  }
  if (client.online_count > 0) {
    return "online";
  }
  return "offline";
}

export function sortAria(field: SortField, sort: SortState | null): "none" | "ascending" | "descending" {
  if (!sort || sort.field !== field) return "none";
  return sort.dir === "asc" ? "ascending" : "descending";
}

export function escapeCSV(value: string): string {
  const escaped = value.replace(/"/g, '""');
  if (/^[=+\-@\t\r]/.test(escaped)) {
    return `"'${escaped}"`;
  }
  return `"${escaped}"`;
}

export function selectedDeleteDescription(selectedIDs: string[], clients: Client[]): string {
  const names = clients
    .filter((client) => selectedIDs.includes(client.id))
    .map((client) => client.username);
  if (names.length === 0) return `Delete ${selectedIDs.length} users?`;
  if (names.length <= 3) return `Delete ${names.join(", ")}?`;
  return `Delete ${names.slice(0, 3).join(", ")} and ${names.length - 3} more?`;
}
