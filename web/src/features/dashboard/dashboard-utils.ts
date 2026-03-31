import { type HistoryWindow } from "./dashboard-types";

export function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, value));
}

export function formatTooltipDate(value: unknown): string {
  const date = value instanceof Date ? value : new Date(String(value));
  if (Number.isNaN(date.getTime())) return "--";
  return `${date.toLocaleDateString([], { day: "2-digit", month: "short", year: "numeric" })} ${date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
}

export function formatTrafficTick(value: Date): string {
  if (!(value instanceof Date) || Number.isNaN(value.getTime())) return "--:--";
  return value.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function formatTrafficTooltipLabel(value: unknown, window: HistoryWindow): string {
  const date = value instanceof Date ? value : new Date(String(value));
  if (Number.isNaN(date.getTime())) return "--";
  if (window === "24h") {
    return `${date.toLocaleDateString([], { day: "2-digit", month: "short" })} ${date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
  }
  return formatTooltipDate(date);
}

export function gaugeColor(percent: number): string {
  if (percent >= 85) return "var(--status-danger)";
  if (percent >= 60) return "var(--status-warning)";
  return "var(--status-success)";
}

export function serviceStatusColor(status: string): string {
  const normalized = status.toLowerCase();
  if (normalized.includes("running") || normalized.includes("active")) {
    return "bg-status-success shadow-[0_0_8px_var(--status-success-soft)]";
  }
  if (normalized.includes("failed") || normalized.includes("error")) {
    return "bg-status-danger";
  }
  if (normalized.includes("inactive") || normalized.includes("stopped")) {
    return "bg-status-warning";
  }
  return "bg-txt-muted";
}
