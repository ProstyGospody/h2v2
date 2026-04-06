import { ArrowDownToLine, ArrowUpFromLine, Clock, Globe, Network, Users2, Zap } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Cell, PolarAngleAxis, RadialBar, RadialBarChart } from "recharts";

import { formatBytes, formatRate } from "@/utils/format";

import { gaugeColor } from "./dashboard-utils";

function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return false;
    return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  });

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    const handler = (event: MediaQueryListEvent) => setReduced(event.matches);
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, []);

  return reduced;
}

const NUMBER_TWEEN_MS = 800;

function useTweenedNumber(target: number): number {
  const reduced = usePrefersReducedMotion();
  const [display, setDisplay] = useState(target);
  const fromRef = useRef(target);

  useEffect(() => {
    if (!Number.isFinite(target)) {
      return;
    }
    if (reduced) {
      fromRef.current = target;
      setDisplay(target);
      return;
    }
    const start = fromRef.current;
    const end = target;
    if (start === end) {
      return;
    }
    const startedAt = performance.now();
    let rafId = 0;
    const tick = (now: number) => {
      const progress = Math.min(1, (now - startedAt) / NUMBER_TWEEN_MS);
      const eased = 1 - Math.pow(1 - progress, 3);
      const current = start + (end - start) * eased;
      setDisplay(current);
      if (progress < 1) {
        rafId = requestAnimationFrame(tick);
      } else {
        fromRef.current = end;
      }
    };
    rafId = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(rafId);
  }, [target, reduced]);

  return display;
}

function AnimatedNumber({ value, format = (n) => n.toFixed(0) }: { value: number; format?: (value: number) => string }) {
  const current = useTweenedNumber(value);
  return <span>{format(current)}</span>;
}

function RadialGauge({
  value,
  size = 56,
  autoColor = false,
  color = "var(--accent)",
  trackColor = "var(--border-hover)",
}: {
  value: number;
  size?: number;
  autoColor?: boolean;
  color?: string;
  trackColor?: string;
}) {
  const reduced = usePrefersReducedMotion();
  const clamped = Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));
  const fill = autoColor ? gaugeColor(clamped) : color;
  const data = useMemo(() => [{ value: clamped, fill }], [clamped, fill]);

  return (
    <RadialBarChart
      width={size}
      height={size}
      cx="50%"
      cy="50%"
      innerRadius="70%"
      outerRadius="100%"
      startAngle={90}
      endAngle={-270}
      barSize={5}
      data={data}
    >
      <PolarAngleAxis type="number" domain={[0, 100]} angleAxisId={0} tick={false} />
      <RadialBar
        dataKey="value"
        angleAxisId={0}
        background={{ fill: trackColor }}
        cornerRadius={10}
        isAnimationActive={!reduced}
        animationDuration={reduced ? 0 : 800}
        animationEasing="ease-out"
      >
        <Cell fill={fill} />
      </RadialBar>
    </RadialBarChart>
  );
}

type DashboardMetricsProps = {
  showInitialLoading: boolean;
  cpuPercent: number;
  ramPercent: number;
  onlineUsers: number;
  uptime: string;
  networkRx: number;
  networkTx: number;
  totalTraffic: number;
  tcpConnections: number;
  udpConnections: number;
};

const GRID_CLASS = "grid grid-cols-1 gap-4 lg:grid-cols-7";

function MetricsSkeletonCard() {
  return (
    <div className="panel-card min-h-[96px] py-4 animate-pulse">
      <div className="flex h-full items-center justify-between gap-3">
        <div className="w-full min-w-0 space-y-2.5">
          <div className="h-3 w-16 rounded bg-surface-3/55" />
          <div className="h-8 w-20 rounded bg-surface-3/60" />
        </div>
        <div className="h-11 w-11 shrink-0 rounded-xl bg-surface-3/55" />
      </div>
    </div>
  );
}

export function DashboardMetrics({
  showInitialLoading,
  cpuPercent,
  ramPercent,
  onlineUsers,
  uptime,
  networkRx,
  networkTx,
  totalTraffic,
  tcpConnections,
  udpConnections,
}: DashboardMetricsProps) {
  if (showInitialLoading) {
    return (
      <div className={GRID_CLASS}>
        {Array.from({ length: 7 }, (_, index) => <MetricsSkeletonCard key={`metric-skeleton-${index}`} />)}
      </div>
    );
  }

  return (
    <div className={GRID_CLASS}>
      <div className="card-hover panel-card min-h-[96px] py-4">
        <div className="flex h-full items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[14px] font-semibold uppercase tracking-wider text-txt-muted">CPU</p>
            <p className="mt-1.5 text-[32px] leading-none text-txt-primary tabular-nums">
              <AnimatedNumber value={cpuPercent} />
              <span className="ml-1 text-[18px] font-medium text-txt-tertiary">%</span>
            </p>
          </div>
          <div className="shrink-0">
            <RadialGauge value={cpuPercent} size={56} autoColor />
          </div>
        </div>
      </div>

      <div className="card-hover panel-card min-h-[96px] py-4">
        <div className="flex h-full items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[14px] font-semibold uppercase tracking-wider text-txt-muted">RAM</p>
            <p className="mt-1.5 text-[32px] leading-none text-txt-primary tabular-nums">
              <AnimatedNumber value={ramPercent} />
              <span className="ml-1 text-[18px] font-medium text-txt-tertiary">%</span>
            </p>
          </div>
          <div className="shrink-0">
            <RadialGauge value={ramPercent} size={56} autoColor />
          </div>
        </div>
      </div>

      <div className="card-hover panel-card min-h-[96px] py-4">
        <div className="flex h-full items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[14px] font-semibold uppercase tracking-wider text-txt-muted">Online</p>
            <p className="mt-1.5 text-[32px] leading-none text-txt-primary tabular-nums">
              <AnimatedNumber value={onlineUsers} />
            </p>
          </div>
          <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-surface-3/35">
            <Users2 size={19} strokeWidth={1.7} className="text-txt-secondary" />
          </div>
        </div>
      </div>

      <div className="card-hover panel-card min-h-[96px] py-4">
        <div className="flex h-full items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[14px] font-semibold uppercase tracking-wider text-txt-muted">Uptime</p>
            <p className="mt-1.5 whitespace-nowrap text-[32px] leading-none text-txt-primary tabular-nums">{uptime}</p>
          </div>
          <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-surface-3/35">
            <Clock size={19} strokeWidth={1.7} className="text-txt-secondary" />
          </div>
        </div>
      </div>

      <div className="card-hover panel-card min-h-[96px] py-4">
        <div className="flex h-full items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[14px] font-semibold uppercase tracking-wider text-txt-muted">Network</p>
            <div className="mt-1.5 space-y-1 text-[16px] font-semibold text-txt-primary tabular-nums">
              <p className="inline-flex items-center gap-1.5 whitespace-nowrap">
                <ArrowDownToLine size={13} strokeWidth={1.8} className="text-status-success" />
                <AnimatedNumber value={networkRx} format={formatRate} />
              </p>
              <p className="inline-flex items-center gap-1.5 whitespace-nowrap">
                <ArrowUpFromLine size={13} strokeWidth={1.8} className="text-status-warning" />
                <AnimatedNumber value={networkTx} format={formatRate} />
              </p>
            </div>
          </div>
          <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-surface-3/35">
            <Network size={19} strokeWidth={1.6} className="text-txt-secondary" />
          </div>
        </div>
      </div>

      <div className="card-hover panel-card min-h-[96px] py-4">
        <div className="flex h-full items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[14px] font-semibold uppercase tracking-wider text-txt-muted">Total Traffic</p>
            <p className="mt-1.5 text-[24px] font-semibold leading-none text-txt-primary tabular-nums">
              <AnimatedNumber value={totalTraffic} format={formatBytes} />
            </p>
          </div>
          <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-surface-3/35">
            <Globe size={19} strokeWidth={1.6} className="text-txt-secondary" />
          </div>
        </div>
      </div>

      <div className="card-hover panel-card min-h-[96px] py-4">
        <div className="flex h-full items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[14px] font-semibold uppercase tracking-wider text-txt-muted">Connections</p>
            <p className="mt-1.5 text-[16px] font-semibold text-txt-primary tabular-nums">
              TCP <AnimatedNumber value={tcpConnections} />
              <span className="mx-1 text-txt-muted">/</span>
              UDP <AnimatedNumber value={udpConnections} />
            </p>
          </div>
          <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-surface-3/35">
            <Zap size={19} strokeWidth={1.6} className="text-txt-secondary" />
          </div>
        </div>
      </div>
    </div>
  );
}
