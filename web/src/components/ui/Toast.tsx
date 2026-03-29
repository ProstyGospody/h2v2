import * as ToastPrimitive from "@radix-ui/react-toast";
import { AnimatePresence, motion } from "framer-motion";
import { AlertCircle, CheckCircle2, Info } from "lucide-react";

import { cn } from "./cn";

type ToastVariant = "success" | "error" | "info";

type ToastProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  message: string;
  variant?: ToastVariant;
  duration?: number;
};

function ToastIcon({ variant }: { variant: ToastVariant }) {
  if (variant === "success") {
    return <CheckCircle2 size={18} strokeWidth={1.6} className="shrink-0 text-status-success" />;
  }
  if (variant === "error") {
    return <AlertCircle size={18} strokeWidth={1.6} className="shrink-0 text-status-danger" />;
  }
  return <Info size={18} strokeWidth={1.6} className="shrink-0 text-status-info" />;
}

export function Toast({ open, onOpenChange, message, variant = "info", duration = 2800 }: ToastProps) {
  return (
    <ToastPrimitive.Provider swipeDirection="right" duration={duration}>
      <AnimatePresence>
        {open ? (
          <ToastPrimitive.Root asChild forceMount open={open} onOpenChange={onOpenChange}>
            <motion.div
              className={cn(
                "fixed bottom-4 left-4 right-4 z-50 flex items-start gap-3 rounded-xl border border-border/70 bg-surface-2/95 px-5 py-4 text-[14px] text-txt shadow-[0_20px_46px_-28px_var(--dialog-shadow)] backdrop-blur-lg sm:bottom-5 sm:left-auto sm:right-5 sm:max-w-[380px]",
                variant === "success" && "border-status-success/20",
                variant === "error" && "border-status-danger/20",
              )}
              initial={{ opacity: 0, x: 40, y: 10 }}
              animate={{ opacity: 1, x: 0, y: 0 }}
              exit={{ opacity: 0, x: 40 }}
              transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
            >
              <ToastIcon variant={variant} />
              <ToastPrimitive.Title className="leading-relaxed">{message}</ToastPrimitive.Title>
            </motion.div>
          </ToastPrimitive.Root>
        ) : null}
      </AnimatePresence>
      <ToastPrimitive.Viewport className="fixed bottom-4 left-4 right-4 z-50 sm:bottom-5 sm:left-auto sm:right-5" />
    </ToastPrimitive.Provider>
  );
}

export type { ToastProps, ToastVariant };
