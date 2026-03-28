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
    return <CheckCircle2 size={16} strokeWidth={1.4} className="shrink-0 text-status-success" />;
  }
  if (variant === "error") {
    return <AlertCircle size={16} strokeWidth={1.4} className="shrink-0 text-status-danger" />;
  }
  return <Info size={16} strokeWidth={1.4} className="shrink-0 text-accent-light" />;
}

export function Toast({ open, onOpenChange, message, variant = "info", duration = 2800 }: ToastProps) {
  return (
    <ToastPrimitive.Provider swipeDirection="right" duration={duration}>
      <AnimatePresence>
        {open ? (
          <ToastPrimitive.Root asChild forceMount open={open} onOpenChange={onOpenChange}>
            <motion.div
              className={cn(
                "fixed bottom-4 right-4 z-50 flex max-w-[340px] items-start gap-2.5 rounded-[12px] border border-border/80 bg-surface-2/95 px-4 py-3.5 text-[12px] text-txt shadow-2xl shadow-black/20 backdrop-blur-lg",
                variant === "success" && "border-status-success/15",
                variant === "error" && "border-status-danger/15",
              )}
              initial={{ opacity: 0, x: 40, y: 10 }}
              animate={{ opacity: 1, x: 0, y: 0 }}
              exit={{ opacity: 0, x: 40 }}
              transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
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
