import * as CheckboxPrimitive from "@radix-ui/react-checkbox";
import { Check, Minus } from "lucide-react";
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from "react";

import { cn } from "./cn";

const Checkbox = forwardRef<ElementRef<typeof CheckboxPrimitive.Root>, ComponentPropsWithoutRef<typeof CheckboxPrimitive.Root>>(
  ({ className, ...props }, ref) => (
    <CheckboxPrimitive.Root
      ref={ref}
      className={cn(
        "h-[18px] w-[18px] shrink-0 rounded-md border-0 bg-[var(--control-bg)] text-accent-secondary shadow-[inset_0_0_0_1px_var(--control-border)] outline-none transition-colors focus-visible:shadow-[inset_0_0_0_1px_var(--accent),0_0_0_3px_var(--accent-soft)] data-[state=checked]:bg-accent-secondary/18 data-[state=checked]:shadow-[inset_0_0_0_1px_rgba(122,108,246,0.62)] data-[state=indeterminate]:bg-accent-secondary/18 data-[state=indeterminate]:shadow-[inset_0_0_0_1px_rgba(122,108,246,0.62)]",
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
