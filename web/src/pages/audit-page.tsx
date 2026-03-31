import { RefreshCw, Shield } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import { Button, Input, Badge, Table, TableBody, TableCell, TableContainer, TableHead, TableHeader, TableRow } from "@/src/components/ui";
import { useAuditFeed } from "@/src/state/audit-feed";
import { formatDateTime } from "@/utils/format";

function actionVariant(action: string): "default" | "success" | "warning" | "danger" {
  const l = action.toLowerCase();
  if (l.includes("delete") || l.includes("remove")) return "danger";
  if (l.includes("create") || l.includes("add")) return "success";
  if (l.includes("update") || l.includes("change") || l.includes("edit")) return "warning";
  return "default";
}

function actionKind(action: string): "create" | "update" | "delete" | "other" {
  const l = action.toLowerCase();
  if (l.includes("delete") || l.includes("remove")) return "delete";
  if (l.includes("create") || l.includes("add")) return "create";
  if (l.includes("update") || l.includes("change") || l.includes("edit")) return "update";
  return "other";
}

export default function AuditPage() {
  const { items, loading, error, refresh, clearError, markSeen } = useAuditFeed();
  const [actionFilter, setActionFilter] = useState<"all" | "create" | "update" | "delete">("all");
  const [actorFilter, setActorFilter] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  useEffect(() => {
    markSeen();
  }, [markSeen, items]);

  const filteredItems = useMemo(() => {
    const actorNeedle = actorFilter.trim().toLowerCase();
    const fromMs = dateFrom ? new Date(`${dateFrom}T00:00:00`).getTime() : Number.NEGATIVE_INFINITY;
    const toMs = dateTo ? new Date(`${dateTo}T23:59:59.999`).getTime() : Number.POSITIVE_INFINITY;

    return items.filter((item) => {
      const kind = actionKind(item.action);
      if (actionFilter !== "all" && kind !== actionFilter) return false;

      const actor = (item.admin_email || "system").toLowerCase();
      if (actorNeedle && !actor.includes(actorNeedle)) return false;

      const itemMs = new Date(item.created_at).getTime();
      if (!Number.isFinite(itemMs)) return false;
      if (itemMs < fromMs || itemMs > toMs) return false;
      return true;
    });
  }, [actionFilter, actorFilter, dateFrom, dateTo, items]);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Audit Log"
        actions={
          <Button variant="primary" onClick={() => void refresh()} className="h-11 w-full rounded-2xl px-4 sm:w-auto">
            <RefreshCw size={18} strokeWidth={1.6} />
            Refresh
          </Button>
        }
      />

      <div className="grid gap-3 md:grid-cols-4">
        <div>
          <label className="mb-2 block text-[13px] font-medium text-txt-secondary">Action</label>
          <select
            value={actionFilter}
            onChange={(event) => setActionFilter(event.target.value as "all" | "create" | "update" | "delete")}
            className="h-10 w-full rounded-xl bg-[var(--control-bg)] px-3 text-[14px] text-txt-primary outline-none focus:shadow-[0_0_0_3px_var(--accent-soft)]"
          >
            <option value="all">All</option>
            <option value="create">Create</option>
            <option value="update">Update</option>
            <option value="delete">Delete</option>
          </select>
        </div>
        <Input
          label="Actor"
          value={actorFilter}
          onChange={(event) => setActorFilter(event.target.value)}
          placeholder="Actor"
          className="h-10 rounded-xl"
        />
        <Input
          label="From"
          type="date"
          value={dateFrom}
          onChange={(event) => setDateFrom(event.target.value)}
          className="h-10 rounded-xl"
        />
        <Input
          label="To"
          type="date"
          value={dateTo}
          onChange={(event) => setDateTo(event.target.value)}
          className="h-10 rounded-xl"
        />
      </div>

      <ErrorBanner message={error} onDismiss={clearError} actionLabel="Retry" onAction={() => void refresh()} />

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
              {filteredItems.map((item, i) => (
                <TableRow key={item.id} style={{ animationDelay: `${i * 0.03}s` }} className="animate-[fadein_0.2s_ease_forwards] opacity-0">
                  <TableCell className="whitespace-nowrap text-[12px] text-txt-secondary sm:text-[14px]">{formatDateTime(item.created_at)}</TableCell>
                  <TableCell><span className="font-medium">{item.admin_email || "system"}</span></TableCell>
                  <TableCell><Badge variant={actionVariant(item.action)}>{item.action}</Badge></TableCell>
                  <TableCell><span className="text-txt-secondary">{item.entity_type}{item.entity_id ? <span className="text-txt-muted">:{item.entity_id}</span> : ""}</span></TableCell>
                  <TableCell className="hidden lg:table-cell">
                    <pre className="m-0 max-w-[340px] truncate whitespace-pre-wrap break-words rounded-lg bg-surface-0/50 p-3 font-mono text-[13px] text-txt-secondary">
                      {item.payload_json || "{}"}
                    </pre>
                  </TableCell>
                </TableRow>
              ))}
              {!filteredItems.length ? (
                <TableRow>
                  <TableCell colSpan={5}>No audit records match filters.</TableCell>
                </TableRow>
              ) : null}
            </TableBody>
          </Table>
        )}
      </TableContainer>
    </div>
  );
}
