import { TrendingDown, TrendingUp } from "lucide-react";
import { type ReactElement } from "react";

import { Badge, cn } from "@/src/components/ui";

type MetricTrend = "up" | "down" | "flat";

function trendIcon(trend?: MetricTrend): ReactElement {
  if (trend === "up") {
    return <TrendingUp size={12} strokeWidth={1.4} />;
  }
  if (trend === "down") {
    return <TrendingDown size={12} strokeWidth={1.4} />;
  }
  return <span className="inline-block h-[7px] w-[7px] rounded-full bg-txt-muted" />;
}

export function MetricCard({
  label,
  value,
  caption,
  tone = "primary",
  trend,
}: {
  label: string;
  value: string;
  caption?: string;
  tone?: "primary" | "secondary" | "success" | "warning" | "error";
  trend?: MetricTrend;
}) {
  const trendLabel = trend === "up" ? "Rising" : trend === "down" ? "Dropping" : "Stable";
  const trendVariant = trend === "up" ? "success" : trend === "down" ? "warning" : "default";

  return (
    <div
      className={cn(
        "group h-full rounded-[12px] border bg-surface-2 p-4 transition-colors",
        tone === "primary" && "border-accent/20 hover:border-accent/35",
        tone === "secondary" && "border-accent-secondary/20 hover:border-accent-secondary/35",
        tone === "success" && "border-status-success/20 hover:border-status-success/35",
        tone === "warning" && "border-status-warning/20 hover:border-status-warning/35",
        tone === "error" && "border-status-danger/20 hover:border-status-danger/35",
      )}
    >
      <div className="flex h-full flex-col gap-1.5">
        <p className="text-[10px] font-medium uppercase tracking-wider text-txt-muted">{label}</p>
        <p className="text-[28px] font-bold leading-none tracking-tight text-white">{value}</p>
        <div className="mt-auto flex items-center justify-between gap-2">
          <p className="text-[11px] text-txt-muted">{caption || " "}</p>
          {trend ? (
            <Badge variant={trendVariant} className="flex items-center gap-1 normal-case tracking-normal">
              {trendIcon(trend)}
              {trendLabel}
            </Badge>
          ) : null}
        </div>
      </div>
    </div>
  );
}
