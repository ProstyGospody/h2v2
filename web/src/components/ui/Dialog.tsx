import * as DialogPrimitive from "@radix-ui/react-dialog";
import { AnimatePresence, motion } from "framer-motion";
import { X } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "./cn";

type DialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title?: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  contentClassName?: string;
  hideClose?: boolean;
};

export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  contentClassName,
  hideClose = false,
}: DialogProps) {
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <AnimatePresence>
        {open ? (
          <DialogPrimitive.Portal forceMount>
            <DialogPrimitive.Overlay asChild forceMount>
              <motion.div
                className="fixed inset-0 z-50 bg-black/60 backdrop-blur-md"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.2 }}
              />
            </DialogPrimitive.Overlay>

            <div className="fixed inset-0 z-50 grid place-items-center p-4">
              <DialogPrimitive.Content asChild forceMount>
                <motion.div
                  className={cn(
                    "relative w-full max-w-md rounded-2xl border border-border/70 bg-surface-2 p-7 shadow-2xl shadow-black/30 outline-none",
                    contentClassName,
                  )}
                  initial={{ opacity: 0, scale: 0.95, y: 10 }}
                  animate={{ opacity: 1, scale: 1, y: 0 }}
                  exit={{ opacity: 0, scale: 0.95, y: 10 }}
                  transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
                >
                  {(title || description || !hideClose) && (
                    <div className="mb-5 pr-8">
                      {title ? <DialogPrimitive.Title className="text-[18px] font-bold text-txt-primary">{title}</DialogPrimitive.Title> : null}
                      {description ? <DialogPrimitive.Description className="mt-2 text-[14px] leading-relaxed text-txt-secondary">{description}</DialogPrimitive.Description> : null}
                    </div>
                  )}

                  {!hideClose ? (
                    <DialogPrimitive.Close
                      className="absolute right-4 top-4 inline-flex h-9 w-9 items-center justify-center rounded-lg text-txt-muted transition-all hover:bg-surface-3 hover:text-txt"
                      aria-label="Close"
                    >
                      <X size={18} strokeWidth={1.6} />
                    </DialogPrimitive.Close>
                  ) : null}

                  <div>{children}</div>

                  {footer ? <div className="mt-6 flex items-center justify-end gap-3 border-t border-border/50 pt-5">{footer}</div> : null}
                </motion.div>
              </DialogPrimitive.Content>
            </div>
          </DialogPrimitive.Portal>
        ) : null}
      </AnimatePresence>
    </DialogPrimitive.Root>
  );
}

export type { DialogProps };
