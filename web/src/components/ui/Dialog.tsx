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
                className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.15 }}
              />
            </DialogPrimitive.Overlay>

            <div className="fixed inset-0 z-50 grid place-items-center p-4">
              <DialogPrimitive.Content asChild forceMount>
                <motion.div
                  className={cn(
                    "relative w-full max-w-md rounded-card border border-border bg-surface-2 p-6 shadow-xl outline-none",
                    contentClassName,
                  )}
                  initial={{ opacity: 0, scale: 0.95 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.95 }}
                  transition={{ duration: 0.15 }}
                >
                  {(title || description || !hideClose) && (
                    <div className="mb-4 pr-8">
                      {title ? <DialogPrimitive.Title className="text-[16px] font-semibold text-white">{title}</DialogPrimitive.Title> : null}
                      {description ? <DialogPrimitive.Description className="mt-1 text-[12px] text-txt-secondary">{description}</DialogPrimitive.Description> : null}
                    </div>
                  )}

                  {!hideClose ? (
                    <DialogPrimitive.Close
                      className="absolute right-3 top-3 inline-flex h-8 w-8 items-center justify-center rounded-btn border border-border bg-surface-1 text-txt-tertiary transition-colors hover:border-border-hover hover:text-txt"
                      aria-label="Close"
                    >
                      <X size={16} strokeWidth={1.4} />
                    </DialogPrimitive.Close>
                  ) : null}

                  <div>{children}</div>

                  {footer ? <div className="mt-5 flex items-center justify-end gap-2 border-t border-border pt-4">{footer}</div> : null}
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
