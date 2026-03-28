import { Inbox, Loader2 } from "lucide-react";
import { type ReactNode } from "react";

export function LoadingState({ message, minHeight = 260 }: { message: string; minHeight?: number }) {
  return (
    <div className="grid place-items-center rounded-card border border-border bg-surface-2" style={{ minHeight }}>
      <div className="flex flex-col items-center gap-2">
        <Loader2 size={20} strokeWidth={1.4} className="animate-spin text-txt-secondary" />
        <p className="text-[12px] text-txt-secondary">{message}</p>
      </div>
    </div>
  );
}

export function EmptyState({
  title,
  description,
  icon,
  minHeight = 220,
}: {
  title: string;
  description?: string;
  icon?: ReactNode;
  minHeight?: number;
}) {
  return (
    <div className="grid place-items-center rounded-card border border-border bg-surface-2 px-4" style={{ minHeight }}>
      <div className="flex flex-col items-center gap-1 text-center">
        {icon || <Inbox size={18} strokeWidth={1.4} className="text-txt-tertiary" />}
        <p className="text-[13px] font-semibold text-white">{title}</p>
        {description ? <p className="text-[12px] text-txt-secondary">{description}</p> : null}
      </div>
    </div>
  );
}
