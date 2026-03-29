import { TrendingDown, TrendingUp } from "lucide-react";
import { type ReactElement } from "react";

import { Badge, cn } from "@/src/components/ui";

type MetricTrend = "up" | "down" | "flat";

function trendIcon(trend?: MetricTrend): ReactElement {
  if (trend === "up") return <TrendingUp size={14} strokeWidth={1.6} />;
  if (trend === "down") return <TrendingDown size={14} strokeWidth={1.6} />;
  return <span className="inline-block h-2 w-2 rounded-full bg-txt-muted" />;
}

export function MetricCard({ label, value, caption, tone = "primary", trend }: {
  label: string; value: string; caption?: string; tone?: "primary" | "secondary" | "success" | "warning" | "error"; trend?: MetricTrend;
}) {
  const trendLabel = trend === "up" ? "Rising" : trend === "down" ? "Dropping" : "Stable";
  const trendVariant = trend === "up" ? "success" : trend === "down" ? "warning" : "default";

  return (
    <div className={cn(
      "card-hover gradient-border group h-full rounded-2xl border border-border/30 bg-surface-2 p-5 transition-colors",
      tone === "primary" && "shadow-[inset_0_1px_0_var(--shell-highlight)]",
      tone === "secondary" && "shadow-[inset_0_1px_0_var(--shell-highlight)]",
      tone === "success" && "shadow-[inset_0_1px_0_var(--status-success-soft)]",
      tone === "warning" && "shadow-[inset_0_1px_0_var(--status-warning-soft)]",
      tone === "error" && "shadow-[inset_0_1px_0_var(--status-danger-soft)]",
    )}>
      <div className="flex h-full flex-col gap-2">
        <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">{label}</p>
        <p className="text-[32px] font-bold leading-none tracking-tight text-txt-primary">{value}</p>
        <div className="mt-auto flex items-center justify-between gap-2">
          <p className="text-[13px] text-txt-secondary">{caption || " "}</p>
          {trend && (
            <Badge variant={trendVariant} className="flex items-center gap-1 normal-case tracking-normal">
              {trendIcon(trend)}{trendLabel}
            </Badge>
          )}
        </div>
      </div>
    </div>
  );
}
