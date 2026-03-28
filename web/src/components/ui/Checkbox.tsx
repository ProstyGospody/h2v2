import * as CheckboxPrimitive from "@radix-ui/react-checkbox";
import { Check, Minus } from "lucide-react";
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from "react";

import { cn } from "./cn";

const Checkbox = forwardRef<ElementRef<typeof CheckboxPrimitive.Root>, ComponentPropsWithoutRef<typeof CheckboxPrimitive.Root>>(
  ({ className, ...props }, ref) => (
    <CheckboxPrimitive.Root
      ref={ref}
      className={cn(
        "h-[18px] w-[18px] shrink-0 rounded-md border border-border bg-surface-1 text-accent-light outline-none transition-all focus:border-accent/50 focus:shadow-[0_0_0_3px_var(--primary-soft)] data-[state=checked]:border-accent/50 data-[state=checked]:bg-accent/15 data-[state=indeterminate]:border-accent/50 data-[state=indeterminate]:bg-accent/15",
        className,
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator className="flex items-center justify-center">
        {props.checked === "indeterminate" ? (
          <Minus size={12} strokeWidth={2.5} />
        ) : (
          <Check size={13} strokeWidth={2.2} />
        )}
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  ),
);

Checkbox.displayName = "Checkbox";

export { Checkbox };
