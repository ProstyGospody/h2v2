import { Inbox } from "lucide-react";
import { type ReactNode } from "react";

export function LoadingState({ message, minHeight = 280 }: { message: string; minHeight?: number }) {
  return (
    <div className="grid place-items-center rounded-2xl bg-surface-2" style={{ minHeight }}>
      <div className="flex flex-col items-center gap-3">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent/20 border-t-accent-light" />
        <p className="text-[14px] text-txt-secondary">{message}</p>
      </div>
    </div>
  );
}

export function EmptyState({ title, description, icon, minHeight = 240 }: { title: string; description?: string; icon?: ReactNode; minHeight?: number }) {
  return (
    <div className="grid place-items-center rounded-2xl bg-surface-2 px-5" style={{ minHeight }}>
      <div className="flex flex-col items-center gap-3 text-center">
        <div className="grid h-14 w-14 place-items-center rounded-full bg-surface-3/50">
          {icon || <Inbox size={24} strokeWidth={1.6} className="text-txt-muted" />}
        </div>
        <p className="text-[15px] font-semibold text-txt-primary">{title}</p>
        {description && <p className="max-w-[300px] text-[14px] text-txt-secondary">{description}</p>}
      </div>
    </div>
  );
}
