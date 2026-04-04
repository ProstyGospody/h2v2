import * as DialogPrimitive from "@radix-ui/react-dialog";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { X } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "./cn";

type DrawerProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title?: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  side?: "right" | "left";
  width?: "sm" | "md" | "lg" | "xl";
  contentClassName?: string;
};

const widthMap = {
  sm: "max-w-sm",
  md: "max-w-md",
  lg: "max-w-lg",
  xl: "max-w-2xl",
};

export function Drawer({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  side = "right",
  width = "lg",
  contentClassName,
}: DrawerProps) {
  const reduce = useReducedMotion();
  const from = side === "right" ? "100%" : "-100%";
  const panelSide = side === "right" ? "right-0" : "left-0";

  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <AnimatePresence initial={false}>
        {open ? (
          <DialogPrimitive.Portal forceMount>
            <DialogPrimitive.Overlay asChild forceMount>
              <motion.div
                className="fixed inset-0 z-50 bg-[var(--dialog-overlay)] backdrop-blur-[2px]"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={reduce ? { duration: 0 } : { duration: 0.18 }}
              />
            </DialogPrimitive.Overlay>

            <DialogPrimitive.Content asChild forceMount>
              <motion.div
                className={cn(
                  "fixed top-0 bottom-0 z-50 flex w-full flex-col bg-[var(--dialog-surface)] shadow-[0_24px_56px_-16px_var(--dialog-shadow)] outline-none backdrop-blur-xl",
                  panelSide,
                  widthMap[width],
                  contentClassName,
                )}
                initial={reduce ? { opacity: 1 } : { x: from }}
                animate={{ x: 0 }}
                exit={reduce ? { opacity: 1 } : { x: from }}
                transition={reduce ? { duration: 0 } : { duration: 0.26, ease: [0.16, 1, 0.3, 1] }}
              >
                {(title || description) && (
                  <div className="flex items-start justify-between gap-3 border-b border-border/40 px-6 py-5">
                    <div className="min-w-0 flex-1">
                      {title ? (
                        <DialogPrimitive.Title className="truncate text-[17px] font-bold text-txt-primary">
                          {title}
                        </DialogPrimitive.Title>
                      ) : null}
                      {description ? (
                        <DialogPrimitive.Description className="mt-1 text-[13px] leading-relaxed text-txt-secondary">
                          {description}
                        </DialogPrimitive.Description>
                      ) : null}
                    </div>
                    <DialogPrimitive.Close
                      className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-txt-muted transition-colors hover:bg-surface-3/75 hover:text-txt-primary"
                      aria-label="Close"
                    >
                      <X size={18} strokeWidth={1.6} />
                    </DialogPrimitive.Close>
                  </div>
                )}
                <div className="flex-1 overflow-y-auto px-6 py-5">{children}</div>
                {footer ? (
                  <div className="border-t border-border/40 px-6 py-4">{footer}</div>
                ) : null}
              </motion.div>
            </DialogPrimitive.Content>
          </DialogPrimitive.Portal>
        ) : null}
      </AnimatePresence>
    </DialogPrimitive.Root>
  );
}

export type { DrawerProps };
