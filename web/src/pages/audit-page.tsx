import { FilterX, RefreshCw } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import {
  Button,
  DateField,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/src/components/ui";
import { AuditTable } from "@/src/features/audit/audit-table";
import { actionKind, rowSearchText, type AuditActionFilter } from "@/src/features/audit/audit-utils";
import { useAuditFeed } from "@/src/state/audit-feed";

function parseBoundary(value: string, endOfDay: boolean): number | null {
  if (!value) return null;
  const iso = endOfDay ? `${value}T23:59:59.999` : `${value}T00:00:00`;
  const ms = new Date(iso).getTime();
  return Number.isFinite(ms) ? ms : null;
}

export default function AuditPage() {
  const { items, loading, error, refresh, clearError, markSeen } = useAuditFeed();
  const [actionFilter, setActionFilter] = useState<AuditActionFilter>("all");
  const [actorFilter, setActorFilter] = useState("");
  const [queryFilter, setQueryFilter] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  useEffect(() => {
    markSeen();
  }, [markSeen]);

  const hasActiveFilters = actionFilter !== "all" || actorFilter.trim() !== "" || queryFilter.trim() !== "" || dateFrom !== "" || dateTo !== "";

  const clearFilters = useCallback(() => {
    setActionFilter("all");
    setActorFilter("");
    setQueryFilter("");
    setDateFrom("");
    setDateTo("");
  }, []);

  const filteredItems = useMemo(() => {
    const actorNeedle = actorFilter.trim().toLowerCase();
    const queryNeedle = queryFilter.trim().toLowerCase();
    const fromMs = parseBoundary(dateFrom, false);
    const toMs = parseBoundary(dateTo, true);

    return items.filter((item) => {
      const kind = actionKind(item.action);
      if (actionFilter !== "all" && kind !== actionFilter) return false;

      const actor = (item.admin_email || "system").toLowerCase();
      if (actorNeedle && !actor.includes(actorNeedle)) return false;

      if (queryNeedle && !rowSearchText(item).includes(queryNeedle)) return false;

      const itemMs = new Date(item.created_at).getTime();
      if (!Number.isFinite(itemMs)) return false;
      if (fromMs !== null && itemMs < fromMs) return false;
      if (toMs !== null && itemMs > toMs) return false;
      return true;
    });
  }, [actionFilter, actorFilter, dateFrom, dateTo, items, queryFilter]);

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
        <div className="grid gap-3 lg:grid-cols-[150px,minmax(0,1fr),minmax(0,1fr),150px,150px] lg:items-end">
          <div>
            <label className="mb-2 block text-[12px] font-semibold uppercase tracking-wide text-txt-muted">Action</label>
            <Select value={actionFilter} onValueChange={(value) => setActionFilter(value as AuditActionFilter)}>
              <SelectTrigger className="h-11 rounded-xl">
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
            className="h-11 rounded-xl"
          />

          <Input
            label="Search"
            value={queryFilter}
            onChange={(event) => setQueryFilter(event.target.value)}
            placeholder="Search"
            className="h-11 rounded-xl"
          />

          <DateField label="From" value={dateFrom} onValueChange={setDateFrom} className="h-11 rounded-xl" />
          <DateField label="To" value={dateTo} onValueChange={setDateTo} className="h-11 rounded-xl" />
        </div>

        <div className="flex items-center justify-between gap-3 text-[12px] text-txt-secondary">
          <span>{filteredItems.length} records{hasActiveFilters ? ` / ${items.length}` : ""}</span>
          {hasActiveFilters ? (
            <button
              type="button"
              onClick={clearFilters}
              className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 transition-colors hover:bg-surface-3/50 hover:text-txt"
            >
              <FilterX size={14} strokeWidth={1.7} />
              Clear
            </button>
          ) : null}
        </div>
      </div>

      <ErrorBanner message={error} onDismiss={clearError} actionLabel="Retry" onAction={() => void refresh()} />

      <AuditTable loading={loading} items={filteredItems} hasSourceItems={items.length > 0} />
    </div>
  );
}
