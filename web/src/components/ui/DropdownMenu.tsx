import * as DropdownMenuPrimitive from "@radix-ui/react-dropdown-menu";
import { motion, AnimatePresence, useReducedMotion } from "framer-motion";
import type { ComponentPropsWithoutRef, ReactNode } from "react";
import { forwardRef } from "react";

import { cn } from "./cn";

export const DropdownMenu = DropdownMenuPrimitive.Root;
export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;
export const DropdownMenuPortal = DropdownMenuPrimitive.Portal;
export const DropdownMenuSeparator = forwardRef<
  HTMLDivElement,
  ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Separator>
>(({ className, ...props }, ref) => (
  <DropdownMenuPrimitive.Separator
    ref={ref}
    className={cn("my-1 h-px bg-border/40", className)}
    {...props}
  />
));
DropdownMenuSeparator.displayName = "DropdownMenuSeparator";

type ContentProps = ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Content> & {
  open?: boolean;
};

export const DropdownMenuContent = forwardRef<HTMLDivElement, ContentProps>(
  ({ className, sideOffset = 6, align = "end", children, ...props }, ref) => {
    const reduce = useReducedMotion();
    return (
      <DropdownMenuPrimitive.Portal>
        <DropdownMenuPrimitive.Content
          ref={ref}
          sideOffset={sideOffset}
          align={align}
          asChild
          {...props}
        >
          <motion.div
            initial={reduce ? { opacity: 1 } : { opacity: 0, y: -4, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={reduce ? { opacity: 1 } : { opacity: 0, y: -4, scale: 0.98 }}
            transition={reduce ? { duration: 0 } : { duration: 0.14, ease: [0.16, 1, 0.3, 1] }}
            className={cn(
              "z-50 min-w-[180px] overflow-hidden rounded-xl border border-border/50 bg-surface-2/95 p-1 shadow-[0_16px_40px_-12px_var(--dialog-shadow)] backdrop-blur-xl outline-none",
              className,
            )}
          >
            {children}
          </motion.div>
        </DropdownMenuPrimitive.Content>
      </DropdownMenuPrimitive.Portal>
    );
  },
);
DropdownMenuContent.displayName = "DropdownMenuContent";

type ItemProps = ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Item> & {
  danger?: boolean;
  icon?: ReactNode;
  shortcut?: string;
};

export const DropdownMenuItem = forwardRef<HTMLDivElement, ItemProps>(
  ({ className, danger, icon, shortcut, children, ...props }, ref) => (
    <DropdownMenuPrimitive.Item
      ref={ref}
      className={cn(
        "relative flex cursor-pointer select-none items-center gap-2.5 rounded-lg px-2.5 py-2 text-[15px] font-medium outline-none transition-colors",
        "data-[highlighted]:bg-surface-3/70 data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
        danger
          ? "text-status-danger data-[highlighted]:bg-status-danger/12"
          : "text-txt-primary",
        className,
      )}
      {...props}
    >
      {icon ? <span className="grid h-4 w-4 place-items-center text-txt-muted">{icon}</span> : null}
      <span className="flex-1 truncate">{children}</span>
      {shortcut ? (
        <span className="text-[13px] tracking-wide text-txt-muted">{shortcut}</span>
      ) : null}
    </DropdownMenuPrimitive.Item>
  ),
);
DropdownMenuItem.displayName = "DropdownMenuItem";

// Convenience wrapper that handles the AnimatePresence of the menu. Use when
// you need mount/unmount animations and the Content is not always rendered.
export function AnimatedDropdown({
  open,
  onOpenChange,
  trigger,
  children,
  align = "end",
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  trigger: ReactNode;
  children: ReactNode;
  align?: "start" | "center" | "end";
}) {
  return (
    <DropdownMenuPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DropdownMenuPrimitive.Trigger asChild>{trigger}</DropdownMenuPrimitive.Trigger>
      <AnimatePresence>
        {open ? <DropdownMenuContent align={align}>{children}</DropdownMenuContent> : null}
      </AnimatePresence>
    </DropdownMenuPrimitive.Root>
  );
}
