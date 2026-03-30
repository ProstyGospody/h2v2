import { AnimatePresence, motion } from "framer-motion";
import { AlertCircle, CheckCircle2, Info, X } from "lucide-react";
import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";

import { cn } from "./cn";

type ToastVariant = "success" | "error" | "info";

type ToastItem = {
  id: number;
  message: string;
  variant: ToastVariant;
};

type ToastContextValue = {
  notify: (message: string, variant?: ToastVariant) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

const MAX_TOASTS = 4;
const AUTO_DISMISS_MS = 3200;

function ToastIcon({ variant }: { variant: ToastVariant }) {
  if (variant === "success") {
    return <CheckCircle2 size={18} strokeWidth={1.6} className="shrink-0 text-status-success" />;
  }
  if (variant === "error") {
    return <AlertCircle size={18} strokeWidth={1.6} className="shrink-0 text-status-danger" />;
  }
  return <Info size={18} strokeWidth={1.6} className="shrink-0 text-status-info" />;
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const idRef = useRef(0);

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const notify = useCallback((message: string, variant: ToastVariant = "success") => {
    const id = ++idRef.current;
    setToasts((prev) => [...prev.slice(-(MAX_TOASTS - 1)), { id, message, variant }]);
    setTimeout(() => dismiss(id), AUTO_DISMISS_MS);
  }, [dismiss]);

  return (
    <ToastContext.Provider value={{ notify }}>
      {children}
      <div className="fixed bottom-4 right-4 left-4 z-50 flex flex-col-reverse items-end gap-2 sm:left-auto sm:bottom-5 sm:right-5">
        <AnimatePresence mode="popLayout">
          {toasts.map((toast) => (
            <motion.div
              key={toast.id}
              layout
              className={cn(
                "flex w-full items-center gap-3 rounded-xl bg-surface-2/95 px-5 py-3.5 text-[14px] text-txt shadow-[0_20px_46px_-28px_var(--dialog-shadow)] backdrop-blur-lg sm:max-w-[380px]",
                toast.variant === "success" && "border-status-success/20",
                toast.variant === "error" && "border-status-danger/20",
              )}
              initial={{ opacity: 0, x: 40, scale: 0.95 }}
              animate={{ opacity: 1, x: 0, scale: 1 }}
              exit={{ opacity: 0, x: 40, scale: 0.95 }}
              transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
            >
              <ToastIcon variant={toast.variant} />
              <span className="flex-1 leading-relaxed">{toast.message}</span>
              <button
                type="button"
                onClick={() => dismiss(toast.id)}
                className="shrink-0 rounded-md p-0.5 text-txt-muted transition-colors hover:text-txt"
              >
                <X size={14} strokeWidth={1.6} />
              </button>
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}

// Legacy single-toast export for backward compat
export function Toast({
  open,
  onOpenChange,
  message,
  variant = "info",
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  message: string;
  variant?: ToastVariant;
  duration?: number;
}) {
  if (!open) return null;
  return (
    <div className="fixed bottom-4 right-4 left-4 z-50 sm:left-auto sm:bottom-5 sm:right-5">
      <AnimatePresence>
        <motion.div
          className={cn(
            "flex w-full items-center gap-3 rounded-xl bg-surface-2/95 px-5 py-3.5 text-[14px] text-txt shadow-[0_20px_46px_-28px_var(--dialog-shadow)] backdrop-blur-lg sm:max-w-[380px]",
            variant === "success" && "border-status-success/20",
            variant === "error" && "border-status-danger/20",
          )}
          initial={{ opacity: 0, x: 40 }}
          animate={{ opacity: 1, x: 0 }}
          exit={{ opacity: 0, x: 40 }}
          transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
        >
          <ToastIcon variant={variant} />
          <span className="flex-1 leading-relaxed">{message}</span>
          <button
            type="button"
            onClick={() => onOpenChange(false)}
            className="shrink-0 rounded-md p-0.5 text-txt-muted transition-colors hover:text-txt"
          >
            <X size={14} strokeWidth={1.6} />
          </button>
        </motion.div>
      </AnimatePresence>
    </div>
  );
}

export type { ToastVariant, ToastItem };
