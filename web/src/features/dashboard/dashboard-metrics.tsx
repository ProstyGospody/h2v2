import { animate, motion, useMotionValue, useReducedMotion, useTransform } from "framer-motion";
import { ArrowDownToLine, ArrowUpFromLine, Clock, Globe, Network, Users2, Zap } from "lucide-react";
import { type ReactNode, useEffect, useRef, useState } from "react";
import {
  Area,
  AreaChart,
  Cell,
  PolarAngleAxis,
  RadialBar,
  RadialBarChart,
  ResponsiveContainer,
} from "recharts";

import { cn } from "@/src/components/ui";
import { formatBytes, formatRate } from "@/utils/format";

import { type SparkPoint } from "./dashboard-types";
import { gaugeColor } from "./dashboard-utils";

function AnimatedNumber({ value, format = (n) => n.toFixed(0) }: { value: number; format?: (value: number) => string }) {
  const mv = useMotionValue(0);
  const display = useTransform(mv, (latest) => format(latest));
  const reduceMotion = useReducedMotion();

  useEffect(() => {
    if (reduceMotion) {
      mv.set(value);
      return;
    }
    const controls = animate(mv, value, { duration: 0.8, ease: "easeOut" });
    return () => controls.stop();
  }, [mv, reduceMotion, value]);

  return <motion.span>{display}</motion.span>;
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
  const reduceMotion = useReducedMotion();
  const clamped = Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));
  const fill = autoColor ? gaugeColor(clamped) : color;
  const data = [{ value: clamped, fill }];

  return (
    <div style={{ width: size, height: size }}>
      <ResponsiveContainer width="100%" height="100%" minWidth={size} minHeight={size} aspect={1}>
        <RadialBarChart
          cx="50%" cy="50%"
          innerRadius="70%" outerRadius="100%"
          startAngle={90} endAngle={-270}
          barSize={5}
          data={data}
        >
          <PolarAngleAxis type="number" domain={[0, 100]} angleAxisId={0} tick={false} />
          <RadialBar
            dataKey="value"
            angleAxisId={0}
            background={{ fill: trackColor }}
            cornerRadius={10}
            isAnimationActive={!reduceMotion}
            animationDuration={reduceMotion ? 0 : 800}
            animationEasing="ease-out"
          >
            <Cell fill={fill} />
          </RadialBar>
        </RadialBarChart>
      </ResponsiveContainer>
    </div>
  );
}

function MiniSparkline({
  data,
  color,
  gradientId,
}: {
  data: SparkPoint[];
  color: string;
  gradientId: string;
}) {
  if (!data.length) {
    return <div className="h-6 w-20 rounded-md bg-surface-3/45" />;
  }

  return (
    <AreaChart width={80} height={24} data={data}>
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity={0.42} />
          <stop offset="100%" stopColor={color} stopOpacity={0.05} />
        </linearGradient>
      </defs>
      <Area
        type="monotone"
        dataKey="value"
        stroke={color}
        strokeWidth={1.5}
        fill={`url(#${gradientId})`}
        isAnimationActive={false}
      />
    </AreaChart>
  );
}

function MetricsCarousel({ children }: { children: ReactNode }) {
  const items = Array.isArray(children) ? children : [children];
  const [currentIndex, setCurrentIndex] = useState(0);
  const constraintsRef = useRef<HTMLDivElement>(null);
  const reduceMotion = useReducedMotion();

  return (
    <>
      <div className="hidden gap-4 sm:grid sm:grid-cols-2 xl:grid-cols-4">
        {items}
      </div>

      <div className="sm:hidden">
        <div ref={constraintsRef} className="overflow-hidden">
          <motion.div
            className="flex gap-3"
            drag="x"
            dragConstraints={constraintsRef}
            dragElastic={reduceMotion ? 0 : 0.2}
            onDragEnd={(_event, info) => {
              const threshold = 60;
              if (info.offset.x < -threshold && currentIndex < items.length - 1) {
                setCurrentIndex((index) => index + 1);
              } else if (info.offset.x > threshold && currentIndex > 0) {
                setCurrentIndex((index) => index - 1);
              }
            }}
            animate={{ x: `-${currentIndex * 100}%` }}
            transition={reduceMotion ? { duration: 0 } : { type: "spring", stiffness: 300, damping: 30 }}
          >
            {items.map((item, index) => (
              <div key={index} className="w-full shrink-0">
                {item}
              </div>
            ))}
          </motion.div>
        </div>

        <div className="mt-3 flex items-center justify-center gap-1.5">
          {items.map((_, index) => (
            <button
              key={index}
              type="button"
              aria-label={`Show metric ${index + 1}`}
              onClick={() => setCurrentIndex(index)}
              className={cn(
                "h-1.5 rounded-full transition-all duration-200",
                index === currentIndex ? "w-6 bg-accent" : "w-1.5 bg-border-hover",
              )}
            />
          ))}
        </div>
      </div>
    </>
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
  networkSparkline: SparkPoint[];
  trafficSparkline: SparkPoint[];
  connectionsSparkline: SparkPoint[];
};

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
  networkSparkline,
  trafficSparkline,
  connectionsSparkline,
}: DashboardMetricsProps) {
  if (showInitialLoading) {
    return (
      <>
        <MetricsCarousel>
          {Array.from({ length: 4 }, (_, index) => (
            <div key={`metric-skeleton-${index}`} className="panel-card min-h-[108px] animate-pulse">
              <div className="flex h-full items-center justify-between gap-4">
                <div className="space-y-3">
                  <div className="h-3 w-12 rounded bg-surface-3/60" />
                  <div className="h-7 w-16 rounded bg-surface-3/60" />
                </div>
                <div className="h-14 w-14 rounded-full bg-surface-3/60" />
              </div>
            </div>
          ))}
        </MetricsCarousel>

        <div className="grid gap-4 sm:grid-cols-3">
          {Array.from({ length: 3 }, (_, index) => (
            <div key={`secondary-skeleton-${index}`} className="panel-card min-h-[102px] animate-pulse">
              <div className="flex h-full items-center gap-4">
                <div className="h-12 w-12 rounded-xl bg-surface-3/55" />
                <div className="min-w-0 flex-1 space-y-2.5">
                  <div className="h-3 w-20 rounded bg-surface-3/55" />
                  <div className="h-4 w-28 rounded bg-surface-3/55" />
                </div>
                <div className="h-6 w-16 rounded bg-surface-3/55" />
              </div>
            </div>
          ))}
        </div>
      </>
    );
  }

  return (
    <>
      <MetricsCarousel>
        <div className="card-hover panel-card min-h-[108px]">
          <div className="flex h-full items-center justify-between gap-4">
            <div className="min-w-0">
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">CPU</p>
              <p className="mt-1.5 text-metric text-txt-primary">
                <AnimatedNumber value={cpuPercent} />
                <span className="ml-1 text-[16px] font-medium text-txt-tertiary">%</span>
              </p>
            </div>
            <div className="shrink-0">
              <RadialGauge value={cpuPercent} size={56} autoColor />
            </div>
          </div>
        </div>

        <div className="card-hover panel-card min-h-[108px]">
          <div className="flex h-full items-center justify-between gap-4">
            <div className="min-w-0">
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">RAM</p>
              <p className="mt-1.5 text-metric text-txt-primary">
                <AnimatedNumber value={ramPercent} />
                <span className="ml-1 text-[16px] font-medium text-txt-tertiary">%</span>
              </p>
            </div>
            <div className="shrink-0">
              <RadialGauge value={ramPercent} size={56} autoColor />
            </div>
          </div>
        </div>

        <div className="card-hover panel-card min-h-[108px]">
          <div className="flex h-full items-center justify-between gap-4">
            <div className="min-w-0">
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Online</p>
              <p className="mt-1.5 text-metric text-txt-primary"><AnimatedNumber value={onlineUsers} /></p>
            </div>
            <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-surface-3/35">
              <Users2 size={20} strokeWidth={1.7} className="text-txt-secondary" />
            </div>
          </div>
        </div>

        <div className="card-hover panel-card min-h-[108px]">
          <div className="flex h-full items-center justify-between gap-4">
            <div className="min-w-0">
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Uptime</p>
              <p className="mt-1.5 text-[28px] leading-none text-txt-primary sm:text-metric">{uptime}</p>
            </div>
            <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-surface-3/35">
              <Clock size={20} strokeWidth={1.7} className="text-txt-secondary" />
            </div>
          </div>
        </div>
      </MetricsCarousel>

      <div className="grid gap-4 sm:grid-cols-3">
        <div className="card-hover panel-card flex min-h-[102px] items-center gap-4">
          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-surface-3/35">
            <Network size={22} strokeWidth={1.6} className="text-txt-secondary" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Network</p>
            <div className="mt-1.5 flex flex-wrap items-center gap-3 text-[15px] font-semibold text-txt-primary">
              <span className="inline-flex items-center gap-1.5"><ArrowDownToLine size={14} strokeWidth={1.8} className="text-status-success" /><AnimatedNumber value={networkRx} format={formatRate} /></span>
              <span className="inline-flex items-center gap-1.5"><ArrowUpFromLine size={14} strokeWidth={1.8} className="text-status-warning" /><AnimatedNumber value={networkTx} format={formatRate} /></span>
            </div>
          </div>
          <div className="shrink-0"><MiniSparkline data={networkSparkline} color="var(--data-2)" gradientId="spark-network" /></div>
        </div>

        <div className="card-hover panel-card flex min-h-[102px] items-center gap-4">
          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-surface-3/35">
            <Globe size={22} strokeWidth={1.6} className="text-txt-secondary" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Total Traffic</p>
            <p className="mt-1.5 text-[15px] font-semibold text-txt-primary"><AnimatedNumber value={totalTraffic} format={formatBytes} /></p>
          </div>
          <div className="shrink-0"><MiniSparkline data={trafficSparkline} color="var(--data-1)" gradientId="spark-traffic" /></div>
        </div>

        <div className="card-hover panel-card flex min-h-[102px] items-center gap-4">
          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-surface-3/35">
            <Zap size={22} strokeWidth={1.6} className="text-txt-secondary" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Connections</p>
            <div className="mt-1.5 flex items-center gap-3 text-[15px] font-semibold text-txt-primary">
              <span>TCP <AnimatedNumber value={tcpConnections} /></span>
              <span className="text-txt-muted">/</span>
              <span>UDP <AnimatedNumber value={udpConnections} /></span>
            </div>
          </div>
          <div className="shrink-0"><MiniSparkline data={connectionsSparkline} color="var(--status-success)" gradientId="spark-connections" /></div>
        </div>
      </div>
    </>
  );
}
