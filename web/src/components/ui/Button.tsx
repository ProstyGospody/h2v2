import { forwardRef, type ButtonHTMLAttributes } from "react";

import { cn } from "./cn";

type Variant = "primary" | "ghost" | "danger";
type Size = "sm" | "md";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = "ghost", size = "md", className, children, ...props }, ref) => (
    <button
      ref={ref}
      className={cn(
        "inline-flex cursor-pointer items-center justify-center gap-2 font-semibold transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-50",
        size === "sm" && "rounded-lg px-3.5 py-2 text-[13px]",
        size === "md" && "rounded-xl px-5 py-2.5 text-[14px]",
        variant === "primary" &&
          "bg-gradient-to-br from-accent to-cyan-500 text-white shadow-lg shadow-accent/20 hover:shadow-xl hover:shadow-accent/30 hover:brightness-110 active:scale-[0.97]",
        variant === "ghost" &&
          "border border-border bg-transparent text-txt-secondary hover:border-border-hover hover:bg-surface-3/40 hover:text-txt active:scale-[0.98]",
        variant === "danger" &&
          "border border-status-danger/25 bg-status-danger/8 text-status-danger hover:bg-status-danger/15 active:scale-[0.98]",
        className,
      )}
      {...props}
    >
      {children}
    </button>
  ),
);

Button.displayName = "Button";

export type { ButtonProps, Size as ButtonSize, Variant as ButtonVariant };
