import { Loader2 } from "lucide-react";
import { forwardRef, type ButtonHTMLAttributes } from "react";

import { cn } from "./cn";

type Variant = "primary" | "ghost" | "danger";
type Size = "sm" | "md";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = "ghost", size = "md", loading, className, children, disabled, ...props }, ref) => (
    <button
      ref={ref}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(
        "inline-flex cursor-pointer items-center justify-center gap-2 rounded-xl font-semibold transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-border-focus",
        size === "sm" && "px-3.5 py-2 text-[13px]",
        size === "md" && "px-5 py-2.5 text-[14px]",
        variant === "primary" &&
          "border-0 bg-gradient-to-br from-accent to-accent-secondary text-white hover:brightness-105 active:brightness-95",
        variant === "ghost" &&
          "border-0 bg-[var(--control-bg)] text-txt-primary shadow-[inset_0_0_0_1px_var(--control-border),inset_0_1px_0_var(--shell-highlight)] hover:bg-[var(--control-bg-hover)] hover:shadow-[inset_0_0_0_1px_var(--control-border-strong),inset_0_1px_0_var(--shell-highlight)]",
        variant === "danger" &&
          "border-0 bg-status-danger/16 text-status-danger shadow-[inset_0_0_0_1px_rgba(185,120,130,0.45),inset_0_1px_0_var(--shell-highlight)] hover:bg-status-danger/24 hover:shadow-[inset_0_0_0_1px_rgba(185,120,130,0.62),inset_0_1px_0_var(--shell-highlight)]",
        className,
      )}
      {...props}
    >
      {loading ? <Loader2 size={size === "sm" ? 14 : 16} strokeWidth={2} className="animate-spin" /> : children}
    </button>
  ),
);

Button.displayName = "Button";

export type { ButtonProps, Size as ButtonSize, Variant as ButtonVariant };
