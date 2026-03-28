import { forwardRef, type InputHTMLAttributes } from "react";

import { cn } from "./cn";

type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  label?: string;
};

export const Input = forwardRef<HTMLInputElement, InputProps>(({ label, className, ...props }, ref) => (
  <div>
    {label ? <label className="mb-2 block text-[13px] font-medium text-txt-secondary">{label}</label> : null}
    <input
      ref={ref}
      className={cn(
        "w-full rounded-lg border border-border bg-surface-1 px-4 py-2.5 text-[14px] text-txt outline-none transition-all placeholder:text-txt-muted focus:border-accent/40 focus:shadow-[0_0_0_3px_var(--primary-soft)]",
        className,
      )}
      {...props}
    />
  </div>
));

Input.displayName = "Input";

export type { InputProps };
