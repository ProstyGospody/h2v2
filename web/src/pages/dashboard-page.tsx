import { animate, motion, useMotionValue, useTransform } from "framer-motion";
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  Cpu,
  Eye,
  Globe,
  HardDrive,
  Loader2,
  RefreshCw,
  RotateCcw,
  TrendingUp,
  Users,
  Wifi,
  Zap,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { ConfirmDialog } from "@/components/dialogs/confirm-dialog";
import { PageHeader } from "@/components/ui/page-header";
import { APIError, apiFetch } from "@/services/api";
import { ServiceDetails, ServiceSummary, SystemHistoryResponse, SystemLiveResponse } from "@/types/common";
import { Button, Dialog, Table, TableBody, TableCell, TableContainer, TableHead, TableHeader, TableRow, cn } from "@/src/components/ui";
import { formatBytes, formatDateTime, formatRate, formatUptime } from "@/utils/format";

const LIVE_POLL_MS = 5000;
const HISTORY_POLL_MS = 15000;
const HISTORY_LIMIT = 20000;

type ActionState = { name: string; action: "restart" | "reload" } | null;
type HistoryWindow = "1h" | "24h";

type HistoryTrendPoint = {
  timestamp: Date;
  cpu: number;
  ram: number;
  download: number;
  upload: number;
};

type TrafficUsageBarPoint = {
  timestamp: Date;
  download_bytes: number;
  upload_bytes: number;
};

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.min(100, value));
}

function formatShortTime(value: Date): string {
  if (!(value instanceof Date) || Number.isNaN(value.getTime())) {
    return "--:--";
  }
  return value.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function AnimatedNumber({
  value,
  format = (next) => next.toFixed(0),
}: {
  value: number;
  format?: (value: number) => string;
}) {
  const motionValue = useMotionValue(0);
  const rounded = useTransform(motionValue, (latest) => format(latest));

  useEffect(() => {
    motionValue.set(0);
    const controls = animate(motionValue, value, { duration: 0.8, ease: "easeOut" });
    return () => controls.stop();
  }, [motionValue, value]);

  return <motion.span>{rounded}</motion.span>;
}

function ProgressRing({
  value,
  size = 44,
  strokeWidth = 3.5,
  color = "#6366f1",
  trackColor = "rgba(255,255,255,0.05)",
}: {
  value: number;
  size?: number;
  strokeWidth?: number;
  color?: string;
  trackColor?: string;
}) {
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (clampPercent(value) / 100) * circumference;

  return (
    <svg width={size} height={size} className="-rotate-90">
      <circle cx={size / 2} cy={size / 2} r={radius} fill="none" stroke={trackColor} strokeWidth={strokeWidth} />
      <motion.circle
        cx={size / 2}
        cy={size / 2}
        r={radius}
        fill="none"
        stroke={color}
        strokeWidth={strokeWidth}
        strokeLinecap="round"
        strokeDasharray={circumference}
        initial={{ strokeDashoffset: circumference }}
        animate={{ strokeDashoffset: offset }}
        transition={{ duration: 1, ease: "easeOut" }}
      />
    </svg>
  );
}

function statusColor(status: string): string {
  const normalized = status.toLowerCase();
  if (normalized.includes("running") || normalized.includes("active")) {
    return "bg-status-success shadow-[0_0_8px_#34d39960]";
  }
  if (normalized.includes("failed") || normalized.includes("error")) {
    return "bg-status-danger";
  }
  if (normalized.includes("inactive") || normalized.includes("stopped")) {
    return "bg-status-warning";
  }
  return "bg-txt-muted";
}

function SectionHeader({ icon, title, children }: { icon: React.ReactNode; title: string; children?: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-2">
        <span className="text-txt-tertiary">{icon}</span>
        <h3 className="text-[13px] font-semibold text-white">{title}</h3>
      </div>
      {children}
    </div>
  );
}

const chartTooltipStyle = {
  backgroundColor: "rgba(24, 24, 28, 0.95)",
  border: "1px solid rgba(30, 30, 36, 0.8)",
  borderRadius: 10,
  color: "#e4e4e7",
  fontSize: 11,
  backdropFilter: "blur(12px)",
  boxShadow: "0 8px 32px rgba(0,0,0,0.3)",
};

export default function DashboardPage() {
  const [live, setLive] = useState<SystemLiveResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [historyItems, setHistoryItems] = useState<SystemHistoryResponse["items"]>([]);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [historyError, setHistoryError] = useState("");
  const [historyWindow, setHistoryWindow] = useState<HistoryWindow>("1h");
  const [serviceItems, setServiceItems] = useState<ServiceSummary[]>([]);
  const [servicesLoading, setServicesLoading] = useState(true);
  const [servicesBusy, setServicesBusy] = useState(false);
  const [servicesError, setServicesError] = useState("");
  const [serviceDetails, setServiceDetails] = useState<ServiceDetails | null>(null);
  const [serviceDetailsOpen, setServiceDetailsOpen] = useState(false);
  const [serviceActionState, setServiceActionState] = useState<ActionState>(null);
  const loadingRef = useRef(false);
  const historyLoadingRef = useRef(false);

  const load = useCallback(async () => {
    if (loadingRef.current) {
      return;
    }

    loadingRef.current = true;
    setError("");
    try {
      const livePayload = await apiFetch<SystemLiveResponse>("/api/system/live", { method: "GET" });
      setLive(livePayload);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to load dashboard data");
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  }, []);

  const loadServices = useCallback(async () => {
    setServicesError("");
    try {
      const payload = await apiFetch<{ items: ServiceSummary[] }>("/api/services", { method: "GET" });
      setServiceItems(payload.items || []);
    } catch (err) {
      setServicesError(err instanceof APIError ? err.message : "Failed to load services");
    } finally {
      setServicesLoading(false);
    }
  }, []);

  const loadHistory = useCallback(async () => {
    if (historyLoadingRef.current) {
      return;
    }

    historyLoadingRef.current = true;
    setHistoryError("");
    try {
      const step = historyWindow === "24h" ? 30 : 5;
      const payload = await apiFetch<SystemHistoryResponse>(
        `/api/system/history?window=${historyWindow}&step=${step}&limit=${HISTORY_LIMIT}`,
        { method: "GET" },
      );
      setHistoryItems(Array.isArray(payload.items) ? payload.items : []);
    } catch (err) {
      setHistoryError(err instanceof APIError ? err.message : "Failed to load history");
    } finally {
      historyLoadingRef.current = false;
      setHistoryLoading(false);
    }
  }, [historyWindow]);

  useEffect(() => {
    void load();
    const timer = setInterval(() => void load(), LIVE_POLL_MS);
    return () => clearInterval(timer);
  }, [load]);

  useEffect(() => {
    void loadServices();
    const timer = setInterval(() => void loadServices(), 15000);
    return () => clearInterval(timer);
  }, [loadServices]);

  useEffect(() => {
    setHistoryLoading(true);
    void loadHistory();
    const timer = setInterval(() => void loadHistory(), HISTORY_POLL_MS);
    return () => clearInterval(timer);
  }, [loadHistory]);

  const warningMessages = useMemo(() => {
    return live?.errors || [];
  }, [live]);

  async function openServiceDetails(name: string) {
    setServicesBusy(true);
    try {
      const payload = await apiFetch<ServiceDetails>(`/api/services/${name}?lines=60`, { method: "GET" });
      setServiceDetails(payload);
      setServiceDetailsOpen(true);
    } catch (err) {
      setServicesError(err instanceof APIError ? err.message : "Failed to load service details");
    } finally {
      setServicesBusy(false);
    }
  }

  async function runServiceAction() {
    if (!serviceActionState) {
      return;
    }
    setServicesBusy(true);
    try {
      await apiFetch<{ ok: boolean }>(`/api/services/${serviceActionState.name}/${serviceActionState.action}`, {
        method: "POST",
        body: JSON.stringify({}),
      });
      setServiceActionState(null);
      await loadServices();
    } catch (err) {
      setServicesError(err instanceof APIError ? err.message : "Failed to run action");
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
      .map((sample) => {
        const timestamp = new Date(sample.timestamp);
        if (Number.isNaN(timestamp.getTime())) {
          return null;
        }
        return {
          timestamp,
          cpu: clampPercent(sample.cpu_usage_percent),
          ram: clampPercent(sample.memory_used_percent),
          download: Math.max(0, sample.network_rx_bps || 0),
          upload: Math.max(0, sample.network_tx_bps || 0),
        };
      })
      .filter((item): item is HistoryTrendPoint => Boolean(item))
      .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
  }, [historyItems]);

  const trafficUsageBars = useMemo<TrafficUsageBarPoint[]>(() => {
    if (!historyPoints.length) {
      return [];
    }

    const bucketMs = 60_000;
    const bucketed = new Map<number, TrafficUsageBarPoint>();

    for (let i = 0; i < historyPoints.length; i += 1) {
      const point = historyPoints[i];
      const currentMs = point.timestamp.getTime();
      const prevMs = i > 0 ? historyPoints[i - 1].timestamp.getTime() : currentMs - 5_000;
      const deltaSeconds = Math.max(1, (currentMs - prevMs) / 1000);
      const bucketMsValue = Math.floor(currentMs / bucketMs) * bucketMs;

      const downloadBytes = Math.max(0, point.download) * deltaSeconds;
      const uploadBytes = Math.max(0, point.upload) * deltaSeconds;

      const current = bucketed.get(bucketMsValue);
      if (current) {
        current.download_bytes += downloadBytes;
        current.upload_bytes += uploadBytes;
      } else {
        bucketed.set(bucketMsValue, {
          timestamp: new Date(bucketMsValue),
          download_bytes: downloadBytes,
          upload_bytes: uploadBytes,
        });
      }
    }

    return [...bucketed.values()].sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
  }, [historyPoints]);

  return (
    <div className="space-y-6">
      <PageHeader title="Dashboard" />

      {showInitialLoading ? (
        <motion.div
          initial={{ opacity: 0, y: -4 }}
          animate={{ opacity: 1, y: 0 }}
          className="rounded-[10px] border border-accent/20 bg-accent/8 px-4 py-3 text-[12px] text-accent-light"
        >
          Loading latest dashboard metrics...
        </motion.div>
      ) : null}
      {error ? <div className="rounded-[10px] border border-status-danger/20 bg-status-danger/8 px-4 py-3 text-[12px] text-status-danger">{error}</div> : null}
      {warningMessages.length ? (
        <div className="rounded-[10px] border border-status-warning/20 bg-status-warning/8 px-4 py-3 text-[12px] text-status-warning">
          {warningMessages.join(" | ")}
        </div>
      ) : null}

      {/* ── Metric cards ── */}
      <motion.div
        variants={{ hidden: {}, show: { transition: { staggerChildren: 0.06 } } }}
        initial="hidden"
        animate="show"
        className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4"
      >
        {/* CPU card with ring */}
        <motion.div
          variants={{ hidden: { opacity: 0, y: 14 }, show: { opacity: 1, y: 0 } }}
          className="group relative overflow-hidden rounded-[12px] border border-border bg-surface-2 p-4 transition-colors hover:border-accent/20"
        >
          <div className="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-accent/4 transition-all group-hover:bg-accent/8" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10px] font-medium uppercase tracking-wider text-txt-muted">CPU</p>
              <p className="mt-1 text-[28px] font-bold leading-none tracking-tight text-white">
                <AnimatedNumber value={cpuPercent} format={(v) => `${v.toFixed(1)}`} />
                <span className="ml-0.5 text-[14px] font-medium text-txt-tertiary">%</span>
              </p>
              <p className="mt-2 text-[11px] text-txt-muted">System load</p>
            </div>
            <ProgressRing value={cpuPercent} color="#6366f1" />
          </div>
        </motion.div>

        {/* RAM card with ring */}
        <motion.div
          variants={{ hidden: { opacity: 0, y: 14 }, show: { opacity: 1, y: 0 } }}
          className="group relative overflow-hidden rounded-[12px] border border-border bg-surface-2 p-4 transition-colors hover:border-accent-secondary/20"
        >
          <div className="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-accent-secondary/4 transition-all group-hover:bg-accent-secondary/8" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10px] font-medium uppercase tracking-wider text-txt-muted">RAM</p>
              <p className="mt-1 text-[28px] font-bold leading-none tracking-tight text-white">
                <AnimatedNumber value={ramPercent} format={(v) => `${v.toFixed(1)}`} />
                <span className="ml-0.5 text-[14px] font-medium text-txt-tertiary">%</span>
              </p>
              <p className="mt-2 text-[11px] text-txt-muted">Memory usage</p>
            </div>
            <ProgressRing value={ramPercent} color="#8b5cf6" />
          </div>
        </motion.div>

        {/* Online users */}
        <motion.div
          variants={{ hidden: { opacity: 0, y: 14 }, show: { opacity: 1, y: 0 } }}
          className="group relative overflow-hidden rounded-[12px] border border-border bg-surface-2 p-4 transition-colors hover:border-status-success/20"
        >
          <div className="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-status-success/4 transition-all group-hover:bg-status-success/8" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10px] font-medium uppercase tracking-wider text-txt-muted">Online</p>
              <p className="mt-1 text-[28px] font-bold leading-none tracking-tight text-white">
                <AnimatedNumber value={onlineUsers} />
              </p>
              <p className="mt-2 text-[11px] text-txt-muted">Connected users</p>
            </div>
            <div className="grid h-[44px] w-[44px] place-items-center rounded-full bg-status-success/8">
              <Users size={18} strokeWidth={1.4} className="text-status-success" />
            </div>
          </div>
        </motion.div>

        {/* Uptime */}
        <motion.div
          variants={{ hidden: { opacity: 0, y: 14 }, show: { opacity: 1, y: 0 } }}
          className="group relative overflow-hidden rounded-[12px] border border-border bg-surface-2 p-4 transition-colors hover:border-status-warning/20"
        >
          <div className="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-status-warning/4 transition-all group-hover:bg-status-warning/8" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10px] font-medium uppercase tracking-wider text-txt-muted">Uptime</p>
              <p className="mt-1 text-[28px] font-bold leading-none tracking-tight text-white">{uptime}</p>
              <p className="mt-2 text-[11px] text-txt-muted">Current session</p>
            </div>
            <div className="grid h-[44px] w-[44px] place-items-center rounded-full bg-status-warning/8">
              <Activity size={18} strokeWidth={1.4} className="text-status-warning" />
            </div>
          </div>
        </motion.div>
      </motion.div>

      {/* ── Secondary metrics row ── */}
      <motion.div
        variants={{ hidden: {}, show: { transition: { staggerChildren: 0.04 } } }}
        initial="hidden"
        animate="show"
        className="grid gap-3 sm:grid-cols-3"
      >
        {/* Network */}
        <motion.div
          variants={{ hidden: { opacity: 0, y: 10 }, show: { opacity: 1, y: 0 } }}
          className="flex items-center gap-4 rounded-[12px] border border-border bg-surface-2 p-4"
        >
          <div className="grid h-10 w-10 shrink-0 place-items-center rounded-[10px] bg-accent/10">
            <Wifi size={18} strokeWidth={1.4} className="text-accent-light" />
          </div>
          <div className="min-w-0">
            <p className="text-[10px] font-medium uppercase tracking-wider text-txt-muted">Network</p>
            <div className="mt-1 flex items-center gap-3 text-[13px] font-semibold text-white">
              <span className="inline-flex items-center gap-1">
                <ArrowDownToLine size={12} strokeWidth={1.6} className="text-accent-light" />
                {formatRate(networkRx)}
              </span>
              <span className="inline-flex items-center gap-1">
                <ArrowUpFromLine size={12} strokeWidth={1.6} className="text-accent-secondary-light" />
                {formatRate(networkTx)}
              </span>
            </div>
          </div>
        </motion.div>

        {/* Total Traffic */}
        <motion.div
          variants={{ hidden: { opacity: 0, y: 10 }, show: { opacity: 1, y: 0 } }}
          className="flex items-center gap-4 rounded-[12px] border border-border bg-surface-2 p-4"
        >
          <div className="grid h-10 w-10 shrink-0 place-items-center rounded-[10px] bg-accent-secondary/10">
            <Globe size={18} strokeWidth={1.4} className="text-accent-secondary-light" />
          </div>
          <div className="min-w-0">
            <p className="text-[10px] font-medium uppercase tracking-wider text-txt-muted">Total Traffic</p>
            <p className="mt-1 text-[13px] font-semibold text-white">{formatBytes(totalTraffic)}</p>
          </div>
        </motion.div>

        {/* Connections */}
        <motion.div
          variants={{ hidden: { opacity: 0, y: 10 }, show: { opacity: 1, y: 0 } }}
          className="flex items-center gap-4 rounded-[12px] border border-border bg-surface-2 p-4"
        >
          <div className="grid h-10 w-10 shrink-0 place-items-center rounded-[10px] bg-accent/10">
            <Zap size={18} strokeWidth={1.4} className="text-accent-light" />
          </div>
          <div className="min-w-0">
            <p className="text-[10px] font-medium uppercase tracking-wider text-txt-muted">Connections</p>
            <div className="mt-1 flex items-center gap-3 text-[13px] font-semibold text-white">
              <span>TCP {tcpConnections}</span>
              <span className="text-txt-muted">/</span>
              <span>UDP {udpConnections}</span>
            </div>
          </div>
        </motion.div>
      </motion.div>

      {/* ── Traffic trends ── */}
      <div className="space-y-3">
        <SectionHeader icon={<TrendingUp size={15} strokeWidth={1.4} />} title="Traffic trends">
          <div className="inline-flex rounded-full bg-surface-3/60 p-0.5 text-[11px]">
            {(["1h", "24h"] as HistoryWindow[]).map((w) => (
              <button
                key={w}
                type="button"
                className={cn(
                  "rounded-full px-3 py-1 font-medium transition-all",
                  historyWindow === w ? "bg-accent text-white shadow-sm shadow-accent/20" : "text-txt-secondary hover:text-txt",
                )}
                onClick={() => setHistoryWindow(w)}
              >
                {w}
              </button>
            ))}
          </div>
        </SectionHeader>

        {historyLoading && !historyPoints.length ? (
          <div className="rounded-[10px] border border-accent/20 bg-accent/8 px-4 py-3 text-[12px] text-accent-light">Loading system history...</div>
        ) : null}
        {historyError ? <div className="rounded-[10px] border border-status-warning/20 bg-status-warning/8 px-4 py-3 text-[12px] text-status-warning">{historyError}</div> : null}

        {/* Area chart */}
        <div className="rounded-[12px] border border-border bg-surface-2 p-5">
          <div className="mb-3 flex items-center gap-4 text-[11px]">
            <span className="inline-flex items-center gap-1.5 text-txt-secondary">
              <span className="h-2 w-2 rounded-full bg-[#6366f1]" />
              Upload
            </span>
            <span className="inline-flex items-center gap-1.5 text-txt-secondary">
              <span className="h-2 w-2 rounded-full bg-[#8b5cf6]" />
              Download
            </span>
          </div>
          <div className="h-[260px]">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={historyPoints}>
                <defs>
                  <linearGradient id="uploadGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#6366f1" stopOpacity={0.25} />
                    <stop offset="100%" stopColor="#6366f1" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="downloadGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#8b5cf6" stopOpacity={0.2} />
                    <stop offset="100%" stopColor="#8b5cf6" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke="rgba(30,30,36,0.6)" strokeDasharray="4 4" vertical={false} />
                <XAxis
                  dataKey="timestamp"
                  tickFormatter={(value) => formatShortTime(new Date(value))}
                  tick={{ fill: "#52525a", fontSize: 10 }}
                  tickLine={false}
                  axisLine={{ stroke: "rgba(30,30,36,0.6)" }}
                />
                <YAxis
                  tickFormatter={(value) => formatBytes(Number(value))}
                  tick={{ fill: "#52525a", fontSize: 10 }}
                  tickLine={false}
                  axisLine={false}
                  width={56}
                />
                <Tooltip
                  labelFormatter={(value) => formatDateTime(value instanceof Date ? value.toISOString() : String(value))}
                  formatter={(value: number) => formatRate(Number(value))}
                  contentStyle={chartTooltipStyle}
                  cursor={{ stroke: "rgba(99,102,241,0.2)", strokeWidth: 1 }}
                />
                <Area type="monotone" dataKey="upload" stroke="#6366f1" fill="url(#uploadGradient)" strokeWidth={2} name="Upload" dot={false} activeDot={{ r: 3, fill: "#6366f1", stroke: "#18181c", strokeWidth: 2 }} />
                <Area type="monotone" dataKey="download" stroke="#8b5cf6" fill="url(#downloadGradient)" strokeWidth={2} name="Download" dot={false} activeDot={{ r: 3, fill: "#8b5cf6", stroke: "#18181c", strokeWidth: 2 }} />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Bar chart */}
        <div className="rounded-[12px] border border-border bg-surface-2 p-5">
          <div className="mb-3 flex items-center gap-4 text-[11px]">
            <span className="inline-flex items-center gap-1.5 text-txt-secondary">
              <span className="h-2 w-2 rounded-sm bg-[#6366f1]" />
              Download
            </span>
            <span className="inline-flex items-center gap-1.5 text-txt-secondary">
              <span className="h-2 w-2 rounded-sm bg-[#8b5cf6]" />
              Upload
            </span>
          </div>
          <div className="h-[200px]">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trafficUsageBars} barGap={1}>
                <CartesianGrid stroke="rgba(30,30,36,0.6)" strokeDasharray="4 4" vertical={false} />
                <XAxis
                  dataKey="timestamp"
                  tickFormatter={(value) => formatShortTime(new Date(value))}
                  tick={{ fill: "#52525a", fontSize: 10 }}
                  tickLine={false}
                  axisLine={{ stroke: "rgba(30,30,36,0.6)" }}
                />
                <YAxis
                  tickFormatter={(value) => formatBytes(Number(value))}
                  tick={{ fill: "#52525a", fontSize: 10 }}
                  tickLine={false}
                  axisLine={false}
                  width={56}
                />
                <Tooltip
                  formatter={(value: number) => formatBytes(Number(value))}
                  contentStyle={chartTooltipStyle}
                  cursor={{ fill: "rgba(99,102,241,0.04)" }}
                />
                <Bar dataKey="download_bytes" fill="#6366f1" radius={[3, 3, 0, 0]} name="Download" />
                <Bar dataKey="upload_bytes" fill="#8b5cf6" radius={[3, 3, 0, 0]} name="Upload" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {/* ── Live feed & Service snapshot ── */}
      <div className="grid gap-3 lg:grid-cols-2">
        <div className="rounded-[12px] border border-border bg-surface-2 p-5">
          <SectionHeader icon={<Activity size={14} strokeWidth={1.4} />} title="Live feed" />
          <div className="mt-4 space-y-2">
            {(live?.services || []).length ? (
              (live?.services || []).map((item, index) => (
                <motion.div
                  key={`${item.service_name}-${index}`}
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: "auto" }}
                  className="flex items-start gap-[10px] border-b border-border/60 pb-2.5 last:border-0"
                >
                  <span className={cn("mt-1.5 h-[6px] w-[6px] shrink-0 rounded-full", statusColor(item.status))} />
                  <div className="min-w-0">
                    <p className="text-[12px]">
                      <span className="font-medium text-white">{item.service_name}</span>
                      <span className="ml-1.5 text-txt-tertiary">status is {item.status}</span>
                    </p>
                    <p className="mt-0.5 text-[10px] text-txt-muted">{formatDateTime(item.last_check_at)}</p>
                  </div>
                </motion.div>
              ))
            ) : (
              <p className="py-4 text-center text-[12px] text-txt-secondary">No recent events.</p>
            )}
          </div>
        </div>

        <div className="rounded-[12px] border border-border bg-surface-2 p-5">
          <SectionHeader icon={<HardDrive size={14} strokeWidth={1.4} />} title="Service snapshot" />
          <div className="mt-4">
            <TableContainer className="border-0 bg-transparent shadow-none">
              <Table>
                <TableHeader>
                  <TableRow className="border-t-0 hover:bg-transparent">
                    <TableHead>Service</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Version</TableHead>
                    <TableHead>Checked</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(live?.services || []).map((item, index) => (
                    <TableRow key={item.service_name} className="cursor-default" style={{ animationDelay: `${index * 0.03}s` }}>
                      <TableCell className="font-medium">{item.service_name}</TableCell>
                      <TableCell>
                        <span className={cn("inline-flex items-center gap-2 text-[11px]", item.status.includes("running") ? "text-status-success" : "text-status-warning")}>
                          <span className={cn("h-[6px] w-[6px] rounded-full", statusColor(item.status))} />
                          {item.status}
                        </span>
                      </TableCell>
                      <TableCell className="text-txt-muted">-</TableCell>
                      <TableCell>{formatDateTime(item.last_check_at)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          </div>
        </div>
      </div>

      {/* ── Managed services ── */}
      <div className="space-y-3">
        <SectionHeader icon={<Cpu size={15} strokeWidth={1.4} />} title="Managed services" />
        {servicesError ? <div className="rounded-[10px] border border-status-danger/20 bg-status-danger/8 px-4 py-3 text-[12px] text-status-danger">{servicesError}</div> : null}
        {servicesLoading ? (
          <div className="flex min-h-[160px] items-center justify-center rounded-[12px] border border-border bg-surface-2">
            <div className="flex flex-col items-center gap-2">
              <Loader2 size={20} strokeWidth={1.4} className="animate-spin text-accent-light" />
              <p className="text-[12px] text-txt-secondary">Loading services...</p>
            </div>
          </div>
        ) : (
          <div className="grid gap-3 md:grid-cols-2">
            {serviceItems.length ? (
              serviceItems.map((item) => (
                <motion.div
                  key={item.service_name}
                  whileHover={{ y: -2 }}
                  transition={{ duration: 0.15 }}
                  className="group relative overflow-hidden rounded-[12px] border border-border bg-surface-2 p-5 transition-colors hover:border-accent/15"
                >
                  {/* Top gradient line */}
                  <div className="absolute left-0 right-0 top-0 h-[2px] bg-gradient-to-r from-accent to-accent-secondary opacity-60 transition-opacity group-hover:opacity-100" />

                  <div className="mb-3 flex items-center justify-between">
                    <h4 className="text-[13px] font-semibold text-white">{item.service_name}</h4>
                    <span className="inline-flex items-center gap-2 text-[11px] text-txt-secondary">
                      <span className={cn("h-[7px] w-[7px] rounded-full", statusColor(item.status || "unknown"))} />
                      {(item.status || "unknown").toLowerCase()}
                    </span>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <p className="text-[10px] font-medium text-txt-muted">Version</p>
                      <p className="mt-0.5 text-[12px] font-medium text-txt">{item.version || "-"}</p>
                    </div>
                    <div>
                      <p className="text-[10px] font-medium text-txt-muted">Last check</p>
                      <p className="mt-0.5 text-[12px] font-medium text-txt">{formatDateTime(item.last_check_at)}</p>
                    </div>
                  </div>
                  <div className="mt-4 flex items-center gap-2 border-t border-border/60 pt-3.5">
                    <Button size="sm" onClick={() => void openServiceDetails(item.service_name)} disabled={servicesBusy}>
                      <Eye size={14} strokeWidth={1.4} />
                      Details
                    </Button>
                    <Button size="sm" onClick={() => setServiceActionState({ name: item.service_name, action: "reload" })} disabled={servicesBusy}>
                      <RefreshCw size={14} strokeWidth={1.4} />
                      Reload
                    </Button>
                    <Button size="sm" onClick={() => setServiceActionState({ name: item.service_name, action: "restart" })} disabled={servicesBusy}>
                      <RotateCcw size={14} strokeWidth={1.4} />
                      Restart
                    </Button>
                  </div>
                </motion.div>
              ))
            ) : (
              <div className="rounded-[12px] border border-border bg-surface-2 p-4 text-[12px] text-txt-secondary">
                Service activity is not available yet.
              </div>
            )}
          </div>
        )}
      </div>

      <Dialog
        open={serviceDetailsOpen}
        onOpenChange={(next) => {
          if (!next) {
            setServiceDetailsOpen(false);
          }
        }}
        title={`${serviceDetails?.name || "Service"} details`}
        contentClassName="max-w-[760px]"
        footer={
          <Button onClick={() => setServiceDetailsOpen(false)}>
            Close
          </Button>
        }
      >
        {serviceDetails ? (
          <div className="space-y-3">
            <div className="grid grid-cols-3 gap-3">
              <div className="rounded-[8px] bg-surface-0/50 p-3">
                <p className="text-[10px] font-medium text-txt-muted">Status</p>
                <p className="mt-1 text-[12px] font-medium text-txt">{serviceDetails.status_text}</p>
              </div>
              <div className="rounded-[8px] bg-surface-0/50 p-3">
                <p className="text-[10px] font-medium text-txt-muted">Active</p>
                <p className="mt-1 text-[12px] font-medium text-txt">{serviceDetails.active} / {serviceDetails.sub_state}</p>
              </div>
              <div className="rounded-[8px] bg-surface-0/50 p-3">
                <p className="text-[10px] font-medium text-txt-muted">PID</p>
                <p className="mt-1 text-[12px] font-medium text-txt">{serviceDetails.main_pid || 0}</p>
              </div>
            </div>
            <p className="text-[10px] font-medium text-txt-muted">Checked: {formatDateTime(serviceDetails.checked_at)}</p>
            <div>
              <p className="mb-2 text-[12px] font-medium text-txt">Recent logs</p>
              <pre className="m-0 max-h-[320px] overflow-auto rounded-[8px] border border-border bg-surface-0 p-3 font-mono text-[11px] leading-5 text-txt-secondary">
                {serviceDetails.last_logs?.length ? serviceDetails.last_logs.join("\n") : "No logs available"}
              </pre>
            </div>
          </div>
        ) : null}
      </Dialog>

      <ConfirmDialog
        open={Boolean(serviceActionState)}
        title="Confirm service action"
        description={`${serviceActionState?.action === "restart" ? "Restart" : "Reload"} ${serviceActionState?.name || "service"} now?`}
        busy={servicesBusy}
        confirmText="Confirm"
        onClose={() => setServiceActionState(null)}
        onConfirm={() => void runServiceAction()}
      />
    </div>
  );
}
