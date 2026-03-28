import type { HTMLAttributes } from "react";

import { cn } from "./cn";

type BadgeVariant =
  | "default"
  | "success"
  | "warning"
  | "danger"
  | "protocol-vless"
  | "protocol-hy2";

type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  variant?: BadgeVariant;
};

export function Badge({ variant = "default", className, children, ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-badge border border-border px-2.5 py-0.5 text-[10px] font-semibold uppercase tracking-[0.02em]",
        variant === "default" && "bg-surface-3 text-txt-secondary",
        variant === "success" && "border-status-success/20 bg-status-success/10 text-status-success",
        variant === "warning" && "border-status-warning/20 bg-status-warning/10 text-status-warning",
        variant === "danger" && "border-status-danger/20 bg-status-danger/10 text-status-danger",
        variant === "protocol-vless" && "border-accent/20 bg-accent/10 text-accent-light",
        variant === "protocol-hy2" && "border-accent-secondary/20 bg-accent-secondary/10 text-accent-secondary-light",
        className,
      )}
      {...props}
    >
      {children}
    </span>
  );
}

export type { BadgeProps, BadgeVariant };
