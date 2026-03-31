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
  cn,
} from "@/src/components/ui";
import { useAuditFeed } from "@/src/state/audit-feed";
import { formatDateTime } from "@/utils/format";

function actionVariant(action: string | null | undefined): "default" | "success" | "warning" | "danger" {
  const normalized = (action || "").toLowerCase();
  if (normalized.includes("delete") || normalized.includes("remove")) return "danger";
  if (normalized.includes("create") || normalized.includes("add")) return "success";
  if (normalized.includes("update") || normalized.includes("change") || normalized.includes("edit")) return "warning";
  return "default";
}

function actionKind(action: string | null | undefined): "create" | "update" | "delete" | "other" {
  const normalized = (action || "").toLowerCase();
  if (normalized.includes("delete") || normalized.includes("remove")) return "delete";
  if (normalized.includes("create") || normalized.includes("add")) return "create";
  if (normalized.includes("update") || normalized.includes("change") || normalized.includes("edit")) return "update";
  return "other";
}

function dayLabel(timestamp: string): string {
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return "Unknown";
  return date.toLocaleDateString([], { day: "2-digit", month: "short", year: "numeric" });
}

function ExpandablePayload({ payload }: { payload: string }) {
  const [expanded, setExpanded] = useState(false);
  const content = payload || "{}";
  const isLong = content.length > 180;

  return (
    <div className="space-y-1.5">
      <pre
        className={cn(
          "m-0 whitespace-pre-wrap break-words rounded-lg bg-surface-0/45 p-3 font-mono text-[12px] leading-5 text-txt-secondary",
          !expanded && isLong && "max-h-[84px] overflow-hidden",
        )}
      >
        {content}
      </pre>
      {isLong && (
        <button
          type="button"
          onClick={() => setExpanded((value) => !value)}
          className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-medium text-txt-secondary transition-colors hover:bg-surface-3/50 hover:text-txt"
        >
          {expanded ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
          {expanded ? "Collapse" : "Expand"}
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

  useEffect(() => {
    markSeen();
  }, [markSeen]);

  const hasActiveFilters = actionFilter !== "all" || actorFilter.trim() !== "" || dateFrom !== "" || dateTo !== "";

  const clearFilters = useCallback(() => {
    setActionFilter("all");
    setActorFilter("");
    setDateFrom("");
    setDateTo("");
  }, []);

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

  const groupedItems = useMemo(() => {
    const groups = new Map<string, typeof filteredItems>();
    filteredItems.forEach((item) => {
      const key = dayLabel(item.created_at);
      const current = groups.get(key);
      if (current) {
        current.push(item);
      } else {
        groups.set(key, [item]);
      }
    });
    return Array.from(groups.entries());
  }, [filteredItems]);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Audit Log"
        actions={
          <Button variant="primary" onClick={() => void refresh()} className="header-btn w-full rounded-2xl px-4 sm:w-auto">
            <RefreshCw size={17} strokeWidth={1.7} />
            Refresh
          </Button>
        }
      />

      <div className="panel-card space-y-3">
        <div className="grid gap-3 md:grid-cols-4">
          <div>
            <label className="mb-2 block text-[12px] font-semibold uppercase tracking-wide text-txt-muted">Action</label>
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

        <div className="flex items-center justify-between gap-3 text-[12px] text-txt-secondary">
          <span>{filteredItems.length} records{hasActiveFilters ? ` / ${items.length}` : ""}</span>
          {hasActiveFilters && (
            <button
              type="button"
              onClick={clearFilters}
              className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 transition-colors hover:bg-surface-3/50 hover:text-txt"
            >
              <FilterX size={14} strokeWidth={1.7} />
              Clear
            </button>
          )}
        </div>
      </div>

      <ErrorBanner message={error} onDismiss={clearError} actionLabel="Retry" onAction={() => void refresh()} />

      {loading ? (
        <StateBlock tone="loading" title="Loading audit" minHeightClassName="min-h-[280px]" />
      ) : filteredItems.length === 0 ? (
        <StateBlock
          tone="empty"
          title={items.length ? "No records" : "No audit records"}
          icon={Shield}
          minHeightClassName="min-h-[260px]"
        />
      ) : (
        <div className="space-y-4">
          {groupedItems.map(([date, events]) => (
            <section key={date} className="space-y-2.5">
              <div className="inline-flex rounded-lg bg-surface-3/35 px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wide text-txt-muted">
                {date}
              </div>

              <div className="space-y-2.5">
                {events.map((item) => (
                  <article key={item.id} className="card-hover panel-card-compact">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="inline-flex items-center gap-2">
                        <Badge variant={actionVariant(item.action)}>{item.action}</Badge>
                        <span className="text-[12px] text-txt-secondary">{formatDateTime(item.created_at)}</span>
                      </div>
                      <span className="text-[12px] text-txt-muted">#{item.id}</span>
                    </div>

                    <div className="mt-2 flex flex-wrap items-center gap-2 text-[13px]">
                      <span className="rounded-lg bg-surface-3/35 px-2.5 py-1 text-txt-secondary">{item.admin_email || "system"}</span>
                      <span className="rounded-lg bg-surface-3/35 px-2.5 py-1 text-txt-secondary">
                        {item.entity_type}{item.entity_id ? `:${item.entity_id}` : ""}
                      </span>
                    </div>

                    <div className="mt-2.5">
                      <ExpandablePayload payload={item.payload_json} />
                    </div>
                  </article>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
