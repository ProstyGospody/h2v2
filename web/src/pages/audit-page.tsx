import { ChevronDown, ChevronUp, FilterX, RefreshCw, Shield } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import {
  Badge,
  Button,
  DateField,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  StateBlock,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableHeader,
  TableRow,
  cn,
} from "@/src/components/ui";
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

function ExpandablePayload({ json }: { json: string }) {
  const [expanded, setExpanded] = useState(false);
  const content = json || "{}";
  const isLong = content.length > 120;

  return (
    <div className="relative">
      <pre
        className={cn(
          "m-0 max-w-[340px] whitespace-pre-wrap break-words rounded-lg bg-surface-0/50 p-3 font-mono text-[13px] text-txt-secondary",
          !expanded && isLong && "max-h-[80px] overflow-hidden",
        )}
      >
        {content}
      </pre>
      {isLong && (
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="mt-1 inline-flex items-center gap-1 text-[11px] font-medium text-accent transition-colors hover:text-accent-light"
        >
          {expanded ? <><ChevronUp size={12} /> Collapse</> : <><ChevronDown size={12} /> Show more</>}
        </button>
      )}
    </div>
  );
}

export default function AuditPage() {
  const { items, loading, error, refresh, clearError, markSeen } = useAuditFeed();
  const [actionFilter, setActionFilter] = useState<"all" | "create" | "update" | "delete">("all");
  const [actorFilter, setActorFilter] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  const hasActiveFilters = actionFilter !== "all" || actorFilter.trim() !== "" || dateFrom !== "" || dateTo !== "";
  const clearFilters = useCallback(() => {
    setActionFilter("all");
    setActorFilter("");
    setDateFrom("");
    setDateTo("");
  }, []);

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
          <Button variant="primary" onClick={() => void refresh()} className="header-btn w-full rounded-2xl px-4 sm:w-auto">
            <RefreshCw size={18} strokeWidth={1.6} />
            Refresh
          </Button>
        }
      />

      <div className="flex items-center justify-between">
        <span className="text-[13px] text-txt-secondary">{filteredItems.length} records{hasActiveFilters ? ` (of ${items.length})` : ""}</span>
        {hasActiveFilters && (
          <button
            type="button"
            onClick={clearFilters}
            className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[12px] font-medium text-txt-secondary transition-colors hover:bg-surface-3/50 hover:text-txt-primary"
          >
            <FilterX size={14} strokeWidth={1.6} />
            Clear filters
          </button>
        )}
      </div>

      <div className="grid gap-3 md:grid-cols-4">
        <div>
          <label className="mb-2 block text-[13px] font-medium text-txt-secondary">Action</label>
          <Select value={actionFilter} onValueChange={(value) => setActionFilter(value as "all" | "create" | "update" | "delete")}>
            <SelectTrigger className="h-10 rounded-xl">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All</SelectItem>
              <SelectItem value="create">Create</SelectItem>
              <SelectItem value="update">Update</SelectItem>
              <SelectItem value="delete">Delete</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Input
          label="Actor"
          value={actorFilter}
          onChange={(event) => setActorFilter(event.target.value)}
          placeholder="Actor"
          className="h-10 rounded-xl"
        />
        <DateField label="From" value={dateFrom} onValueChange={setDateFrom} className="h-10 rounded-xl" />
        <DateField label="To" value={dateTo} onValueChange={setDateTo} className="h-10 rounded-xl" />
      </div>

      <ErrorBanner message={error} onDismiss={clearError} actionLabel="Retry" onAction={() => void refresh()} />

      <div className="space-y-3 sm:hidden">
        {loading ? (
          <StateBlock tone="loading" title="Loading audit log" minHeightClassName="min-h-[240px]" />
        ) : filteredItems.length ? (
          filteredItems.map((item) => (
            <div key={item.id} className="panel-card-compact space-y-2.5">
              <div className="flex items-start justify-between gap-3">
                <p className="text-[12px] text-txt-secondary">{formatDateTime(item.created_at)}</p>
                <Badge variant={actionVariant(item.action)}>{item.action}</Badge>
              </div>
              <div className="flex items-center justify-between gap-3 text-[13px]">
                <span className="truncate font-medium text-txt-primary">{item.admin_email || "system"}</span>
                <span className="truncate text-txt-secondary">
                  {item.entity_type}
                  {item.entity_id ? <span className="text-txt-muted">:{item.entity_id}</span> : null}
                </span>
              </div>
              <ExpandablePayload json={item.payload_json} />
            </div>
          ))
        ) : (
          <StateBlock tone="empty" title={items.length ? "No audit records match filters" : "No audit records"} icon={Shield} minHeightClassName="min-h-[200px]" />
        )}
      </div>

      <TableContainer className="hidden max-h-[72vh] overflow-auto sm:block">
        {loading ? (
          <StateBlock tone="loading" title="Loading audit log" minHeightClassName="min-h-[280px]" />
        ) : items.length === 0 ? (
          <StateBlock tone="empty" title="No audit records" icon={Shield} minHeightClassName="min-h-[240px]" />
        ) : (
          <Table aria-rowcount={filteredItems.length + 1}>
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
                  {filteredItems.map((item, index) => {
                    return (
                      <TableRow
                        key={item.id}
                        aria-rowindex={index + 2}
                        style={{ animationDelay: `${index * 0.02}s` }}
                        className="animate-[fadein_0.2s_ease_forwards] opacity-0"
                      >
                        <TableCell className="whitespace-nowrap text-[12px] text-txt-secondary sm:text-[14px]">{formatDateTime(item.created_at)}</TableCell>
                        <TableCell><span className="font-medium">{item.admin_email || "system"}</span></TableCell>
                        <TableCell><Badge variant={actionVariant(item.action)}>{item.action}</Badge></TableCell>
                        <TableCell><span className="text-txt-secondary">{item.entity_type}{item.entity_id ? <span className="text-txt-muted">:{item.entity_id}</span> : ""}</span></TableCell>
                        <TableCell className="hidden lg:table-cell">
                          <ExpandablePayload json={item.payload_json} />
                        </TableCell>
                      </TableRow>
                    );
                  })}
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
