import { TrendingUp } from "lucide-react";
import { useId, useMemo, type ReactNode } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { ErrorBanner } from "@/components/ui/error-banner";
import { StateBlock, cn } from "@/src/components/ui";
import { formatBytes } from "@/utils/format";

import { type HistoryWindow, type TrafficUsageBarPoint } from "./dashboard-types";
import { formatTrafficTick, formatTrafficTooltipLabel } from "./dashboard-utils";

function SectionHeader({ icon, title, children }: { icon: ReactNode; title: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-2.5">
        <div className="grid h-8 w-8 place-items-center rounded-lg bg-surface-3/50 text-txt-tertiary">{icon}</div>
        <h3 className="text-[16px] font-bold text-txt-primary">{title}</h3>
      </div>
      {children}
    </div>
  );
}

type DashboardTrafficProps = {
  historyWindow: HistoryWindow;
  onHistoryWindowChange: (window: HistoryWindow) => void;
  historyError: string;
  onDismissHistoryError: () => void;
  onRetryHistory: () => void;
  showHistorySkeleton: boolean;
  trafficTotals: {
    download: number;
    upload: number;
  };
  trafficUsageBars: TrafficUsageBarPoint[];
};

type TrafficChartPoint = {
  ts: number;
  download: number;
  upload: number;
};

export function DashboardTraffic({
  historyWindow,
  onHistoryWindowChange,
  historyError,
  onDismissHistoryError,
  onRetryHistory,
  showHistorySkeleton,
  trafficTotals,
  trafficUsageBars,
}: DashboardTrafficProps) {
  const gradientIdBase = useId().replace(/[^a-zA-Z0-9_-]/g, "-");
  const downGradientId = `traffic-down-${gradientIdBase}`;
  const upGradientId = `traffic-up-${gradientIdBase}`;

  const chartData = useMemo<TrafficChartPoint[]>(() => {
    const merged = new Map<number, TrafficChartPoint>();

    for (const entry of trafficUsageBars) {
      const ts = entry.timestamp instanceof Date ? entry.timestamp.getTime() : new Date(entry.timestamp).getTime();
      if (!Number.isFinite(ts)) {
        continue;
      }

      const current = merged.get(ts);
      const download = Math.max(0, Number(entry.download_bytes) || 0);
      const upload = Math.max(0, Number(entry.upload_bytes) || 0);

      if (current) {
        current.download += download;
        current.upload += upload;
        continue;
      }

      merged.set(ts, { ts, download, upload });
    }

    return Array.from(merged.values()).sort((a, b) => a.ts - b.ts);
  }, [trafficUsageBars]);

  return (
    <div className="space-y-4">
      <SectionHeader icon={<TrendingUp size={18} strokeWidth={1.6} />} title="Traffic Consumption">
        <div className="inline-flex w-full rounded-xl bg-surface-3/50 p-1 text-[13px] sm:w-auto">
          {(["1h", "24h"] as HistoryWindow[]).map((window) => (
            <button
              key={window}
              type="button"
              onClick={() => onHistoryWindowChange(window)}
              className={cn(
                "flex-1 rounded-lg px-4 py-1.5 font-semibold transition-colors sm:flex-none",
                historyWindow === window ? "bg-surface-4 text-txt-primary shadow-sm" : "text-txt-secondary hover:text-txt",
              )}
            >
              {window}
            </button>
          ))}
        </div>
      </SectionHeader>

      <ErrorBanner
        message={historyError}
        onDismiss={onDismissHistoryError}
        actionLabel="Retry"
        onAction={onRetryHistory}
      />

      <div className="panel-card p-4 sm:p-6">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3 text-[13px]">
          <div className="flex flex-wrap items-center gap-4">
            <span className="inline-flex items-center gap-2 text-txt-secondary">
              <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: "var(--data-1)" }} />
              Download
            </span>
            <span className="inline-flex items-center gap-2 text-txt-secondary">
              <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: "var(--data-2)" }} />
              Upload
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-lg bg-surface-3/35 px-2.5 py-1 text-[12px] font-medium text-txt-secondary">
              Down total: {formatBytes(trafficTotals.download)}
            </span>
            <span className="rounded-lg bg-surface-3/35 px-2.5 py-1 text-[12px] font-medium text-txt-secondary">
              Up total: {formatBytes(trafficTotals.upload)}
            </span>
          </div>
        </div>

        {showHistorySkeleton ? (
          <StateBlock
            tone="loading"
            title="Loading chart"
            minHeightClassName="h-[320px]"
            className="rounded-xl bg-surface-3/28"
          />
        ) : chartData.length === 0 ? (
          <StateBlock tone="empty" title="No data" minHeightClassName="h-[320px]" className="rounded-xl bg-surface-3/28" />
        ) : (
          <div className="h-[320px]">
            <ResponsiveContainer width="100%" height="100%" minHeight={320} debounce={120}>
              <AreaChart
                data={chartData}
                margin={{ top: 12, right: 8, bottom: 0, left: 0 }}
              >
                <defs>
                  <linearGradient id={downGradientId} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="var(--data-1)" stopOpacity={0.3} />
                    <stop offset="100%" stopColor="var(--data-1)" stopOpacity={0.02} />
                  </linearGradient>
                  <linearGradient id={upGradientId} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="var(--data-2)" stopOpacity={0.28} />
                    <stop offset="100%" stopColor="var(--data-2)" stopOpacity={0.02} />
                  </linearGradient>
                </defs>

                <CartesianGrid stroke="var(--border)" strokeDasharray="4 4" vertical={false} />

                <XAxis
                  type="number"
                  dataKey="ts"
                  domain={["dataMin", "dataMax"]}
                  scale="time"
                  tickCount={historyWindow === "24h" ? 8 : 12}
                  minTickGap={22}
                  tickFormatter={(value) => formatTrafficTick(new Date(Number(value)))}
                  tick={{ fill: "var(--txt-icon)", fontSize: 12 }}
                  tickLine={false}
                  axisLine={{ stroke: "var(--border)" }}
                />

                <YAxis
                  type="number"
                  domain={[0, "auto"]}
                  allowDecimals={false}
                  tickFormatter={(value) => formatBytes(Number(value) || 0)}
                  tick={{ fill: "var(--txt-icon)", fontSize: 12 }}
                  tickLine={false}
                  axisLine={false}
                  width={64}
                />

                <Tooltip
                  isAnimationActive={false}
                  formatter={(value) => formatBytes(Number(value) || 0)}
                  labelFormatter={(label) => formatTrafficTooltipLabel(Number(label), historyWindow)}
                  cursor={{ stroke: "var(--border-hover)", strokeDasharray: "4 4" }}
                  contentStyle={{
                    backgroundColor: "var(--surface-2)",
                    border: "1px solid var(--border)",
                    borderRadius: 12,
                    color: "var(--txt-body)",
                    fontSize: 13,
                    backdropFilter: "blur(12px)",
                    boxShadow: "0 8px 32px var(--shell-shadow)",
                  }}
                />

                <Area
                  type="monotone"
                  dataKey="download"
                  name="Download"
                  stroke="var(--data-1)"
                  strokeWidth={2}
                  fill={`url(#${downGradientId})`}
                  dot={false}
                  activeDot={{ r: 3, strokeWidth: 0 }}
                  isAnimationActive={false}
                />

                <Area
                  type="monotone"
                  dataKey="upload"
                  name="Upload"
                  stroke="var(--data-2)"
                  strokeWidth={2}
                  fill={`url(#${upGradientId})`}
                  dot={false}
                  activeDot={{ r: 3, strokeWidth: 0 }}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>
    </div>
  );
}
