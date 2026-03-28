import * as SwitchPrimitive from "@radix-ui/react-switch";
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from "react";

import { cn } from "./cn";

const Toggle = forwardRef<ElementRef<typeof SwitchPrimitive.Root>, ComponentPropsWithoutRef<typeof SwitchPrimitive.Root>>(
  ({ className, ...props }, ref) => (
    <SwitchPrimitive.Root
      ref={ref}
      className={cn(
        "relative h-6 w-11 cursor-pointer rounded-full transition-all data-[state=checked]:bg-gradient-to-r data-[state=checked]:from-accent data-[state=checked]:to-cyan-500 data-[state=checked]:shadow-sm data-[state=checked]:shadow-accent/25 data-[state=unchecked]:bg-surface-4",
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb className="block h-5 w-5 rounded-full bg-white shadow-sm transition-transform data-[state=checked]:translate-x-[22px] data-[state=unchecked]:translate-x-0.5" />
    </SwitchPrimitive.Root>
  ),
);

Toggle.displayName = "Toggle";

export { Toggle };
