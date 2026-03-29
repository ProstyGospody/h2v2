import { RefreshCw, Shield } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { PageHeader } from "@/components/ui/page-header";
import { APIError, apiFetch } from "@/services/api";
import { Button, Badge, Table, TableBody, TableCell, TableContainer, TableHead, TableHeader, TableRow } from "@/src/components/ui";
import { AuditLogItem } from "@/types/common";
import { formatDateTime } from "@/utils/format";

function actionVariant(action: string): "default" | "success" | "warning" | "danger" {
  const l = action.toLowerCase();
  if (l.includes("delete") || l.includes("remove")) return "danger";
  if (l.includes("create") || l.includes("add")) return "success";
  if (l.includes("update") || l.includes("change") || l.includes("edit")) return "warning";
  return "default";
}

export default function AuditPage() {
  const [items, setItems] = useState<AuditLogItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setError("");
    try {
      const p = await apiFetch<{ items: AuditLogItem[] }>("/api/audit?limit=250", { method: "GET" });
      setItems(p.items || []);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to load audit log");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Audit Log"
        actions={
          <Button variant="primary" onClick={() => void load()} className="w-full sm:w-auto">
            <RefreshCw size={18} strokeWidth={1.6} />
            Refresh
          </Button>
        }
      />

      {error && <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{error}</div>}

      <TableContainer className="overflow-x-auto">
        {loading ? (
          <div className="flex min-h-[280px] items-center justify-center">
            <div className="flex flex-col items-center gap-3">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent/20 border-t-accent-light" />
              <p className="text-[14px] text-txt-secondary">Loading audit records...</p>
            </div>
          </div>
        ) : items.length === 0 ? (
          <div className="flex min-h-[240px] flex-col items-center justify-center gap-4 py-10">
            <div className="grid h-14 w-14 place-items-center rounded-full bg-surface-3/50">
              <Shield size={24} strokeWidth={1.6} className="text-txt-muted" />
            </div>
            <p className="text-[15px] text-txt-secondary">No audit records yet</p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow className="border-t-0 hover:bg-transparent">
                <TableHead>Timestamp</TableHead>
                <TableHead>Actor</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Entity</TableHead>
                <TableHead className="hidden lg:table-cell">Payload</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item, i) => (
                <TableRow key={item.id} style={{ animationDelay: `${i * 0.03}s` }} className="animate-[fadein_0.2s_ease_forwards] opacity-0">
                  <TableCell className="whitespace-nowrap text-[12px] text-txt-secondary sm:text-[14px]">{formatDateTime(item.created_at)}</TableCell>
                  <TableCell><span className="font-medium">{item.admin_email || "system"}</span></TableCell>
                  <TableCell><Badge variant={actionVariant(item.action)}>{item.action}</Badge></TableCell>
                  <TableCell><span className="text-txt-secondary">{item.entity_type}{item.entity_id ? <span className="text-txt-muted">:{item.entity_id}</span> : ""}</span></TableCell>
                  <TableCell className="hidden lg:table-cell">
                    <pre className="m-0 max-w-[340px] truncate whitespace-pre-wrap break-words rounded-lg border border-border/50 bg-surface-0/50 p-3 font-mono text-[13px] text-txt-secondary">
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
