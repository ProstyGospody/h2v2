import { useVirtualizer } from "@tanstack/react-virtual";
import { RefreshCw, Shield } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import { Badge, Button, Input, StateBlock, Table, TableBody, TableCell, TableContainer, TableHead, TableHeader, TableRow } from "@/src/components/ui";
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
  const tableScrollRef = useRef<HTMLDivElement | null>(null);
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
  const rowVirtualizer = useVirtualizer({
    count: filteredItems.length,
    getScrollElement: () => tableScrollRef.current,
    estimateSize: () => 58,
    overscan: 12,
  });
  const virtualRows = rowVirtualizer.getVirtualItems();
  const virtualTopPadding = virtualRows.length > 0 ? virtualRows[0].start : 0;
  const virtualBottomPadding =
    virtualRows.length > 0
      ? rowVirtualizer.getTotalSize() - virtualRows[virtualRows.length - 1].end
      : 0;

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

      <TableContainer ref={tableScrollRef} className="max-h-[72vh] overflow-auto">
        {loading ? (
          <StateBlock tone="loading" title="Loading audit log" minHeightClassName="min-h-[280px]" />
        ) : items.length === 0 ? (
          <StateBlock tone="empty" title="No audit records" icon={Shield} minHeightClassName="min-h-[240px]" />
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
              {filteredItems.length ? (
                <>
                  {virtualTopPadding > 0 ? (
                    <TableRow className="border-0 hover:bg-transparent">
                      <TableCell colSpan={5} style={{ height: virtualTopPadding, padding: 0 }} />
                    </TableRow>
                  ) : null}
                  {virtualRows.map((virtualRow) => {
                    const item = filteredItems[virtualRow.index];
                    if (!item) return null;
                    return (
                      <TableRow key={item.id} style={{ animationDelay: `${virtualRow.index * 0.02}s` }} className="animate-[fadein_0.2s_ease_forwards] opacity-0">
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
                    );
                  })}
                  {virtualBottomPadding > 0 ? (
                    <TableRow className="border-0 hover:bg-transparent">
                      <TableCell colSpan={5} style={{ height: virtualBottomPadding, padding: 0 }} />
                    </TableRow>
                  ) : null}
                </>
              ) : (
                <TableRow>
                  <TableCell colSpan={5}>No audit records match filters.</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        )}
      </TableContainer>
    </div>
  );
}
