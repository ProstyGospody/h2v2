import { forwardRef, useId, type InputHTMLAttributes } from "react";

import { cn } from "./cn";

type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  label?: string;
};

export const Input = forwardRef<HTMLInputElement, InputProps>(({ label, className, id, ...props }, ref) => {
  const autoId = useId();
  const inputId = id || autoId;

  return (
    <div>
      {label ? <label htmlFor={inputId} className="mb-2 block text-[15px] font-medium text-txt-secondary">{label}</label> : null}
      <input
        ref={ref}
        id={inputId}
        className={cn(
          "w-full rounded-lg border-0 bg-[var(--control-bg)] px-4 py-2.5 text-[16px] font-medium text-txt-primary shadow-[inset_0_0_0_1px_var(--control-border)] outline-none transition-colors placeholder:text-txt-tertiary focus:bg-[var(--control-bg-hover)] focus:shadow-[inset_0_0_0_1px_var(--accent),0_0_0_3px_var(--accent-soft)]",
          className,
        )}
        {...props}
      />
    </div>
  );
});

Input.displayName = "Input";

export type { InputProps };
