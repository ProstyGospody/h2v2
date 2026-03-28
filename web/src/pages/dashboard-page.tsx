import { animate, motion, useMotionValue, useTransform } from "framer-motion";
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  BarChart3,
  Clock,
  Cpu,
  Eye,
  Globe,
  HardDrive,
  Loader2,
  Network,
  RefreshCw,
  RotateCcw,
  TrendingUp,
  Users2,
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
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, value));
}

function formatShortTime(value: Date): string {
  if (!(value instanceof Date) || Number.isNaN(value.getTime())) return "--:--";
  return value.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function AnimatedNumber({ value, format = (n) => n.toFixed(0) }: { value: number; format?: (v: number) => string }) {
  const mv = useMotionValue(0);
  const display = useTransform(mv, (latest) => format(latest));
  useEffect(() => {
    mv.set(0);
    const c = animate(mv, value, { duration: 0.8, ease: "easeOut" });
    return () => c.stop();
  }, [mv, value]);
  return <motion.span>{display}</motion.span>;
}

function ProgressRing({ value, size = 52, strokeWidth = 4, color = "var(--data-2)" }: { value: number; size?: number; strokeWidth?: number; color?: string }) {
  const r = (size - strokeWidth) / 2;
  const c = 2 * Math.PI * r;
  const offset = c - (clampPercent(value) / 100) * c;
  return (
    <svg width={size} height={size} className="-rotate-90">
      <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--border)" strokeWidth={strokeWidth} />
      <motion.circle
        cx={size / 2} cy={size / 2} r={r} fill="none" stroke={color} strokeWidth={strokeWidth} strokeLinecap="round"
        strokeDasharray={c} initial={{ strokeDashoffset: c }} animate={{ strokeDashoffset: offset }}
        transition={{ duration: 1, ease: "easeOut" }}
      />
    </svg>
  );
}

function statusColor(status: string): string {
  const n = status.toLowerCase();
  if (n.includes("running") || n.includes("active")) return "bg-status-success shadow-[0_0_8px_var(--status-success-soft)]";
  if (n.includes("failed") || n.includes("error")) return "bg-status-danger";
  if (n.includes("inactive") || n.includes("stopped")) return "bg-status-warning";
  return "bg-txt-muted";
}

function SectionHeader({ icon, title, children }: { icon: React.ReactNode; title: string; children?: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-2.5">
        <span className="text-txt-tertiary">{icon}</span>
        <h3 className="text-[16px] font-bold text-txt-primary">{title}</h3>
      </div>
      {children}
    </div>
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
    if (loadingRef.current) return;
    loadingRef.current = true;
    setError("");
    try {
      setLive(await apiFetch<SystemLiveResponse>("/api/system/live", { method: "GET" }));
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
      const p = await apiFetch<{ items: ServiceSummary[] }>("/api/services", { method: "GET" });
      setServiceItems(p.items || []);
    } catch (err) {
      setServicesError(err instanceof APIError ? err.message : "Failed to load services");
    } finally {
      setServicesLoading(false);
    }
  }, []);

  const loadHistory = useCallback(async () => {
    if (historyLoadingRef.current) return;
    historyLoadingRef.current = true;
    setHistoryError("");
    try {
      const step = historyWindow === "24h" ? 30 : 5;
      const p = await apiFetch<SystemHistoryResponse>(`/api/system/history?window=${historyWindow}&step=${step}&limit=${HISTORY_LIMIT}`, { method: "GET" });
      setHistoryItems(Array.isArray(p.items) ? p.items : []);
    } catch (err) {
      setHistoryError(err instanceof APIError ? err.message : "Failed to load history");
    } finally {
      historyLoadingRef.current = false;
      setHistoryLoading(false);
    }
  }, [historyWindow]);

  useEffect(() => { void load(); const t = setInterval(() => void load(), LIVE_POLL_MS); return () => clearInterval(t); }, [load]);
  useEffect(() => { void loadServices(); const t = setInterval(() => void loadServices(), 15000); return () => clearInterval(t); }, [loadServices]);
  useEffect(() => { setHistoryLoading(true); void loadHistory(); const t = setInterval(() => void loadHistory(), HISTORY_POLL_MS); return () => clearInterval(t); }, [loadHistory]);

  const warningMessages = useMemo(() => live?.errors || [], [live]);

  async function openServiceDetails(name: string) {
    setServicesBusy(true);
    try { setServiceDetails(await apiFetch<ServiceDetails>(`/api/services/${name}?lines=60`, { method: "GET" })); setServiceDetailsOpen(true); }
    catch (err) { setServicesError(err instanceof APIError ? err.message : "Failed to load service details"); }
    finally { setServicesBusy(false); }
  }

  async function runServiceAction() {
    if (!serviceActionState) return;
    setServicesBusy(true);
    try { await apiFetch<{ ok: boolean }>(`/api/services/${serviceActionState.name}/${serviceActionState.action}`, { method: "POST", body: JSON.stringify({}) }); setServiceActionState(null); await loadServices(); }
    catch (err) { setServicesError(err instanceof APIError ? err.message : "Failed to run action"); }
    finally { setServicesBusy(false); }
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
      .map((s) => { const t = new Date(s.timestamp); return Number.isNaN(t.getTime()) ? null : { timestamp: t, cpu: clampPercent(s.cpu_usage_percent), ram: clampPercent(s.memory_used_percent), download: Math.max(0, s.network_rx_bps || 0), upload: Math.max(0, s.network_tx_bps || 0) }; })
      .filter((x): x is HistoryTrendPoint => Boolean(x))
      .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
  }, [historyItems]);

  const trafficUsageBars = useMemo<TrafficUsageBarPoint[]>(() => {
    if (!historyPoints.length) return [];
    const bucketMs = 60_000;
    const bucketed = new Map<number, TrafficUsageBarPoint>();
    for (let i = 0; i < historyPoints.length; i++) {
      const pt = historyPoints[i];
      const ms = pt.timestamp.getTime();
      const prevMs = i > 0 ? historyPoints[i - 1].timestamp.getTime() : ms - 5_000;
      const dt = Math.max(1, (ms - prevMs) / 1000);
      const key = Math.floor(ms / bucketMs) * bucketMs;
      const cur = bucketed.get(key);
      if (cur) { cur.download_bytes += Math.max(0, pt.download) * dt; cur.upload_bytes += Math.max(0, pt.upload) * dt; }
      else { bucketed.set(key, { timestamp: new Date(key), download_bytes: Math.max(0, pt.download) * dt, upload_bytes: Math.max(0, pt.upload) * dt }); }
    }
    return [...bucketed.values()].sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
  }, [historyPoints]);

  return (
    <div className="space-y-8">
      <PageHeader title="Dashboard" />

      {showInitialLoading && (
        <motion.div initial={{ opacity: 0, y: -4 }} animate={{ opacity: 1, y: 0 }} className="rounded-xl border border-accent/20 bg-accent/8 px-5 py-3.5 text-[14px] text-accent-light">
          Loading latest dashboard metrics...
        </motion.div>
      )}
      {error && <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{error}</div>}
      {warningMessages.length > 0 && (
        <div className="rounded-xl border border-status-warning/20 bg-status-warning/8 px-5 py-3.5 text-[14px] text-status-warning">{warningMessages.join(" | ")}</div>
      )}

      {/* ── Primary metrics ── */}
      <motion.div variants={{ hidden: {}, show: { transition: { staggerChildren: 0.07 } } }} initial="hidden" animate="show" className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">

        {/* CPU */}
        <motion.div variants={{ hidden: { opacity: 0, y: 16 }, show: { opacity: 1, y: 0 } }} className="group relative overflow-hidden rounded-2xl border border-border/70 bg-surface-2 p-5 transition-colors hover:border-accent/25">
          <div className="absolute -right-8 -top-8 h-24 w-24 rounded-full bg-accent/5 transition-all group-hover:bg-accent/10" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">CPU</p>
              <p className="mt-2 text-metric text-txt-primary">
                <AnimatedNumber value={cpuPercent} format={(v) => v.toFixed(1)} />
                <span className="ml-1 text-[16px] font-medium text-txt-tertiary">%</span>
              </p>
              <p className="mt-2 text-[13px] text-txt-secondary">System load</p>
            </div>
            <ProgressRing value={cpuPercent} color="var(--data-2)" />
          </div>
        </motion.div>

        {/* RAM */}
        <motion.div variants={{ hidden: { opacity: 0, y: 16 }, show: { opacity: 1, y: 0 } }} className="group relative overflow-hidden rounded-2xl border border-border/70 bg-surface-2 p-5 transition-colors hover:border-accent-secondary/25">
          <div className="absolute -right-8 -top-8 h-24 w-24 rounded-full bg-accent-secondary/5 transition-all group-hover:bg-accent-secondary/10" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">RAM</p>
              <p className="mt-2 text-metric text-txt-primary">
                <AnimatedNumber value={ramPercent} format={(v) => v.toFixed(1)} />
                <span className="ml-1 text-[16px] font-medium text-txt-tertiary">%</span>
              </p>
              <p className="mt-2 text-[13px] text-txt-secondary">Memory usage</p>
            </div>
            <ProgressRing value={ramPercent} color="var(--data-4)" />
          </div>
        </motion.div>

        {/* Online */}
        <motion.div variants={{ hidden: { opacity: 0, y: 16 }, show: { opacity: 1, y: 0 } }} className="group relative overflow-hidden rounded-2xl border border-border/70 bg-surface-2 p-5 transition-colors hover:border-status-success/25">
          <div className="absolute -right-8 -top-8 h-24 w-24 rounded-full bg-status-success/5 transition-all group-hover:bg-status-success/10" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Online</p>
              <p className="mt-2 text-metric text-txt-primary"><AnimatedNumber value={onlineUsers} /></p>
              <p className="mt-2 text-[13px] text-txt-secondary">Connected users</p>
            </div>
            <div className="grid h-[52px] w-[52px] place-items-center rounded-full bg-status-success/10">
              <Users2 size={22} strokeWidth={1.6} className="text-status-success" />
            </div>
          </div>
        </motion.div>

        {/* Uptime */}
        <motion.div variants={{ hidden: { opacity: 0, y: 16 }, show: { opacity: 1, y: 0 } }} className="group relative overflow-hidden rounded-2xl border border-border/70 bg-surface-2 p-5 transition-colors hover:border-status-warning/25">
          <div className="absolute -right-8 -top-8 h-24 w-24 rounded-full bg-status-warning/5 transition-all group-hover:bg-status-warning/10" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Uptime</p>
              <p className="mt-2 text-metric text-txt-primary">{uptime}</p>
              <p className="mt-2 text-[13px] text-txt-secondary">Current session</p>
            </div>
            <div className="grid h-[52px] w-[52px] place-items-center rounded-full bg-status-warning/10">
              <Clock size={22} strokeWidth={1.6} className="text-status-warning" />
            </div>
          </div>
        </motion.div>
      </motion.div>

      {/* ── Secondary stats ── */}
      <motion.div variants={{ hidden: {}, show: { transition: { staggerChildren: 0.05 } } }} initial="hidden" animate="show" className="grid gap-4 sm:grid-cols-3">
        <motion.div variants={{ hidden: { opacity: 0, y: 12 }, show: { opacity: 1, y: 0 } }} className="flex items-center gap-4 rounded-2xl border border-border/70 bg-surface-2 p-5">
          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-accent/10">
            <Network size={22} strokeWidth={1.6} className="text-accent-light" />
          </div>
          <div>
            <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Network</p>
            <div className="mt-1.5 flex items-center gap-4 text-[15px] font-semibold text-txt-primary">
              <span className="inline-flex items-center gap-1.5"><ArrowDownToLine size={14} strokeWidth={1.8} className="text-accent-light" />{formatRate(networkRx)}</span>
              <span className="inline-flex items-center gap-1.5"><ArrowUpFromLine size={14} strokeWidth={1.8} className="text-accent-secondary-light" />{formatRate(networkTx)}</span>
            </div>
          </div>
        </motion.div>

        <motion.div variants={{ hidden: { opacity: 0, y: 12 }, show: { opacity: 1, y: 0 } }} className="flex items-center gap-4 rounded-2xl border border-border/70 bg-surface-2 p-5">
          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-accent-secondary/10">
            <Globe size={22} strokeWidth={1.6} className="text-accent-secondary-light" />
          </div>
          <div>
            <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Total Traffic</p>
            <p className="mt-1.5 text-[15px] font-semibold text-txt-primary">{formatBytes(totalTraffic)}</p>
          </div>
        </motion.div>

        <motion.div variants={{ hidden: { opacity: 0, y: 12 }, show: { opacity: 1, y: 0 } }} className="flex items-center gap-4 rounded-2xl border border-border/70 bg-surface-2 p-5">
          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-accent/10">
            <Zap size={22} strokeWidth={1.6} className="text-accent-light" />
          </div>
          <div>
            <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Connections</p>
            <div className="mt-1.5 flex items-center gap-3 text-[15px] font-semibold text-txt-primary">
              <span>TCP {tcpConnections}</span>
              <span className="text-txt-muted">/</span>
              <span>UDP {udpConnections}</span>
            </div>
          </div>
        </motion.div>
      </motion.div>

      {/* ── Charts ── */}
      <div className="space-y-4">
        <SectionHeader icon={<TrendingUp size={18} strokeWidth={1.6} />} title="Traffic Trends">
          <div className="inline-flex rounded-xl bg-surface-3/50 p-1 text-[13px]">
            {(["1h", "24h"] as HistoryWindow[]).map((w) => (
              <button key={w} type="button" onClick={() => setHistoryWindow(w)}
                className={cn("rounded-lg px-4 py-1.5 font-semibold transition-all", historyWindow === w ? "bg-accent text-white shadow-sm shadow-accent/25" : "text-txt-secondary hover:text-txt")}>
                {w}
              </button>
            ))}
          </div>
        </SectionHeader>

        {historyLoading && !historyPoints.length && (
          <div className="rounded-xl border border-accent/20 bg-accent/8 px-5 py-3.5 text-[14px] text-accent-light">Loading system history...</div>
        )}
        {historyError && <div className="rounded-xl border border-status-warning/20 bg-status-warning/8 px-5 py-3.5 text-[14px] text-status-warning">{historyError}</div>}

        <div className="rounded-2xl border border-border/70 bg-surface-2 p-6">
          <div className="mb-4 flex items-center gap-5 text-[13px]">
            <span className="inline-flex items-center gap-2 text-txt-secondary"><span className="h-2.5 w-2.5 rounded-full bg-accent" />Upload</span>
            <span className="inline-flex items-center gap-2 text-txt-secondary"><span className="h-2.5 w-2.5 rounded-full bg-accent-secondary" />Download</span>
          </div>
          <div className="h-[280px]">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={historyPoints}>
                <defs>
                  <linearGradient id="upG" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="var(--data-2)" stopOpacity={0.25} /><stop offset="100%" stopColor="var(--data-2)" stopOpacity={0} /></linearGradient>
                  <linearGradient id="dnG" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="var(--data-1)" stopOpacity={0.2} /><stop offset="100%" stopColor="var(--data-1)" stopOpacity={0} /></linearGradient>
                </defs>
                <CartesianGrid stroke="var(--border)" strokeDasharray="4 4" vertical={false} />
                <XAxis dataKey="timestamp" tickFormatter={(v) => formatShortTime(new Date(v))} tick={{ fill: "var(--txt-icon)", fontSize: 12 }} tickLine={false} axisLine={{ stroke: "var(--border)" }} />
                <YAxis tickFormatter={(v) => formatBytes(Number(v))} tick={{ fill: "var(--txt-icon)", fontSize: 12 }} tickLine={false} axisLine={false} width={60} />
                <Tooltip labelFormatter={(v) => formatDateTime(v instanceof Date ? v.toISOString() : String(v))} formatter={(v: number) => formatRate(Number(v))} contentStyle={tooltipStyle} cursor={{ stroke: "var(--primary-soft)", strokeWidth: 1 }} />
                <Area type="monotone" dataKey="upload" stroke="var(--data-2)" fill="url(#upG)" strokeWidth={2} dot={false} activeDot={{ r: 4, fill: "var(--data-2)", stroke: "var(--surface-2)", strokeWidth: 2 }} name="Upload" />
                <Area type="monotone" dataKey="download" stroke="var(--data-1)" fill="url(#dnG)" strokeWidth={2} dot={false} activeDot={{ r: 4, fill: "var(--data-1)", stroke: "var(--surface-2)", strokeWidth: 2 }} name="Download" />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="rounded-2xl border border-border/70 bg-surface-2 p-6">
          <div className="mb-4 flex items-center gap-5 text-[13px]">
            <span className="inline-flex items-center gap-2 text-txt-secondary"><span className="h-2.5 w-2.5 rounded-sm bg-accent" />Download</span>
            <span className="inline-flex items-center gap-2 text-txt-secondary"><span className="h-2.5 w-2.5 rounded-sm bg-accent-secondary" />Upload</span>
          </div>
          <div className="h-[220px]">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trafficUsageBars} barGap={2}>
                <CartesianGrid stroke="var(--border)" strokeDasharray="4 4" vertical={false} />
                <XAxis dataKey="timestamp" tickFormatter={(v) => formatShortTime(new Date(v))} tick={{ fill: "var(--txt-icon)", fontSize: 12 }} tickLine={false} axisLine={{ stroke: "var(--border)" }} />
                <YAxis tickFormatter={(v) => formatBytes(Number(v))} tick={{ fill: "var(--txt-icon)", fontSize: 12 }} tickLine={false} axisLine={false} width={60} />
                <Tooltip formatter={(v: number) => formatBytes(Number(v))} contentStyle={tooltipStyle} cursor={{ fill: "var(--accent-soft)" }} />
                <Bar dataKey="download_bytes" fill="var(--data-2)" radius={[4, 4, 0, 0]} name="Download" />
                <Bar dataKey="upload_bytes" fill="var(--data-4)" radius={[4, 4, 0, 0]} name="Upload" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {/* ── Live & Snapshot ── */}
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-2xl border border-border/70 bg-surface-2 p-6">
          <SectionHeader icon={<Activity size={18} strokeWidth={1.6} />} title="Live Feed" />
          <div className="mt-5 space-y-3">
            {(live?.services || []).length ? (live?.services || []).map((item, i) => (
              <motion.div key={`${item.service_name}-${i}`} initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: "auto" }} className="flex items-start gap-3 border-b border-border/30 pb-3 last:border-0">
                <span className={cn("mt-2 h-2 w-2 shrink-0 rounded-full", statusColor(item.status))} />
                <div>
                  <p className="text-[14px]"><span className="font-semibold text-txt-primary">{item.service_name}</span><span className="ml-2 text-txt-tertiary">{item.status}</span></p>
                  <p className="mt-0.5 text-[12px] text-txt-muted">{formatDateTime(item.last_check_at)}</p>
                </div>
              </motion.div>
            )) : <p className="py-6 text-center text-[14px] text-txt-secondary">No recent events.</p>}
          </div>
        </div>

        <div className="rounded-2xl border border-border/70 bg-surface-2 p-6">
          <SectionHeader icon={<HardDrive size={18} strokeWidth={1.6} />} title="Service Snapshot" />
          <div className="mt-5">
            <TableContainer className="border-0 bg-transparent shadow-none">
              <Table>
                <TableHeader><TableRow className="border-t-0 hover:bg-transparent"><TableHead>Service</TableHead><TableHead>Status</TableHead><TableHead>Checked</TableHead></TableRow></TableHeader>
                <TableBody>
                  {(live?.services || []).map((item) => (
                    <TableRow key={item.service_name} className="cursor-default">
                      <TableCell className="font-medium">{item.service_name}</TableCell>
                      <TableCell>
                        <span className={cn("inline-flex items-center gap-2", item.status.includes("running") ? "text-status-success" : "text-status-warning")}>
                          <span className={cn("h-2 w-2 rounded-full", statusColor(item.status))} />{item.status}
                        </span>
                      </TableCell>
                      <TableCell className="text-txt-secondary">{formatDateTime(item.last_check_at)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          </div>
        </div>
      </div>

      {/* ── Managed services ── */}
      <div className="space-y-4">
        <SectionHeader icon={<Cpu size={18} strokeWidth={1.6} />} title="Managed Services" />
        {servicesError && <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{servicesError}</div>}
        {servicesLoading ? (
          <div className="flex min-h-[180px] items-center justify-center rounded-2xl border border-border/70 bg-surface-2">
            <div className="flex flex-col items-center gap-3">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent/20 border-t-accent-light" />
              <p className="text-[14px] text-txt-secondary">Loading services...</p>
            </div>
          </div>
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            {serviceItems.length ? serviceItems.map((item) => (
              <motion.div key={item.service_name} whileHover={{ y: -3 }} transition={{ duration: 0.15 }} className="group relative overflow-hidden rounded-2xl border border-border/70 bg-surface-2 p-6 transition-colors hover:border-accent/20">
                <div className="absolute left-0 right-0 top-0 h-[2px] bg-gradient-to-r from-accent to-accent-secondary opacity-50 transition-opacity group-hover:opacity-100" />
                <div className="mb-4 flex items-center justify-between">
                  <h4 className="text-[15px] font-bold text-txt-primary">{item.service_name}</h4>
                  <span className="inline-flex items-center gap-2 text-[13px] text-txt-secondary">
                    <span className={cn("h-2 w-2 rounded-full", statusColor(item.status || "unknown"))} />{(item.status || "unknown").toLowerCase()}
                  </span>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div><p className="text-[12px] font-medium text-txt-muted">Version</p><p className="mt-1 text-[14px] font-medium text-txt">{item.version || "-"}</p></div>
                  <div><p className="text-[12px] font-medium text-txt-muted">Last check</p><p className="mt-1 text-[14px] font-medium text-txt">{formatDateTime(item.last_check_at)}</p></div>
                </div>
                <div className="mt-5 flex items-center gap-3 border-t border-border/40 pt-4">
                  <Button size="sm" onClick={() => void openServiceDetails(item.service_name)} disabled={servicesBusy}><Eye size={16} strokeWidth={1.6} />Details</Button>
                  <Button size="sm" onClick={() => setServiceActionState({ name: item.service_name, action: "reload" })} disabled={servicesBusy}><RefreshCw size={16} strokeWidth={1.6} />Reload</Button>
                  <Button size="sm" onClick={() => setServiceActionState({ name: item.service_name, action: "restart" })} disabled={servicesBusy}><RotateCcw size={16} strokeWidth={1.6} />Restart</Button>
                </div>
              </motion.div>
            )) : <div className="rounded-2xl border border-border/70 bg-surface-2 p-6 text-[14px] text-txt-secondary">Service activity is not available yet.</div>}
          </div>
        )}
      </div>

      {/* Dialogs */}
      <Dialog open={serviceDetailsOpen} onOpenChange={(n) => { if (!n) setServiceDetailsOpen(false); }} title={`${serviceDetails?.name || "Service"} details`} contentClassName="max-w-[760px]"
        footer={<Button onClick={() => setServiceDetailsOpen(false)}>Close</Button>}>
        {serviceDetails && (
          <div className="space-y-4">
            <div className="grid grid-cols-3 gap-3">
              <div className="rounded-xl bg-surface-0/50 p-4"><p className="text-[12px] font-medium text-txt-muted">Status</p><p className="mt-1.5 text-[14px] font-medium text-txt">{serviceDetails.status_text}</p></div>
              <div className="rounded-xl bg-surface-0/50 p-4"><p className="text-[12px] font-medium text-txt-muted">Active</p><p className="mt-1.5 text-[14px] font-medium text-txt">{serviceDetails.active} / {serviceDetails.sub_state}</p></div>
              <div className="rounded-xl bg-surface-0/50 p-4"><p className="text-[12px] font-medium text-txt-muted">PID</p><p className="mt-1.5 text-[14px] font-medium text-txt">{serviceDetails.main_pid || 0}</p></div>
            </div>
            <p className="text-[13px] text-txt-muted">Checked: {formatDateTime(serviceDetails.checked_at)}</p>
            <div>
              <p className="mb-2 text-[14px] font-semibold text-txt">Recent logs</p>
              <pre className="m-0 max-h-[320px] overflow-auto rounded-xl border border-border/50 bg-surface-0 p-4 font-mono text-[13px] leading-6 text-txt-secondary">
                {serviceDetails.last_logs?.length ? serviceDetails.last_logs.join("\n") : "No logs available"}
              </pre>
            </div>
          </div>
        )}
      </Dialog>

      <ConfirmDialog open={Boolean(serviceActionState)} title="Confirm service action"
        description={`${serviceActionState?.action === "restart" ? "Restart" : "Reload"} ${serviceActionState?.name || "service"} now?`}
        busy={servicesBusy} confirmText="Confirm" onClose={() => setServiceActionState(null)} onConfirm={() => void runServiceAction()} />
    </div>
  );
}
