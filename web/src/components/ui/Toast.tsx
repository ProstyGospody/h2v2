import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { AlertCircle, CheckCircle2, Info, X } from "lucide-react";
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";

import { cn } from "./cn";

type ToastVariant = "success" | "error" | "info";

type ToastItem = {
  id: number;
  message: string;
  variant: ToastVariant;
};

type ToastContextValue = {
  notify: (message: string, variant?: ToastVariant) => number;
  update: (id: number, message: string, variant?: ToastVariant) => void;
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
  const timersRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map());
  const reduceMotion = useReducedMotion();

  useEffect(() => {
    const timers = timersRef.current;
    return () => { timers.forEach((t) => clearTimeout(t)); timers.clear(); };
  }, []);

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
    const timer = timersRef.current.get(id);
    if (timer) { clearTimeout(timer); timersRef.current.delete(id); }
  }, []);

  const notify = useCallback((message: string, variant: ToastVariant = "success") => {
    const id = ++idRef.current;
    setToasts((prev) => {
      const keepCount = MAX_TOASTS - 1;
      const dropped = prev.slice(0, Math.max(0, prev.length - keepCount));
      for (const item of dropped) {
        const timer = timersRef.current.get(item.id);
        if (timer) {
          clearTimeout(timer);
          timersRef.current.delete(item.id);
        }
      }
      return [...prev.slice(-keepCount), { id, message, variant }];
    });
    const timer = setTimeout(() => { timersRef.current.delete(id); dismiss(id); }, AUTO_DISMISS_MS);
    timersRef.current.set(id, timer);
    return id;
  }, [dismiss]);

  const update = useCallback((id: number, message: string, variant?: ToastVariant) => {
    if (!timersRef.current.has(id)) {
      return;
    }
    setToasts((prev) =>
      prev.map((t) => (t.id === id ? { ...t, message, variant: variant ?? t.variant } : t)),
    );
    // Reset auto-dismiss timer
    const existing = timersRef.current.get(id);
    if (existing) clearTimeout(existing);
    const timer = setTimeout(() => { timersRef.current.delete(id); dismiss(id); }, AUTO_DISMISS_MS);
    timersRef.current.set(id, timer);
  }, [dismiss]);

  return (
    <ToastContext.Provider value={{ notify, update }}>
      {children}
      <div className="fixed bottom-4 right-4 left-4 z-50 flex flex-col-reverse items-end gap-2 sm:left-auto sm:bottom-5 sm:right-5">
        <AnimatePresence initial={false} mode="sync">
          {toasts.map((toast) => (
            <motion.div
              key={toast.id}
              role={toast.variant === "error" ? "alert" : "status"}
              aria-live={toast.variant === "error" ? "assertive" : "polite"}
              className={cn(
                "flex w-full items-center gap-3 rounded-xl bg-surface-2/95 px-5 py-3.5 text-[14px] text-txt shadow-[0_20px_46px_-28px_var(--dialog-shadow)] backdrop-blur-lg sm:max-w-[380px]",
                toast.variant === "success" && "border-status-success/20",
                toast.variant === "error" && "border-status-danger/20",
              )}
              initial={reduceMotion ? { opacity: 1 } : { opacity: 0, x: 40, scale: 0.96 }}
              animate={{ opacity: 1, x: 0, scale: 1 }}
              exit={reduceMotion ? { opacity: 1 } : { opacity: 0, x: 24, scale: 0.96 }}
              transition={reduceMotion ? { duration: 0 } : { duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
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

export type { ToastVariant, ToastItem };
