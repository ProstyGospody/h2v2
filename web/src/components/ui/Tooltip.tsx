import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { forwardRef, type ReactNode } from "react";

import { cn } from "./cn";

type TooltipProps = {
  content: ReactNode;
  side?: "top" | "right" | "bottom" | "left";
  sideOffset?: number;
  delayDuration?: number;
  children: ReactNode;
  className?: string;
};

export function TooltipProvider({ children }: { children: ReactNode }) {
  return <TooltipPrimitive.Provider delayDuration={200}>{children}</TooltipPrimitive.Provider>;
}

export function Tooltip({ content, side = "top", sideOffset = 6, delayDuration, children, className }: TooltipProps) {
  if (!content) return <>{children}</>;

  return (
    <TooltipPrimitive.Root delayDuration={delayDuration}>
      <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Content
          side={side}
          sideOffset={sideOffset}
          className={cn(
            "z-50 max-w-[280px] rounded-lg bg-surface-2/95 px-3 py-2 text-[12px] font-medium text-txt shadow-[0_8px_24px_-8px_var(--dialog-shadow)] backdrop-blur-lg",
            "animate-in fade-in-0 zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95",
            "data-[side=top]:slide-in-from-bottom-2 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2",
            className,
          )}
        >
          {content}
        </TooltipPrimitive.Content>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}

export const TooltipTrigger = TooltipPrimitive.Trigger;
