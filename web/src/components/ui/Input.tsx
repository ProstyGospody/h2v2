import { forwardRef, type InputHTMLAttributes } from "react";

import { cn } from "./cn";

type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  label?: string;
};

export const Input = forwardRef<HTMLInputElement, InputProps>(({ label, className, ...props }, ref) => (
  <div>
    {label ? <label className="mb-1.5 block text-[11px] font-medium text-txt-muted">{label}</label> : null}
    <input
      ref={ref}
      className={cn(
        "w-full rounded-btn border border-border bg-surface-1 px-3 py-2 text-[12px] text-txt outline-none transition-colors placeholder:text-txt-muted focus:border-accent/50",
        className,
      )}
      {...props}
    />
  </div>
));

Input.displayName = "Input";

export type { InputProps };
