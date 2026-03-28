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
        "inline-flex cursor-pointer items-center justify-center gap-2 rounded-xl border font-semibold transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50",
        size === "sm" && "px-3.5 py-2 text-[13px]",
        size === "md" && "px-5 py-2.5 text-[14px]",
        variant === "primary" &&
          "border-transparent bg-gradient-to-br from-accent to-accent-secondary text-white shadow-lg shadow-accent/25 hover:brightness-110 hover:shadow-xl hover:shadow-accent/35 active:scale-[0.98]",
        variant === "ghost" &&
          "border-[var(--control-border)] bg-[var(--control-bg)] text-txt-primary shadow-[inset_0_1px_0_var(--shell-highlight)] hover:border-[var(--control-border-strong)] hover:bg-[var(--control-bg-hover)]",
        variant === "danger" &&
          "border-status-danger/38 bg-status-danger/16 text-status-danger shadow-[inset_0_1px_0_var(--shell-highlight)] hover:border-status-danger/55 hover:bg-status-danger/24",
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
