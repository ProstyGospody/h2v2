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
    return <CheckCircle2 size={16} strokeWidth={1.4} className="text-status-success" />;
  }
  if (variant === "error") {
    return <AlertCircle size={16} strokeWidth={1.4} className="text-status-danger" />;
  }
  return <Info size={16} strokeWidth={1.4} className="text-accent-light" />;
}

export function Toast({ open, onOpenChange, message, variant = "info", duration = 2800 }: ToastProps) {
  return (
    <ToastPrimitive.Provider swipeDirection="right" duration={duration}>
      <AnimatePresence>
        {open ? (
          <ToastPrimitive.Root asChild forceMount open={open} onOpenChange={onOpenChange}>
            <motion.div
              className={cn(
                "fixed bottom-4 right-4 z-50 flex max-w-[320px] items-start gap-2 rounded-card border border-border bg-surface-2 px-4 py-3 text-[12px] text-txt shadow-xl",
                variant === "success" && "border-status-success/20",
                variant === "error" && "border-status-danger/20",
              )}
              initial={{ opacity: 0, x: 40 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 40 }}
              transition={{ duration: 0.2 }}
            >
              <ToastIcon variant={variant} />
              <ToastPrimitive.Title className="leading-5">{message}</ToastPrimitive.Title>
            </motion.div>
          </ToastPrimitive.Root>
        ) : null}
      </AnimatePresence>
      <ToastPrimitive.Viewport className="fixed bottom-4 right-4 z-50" />
    </ToastPrimitive.Provider>
  );
}

export type { ToastProps, ToastVariant };
