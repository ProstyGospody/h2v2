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
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
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
import { Button, Dialog, cn } from "@/src/components/ui";
import { formatBytes, formatDateTime, formatRate, formatUptime } from "@/utils/format";

const LIVE_POLL_MS = 5000;
const HISTORY_POLL_MS = 15000;
const HISTORY_LIMIT = 20000;

type ActionState = { name: string; action: "restart" | "reload" } | null;
type HistoryWindow = "1h" | "24h";

type HistoryTrendPoint = {
  timestamp: Date;
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
      .map((s) => { const t = new Date(s.timestamp); return Number.isNaN(t.getTime()) ? null : { timestamp: t, download: Math.max(0, s.network_rx_bps || 0), upload: Math.max(0, s.network_tx_bps || 0) }; })
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
        <motion.div initial={{ opacity: 0, y: -4 }} animate={{ opacity: 1, y: 0 }} className="rounded-xl border border-status-info/20 bg-status-info/8 px-5 py-3.5 text-[14px] text-status-info">
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
        <motion.div variants={{ hidden: { opacity: 0, y: 16 }, show: { opacity: 1, y: 0 } }} className="metric-glow card-hover gradient-border rounded-2xl border border-border/30 bg-surface-2 p-5" style={{ "--metric-glow-color": "var(--data-2)" } as React.CSSProperties}>
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">CPU</p>
              <p className="mt-2 text-metric text-txt-primary">
                <AnimatedNumber value={cpuPercent} format={(v) => v.toFixed(1)} />
                <span className="ml-1 text-[16px] font-medium text-txt-tertiary">%</span>
              </p>
            </div>
            <ProgressRing value={cpuPercent} color="var(--data-2)" />
          </div>
        </motion.div>

        {/* RAM */}
        <motion.div variants={{ hidden: { opacity: 0, y: 16 }, show: { opacity: 1, y: 0 } }} className="metric-glow card-hover gradient-border rounded-2xl border border-border/30 bg-surface-2 p-5" style={{ "--metric-glow-color": "var(--data-4)" } as React.CSSProperties}>
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">RAM</p>
              <p className="mt-2 text-metric text-txt-primary">
                <AnimatedNumber value={ramPercent} format={(v) => v.toFixed(1)} />
                <span className="ml-1 text-[16px] font-medium text-txt-tertiary">%</span>
              </p>
            </div>
            <ProgressRing value={ramPercent} color="var(--data-4)" />
          </div>
        </motion.div>

        {/* Online */}
        <motion.div variants={{ hidden: { opacity: 0, y: 16 }, show: { opacity: 1, y: 0 } }} className="metric-glow card-hover gradient-border rounded-2xl border border-border/30 bg-surface-2 p-5" style={{ "--metric-glow-color": "var(--status-success)" } as React.CSSProperties}>
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Online</p>
              <p className="mt-2 text-metric text-txt-primary"><AnimatedNumber value={onlineUsers} /></p>
            </div>
            <div className="grid h-[52px] w-[52px] place-items-center rounded-full bg-status-success/10 ring-1 ring-status-success/15">
              <Users2 size={22} strokeWidth={1.6} className="text-status-success" />
            </div>
          </div>
        </motion.div>

        {/* Uptime */}
        <motion.div variants={{ hidden: { opacity: 0, y: 16 }, show: { opacity: 1, y: 0 } }} className="metric-glow card-hover gradient-border rounded-2xl border border-border/30 bg-surface-2 p-5" style={{ "--metric-glow-color": "var(--status-warning)" } as React.CSSProperties}>
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Uptime</p>
              <p className="mt-2 text-[28px] leading-none text-txt-primary sm:text-metric">{uptime}</p>
            </div>
            <div className="grid h-[52px] w-[52px] place-items-center rounded-full bg-status-warning/10 ring-1 ring-status-warning/15">
              <Clock size={22} strokeWidth={1.6} className="text-status-warning" />
            </div>
          </div>
        </motion.div>
      </motion.div>

      {/* ── Secondary stats ── */}
      <motion.div variants={{ hidden: {}, show: { transition: { staggerChildren: 0.05 } } }} initial="hidden" animate="show" className="grid gap-4 sm:grid-cols-3">
        <motion.div variants={{ hidden: { opacity: 0, y: 12 }, show: { opacity: 1, y: 0 } }} className="card-hover flex items-center gap-4 rounded-2xl border border-border/30 bg-surface-2 p-5">
          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-surface-3/60 ring-1 ring-border/20">
            <Network size={22} strokeWidth={1.6} className="text-txt-secondary" />
          </div>
          <div>
            <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Network</p>
            <div className="mt-1.5 flex flex-wrap items-center gap-3 text-[15px] font-semibold text-txt-primary">
              <span className="inline-flex items-center gap-1.5"><ArrowDownToLine size={14} strokeWidth={1.8} className="text-status-success" />{formatRate(networkRx)}</span>
              <span className="inline-flex items-center gap-1.5"><ArrowUpFromLine size={14} strokeWidth={1.8} className="text-status-warning" />{formatRate(networkTx)}</span>
            </div>
          </div>
        </motion.div>

        <motion.div variants={{ hidden: { opacity: 0, y: 12 }, show: { opacity: 1, y: 0 } }} className="card-hover flex items-center gap-4 rounded-2xl border border-border/30 bg-surface-2 p-5">
          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-accent-secondary/10 ring-1 ring-accent-secondary/15">
            <Globe size={22} strokeWidth={1.6} className="text-accent-secondary-light" />
          </div>
          <div>
            <p className="text-[12px] font-semibold uppercase tracking-wider text-txt-muted">Total Traffic</p>
            <p className="mt-1.5 text-[15px] font-semibold text-txt-primary">{formatBytes(totalTraffic)}</p>
          </div>
        </motion.div>

        <motion.div variants={{ hidden: { opacity: 0, y: 12 }, show: { opacity: 1, y: 0 } }} className="card-hover flex items-center gap-4 rounded-2xl border border-border/30 bg-surface-2 p-5">
          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-accent/10 ring-1 ring-accent/15">
            <Zap size={22} strokeWidth={1.6} className="text-accent" />
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

        {historyLoading && !historyPoints.length && (
          <div className="rounded-xl border border-status-info/20 bg-status-info/8 px-5 py-3.5 text-[14px] text-status-info">Loading system history...</div>
        )}
        {historyError && <div className="rounded-xl border border-status-warning/20 bg-status-warning/8 px-5 py-3.5 text-[14px] text-status-warning">{historyError}</div>}

        <div className="rounded-2xl border border-border/30 bg-surface-2 p-4 sm:p-6">
          <div className="mb-4 flex flex-wrap items-center gap-4 text-[13px]">
            <span className="inline-flex items-center gap-2 text-txt-secondary"><span className="h-2.5 w-2.5 rounded-sm bg-accent" />Download</span>
            <span className="inline-flex items-center gap-2 text-txt-secondary"><span className="h-2.5 w-2.5 rounded-sm bg-accent-secondary" />Upload</span>
          </div>
          <div className="h-[300px]">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trafficUsageBars} barGap={4} barCategoryGap="25%">
                <CartesianGrid stroke="var(--border)" strokeDasharray="4 4" vertical={false} />
                <XAxis dataKey="timestamp" tickFormatter={(v) => formatShortTime(new Date(v))} tick={{ fill: "var(--txt-icon)", fontSize: 12 }} tickLine={false} axisLine={{ stroke: "var(--border)" }} />
                <YAxis tickFormatter={(v) => formatBytes(Number(v))} tick={{ fill: "var(--txt-icon)", fontSize: 12 }} tickLine={false} axisLine={false} width={58} />
                <Tooltip formatter={(v: number) => formatBytes(Number(v))} contentStyle={tooltipStyle} cursor={{ fill: "var(--accent-soft)" }} />
                <Bar dataKey="download_bytes" fill="var(--data-2)" radius={[5, 5, 0, 0]} name="Download" animationDuration={420} />
                <Bar dataKey="upload_bytes" fill="var(--data-4)" radius={[5, 5, 0, 0]} name="Upload" animationDuration={520} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>


      {/* ── Managed services ── */}
      <div className="space-y-2.5">
        <SectionHeader icon={<Cpu size={18} strokeWidth={1.6} />} title="Managed Services" />
        {servicesError && <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{servicesError}</div>}
        {servicesLoading ? (
          <div className="flex min-h-[120px] items-center justify-center rounded-2xl bg-surface-2">
            <div className="flex flex-col items-center gap-3">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent/20 border-t-accent-light" />
              <p className="text-[14px] text-txt-secondary">Loading services...</p>
            </div>
          </div>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {serviceItems.length ? serviceItems.map((item) => (
              <motion.div key={item.service_name} className="card-hover relative overflow-hidden rounded-2xl border border-border/30 bg-surface-2 p-4">
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
                  <Button size="sm" className="h-8 min-w-[96px] flex-1 px-3 sm:flex-none" onClick={() => setServiceActionState({ name: item.service_name, action: "reload" })} disabled={servicesBusy}><RefreshCw size={15} strokeWidth={1.6} />Reload</Button>
                  <Button size="sm" className="h-8 min-w-[96px] flex-1 px-3 sm:flex-none" onClick={() => setServiceActionState({ name: item.service_name, action: "restart" })} disabled={servicesBusy}><RotateCcw size={15} strokeWidth={1.6} />Restart</Button>
                </div>
              </motion.div>
            )) : <div className="rounded-2xl bg-surface-2 p-6 text-[14px] text-txt-secondary">Service activity is not available yet.</div>}
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
