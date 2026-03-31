import { FilterX, RefreshCw, Search, User } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import { Button, DateField, Input, cn } from "@/src/components/ui";
import { AuditTable } from "@/src/features/audit/audit-table";
import { actionKind, rowSearchText, type AuditActionFilter } from "@/src/features/audit/audit-utils";
import { useAuditFeed } from "@/src/state/audit-feed";

const ACTION_OPTIONS: AuditActionFilter[] = ["all", "create", "update", "delete"];

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
          <>
            <Button variant="primary" onClick={() => void refresh()} className="header-btn w-full rounded-2xl px-4 sm:w-auto">
              <RefreshCw size={17} strokeWidth={1.7} />
              Refresh
            </Button>

            <div className="flex w-full items-center gap-1 rounded-2xl bg-surface-2/70 p-1 shadow-[inset_0_1px_0_var(--shell-highlight)] sm:w-auto">
              {ACTION_OPTIONS.map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => setActionFilter(item)}
                  className={cn(
                    "flex-1 rounded-xl px-3 py-2 text-center text-[13px] font-semibold capitalize transition-colors sm:flex-none sm:px-4",
                    actionFilter === item ? "bg-surface-4 text-txt-primary" : "text-txt-secondary hover:text-txt-primary",
                  )}
                >
                  {item}
                </button>
              ))}
            </div>

            <div className="relative w-full sm:w-[220px]">
              <User size={15} strokeWidth={1.8} className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-txt-tertiary" />
              <Input
                value={actorFilter}
                onChange={(event) => setActorFilter(event.target.value)}
                placeholder="Actor"
                className="header-btn rounded-2xl border-border/80 bg-surface-2/70 pl-11 shadow-[inset_0_1px_0_var(--shell-highlight)]"
              />
            </div>

            <div className="relative w-full sm:w-[280px]">
              <Search size={16} strokeWidth={1.7} className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-txt-tertiary" />
              <Input
                value={queryFilter}
                onChange={(event) => setQueryFilter(event.target.value)}
                placeholder="Search"
                className="header-btn rounded-2xl border-border/80 bg-surface-2/70 pl-11 shadow-[inset_0_1px_0_var(--shell-highlight)]"
              />
            </div>

            <div className="w-full sm:w-[150px]">
              <DateField
                value={dateFrom}
                onValueChange={setDateFrom}
                placeholder="From"
                className="header-btn rounded-2xl border-border/80 bg-surface-2/70 shadow-[inset_0_1px_0_var(--shell-highlight)]"
              />
            </div>

            <div className="w-full sm:w-[150px]">
              <DateField
                value={dateTo}
                onValueChange={setDateTo}
                placeholder="To"
                className="header-btn rounded-2xl border-border/80 bg-surface-2/70 shadow-[inset_0_1px_0_var(--shell-highlight)]"
              />
            </div>

            {hasActiveFilters ? (
              <Button onClick={clearFilters} className="header-btn w-full rounded-2xl px-4 sm:w-auto">
                <FilterX size={16} strokeWidth={1.8} />
                Clear
              </Button>
            ) : null}
          </>
        }
      />

      <div className="text-[13px] text-txt-secondary">
        {filteredItems.length} records{hasActiveFilters ? ` / ${items.length}` : ""}
      </div>

      <ErrorBanner message={error} onDismiss={clearError} actionLabel="Retry" onAction={() => void refresh()} />

      <AuditTable loading={loading} items={filteredItems} hasSourceItems={items.length > 0} />
    </div>
  );
}
