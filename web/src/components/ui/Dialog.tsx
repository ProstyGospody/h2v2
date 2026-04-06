import * as DialogPrimitive from "@radix-ui/react-dialog";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
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
  const reduceMotion = useReducedMotion();

  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <AnimatePresence initial={false} mode="wait">
        {open ? (
          <DialogPrimitive.Portal forceMount>
            <DialogPrimitive.Overlay asChild forceMount>
              <motion.div
                className="fixed inset-0 z-50 bg-[var(--dialog-overlay)] backdrop-blur-[3px]"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={reduceMotion ? { duration: 0 } : { duration: 0.2 }}
              />
            </DialogPrimitive.Overlay>

            <div className="fixed inset-0 z-50 grid place-items-center p-4">
              <DialogPrimitive.Content asChild forceMount>
                <motion.div
                  className={cn(
                    "relative w-full max-w-md max-h-[calc(100dvh-2rem)] overflow-y-auto rounded-2xl bg-[var(--dialog-surface)] p-5 shadow-[0_24px_56px_-16px_var(--dialog-shadow)] outline-none backdrop-blur-xl sm:p-7",
                    contentClassName,
                  )}
                  initial={reduceMotion ? { opacity: 1 } : { opacity: 0, scale: 0.97 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={reduceMotion ? { opacity: 1 } : { opacity: 0, scale: 0.97 }}
                  transition={reduceMotion ? { duration: 0 } : { duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
                >
                  {(title || description || !hideClose) && (
                    <div className="mb-5 pr-8">
                      {title ? <DialogPrimitive.Title className="text-[20px] font-bold text-txt-primary">{title}</DialogPrimitive.Title> : null}
                      {description ? <DialogPrimitive.Description className="mt-2 text-[16px] leading-relaxed text-txt-secondary">{description}</DialogPrimitive.Description> : null}
                    </div>
                  )}

                  {!hideClose ? (
                    <DialogPrimitive.Close
                      className="absolute right-4 top-4 inline-flex h-9 w-9 items-center justify-center rounded-lg text-txt-muted transition-colors hover:bg-surface-3/75 hover:text-txt-primary"
                      aria-label="Close"
                    >
                      <X size={18} strokeWidth={1.6} />
                    </DialogPrimitive.Close>
                  ) : null}

                  <div>{children}</div>

                  {footer ? <div className="mt-6 flex flex-wrap items-center justify-end gap-3 pt-5">{footer}</div> : null}
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
