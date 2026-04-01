import { TrendingUp } from "lucide-react";
import { memo, useMemo, type ReactNode } from "react";
import {
  Bar,
  BarChart,
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
  trafficTotal: number;
  trafficUsageBars: TrafficUsageBarPoint[];
};

type TrafficChartPoint = {
  bucketKey: string;
  ts: number;
  total: number;
};

const WINDOW_TABS: Array<{ value: HistoryWindow; label: string }> = [
  { value: "1h", label: "1h" },
  { value: "24h", label: "24h" },
  { value: "7d", label: "7d" },
];

const CHART_RESIZE_DEBOUNCE_MS = 380;

function chartCategoryGap(window: HistoryWindow): number {
  if (window === "1h") return 12;
  if (window === "24h") return 10;
  return 18;
}

function DashboardTrafficComponent({
  historyWindow,
  onHistoryWindowChange,
  historyError,
  onDismissHistoryError,
  onRetryHistory,
  showHistorySkeleton,
  trafficTotal,
  trafficUsageBars,
}: DashboardTrafficProps) {
  const chartData = useMemo<TrafficChartPoint[]>(() => {
    const byTimestamp = new Map<number, number>();

    for (const entry of trafficUsageBars) {
      const ts = entry.timestamp instanceof Date ? entry.timestamp.getTime() : new Date(entry.timestamp).getTime();
      if (!Number.isFinite(ts)) {
        continue;
      }

      const value = Math.max(0, Number(entry.download_bytes) || 0) + Math.max(0, Number(entry.upload_bytes) || 0);
      byTimestamp.set(ts, (byTimestamp.get(ts) || 0) + value);
    }

    return Array.from(byTimestamp.entries())
      .sort((a, b) => a[0] - b[0])
      .map(([ts, total]) => ({ bucketKey: String(ts), ts, total }));
  }, [trafficUsageBars]);

  const xTickMinGap = historyWindow === "7d" ? 26 : 14;

  return (
    <div className="space-y-4">
      <SectionHeader icon={<TrendingUp size={18} strokeWidth={1.6} />} title="Traffic Consumption">
        <div className="inline-flex w-full rounded-xl bg-surface-3/50 p-1 text-[13px] sm:w-auto">
          {WINDOW_TABS.map((window) => (
            <button
              key={window.value}
              type="button"
              onClick={() => onHistoryWindowChange(window.value)}
              className={cn(
                "flex-1 rounded-lg px-4 py-1.5 font-semibold transition-colors sm:flex-none",
                historyWindow === window.value ? "bg-surface-4 text-txt-primary shadow-sm" : "text-txt-secondary hover:text-txt",
              )}
            >
              {window.label}
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
        <div className="mb-4 flex flex-wrap items-center justify-end gap-3 text-[13px]">
          <span className="rounded-lg bg-surface-3/35 px-2.5 py-1 text-[12px] font-medium text-txt-secondary">
            Total: {formatBytes(trafficTotal)}
          </span>
        </div>

        {showHistorySkeleton ? (
          <StateBlock tone="loading" title="Loading chart" minHeightClassName="h-[320px]" className="rounded-xl bg-surface-3/28" />
        ) : chartData.length === 0 ? (
          <StateBlock tone="empty" title="No data" minHeightClassName="h-[320px]" className="rounded-xl bg-surface-3/28" />
        ) : (
          <div className="h-[320px] overflow-hidden">
            <ResponsiveContainer width="100%" height="100%" minHeight={320} debounce={CHART_RESIZE_DEBOUNCE_MS}>
              <BarChart
                data={chartData}
                margin={{ top: 8, right: 8, bottom: 0, left: 0 }}
                barGap={0}
                barCategoryGap={chartCategoryGap(historyWindow)}
              >
                <CartesianGrid stroke="var(--border)" strokeDasharray="4 4" vertical={false} />

                <XAxis
                  dataKey="bucketKey"
                  type="category"
                  interval="preserveStartEnd"
                  minTickGap={xTickMinGap}
                  padding={{ left: 10, right: 10 }}
                  tickFormatter={(value) => formatTrafficTick(new Date(Number(value)), historyWindow)}
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
                  cursor={{ fill: "var(--surface-3)", opacity: 0.18 }}
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

                <Bar
                  dataKey="total"
                  name="Total"
                  fill="var(--data-1)"
                  radius={[4, 4, 0, 0]}
                  minPointSize={2}
                  isAnimationActive={false}
                />
              </BarChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>
    </div>
  );
}

export const DashboardTraffic = memo(DashboardTrafficComponent);
