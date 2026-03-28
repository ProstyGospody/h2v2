import { animate, motion, useMotionValue, useTransform } from "framer-motion";
import {
  Activity,
  Cpu,
  Eye,
  HardDrive,
  Loader2,
  RefreshCw,
  RotateCcw,
  Users,
  Wifi,
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

  const metricCards = [
    {
      label: "CPU",
      icon: <Cpu size={16} strokeWidth={1.4} className="text-accent-light" />,
      value: <AnimatedNumber value={cpuPercent} format={(value) => `${value.toFixed(1)}%`} />,
      sub: "System load",
      dot: "bg-accent/3",
    },
    {
      label: "RAM",
      icon: <HardDrive size={16} strokeWidth={1.4} className="text-accent-secondary-light" />,
      value: <AnimatedNumber value={ramPercent} format={(value) => `${value.toFixed(1)}%`} />,
      sub: "Memory usage",
      dot: "bg-accent-secondary/3",
    },
    {
      label: "ONLINE",
      icon: <Users size={16} strokeWidth={1.4} className="text-status-success" />,
      value: <AnimatedNumber value={onlineUsers} />,
      sub: "Connected users",
      dot: "bg-status-success/5",
    },
    {
      label: "NETWORK",
      icon: <Wifi size={16} strokeWidth={1.4} className="text-accent-light" />,
      value: `RX ${formatRate(networkRx)}`,
      sub: `TX ${formatRate(networkTx)}`,
      dot: "bg-accent/3",
    },
    {
      label: "UPTIME",
      icon: <Activity size={16} strokeWidth={1.4} className="text-status-warning" />,
      value: uptime,
      sub: "Current session",
      dot: "bg-status-warning/5",
    },
    {
      label: "TRAFFIC",
      icon: <HardDrive size={16} strokeWidth={1.4} className="text-accent-light" />,
      value: formatBytes(totalTraffic),
      sub: "Total transfer",
      dot: "bg-accent-secondary/3",
    },
    {
      label: "PACKETS",
      icon: <Activity size={16} strokeWidth={1.4} className="text-accent-secondary-light" />,
      value: `TCP ${tcpConnections}`,
      sub: `UDP ${udpConnections}`,
      dot: "bg-accent/3",
    },
  ];

  return (
    <div className="space-y-4">
      <PageHeader title="Dashboard" />

      {showInitialLoading ? (
        <div className="rounded-btn border border-accent/20 bg-accent/10 px-3 py-2 text-[12px] text-accent-light">
          Loading latest dashboard metrics...
        </div>
      ) : null}
      {error ? <div className="rounded-btn border border-status-danger/20 bg-status-danger/10 px-3 py-2 text-[12px] text-status-danger">{error}</div> : null}
      {warningMessages.length ? (
        <div className="rounded-btn border border-status-warning/20 bg-status-warning/10 px-3 py-2 text-[12px] text-status-warning">
          {warningMessages.join(" | ")}
        </div>
      ) : null}

      <motion.div
        variants={{ hidden: {}, show: { transition: { staggerChildren: 0.05 } } }}
        initial="hidden"
        animate="show"
        className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
      >
        {metricCards.map((card) => (
          <motion.div
            key={card.label}
            variants={{ hidden: { opacity: 0, y: 12 }, show: { opacity: 1, y: 0 } }}
            className="relative overflow-hidden rounded-card border border-border bg-surface-2 p-4"
          >
            <div className={`absolute -right-5 -top-5 h-16 w-16 rounded-full ${card.dot}`} />
            <div className="mb-2 flex items-center justify-between gap-2">
              <p className="text-[11px] font-medium text-txt-tertiary">{card.label}</p>
              {card.icon}
            </div>
            <p className="text-metric text-white">{card.value}</p>
            <p className="mt-2 text-[11px] text-txt-muted">{card.sub}</p>
          </motion.div>
        ))}
      </motion.div>

      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-[13px] font-semibold text-white">Traffic trends</h3>
          <div className="inline-flex rounded-btn bg-surface-4 p-0.5 text-[11px]">
            <button
              type="button"
              className={cn(
                "rounded-btn px-2.5 py-1 text-txt-secondary transition-colors",
                historyWindow === "1h" && "bg-accent text-white",
              )}
              onClick={() => setHistoryWindow("1h")}
            >
              1h
            </button>
            <button
              type="button"
              className={cn(
                "rounded-btn px-2.5 py-1 text-txt-secondary transition-colors",
                historyWindow === "24h" && "bg-accent text-white",
              )}
              onClick={() => setHistoryWindow("24h")}
            >
              24h
            </button>
          </div>
        </div>

        {historyLoading && !historyPoints.length ? (
          <div className="rounded-btn border border-accent/20 bg-accent/10 px-3 py-2 text-[12px] text-accent-light">Loading system history...</div>
        ) : null}
        {historyError ? <div className="rounded-btn border border-status-warning/20 bg-status-warning/10 px-3 py-2 text-[12px] text-status-warning">{historyError}</div> : null}

        <div className="rounded-card border border-border bg-surface-2 p-5">
          <div className="h-[280px]">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={historyPoints}>
                <defs>
                  <linearGradient id="uploadGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#6366f1" stopOpacity={0.3} />
                    <stop offset="100%" stopColor="#6366f1" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="downloadGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#8b5cf6" stopOpacity={0.25} />
                    <stop offset="100%" stopColor="#8b5cf6" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke="#1e1e24" vertical={false} />
                <XAxis
                  dataKey="timestamp"
                  tickFormatter={(value) => formatShortTime(new Date(value))}
                  tick={{ fill: "#52525a", fontSize: 9 }}
                  tickLine={false}
                  axisLine={{ stroke: "#1e1e24" }}
                />
                <YAxis
                  tickFormatter={(value) => formatBytes(Number(value))}
                  tick={{ fill: "#52525a", fontSize: 9 }}
                  tickLine={false}
                  axisLine={{ stroke: "#1e1e24" }}
                />
                <Tooltip
                  labelFormatter={(value) => formatDateTime(value instanceof Date ? value.toISOString() : String(value))}
                  formatter={(value: number) => formatRate(Number(value))}
                  contentStyle={{ backgroundColor: "#18181c", border: "1px solid #1e1e24", borderRadius: 10, color: "#e4e4e7", fontSize: 11 }}
                />
                <Area type="monotone" dataKey="upload" stroke="#6366f1" fill="url(#uploadGradient)" strokeWidth={2} name="Upload" />
                <Area type="monotone" dataKey="download" stroke="#8b5cf6" fill="url(#downloadGradient)" strokeWidth={2} name="Download" />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="rounded-card border border-border bg-surface-2 p-5">
          <div className="h-[220px]">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trafficUsageBars}>
                <CartesianGrid stroke="#1e1e24" vertical={false} />
                <XAxis
                  dataKey="timestamp"
                  tickFormatter={(value) => formatShortTime(new Date(value))}
                  tick={{ fill: "#52525a", fontSize: 9 }}
                  tickLine={false}
                  axisLine={{ stroke: "#1e1e24" }}
                />
                <YAxis
                  tickFormatter={(value) => formatBytes(Number(value))}
                  tick={{ fill: "#52525a", fontSize: 9 }}
                  tickLine={false}
                  axisLine={{ stroke: "#1e1e24" }}
                />
                <Tooltip
                  formatter={(value: number) => formatBytes(Number(value))}
                  contentStyle={{ backgroundColor: "#18181c", border: "1px solid #1e1e24", borderRadius: 10, color: "#e4e4e7", fontSize: 11 }}
                />
                <Bar dataKey="download_bytes" fill="#6366f1" radius={[4, 4, 0, 0]} />
                <Bar dataKey="upload_bytes" fill="#8b5cf6" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        <div className="rounded-card border border-border bg-surface-2 p-5">
          <h3 className="mb-3 text-[13px] font-semibold text-white">Live feed</h3>
          <div className="space-y-2">
            {(live?.services || []).length ? (
              (live?.services || []).map((item, index) => (
                <motion.div
                  key={`${item.service_name}-${index}`}
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: "auto" }}
                  className="flex items-start gap-[10px] border-b border-border pb-2 last:border-0"
                >
                  <span className={cn("mt-1 h-[6px] w-[6px] rounded-full", statusColor(item.status))} />
                  <div>
                    <p className="text-[12px]">
                      <span className="font-semibold text-white">{item.service_name}</span>
                      <span className="ml-1 text-txt-tertiary">status is {item.status}</span>
                    </p>
                    <p className="text-[10px] text-txt-muted">{formatDateTime(item.last_check_at)}</p>
                  </div>
                </motion.div>
              ))
            ) : (
              <p className="text-[12px] text-txt-secondary">No recent events.</p>
            )}
          </div>
        </div>

        <div className="rounded-card border border-border bg-surface-2 p-5">
          <h3 className="mb-3 text-[13px] font-semibold text-white">Service snapshot</h3>
          <TableContainer className="bg-surface-1">
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
                    <TableCell>{item.service_name}</TableCell>
                    <TableCell>
                      <span className={cn("inline-flex items-center gap-2 text-[11px]", item.status.includes("running") ? "text-status-success" : "text-status-warning")}>
                        <span className={cn("h-[6px] w-[6px] rounded-full", statusColor(item.status))} />
                        {item.status}
                      </span>
                    </TableCell>
                    <TableCell>-</TableCell>
                    <TableCell>{formatDateTime(item.last_check_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </div>
      </div>

      <div className="space-y-3">
        <h3 className="text-[13px] font-semibold text-white">Managed services</h3>
        {servicesError ? <div className="rounded-btn border border-status-danger/20 bg-status-danger/10 px-3 py-2 text-[12px] text-status-danger">{servicesError}</div> : null}
        {servicesLoading ? (
          <div className="flex min-h-[160px] items-center justify-center rounded-card border border-border bg-surface-2">
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
                  className="relative overflow-hidden rounded-card border border-border bg-surface-2 p-5"
                >
                  <div className="absolute left-0 right-0 top-0 h-[2px] bg-gradient-to-r from-accent-secondary to-accent-secondary-light" />
                  <div className="mb-3 flex items-center justify-between">
                    <h4 className="text-[13px] font-semibold text-white">{item.service_name}</h4>
                    <span className="inline-flex items-center gap-2 text-[11px] text-txt-secondary">
                      <span className={cn("h-[7px] w-[7px] rounded-full", statusColor(item.status || "unknown"))} />
                      {(item.status || "unknown").toLowerCase()}
                    </span>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <p className="text-[11px] text-txt-muted">Version</p>
                      <p className="text-[12px] font-medium text-txt">{item.version || "-"}</p>
                    </div>
                    <div>
                      <p className="text-[11px] text-txt-muted">Last check</p>
                      <p className="text-[12px] font-medium text-txt">{formatDateTime(item.last_check_at)}</p>
                    </div>
                  </div>
                  <div className="mt-4 flex items-center gap-2 border-t border-border pt-3.5">
                    <Button size="sm" onClick={() => void openServiceDetails(item.service_name)} disabled={servicesBusy}>
                      <Eye size={16} strokeWidth={1.4} />
                      Details
                    </Button>
                    <Button size="sm" onClick={() => setServiceActionState({ name: item.service_name, action: "reload" })} disabled={servicesBusy}>
                      <RefreshCw size={16} strokeWidth={1.4} />
                      Reload
                    </Button>
                    <Button size="sm" onClick={() => setServiceActionState({ name: item.service_name, action: "restart" })} disabled={servicesBusy}>
                      <RotateCcw size={16} strokeWidth={1.4} />
                      Restart
                    </Button>
                  </div>
                </motion.div>
              ))
            ) : (
              <div className="rounded-card border border-border bg-surface-2 p-4 text-[12px] text-txt-secondary">
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
          <div className="space-y-2">
            <p className="text-[12px] text-txt-secondary">
              Status: <span className="text-txt">{serviceDetails.status_text}</span>
            </p>
            <p className="text-[12px] text-txt-secondary">
              Active: {serviceDetails.active} / {serviceDetails.sub_state}
            </p>
            <p className="text-[12px] text-txt-secondary">
              PID: {serviceDetails.main_pid || 0} | Checked: {formatDateTime(serviceDetails.checked_at)}
            </p>
            <p className="pt-1 text-[12px] font-medium text-txt">Recent logs</p>
            <pre className="m-0 max-h-[320px] overflow-auto rounded-btn border border-border bg-surface-0 p-3 font-mono text-[11px] leading-5 text-txt-secondary">
              {serviceDetails.last_logs?.length ? serviceDetails.last_logs.join("\n") : "No logs available"}
            </pre>
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
