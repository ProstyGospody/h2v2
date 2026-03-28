import { Loader2, RefreshCw, Shield } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { PageHeader } from "@/components/ui/page-header";
import { APIError, apiFetch } from "@/services/api";
import { Button, Badge, Table, TableBody, TableCell, TableContainer, TableHead, TableHeader, TableRow } from "@/src/components/ui";
import { AuditLogItem } from "@/types/common";
import { formatDateTime } from "@/utils/format";

function actionVariant(action: string): "default" | "success" | "warning" | "danger" {
  const lower = action.toLowerCase();
  if (lower.includes("delete") || lower.includes("remove")) return "danger";
  if (lower.includes("create") || lower.includes("add")) return "success";
  if (lower.includes("update") || lower.includes("change") || lower.includes("edit")) return "warning";
  return "default";
}

export default function AuditPage() {
  const [items, setItems] = useState<AuditLogItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setError("");
    try {
      const payload = await apiFetch<{ items: AuditLogItem[] }>("/api/audit?limit=250", { method: "GET" });
      setItems(payload.items || []);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to load audit log");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-4">
      <PageHeader
        title="Audit"
        actions={
          <Button variant="primary" onClick={() => void load()}>
            <RefreshCw size={16} strokeWidth={1.4} />
            Refresh
          </Button>
        }
      />

      {error ? <div className="rounded-[10px] border border-status-danger/20 bg-status-danger/8 px-4 py-3 text-[12px] text-status-danger">{error}</div> : null}

      <TableContainer>
        {loading ? (
          <div className="flex min-h-[240px] items-center justify-center">
            <div className="flex flex-col items-center gap-2">
              <Loader2 size={20} strokeWidth={1.4} className="animate-spin text-accent-light" />
              <p className="text-[12px] text-txt-secondary">Loading audit records...</p>
            </div>
          </div>
        ) : items.length === 0 ? (
          <div className="flex min-h-[200px] flex-col items-center justify-center gap-3 py-8">
            <div className="grid h-12 w-12 place-items-center rounded-full bg-surface-3/60">
              <Shield size={20} strokeWidth={1.4} className="text-txt-muted" />
            </div>
            <p className="text-[13px] text-txt-secondary">No audit records yet</p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow className="border-t-0 hover:bg-transparent">
                <TableHead>Timestamp</TableHead>
                <TableHead>Actor</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Entity</TableHead>
                <TableHead>Payload</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item, index) => (
                <TableRow
                  key={item.id}
                  style={{ animationDelay: `${index * 0.03}s` }}
                  className="animate-[fadein_0.2s_ease_forwards] opacity-0"
                >
                  <TableCell className="whitespace-nowrap text-txt-secondary">{formatDateTime(item.created_at)}</TableCell>
                  <TableCell>
                    <span className="font-medium">{item.admin_email || "system"}</span>
                  </TableCell>
                  <TableCell>
                    <Badge variant={actionVariant(item.action)}>{item.action}</Badge>
                  </TableCell>
                  <TableCell>
                    <span className="text-txt-secondary">
                      {item.entity_type}
                      {item.entity_id ? <span className="text-txt-muted">:{item.entity_id}</span> : ""}
                    </span>
                  </TableCell>
                  <TableCell>
                    <pre className="m-0 max-w-[320px] truncate whitespace-pre-wrap break-words rounded-[8px] border border-border/60 bg-surface-0/50 p-2 font-mono text-[11px] text-txt-secondary">
                      {item.payload_json || "{}"}
                    </pre>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </TableContainer>
    </div>
  );
}
