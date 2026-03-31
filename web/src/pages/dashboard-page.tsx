import { useQuery, useQueryClient } from "@tanstack/react-query";
import { animate, motion, useMotionValue, useTransform } from "framer-motion";
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Clock,
  Cpu,
  Eye,
  Globe,
  Network,
  RefreshCw,
  RotateCcw,
  TrendingUp,
  Users2,
  Zap,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import {
  Area,
  AreaChart,
  CartesianGrid,
  Cell,
  PolarAngleAxis,
  RadialBar,
  RadialBarChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import { apiFetch, getAPIErrorMessage } from "@/services/api";
import { queryRefetchInterval } from "@/src/queries/polling";
import { ServiceDetails, ServiceSummary, SystemHistoryResponse, SystemLiveResponse } from "@/types/common";
import { Button, Dialog, StateBlock, cn } from "@/src/components/ui";
import { formatBytes, formatDateTime, formatRate, formatUptime } from "@/utils/format";

const LIVE_POLL_MS = 5000;
const HISTORY_POLL_MS = 15000;
const HISTORY_LIMIT_1H = 900;
const HISTORY_LIMIT_24H = 3200;
const TRAFFIC_BUCKET_MS_1H = 5 * 60 * 1000;
const TRAFFIC_BUCKET_MS_24H = 60 * 60 * 1000;

type HistoryWindow = "1h" | "24h";

type HistoryTrendPoint = {
  timestamp: Date;
  download: number;
  upload: number;
  connections: number;
  cpu: number;
  ram: number;
};

type TrafficUsageBarPoint = {
  timestamp: Date;
  download_bytes: number;
  upload_bytes: number;
};

type SparkPoint = {
  idx: number;
  value: number;
};

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, value));
}

function formatTooltipDate(value: unknown): string {
  const date = value instanceof Date ? value : new Date(String(value));
  if (Number.isNaN(date.getTime())) return "--";
  return `${date.toLocaleDateString([], { day: "2-digit", month: "short", year: "numeric" })} ${date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
}

function formatTrafficTick(value: Date, window: HistoryWindow): string {
  if (!(value instanceof Date) || Number.isNaN(value.getTime())) return "--:--";
  if (window === "24h") {
    return `${value.toLocaleDateString([], { day: "2-digit", month: "short" })} ${value.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
  }
  return value.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function formatTrafficTooltipLabel(value: unknown, window: HistoryWindow): string {
  const date = value instanceof Date ? value : new Date(String(value));
  if (Number.isNaN(date.getTime())) return "--";
  if (window === "24h") {
    return `${date.toLocaleDateString([], { day: "2-digit", month: "short" })} ${date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
  }
  return formatTooltipDate(date);
}

function AnimatedNumber({ value, format = (n) => n.toFixed(0) }: { value: number; format?: (v: number) => string }) {
  const mv = useMotionValue(0);
  const display = useTransform(mv, (latest) => format(latest));
  useEffect(() => {
    const c = animate(mv, value, { duration: 0.8, ease: "easeOut" });
    return () => c.stop();
  }, [mv, value]);
  return <motion.span>{display}</motion.span>;
}

function gaugeColor(percent: number): string {
  if (percent >= 85) return "var(--status-danger)";
  if (percent >= 60) return "var(--status-warning)";
  return "var(--status-success)";
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
  const clamped = clampPercent(value);
  const fill = autoColor ? gaugeColor(clamped) : color;
  const data = [{ value: clamped, fill }];
  return (
    <div style={{ width: size, height: size }}>
      <ResponsiveContainer width="100%" height="100%">
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
            isAnimationActive
            animationDuration={800}
            animationEasing="ease-out"
          >
            <Cell fill={fill} />
          </RadialBar>
        </RadialBarChart>
      </ResponsiveContainer>
    </div>
  );
}

function statusColor(status: string): string {
  const n = status.toLowerCase();
  if (n.includes("running") || n.includes("active")) return "bg-status-success shadow-[0_0_8px_var(--status-success-soft)]";
  if (n.includes("failed") || n.includes("error")) return "bg-status-danger";
  if (n.includes("inactive") || n.includes("stopped")) return "bg-status-warning";
  return "bg-txt-muted";
}

function SectionHeader({ icon, title, children }: { icon: ReactNode; title: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-2.5">
        <div className="grid h-8 w-8 place-items-center rounded-lg bg-surface-3/50 text-txt-tertiary">
          {icon}
        </div>
        <h3 className="text-[16px] font-bold text-txt-primary">{title}</h3>
      </div>
      {children}
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

function MetricsCarousel({ children }: { children: React.ReactNode }) {
  const items = Array.isArray(children) ? children : [children];
  const [currentIndex, setCurrentIndex] = useState(0);
  const constraintsRef = useRef<HTMLDivElement>(null);

  return (
    <>
      {/* Desktop: grid */}
      <div className="hidden gap-4 sm:grid sm:grid-cols-2 xl:grid-cols-4">
        {items}
      </div>
      {/* Mobile: swipeable carousel */}
      <div className="sm:hidden">
        <div ref={constraintsRef} className="overflow-hidden">
          <motion.div
            className="flex gap-3"
            drag="x"
            dragConstraints={constraintsRef}
            dragElastic={0.2}
            onDragEnd={(_e, info) => {
              const threshold = 60;
              if (info.offset.x < -threshold && currentIndex < items.length - 1) {
                setCurrentIndex((i) => i + 1);
              } else if (info.offset.x > threshold && currentIndex > 0) {
                setCurrentIndex((i) => i - 1);
              }
            }}
            animate={{ x: `-${currentIndex * 100}%` }}
            transition={{ type: "spring", stiffness: 300, damping: 30 }}
          >
            {items.map((item, i) => (
              <div key={i} className="w-full shrink-0">
                {item}
              </div>
            ))}
          </motion.div>
        </div>
        {/* Dots */}
        <div className="mt-3 flex items-center justify-center gap-1.5">
          {items.map((_, i) => (
            <button
              key={i}
              type="button"
              onClick={() => setCurrentIndex(i)}
              className={cn(
                "h-1.5 rounded-full transition-all duration-200",
                i === currentIndex ? "w-6 bg-accent" : "w-1.5 bg-border-hover",
              )}
            />
          ))}
        </div>
      </div>
    </>
  );
}

const tooltipStyle = {
  backgroundColor: "var(--surface-2)",
  border: "1px solid var(--border)",
  borderRadius: 12,
  color: "var(--txt-body)",
  fontSize: 13,
  backdropFilter: "blur(12px)",
  boxShadow: "0 8px 32px var(--shell-shadow)",
};

export default function DashboardPage() {
  const [historyWindow, setHistoryWindow] = useState<HistoryWindow>("1h");
  const [servicesBusy, setServicesBusy] = useState(false);
  const [servicesActionError, setServicesActionError] = useState("");
  const [dismissedLiveError, setDismissedLiveError] = useState(false);
  const [dismissedHistoryError, setDismissedHistoryError] = useState(false);
  const [dismissedServicesError, setDismissedServicesError] = useState(false);
  const [serviceDetails, setServiceDetails] = useState<ServiceDetails | null>(null);
  const [serviceDetailsOpen, setServiceDetailsOpen] = useState(false);
  const queryClient = useQueryClient();
  const liveQuery = useQuery({
    queryKey: ["dashboard", "live"],
    queryFn: ({ signal }) => apiFetch<SystemLiveResponse>("/api/system/live", { method: "GET", signal }),
    staleTime: 3_000,
    refetchOnWindowFocus: true,
    refetchInterval: (query) => queryRefetchInterval(LIVE_POLL_MS, query),
  });
  const servicesQuery = useQuery({
    queryKey: ["dashboard", "services"],
    queryFn: ({ signal }) => apiFetch<{ items: ServiceSummary[] }>("/api/services", { method: "GET", signal }),
    staleTime: 10_000,
    refetchOnWindowFocus: true,
    refetchInterval: (query) => queryRefetchInterval(15_000, query),
  });
  const historyQuery = useQuery({
    queryKey: ["dashboard", "history", historyWindow],
    queryFn: ({ signal }) => {
      const step = historyWindow === "24h" ? 60 : 5;
      const limit = historyWindow === "24h" ? HISTORY_LIMIT_24H : HISTORY_LIMIT_1H;
      return apiFetch<SystemHistoryResponse>(`/api/system/history?window=${historyWindow}&step=${step}&limit=${limit}`, { method: "GET", signal });
    },
    staleTime: 10_000,
    refetchOnWindowFocus: true,
    refetchInterval: (query) => queryRefetchInterval(HISTORY_POLL_MS, query),
  });

  useEffect(() => {
    if (liveQuery.isSuccess) {
      setDismissedLiveError(false);
    }
  }, [liveQuery.dataUpdatedAt, liveQuery.isSuccess]);

  useEffect(() => {
    if (historyQuery.isSuccess) {
      setDismissedHistoryError(false);
    }
  }, [historyQuery.dataUpdatedAt, historyQuery.isSuccess]);

  useEffect(() => {
    if (servicesQuery.isSuccess) {
      setDismissedServicesError(false);
    }
  }, [servicesQuery.dataUpdatedAt, servicesQuery.isSuccess]);

  const live = liveQuery.data || null;
  const historyItems = historyQuery.data?.items || [];
  const serviceItems = servicesQuery.data?.items || [];
  const loading = liveQuery.isPending;
  const historyLoading = historyQuery.isPending;
  const servicesLoading = servicesQuery.isPending;
  const liveError = dismissedLiveError ? "" : (liveQuery.error ? getAPIErrorMessage(liveQuery.error, "Failed to load dashboard data") : "");
  const historyError = dismissedHistoryError ? "" : (historyQuery.error ? getAPIErrorMessage(historyQuery.error, "Failed to load history") : "");
  const servicesQueryError =
    dismissedServicesError ? "" : (servicesQuery.error ? getAPIErrorMessage(servicesQuery.error, "Failed to load services") : "");
  const servicesError = servicesActionError || servicesQueryError;

  const retryLive = useCallback(() => {
    setDismissedLiveError(false);
    void liveQuery.refetch();
  }, [liveQuery]);
  const retryHistory = useCallback(() => {
    setDismissedHistoryError(false);
    void historyQuery.refetch();
  }, [historyQuery]);
  const retryServices = useCallback(() => {
    setDismissedServicesError(false);
    setServicesActionError("");
    void servicesQuery.refetch();
  }, [servicesQuery]);
  const retryAll = useCallback(() => {
    retryLive();
    retryServices();
    retryHistory();
  }, [retryHistory, retryLive, retryServices]);

  const warningMessages = useMemo(() => live?.errors || [], [live]);

  async function openServiceDetails(name: string) {
    setServicesBusy(true);
    setServicesActionError("");
    try {
      setServiceDetails(await apiFetch<ServiceDetails>(`/api/services/${name}?lines=60`, { method: "GET" }));
      setServiceDetailsOpen(true);
    } catch (err) {
      setServicesActionError(getAPIErrorMessage(err, "Failed to load service details"));
    } finally {
      setServicesBusy(false);
    }
  }

  async function runServiceAction(name: string, action: "restart" | "reload") {
    setServicesBusy(true);
    setServicesActionError("");
    try {
      await apiFetch<{ ok: boolean }>(`/api/services/${name}/${action}`, { method: "POST", body: JSON.stringify({}) });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["dashboard", "services"] }),
        queryClient.invalidateQueries({ queryKey: ["dashboard", "live"] }),
      ]);
    } catch (err) {
      setServicesActionError(getAPIErrorMessage(err, "Failed to run action"));
    } finally {
      setServicesBusy(false);
    }
  }

  const showInitialLoading = loading && !live;
  const cpuPercent = clampPercent(live?.system.cpu_usage_percent ?? 0);
  const ramPercent = clampPercent(live?.system.memory_used_percent ?? 0);
  const onlineUsers = Math.max(0, live?.hysteria.online_count ?? 0);
  const networkRx = Math.max(0, live?.system.network_rx_bps ?? 0);
  const networkTx = Math.max(0, live?.system.network_tx_bps ?? 0);
  const uptime = formatUptime(live?.system.uptime_seconds ?? 0);
  const totalTraffic = Math.max(0, (live?.hysteria.total_rx_bytes ?? 0) + (live?.hysteria.total_tx_bytes ?? 0));
  const tcpConnections = Math.max(0, Math.round(live?.system.tcp_sockets ?? 0));
  const udpConnections = Math.max(0, Math.round(live?.system.udp_sockets ?? 0));

  const historyPoints = useMemo<HistoryTrendPoint[]>(() => {
    return historyItems
      .map((s) => {
        const t = new Date(s.timestamp);
        if (Number.isNaN(t.getTime())) return null;
        const tcp = Number(s.tcp_sockets || 0);
        const udp = Number(s.udp_sockets || 0);
        return {
          timestamp: t,
          download: Math.max(0, s.network_rx_bps || 0),
          upload: Math.max(0, s.network_tx_bps || 0),
          connections: Math.max(0, (Number.isFinite(tcp) ? tcp : 0) + (Number.isFinite(udp) ? udp : 0)),
          cpu: clampPercent(Number(s.cpu_usage_percent || 0)),
          ram: clampPercent(Number(s.memory_used_percent || 0)),
        };
      })
      .filter((x): x is HistoryTrendPoint => Boolean(x))
      .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
  }, [historyItems]);

  const networkSparkline = useMemo<SparkPoint[]>(() => {
    return historyPoints.map((pt, idx) => ({ idx, value: pt.download + pt.upload })).slice(-24);
  }, [historyPoints]);

  const trafficSparkline = useMemo<SparkPoint[]>(() => {
    if (!historyPoints.length) return [];
    const points: SparkPoint[] = [];
    let total = 0;
    for (let idx = 0; idx < historyPoints.length; idx++) {
      const current = historyPoints[idx];
      const prevMs = idx > 0 ? historyPoints[idx - 1].timestamp.getTime() : current.timestamp.getTime() - 5_000;
      const dt = Math.max(1, (current.timestamp.getTime() - prevMs) / 1000);
      total += (current.download + current.upload) * dt;
      points.push({ idx, value: total });
    }
    return points.slice(-24);
  }, [historyPoints]);

  const connectionsSparkline = useMemo<SparkPoint[]>(() => {
    const hasConnectionHistory = historyPoints.some((pt) => pt.connections > 0);
    if (hasConnectionHistory) {
      return historyPoints.map((pt, idx) => ({ idx, value: pt.connections })).slice(-24);
    }
    const fallback = Math.max(0, tcpConnections + udpConnections);
    return Array.from({ length: 12 }, (_unused, idx) => ({ idx, value: fallback }));
  }, [historyPoints, tcpConnections, udpConnections]);

  const trafficUsageBars = useMemo<TrafficUsageBarPoint[]>(() => {
    const bucketMs = historyWindow === "24h" ? TRAFFIC_BUCKET_MS_24H : TRAFFIC_BUCKET_MS_1H;
    const bucketCount = historyWindow === "24h" ? 24 : 12;
    const nowMs = Date.now();
    const endBucketMs = Math.floor(nowMs / bucketMs) * bucketMs;
    const startBucketMs = endBucketMs - (bucketCount - 1) * bucketMs;

    const buckets: TrafficUsageBarPoint[] = Array.from({ length: bucketCount }, (_unused, idx) => ({
      timestamp: new Date(startBucketMs + idx * bucketMs),
      download_bytes: 0,
      upload_bytes: 0,
    }));
    const bucketIndex = new Map<number, number>();
    for (let idx = 0; idx < buckets.length; idx++) {
      bucketIndex.set(buckets[idx].timestamp.getTime(), idx);
    }

    if (!historyPoints.length) {
      return buckets;
    }

    for (let i = 0; i < historyPoints.length; i++) {
      const pt = historyPoints[i];
      const ms = pt.timestamp.getTime();
      const prevMs = i > 0 ? historyPoints[i - 1].timestamp.getTime() : ms - 5_000;
      const dt = Math.max(1, (ms - prevMs) / 1000);
      const key = Math.floor(ms / bucketMs) * bucketMs;
      if (key < startBucketMs || key > endBucketMs) {
        continue;
      }
      const idx = bucketIndex.get(key);
      if (idx === undefined) {
        continue;
      }
      buckets[idx].download_bytes += Math.max(0, pt.download) * dt;
      buckets[idx].upload_bytes += Math.max(0, pt.upload) * dt;
    }
    return buckets;
  }, [historyPoints, historyWindow]);

  const trafficTotals = useMemo(() => {
    return trafficUsageBars.reduce(
      (acc, item) => {
        acc.download += item.download_bytes;
        acc.upload += item.upload_bytes;
        return acc;
      },
      { download: 0, upload: 0 },
    );
  }, [trafficUsageBars]);
  const showHistorySkeleton = historyLoading && !historyPoints.length;

  return (
    <div className="space-y-8">
      <PageHeader title="Dashboard" />

      <ErrorBanner
        message={liveError}
        onDismiss={() => setDismissedLiveError(true)}
        actionLabel="Retry"
        onAction={retryAll}
      />
      {warningMessages.length > 0 && (
        <div className="rounded-xl border border-status-warning/20 bg-status-warning/8 px-5 py-3.5 text-[14px] text-status-warning">{warningMessages.join(" | ")}</div>
      )}

      {showInitialLoading ? (
        <>
          <MetricsCarousel>
            {Array.from({ length: 4 }, (_, i) => (
              <div key={`metric-skeleton-${i}`} className="panel-card min-h-[108px] animate-pulse">
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
            {Array.from({ length: 3 }, (_, i) => (
              <div key={`secondary-skeleton-${i}`} className="panel-card min-h-[102px] animate-pulse">
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
      ) : (
        <>
      {/* ── Primary metrics ── */}
      <MetricsCarousel>
        {/* CPU */}
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

        {/* RAM */}
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

        {/* Online */}
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

        {/* Uptime */}
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

      {/* ── Secondary stats ── */}
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
      )}
      {/* Traffic Consumption */}
      <div className="space-y-4">
        <SectionHeader icon={<TrendingUp size={18} strokeWidth={1.6} />} title="Traffic Consumption">
          <div className="inline-flex w-full rounded-xl bg-surface-3/50 p-1 text-[13px] sm:w-auto">
            {(["1h", "24h"] as HistoryWindow[]).map((w) => (
              <button key={w} type="button" onClick={() => setHistoryWindow(w)}
                className={cn("flex-1 rounded-lg px-4 py-1.5 font-semibold transition-all sm:flex-none", historyWindow === w ? "bg-surface-4 text-txt-primary shadow-sm" : "text-txt-secondary hover:text-txt")}>
                {w}
              </button>
            ))}
          </div>
        </SectionHeader>

        <ErrorBanner
          message={historyError}
          onDismiss={() => setDismissedHistoryError(true)}
          actionLabel="Retry"
          onAction={retryHistory}
        />

        <div className="panel-card p-4 sm:p-6">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3 text-[13px]">
            <div className="flex flex-wrap items-center gap-4">
              <span className="inline-flex items-center gap-2 text-txt-secondary">
                <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: "var(--data-1)" }} />
                Download
              </span>
              <span className="inline-flex items-center gap-2 text-txt-secondary">
                <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: "var(--data-2)" }} />
                Upload
              </span>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-lg bg-surface-3/35 px-2.5 py-1 text-[12px] font-medium text-txt-secondary">Down total: {formatBytes(trafficTotals.download)}</span>
              <span className="rounded-lg bg-surface-3/35 px-2.5 py-1 text-[12px] font-medium text-txt-secondary">Up total: {formatBytes(trafficTotals.upload)}</span>
            </div>
          </div>
          {showHistorySkeleton ? (
            <StateBlock tone="loading" title="Loading chart" minHeightClassName="h-[300px]" className="rounded-xl bg-surface-3/28" />
          ) : (
            <div className="h-[300px]">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={trafficUsageBars}>
                  <defs>
                    <linearGradient id="gradDown" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="var(--data-1)" stopOpacity={0.3} />
                      <stop offset="100%" stopColor="var(--data-1)" stopOpacity={0.02} />
                    </linearGradient>
                    <linearGradient id="gradUp" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="var(--data-2)" stopOpacity={0.3} />
                      <stop offset="100%" stopColor="var(--data-2)" stopOpacity={0.02} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid stroke="var(--border)" strokeDasharray="4 4" vertical={false} />
                  <XAxis dataKey="timestamp" tickFormatter={(v) => formatTrafficTick(new Date(v), historyWindow)} tick={{ fill: "var(--txt-icon)", fontSize: 12 }} tickLine={false} axisLine={{ stroke: "var(--border)" }} />
                  <YAxis tickFormatter={(v) => formatBytes(Number(v))} tick={{ fill: "var(--txt-icon)", fontSize: 12 }} tickLine={false} axisLine={false} width={58} />
                  <Tooltip
                    formatter={(v: number) => formatBytes(Number(v))}
                    labelFormatter={(label) => formatTrafficTooltipLabel(label, historyWindow)}
                    contentStyle={tooltipStyle}
                    cursor={{ stroke: "var(--border-hover)", strokeDasharray: "4 4" }}
                  />
                  <Area
                    type="monotone"
                    dataKey="download_bytes"
                    stroke="var(--data-1)"
                    strokeWidth={2}
                    fill="url(#gradDown)"
                    name="Download"
                    isAnimationActive={false}
                  />
                  <Area
                    type="monotone"
                    dataKey="upload_bytes"
                    stroke="var(--data-2)"
                    strokeWidth={2}
                    fill="url(#gradUp)"
                    name="Upload"
                    isAnimationActive={false}
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          )}
        </div>
      </div>


      {/* ── Managed services ── */}
      <div className="space-y-2.5">
        <SectionHeader icon={<Cpu size={18} strokeWidth={1.6} />} title="Managed Services" />
        <ErrorBanner
          message={servicesError}
          onDismiss={() => {
            if (servicesActionError) {
              setServicesActionError("");
              return;
            }
            setDismissedServicesError(true);
          }}
          actionLabel={servicesQueryError || servicesActionError ? "Retry" : undefined}
          onAction={servicesQueryError || servicesActionError ? retryServices : undefined}
        />
        {servicesLoading ? (
          <StateBlock tone="loading" title="Loading services" minHeightClassName="min-h-[120px]" />
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {serviceItems.length ? serviceItems.map((item) => (
              <div key={item.service_name} className="card-hover panel-card-compact relative overflow-hidden">
                <div className="mb-2.5 flex items-center justify-between gap-3">
                  <h4 className="min-w-0 flex-1 truncate text-[15px] font-bold text-txt-primary">{item.service_name}</h4>
                  <span className="inline-flex shrink-0 items-center gap-2 rounded-lg bg-surface-3/40 px-2.5 py-1 text-[12px] font-medium text-txt-secondary">
                    <span className={cn("h-2 w-2 rounded-full", statusColor(item.status || "unknown"))} />{(item.status || "unknown").toLowerCase()}
                  </span>
                </div>
                <div className="grid grid-cols-2 gap-2.5">
                  <div><p className="text-[12px] font-medium text-txt-muted">Version</p><p className="mt-1 text-[14px] font-medium text-txt">{item.version || "-"}</p></div>
                  <div><p className="text-[12px] font-medium text-txt-muted">Last check</p><p className="mt-1 text-[14px] font-medium text-txt">{formatDateTime(item.last_check_at)}</p></div>
                </div>
                <div className="mt-2.5 flex flex-wrap items-center gap-2 border-t border-border/30 pt-2.5">
                  <Button size="sm" className="h-8 min-w-[96px] flex-1 px-3 sm:flex-none" onClick={() => void openServiceDetails(item.service_name)} disabled={servicesBusy}><Eye size={15} strokeWidth={1.6} />Details</Button>
                  <ConfirmPopover
                    title="Reload service"
                    description={`Reload ${item.service_name}?`}
                    confirmText="Reload"
                    onConfirm={() => void runServiceAction(item.service_name, "reload")}
                  >
                    <Button size="sm" className="h-8 min-w-[96px] flex-1 px-3 sm:flex-none" disabled={servicesBusy}><RefreshCw size={15} strokeWidth={1.6} />Reload</Button>
                  </ConfirmPopover>
                  <ConfirmPopover
                    title="Restart service"
                    description={`Restart ${item.service_name}?`}
                    confirmText="Restart"
                    onConfirm={() => void runServiceAction(item.service_name, "restart")}
                  >
                    <Button size="sm" className="h-8 min-w-[96px] flex-1 px-3 sm:flex-none" disabled={servicesBusy}><RotateCcw size={15} strokeWidth={1.6} />Restart</Button>
                  </ConfirmPopover>
                </div>
              </div>
            )) : <StateBlock tone="empty" title="No services" minHeightClassName="min-h-[120px]" />}
          </div>
        )}
      </div>

      {/* Dialogs */}
      <Dialog open={serviceDetailsOpen} onOpenChange={(n) => { if (!n) setServiceDetailsOpen(false); }} title={`${serviceDetails?.name || "Service"} details`} contentClassName="max-w-[760px]"
        footer={<Button onClick={() => setServiceDetailsOpen(false)}>Close</Button>}>
        {serviceDetails && (
          <div className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-xl bg-surface-0/50 p-4"><p className="text-[12px] font-medium text-txt-muted">Status</p><p className="mt-1.5 text-[14px] font-medium text-txt">{serviceDetails.status_text}</p></div>
              <div className="rounded-xl bg-surface-0/50 p-4"><p className="text-[12px] font-medium text-txt-muted">Active</p><p className="mt-1.5 text-[14px] font-medium text-txt">{serviceDetails.active} / {serviceDetails.sub_state}</p></div>
              <div className="rounded-xl bg-surface-0/50 p-4"><p className="text-[12px] font-medium text-txt-muted">PID</p><p className="mt-1.5 text-[14px] font-medium text-txt">{serviceDetails.main_pid || 0}</p></div>
            </div>
            <p className="text-[13px] text-txt-muted">Checked: {formatDateTime(serviceDetails.checked_at)}</p>
            <div>
              <p className="mb-2 text-[14px] font-semibold text-txt">Recent logs</p>
              <pre className="m-0 max-h-[320px] overflow-auto rounded-xl border border-[var(--control-border)] bg-[var(--control-bg)] p-4 font-mono text-[13px] leading-6 text-txt-secondary shadow-[inset_0_0_0_1px_var(--control-border)]">
                {serviceDetails.last_logs?.length ? serviceDetails.last_logs.join("\n") : "No logs available"}
              </pre>
            </div>
          </div>
        )}
      </Dialog>

    </div>
  );
}
