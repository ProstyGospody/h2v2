import { AlertCircle, Inbox, Loader2 } from "lucide-react";
import type { ComponentType } from "react";

import { Button } from "./Button";
import { cn } from "./cn";

type StateTone = "loading" | "empty" | "error";
type StateIcon = ComponentType<{ size?: number; strokeWidth?: number; className?: string }>;

type StateBlockProps = {
  tone: StateTone;
  title: string;
  actionLabel?: string;
  onAction?: () => void;
  className?: string;
  minHeightClassName?: string;
  icon?: StateIcon;
};

function toneIcon(tone: StateTone): StateIcon {
  if (tone === "loading") return Loader2;
  if (tone === "error") return AlertCircle;
  return Inbox;
}

export function StateBlock({
  tone,
  title,
  actionLabel,
  onAction,
  className,
  minHeightClassName = "min-h-[220px]",
  icon,
}: StateBlockProps) {
  const Icon = icon || toneIcon(tone);
  return (
    <div className={cn("panel-state", minHeightClassName, className)}>
      <div
        className={cn(
          "grid h-12 w-12 place-items-center rounded-full bg-surface-3/55",
          tone === "error" && "text-status-danger",
          tone !== "error" && "text-txt-muted",
        )}
      >
        <Icon size={20} strokeWidth={1.8} className={tone === "loading" ? "animate-spin" : undefined} />
      </div>
      <p className={cn("text-[14px] font-medium", tone === "error" ? "text-status-danger" : "text-txt-secondary")}>
        {title}
      </p>
      {actionLabel && onAction ? (
        <Button size="sm" variant={tone === "error" ? "danger" : "ghost"} onClick={onAction}>
          {actionLabel}
        </Button>
      ) : null}
    </div>
  );
}

export type { StateBlockProps, StateTone };
