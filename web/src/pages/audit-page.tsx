import { Loader2, RefreshCw } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { PageHeader } from "@/components/ui/page-header";
import { APIError, apiFetch } from "@/services/api";
import { Button, Table, TableBody, TableCell, TableContainer, TableHead, TableHeader, TableRow } from "@/src/components/ui";
import { AuditLogItem } from "@/types/common";
import { formatDateTime } from "@/utils/format";

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

      {error ? <div className="rounded-btn border border-status-danger/20 bg-status-danger/10 px-3 py-2 text-[12px] text-status-danger">{error}</div> : null}

      <TableContainer>
        {loading ? (
          <div className="flex min-h-[240px] items-center justify-center">
            <div className="flex flex-col items-center gap-2">
              <Loader2 size={20} strokeWidth={1.4} className="animate-spin text-accent-light" />
              <p className="text-[12px] text-txt-secondary">Loading audit records...</p>
            </div>
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
                  <TableCell>{formatDateTime(item.created_at)}</TableCell>
                  <TableCell>{item.admin_email || "system"}</TableCell>
                  <TableCell>{item.action}</TableCell>
                  <TableCell>
                    {item.entity_type}
                    {item.entity_id ? `:${item.entity_id}` : ""}
                  </TableCell>
                  <TableCell>
                    <pre className="m-0 whitespace-pre-wrap break-words rounded-btn border border-border bg-surface-0 p-2 font-mono text-[11px] text-txt-secondary">
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
