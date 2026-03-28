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
        "inline-flex cursor-pointer items-center justify-center gap-2 rounded-xl border font-semibold backdrop-blur-xl transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-50",
        size === "sm" && "px-3.5 py-2 text-[13px]",
        size === "md" && "px-5 py-2.5 text-[14px]",
        variant === "primary" &&
          "border-transparent bg-gradient-to-br from-accent to-accent-secondary text-white shadow-lg shadow-accent/25 hover:brightness-110 hover:shadow-xl hover:shadow-accent/35 active:scale-[0.97]",
        variant === "ghost" &&
          "border-border/80 bg-surface-2/70 text-txt-secondary shadow-[inset_0_1px_0_var(--shell-highlight)] hover:border-border-hover hover:bg-surface-3/70 hover:text-txt-primary active:scale-[0.98]",
        variant === "danger" &&
          "border-status-danger/35 bg-status-danger/16 text-status-danger shadow-[inset_0_1px_0_var(--shell-highlight)] hover:bg-status-danger/24 active:scale-[0.98]",
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
