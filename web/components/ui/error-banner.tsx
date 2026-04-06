import { X } from "lucide-react";

export function ErrorBanner({
  message,
  onDismiss,
  actionLabel,
  onAction,
}: {
  message: string;
  onDismiss?: () => void;
  actionLabel?: string;
  onAction?: () => void;
}) {
  if (!message) return null;
  return (
    <div className="flex items-center gap-3 rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[16px] text-status-danger">
      <span className="flex-1">{message}</span>
      {actionLabel && onAction && (
        <button
          type="button"
          onClick={onAction}
          className="shrink-0 rounded-lg px-2.5 py-1 text-[14px] font-semibold text-status-danger transition-colors hover:bg-status-danger/12"
        >
          {actionLabel}
        </button>
      )}
      {onDismiss && (
        <button type="button" onClick={onDismiss} className="shrink-0 rounded-md p-0.5 text-status-danger/60 transition-colors hover:text-status-danger">
          <X size={16} strokeWidth={1.6} />
        </button>
      )}
    </div>
  );
}
