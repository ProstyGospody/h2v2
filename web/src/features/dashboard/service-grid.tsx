import { Cpu, Eye, RefreshCw, RotateCcw } from "lucide-react";
import type { ReactNode } from "react";

import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { ErrorBanner } from "@/components/ui/error-banner";
import { Button, StateBlock, cn } from "@/src/components/ui";
import { type ServiceSummary } from "@/types/common";
import { formatDateTime } from "@/utils/format";

import { serviceStatusColor } from "./dashboard-utils";

function SectionHeader({ icon, title, children }: { icon: ReactNode; title: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-2.5">
        <div className="grid h-8 w-8 place-items-center rounded-lg bg-surface-3/50 text-txt-tertiary">
          {icon}
        </div>
        <h3 className="text-[16px] font-bold text-txt-primary">{title}</h3>
      </div>
      {children}
    </div>
  );
}

type ServiceGridProps = {
  loading: boolean;
  items: ServiceSummary[];
  busy: boolean;
  error: string;
  canRetry: boolean;
  onDismissError: () => void;
  onRetryError: () => void;
  onOpenDetails: (name: string) => void;
  onRunAction: (name: string, action: "reload" | "restart") => void;
};

export function ServiceGrid({
  loading,
  items,
  busy,
  error,
  canRetry,
  onDismissError,
  onRetryError,
  onOpenDetails,
  onRunAction,
}: ServiceGridProps) {
  return (
    <div className="space-y-2.5">
      <SectionHeader icon={<Cpu size={18} strokeWidth={1.6} />} title="Managed Services" />

      <ErrorBanner
        message={error}
        onDismiss={onDismissError}
        actionLabel={canRetry ? "Retry" : undefined}
        onAction={canRetry ? onRetryError : undefined}
      />

      {loading ? (
        <StateBlock tone="loading" title="Loading services" minHeightClassName="min-h-[120px]" />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {items.length ? items.map((item) => (
            <div key={item.service_name} className="card-hover panel-card-compact relative overflow-hidden">
              <div className="mb-2.5 flex items-center justify-between gap-3">
                <h4 className="min-w-0 flex-1 truncate text-[15px] font-bold text-txt-primary">{item.service_name}</h4>
                <span className="inline-flex shrink-0 items-center gap-2 rounded-lg bg-surface-3/40 px-2.5 py-1 text-[12px] font-medium text-txt-secondary">
                  <span className={cn("h-2 w-2 rounded-full", serviceStatusColor(item.status || "unknown"))} />
                  {(item.status || "unknown").toLowerCase()}
                </span>
              </div>

              <div className="grid grid-cols-2 gap-2.5">
                <div>
                  <p className="text-[12px] font-medium text-txt-muted">Version</p>
                  <p className="mt-1 text-[14px] font-medium text-txt">{item.version || "-"}</p>
                </div>
                <div>
                  <p className="text-[12px] font-medium text-txt-muted">Last check</p>
                  <p className="mt-1 text-[14px] font-medium text-txt">{formatDateTime(item.last_check_at)}</p>
                </div>
              </div>

              <div className="mt-2.5 flex flex-wrap items-center gap-2 border-t border-border/30 pt-2.5">
                <Button
                  size="sm"
                  className="h-8 min-w-[96px] flex-1 px-3 sm:flex-none"
                  onClick={() => onOpenDetails(item.service_name)}
                  disabled={busy}
                >
                  <Eye size={15} strokeWidth={1.6} />
                  Details
                </Button>

                <ConfirmPopover
                  title="Reload service"
                  description={`Reload ${item.service_name}?`}
                  confirmText="Reload"
                  onConfirm={() => onRunAction(item.service_name, "reload")}
                >
                  <Button size="sm" className="h-8 min-w-[96px] flex-1 px-3 sm:flex-none" disabled={busy}>
                    <RefreshCw size={15} strokeWidth={1.6} />
                    Reload
                  </Button>
                </ConfirmPopover>

                <ConfirmPopover
                  title="Restart service"
                  description={`Restart ${item.service_name}?`}
                  confirmText="Restart"
                  onConfirm={() => onRunAction(item.service_name, "restart")}
                >
                  <Button size="sm" className="h-8 min-w-[96px] flex-1 px-3 sm:flex-none" disabled={busy}>
                    <RotateCcw size={15} strokeWidth={1.6} />
                    Restart
                  </Button>
                </ConfirmPopover>
              </div>
            </div>
          )) : <StateBlock tone="empty" title="No services" minHeightClassName="min-h-[120px]" />}
        </div>
      )}
    </div>
  );
}
