import * as SwitchPrimitive from "@radix-ui/react-switch";
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from "react";

import { cn } from "./cn";

const Toggle = forwardRef<ElementRef<typeof SwitchPrimitive.Root>, ComponentPropsWithoutRef<typeof SwitchPrimitive.Root>>(
  ({ className, ...props }, ref) => (
    <SwitchPrimitive.Root
      ref={ref}
      className={cn(
        "relative h-6 w-11 cursor-pointer rounded-full border border-border/80 bg-surface-3/80 shadow-[inset_0_1px_0_var(--shell-highlight)] transition-all data-[state=checked]:border-transparent data-[state=checked]:bg-gradient-to-r data-[state=checked]:from-accent data-[state=checked]:to-accent-secondary data-[state=checked]:shadow-md data-[state=checked]:shadow-accent/30",
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb className="block h-5 w-5 rounded-full bg-txt-primary shadow-sm transition-transform data-[state=checked]:translate-x-[22px] data-[state=unchecked]:translate-x-0.5" />
    </SwitchPrimitive.Root>
  ),
);

Toggle.displayName = "Toggle";

export { Toggle };
