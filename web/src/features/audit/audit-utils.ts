import { type AuditLogItem } from "@/types/common";

export type AuditActionFilter = "all" | "create" | "update" | "delete";

export function actionVariant(action: string | null | undefined): "default" | "success" | "warning" | "danger" {
  const normalized = (action || "").toLowerCase();
  if (normalized.includes("delete") || normalized.includes("remove")) return "danger";
  if (normalized.includes("create") || normalized.includes("add")) return "success";
  if (normalized.includes("update") || normalized.includes("change") || normalized.includes("edit")) return "warning";
  return "default";
}

export function actionKind(action: string | null | undefined): "create" | "update" | "delete" | "other" {
  const normalized = (action || "").toLowerCase();
  if (normalized.includes("delete") || normalized.includes("remove")) return "delete";
  if (normalized.includes("create") || normalized.includes("add")) return "create";
  if (normalized.includes("update") || normalized.includes("change") || normalized.includes("edit")) return "update";
  return "other";
}

export function entityLabel(item: AuditLogItem): string {
  return item.entity_id ? `${item.entity_type}:${item.entity_id}` : item.entity_type;
}

export function rowSearchText(item: AuditLogItem): string {
  return `${item.id} ${item.action} ${item.admin_email || "system"} ${entityLabel(item)} ${item.payload_json}`.toLowerCase();
}
