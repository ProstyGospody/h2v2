import { Inbox } from "lucide-react";
import { type ReactNode } from "react";

export function LoadingState({ message, minHeight = 260 }: { message: string; minHeight?: number }) {
  return (
    <div className="grid place-items-center rounded-[12px] border border-border/80 bg-surface-2" style={{ minHeight }}>
      <div className="flex flex-col items-center gap-3">
        <div className="h-5 w-5 animate-spin rounded-full border-2 border-accent/20 border-t-accent-light" />
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
    <div className="grid place-items-center rounded-[12px] border border-border/80 bg-surface-2 px-4" style={{ minHeight }}>
      <div className="flex flex-col items-center gap-2 text-center">
        <div className="grid h-12 w-12 place-items-center rounded-full bg-surface-3/60">
          {icon || <Inbox size={20} strokeWidth={1.4} className="text-txt-muted" />}
        </div>
        <p className="text-[13px] font-medium text-white">{title}</p>
        {description ? <p className="max-w-[280px] text-[12px] text-txt-secondary">{description}</p> : null}
      </div>
    </div>
  );
}
