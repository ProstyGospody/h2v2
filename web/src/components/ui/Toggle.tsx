import * as SwitchPrimitive from "@radix-ui/react-switch";
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from "react";

import { cn } from "./cn";

const Toggle = forwardRef<ElementRef<typeof SwitchPrimitive.Root>, ComponentPropsWithoutRef<typeof SwitchPrimitive.Root>>(
  ({ className, ...props }, ref) => (
    <SwitchPrimitive.Root
      ref={ref}
      className={cn(
        "relative h-7 w-12 cursor-pointer rounded-full border border-[var(--control-border)] bg-[var(--control-bg)] transition-colors",
        "hover:border-[var(--control-border-strong)]",
        "data-[state=checked]:border-transparent data-[state=checked]:bg-gradient-to-r data-[state=checked]:from-accent data-[state=checked]:to-accent-secondary",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-border-focus",
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb className="block h-5 w-5 rounded-full border border-white/70 bg-white shadow-sm transition-transform data-[state=checked]:translate-x-[24px] data-[state=unchecked]:translate-x-0.5" />
    </SwitchPrimitive.Root>
  ),
);

Toggle.displayName = "Toggle";

export { Toggle };
