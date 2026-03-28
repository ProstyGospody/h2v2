import { forwardRef, type InputHTMLAttributes } from "react";

import { cn } from "./cn";

type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  label?: string;
};

export const Input = forwardRef<HTMLInputElement, InputProps>(({ label, className, ...props }, ref) => (
  <div>
    {label ? <label className="mb-1.5 block text-[11px] font-medium text-txt-secondary">{label}</label> : null}
    <input
      ref={ref}
      className={cn(
        "w-full rounded-[8px] border border-border bg-surface-1 px-3 py-2 text-[12px] text-txt outline-none transition-all placeholder:text-txt-muted focus:border-accent/40 focus:shadow-[0_0_0_3px_rgba(99,102,241,0.06)]",
        className,
      )}
      {...props}
    />
  </div>
));

Input.displayName = "Input";

export type { InputProps };
