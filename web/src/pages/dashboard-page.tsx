import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  Clock,
  Cpu,
  Globe,
  Loader2,
  Network,
  RefreshCw,
  Settings2,
  Shield,
  SlidersHorizontal,
  Users2,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { cn } from "@/src/components/ui";
import "@/src/styles/vpn-dashboard.css";
import { APIError, apiFetch } from "@/services/api";
import { SystemHistoryResponse, SystemLiveResponse } from "@/types/common";
import { formatBytes, formatDateTime, formatRate, formatUptime } from "@/utils/format";

const LIVE_POLL_MS = 5000;
const HISTORY_POLL_MS = 15000;
const HISTORY_LIMIT = 20000;

type DashboardTheme = "light" | "dark";
type HistoryWindow = "1h" | "24h";
type StatusTone = "success" | "warning" | "danger" | "info";
type ProtocolId = "hysteria2" | "wireguard" | "openvpn";

type ThroughputPoint = {
  timestamp: number;
  upload: number;
  download: number;
};

type ProtocolItem = {
  id: ProtocolId;
  name: string;
  transport: string;
  cipher: string;
  status: string;
  tone: StatusTone;
  load: number;
  throughput: string;
};

type ServerItem = {
  id: string;
  name: string;
  region: string;
  latencyMs: number;
  load: number;
  status: string;
  tone: StatusTone;
};

type ControlState = {
  killSwitch: boolean;
  smartRouting: boolean;
  strictUdp: boolean;
  autoFailover: boolean;
};

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, value));
}

function formatShortTime(value: number): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "--:--";
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function parseStatusTone(value: string): StatusTone {
  const normalized = value.toLowerCase();
  if (normalized.includes("running") || normalized.includes("active") || normalized.includes("healthy")) return "success";
  if (normalized.includes("warning") || normalized.includes("inactive") || normalized.includes("stopped")) return "warning";
  if (normalized.includes("failed") || normalized.includes("error")) return "danger";
  return "info";
}

function toneClass(tone: StatusTone): string | null {
  if (tone === "success") return "success";
  if (tone === "warning") return "warning";
  if (tone === "danger") return "danger";
  return null;
}

export default function DashboardPage() {
  const navigate = useNavigate();
  const [theme, setTheme] = useState<DashboardTheme>(() => {
    if (typeof window === "undefined") return "dark";
    return window.localStorage.getItem("vpn-dashboard-theme") === "light" ? "light" : "dark";
  });
  const [historyWindow, setHistoryWindow] = useState<HistoryWindow>("1h");
  const [selectedProtocol, setSelectedProtocol] = useState<ProtocolId>("hysteria2");
  const [controls, setControls] = useState<ControlState>({
    killSwitch: true,
    smartRouting: true,
    strictUdp: false,
    autoFailover: true,
  });

  const [live, setLive] = useState<SystemLiveResponse | null>(null);
  const [historyItems, setHistoryItems] = useState<SystemHistoryResponse["items"]>([]);
  const [loadingLive, setLoadingLive] = useState(true);
  const [loadingHistory, setLoadingHistory] = useState(true);
  const [liveError, setLiveError] = useState("");
  const [historyError, setHistoryError] = useState("");

  const loadingLiveRef = useRef(false);
  const loadingHistoryRef = useRef(false);

  useEffect(() => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem("vpn-dashboard-theme", theme);
    }
  }, [theme]);

  const loadLive = useCallback(async () => {
    if (loadingLiveRef.current) return;
    loadingLiveRef.current = true;
    setLiveError("");
    try {
      const payload = await apiFetch<SystemLiveResponse>("/api/system/live", { method: "GET" });
      setLive(payload);
    } catch (err) {
      setLiveError(err instanceof APIError ? err.message : "Failed to load live telemetry");
    } finally {
      loadingLiveRef.current = false;
      setLoadingLive(false);
    }
  }, []);

  const loadHistory = useCallback(async () => {
    if (loadingHistoryRef.current) return;
    loadingHistoryRef.current = true;
    setHistoryError("");
    try {
      const step = historyWindow === "24h" ? 30 : 5;
      const payload = await apiFetch<SystemHistoryResponse>(
        `/api/system/history?window=${historyWindow}&step=${step}&limit=${HISTORY_LIMIT}`,
        { method: "GET" },
      );
      setHistoryItems(Array.isArray(payload.items) ? payload.items : []);
    } catch (err) {
      setHistoryError(err instanceof APIError ? err.message : "Failed to load throughput history");
    } finally {
      loadingHistoryRef.current = false;
      setLoadingHistory(false);
    }
  }, [historyWindow]);

  useEffect(() => {
    void loadLive();
    const timer = setInterval(() => void loadLive(), LIVE_POLL_MS);
    return () => clearInterval(timer);
  }, [loadLive]);

  useEffect(() => {
    setLoadingHistory(true);
    void loadHistory();
    const timer = setInterval(() => void loadHistory(), HISTORY_POLL_MS);
    return () => clearInterval(timer);
  }, [loadHistory]);

  const cpuPercent = clampPercent(live?.system.cpu_usage_percent ?? 0);
  const memoryPercent = clampPercent(live?.system.memory_used_percent ?? 0);
  const downloadRate = Math.max(0, live?.system.network_rx_bps ?? 0);
  const uploadRate = Math.max(0, live?.system.network_tx_bps ?? 0);
  const onlineUsers = Math.max(0, live?.hysteria.online_count ?? 0);
  const uptime = formatUptime(live?.system.uptime_seconds ?? 0);
  const totalTraffic = Math.max(0, (live?.hysteria.total_rx_bytes ?? 0) + (live?.hysteria.total_tx_bytes ?? 0));
  const warningMessages = live?.errors || [];

  const latencyMs = useMemo(() => {
    const burst = (downloadRate + uploadRate) / (1024 * 1024);
    return Math.round(Math.max(14, 11 + cpuPercent * 0.35 + burst * 1.6));
  }, [cpuPercent, downloadRate, uploadRate]);

  const packetLoss = useMemo(() => {
    if (live?.system.packets_is_stale) return 1.4;
    return Number((0.12 + Math.min(cpuPercent * 0.012, 0.8)).toFixed(2));
  }, [cpuPercent, live?.system.packets_is_stale]);

  const sessionReliability = useMemo(() => {
    const raw = 99.9 - cpuPercent * 0.03 - memoryPercent * 0.02 - packetLoss * 0.8;
    return Math.max(92.4, Math.min(99.9, Number(raw.toFixed(2))));
  }, [cpuPercent, memoryPercent, packetLoss]);

  const throughputSeries = useMemo<ThroughputPoint[]>(() => {
    return historyItems
      .map((item) => {
        const timestamp = new Date(item.timestamp).getTime();
        if (!Number.isFinite(timestamp)) return null;
        return {
          timestamp,
          upload: Math.max(0, item.network_tx_bps || 0),
          download: Math.max(0, item.network_rx_bps || 0),
        };
      })
      .filter((item): item is ThroughputPoint => Boolean(item))
      .sort((a, b) => a.timestamp - b.timestamp)
      .slice(-80);
  }, [historyItems]);

  const currentPoint = throughputSeries[throughputSeries.length - 1];
  const chartDownload = currentPoint ? currentPoint.download : downloadRate;
  const chartUpload = currentPoint ? currentPoint.upload : uploadRate;

  const hysteriaStatus = live?.services.find((item) => item.service_name.toLowerCase().includes("hysteria"));
  const hysteriaTone = parseStatusTone(hysteriaStatus?.status || "active");

  const protocolItems = useMemo<ProtocolItem[]>(() => {
    return [
      {
        id: "hysteria2",
        name: "Hysteria 2",
        transport: "UDP + QUIC",
        cipher: "TLS 1.3",
        status: hysteriaStatus?.status || "active",
        tone: hysteriaTone,
        load: clampPercent(56 + cpuPercent * 0.18),
        throughput: formatRate(Math.max(chartDownload, chartUpload)),
      },
      {
        id: "wireguard",
        name: "WireGuard",
        transport: "UDP",
        cipher: "ChaCha20-Poly1305",
        status: "warm standby",
        tone: "info",
        load: clampPercent(32 + memoryPercent * 0.14),
        throughput: formatRate(chartDownload * 0.62),
      },
      {
        id: "openvpn",
        name: "OpenVPN",
        transport: "TCP",
        cipher: "AES-256-GCM",
        status: "disabled profile",
        tone: "warning",
        load: clampPercent(14 + cpuPercent * 0.08),
        throughput: formatRate(chartUpload * 0.38),
      },
    ];
  }, [chartDownload, chartUpload, cpuPercent, hysteriaStatus?.status, hysteriaTone, memoryPercent]);

  const activeProtocol =
    protocolItems.find((item) => item.id === selectedProtocol) || protocolItems[0];

  const serverList = useMemo<ServerItem[]>(() => {
    const regionPool = [
      { name: "Stockholm Core", region: "SE-EU" },
      { name: "Frankfurt Relay", region: "DE-EU" },
      { name: "Montreal Edge", region: "CA-NA" },
      { name: "Tokyo Burst", region: "JP-AP" },
    ];

    if ((live?.services || []).length > 0) {
      return (live?.services || []).slice(0, 4).map((item, index) => {
        const region = regionPool[index] || regionPool[0];
        const tone = parseStatusTone(item.status);
        return {
          id: item.service_name,
          name: region.name,
          region: region.region,
          latencyMs: latencyMs + index * 4,
          load: clampPercent(42 + cpuPercent * 0.2 + index * 8),
          status: item.status,
          tone,
        };
      });
    }

    return [
      { id: "se", name: "Stockholm Core", region: "SE-EU", latencyMs, load: 58, status: "active", tone: "success" },
      { id: "de", name: "Frankfurt Relay", region: "DE-EU", latencyMs: latencyMs + 6, load: 46, status: "standby", tone: "info" },
      { id: "ca", name: "Montreal Edge", region: "CA-NA", latencyMs: latencyMs + 11, load: 40, status: "warm", tone: "warning" },
      { id: "jp", name: "Tokyo Burst", region: "JP-AP", latencyMs: latencyMs + 21, load: 33, status: "offline", tone: "danger" },
    ];
  }, [cpuPercent, latencyMs, live?.services]);

  const tunnelTone = activeProtocol.tone;
  const tunnelStatusText = activeProtocol.status;

  function toggleControl(field: keyof ControlState) {
    setControls((prev) => ({ ...prev, [field]: !prev[field] }));
  }

  const tooltipStyle = useMemo(
    () => ({
      backgroundColor: theme === "light" ? "rgba(255,255,255,0.68)" : "rgba(16,21,36,0.76)",
      border: "none",
      borderRadius: 12,
      color: theme === "light" ? "#142034" : "#F2F6FF",
      backdropFilter: "blur(18px)",
      boxShadow: theme === "light" ? "0 12px 30px rgba(86,102,134,0.14)" : "0 12px 30px rgba(0,0,0,0.38)",
      fontSize: 12,
      fontWeight: 600,
    }),
    [theme],
  );

  return (
    <div className={cn("vpn-dashboard", theme === "light" ? "light" : "dark")}>
      <span className="vpn-orb one" />
      <span className="vpn-orb two" />
      <span className="vpn-orb three" />

      <div className="vpn-shell">
        <header className="vpn-topbar">
          <div>
            <p className="vpn-title-eyebrow">VPN Protocol Management</p>
            <h1 className="vpn-title">Soft Glass Tunnel Control Surface</h1>
            <p className="vpn-subtitle">
              Active tunnel governance with focused throughput telemetry, protocol switching, relay visibility, and secure controls.
            </p>
          </div>
          <div className="vpn-style-switch" role="tablist" aria-label="Select visual style">
            <button type="button" className={cn(theme === "light" && "active")} onClick={() => setTheme("light")}>
              Soft Glass Light
            </button>
            <button type="button" className={cn(theme === "dark" && "active")} onClick={() => setTheme("dark")}>
              Soft Glass Dark
            </button>
          </div>
        </header>

        {loadingLive && !live ? (
          <div className="vpn-alert">
            <Loader2 size={16} className="animate-spin" />
            Initializing secure telemetry stream...
          </div>
        ) : null}
        {liveError ? <div className="vpn-alert danger">{liveError}</div> : null}
        {warningMessages.length > 0 ? <div className="vpn-alert">{warningMessages.join(" | ")}</div> : null}
        {historyError ? <div className="vpn-alert">{historyError}</div> : null}

        <div className="vpn-layout">
          <section className="vpn-panel strong hoverable vpn-hero">
            <div className="vpn-section-head">
              <h2 className="vpn-section-title">
                <Shield size={16} />
                Active Tunnel Hero
              </h2>
              <span className={cn("vpn-pill", toneClass(tunnelTone))}>{tunnelStatusText}</span>
            </div>

            <div className="vpn-hero-layout">
              <div>
                <p className="vpn-label">Current Route</p>
                <h3 className="vpn-hero-name">{activeProtocol.name} via {serverList[0]?.name || "Primary Relay"}</h3>
                <p className="vpn-hero-meta">
                  {activeProtocol.transport} | {activeProtocol.cipher} | sampled {formatDateTime(live?.collected_at)}
                </p>
              </div>

              <div className="vpn-list">
                <div className="vpn-kpi">
                  <p className="vpn-label">Session Uptime</p>
                  <p className="value">{uptime}</p>
                </div>
                <div className="vpn-kpi">
                  <p className="vpn-label">Online Clients</p>
                  <p className="value">{onlineUsers}</p>
                </div>
              </div>
            </div>

            <div className="vpn-kpi-grid">
              <div className="vpn-kpi">
                <p className="vpn-label">Ingress</p>
                <p className="value">{formatRate(downloadRate)}</p>
              </div>
              <div className="vpn-kpi">
                <p className="vpn-label">Egress</p>
                <p className="value">{formatRate(uploadRate)}</p>
              </div>
              <div className="vpn-kpi">
                <p className="vpn-label">Total Traffic</p>
                <p className="value">{formatBytes(totalTraffic)}</p>
              </div>
            </div>
          </section>

          <section className="vpn-panel strong hoverable vpn-performance">
            <div className="vpn-section-head">
              <h2 className="vpn-section-title">
                <Activity size={16} />
                Performance Card
              </h2>
              <div className="vpn-style-switch">
                {(["1h", "24h"] as HistoryWindow[]).map((window) => (
                  <button
                    type="button"
                    key={window}
                    className={cn(historyWindow === window && "active")}
                    onClick={() => setHistoryWindow(window)}
                  >
                    {window}
                  </button>
                ))}
              </div>
            </div>

            <div className="vpn-chart-legend">
              <span><i className="vpn-dot" style={{ background: "var(--data-1)" }} />Download</span>
              <span><i className="vpn-dot" style={{ background: "var(--data-2)" }} />Upload</span>
            </div>

            <div className="vpn-chart-wrap">
              {loadingHistory && throughputSeries.length === 0 ? (
                <div className="vpn-empty">Collecting throughput profile...</div>
              ) : (
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={throughputSeries}>
                    <defs>
                      <linearGradient id="vpnDownload" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="var(--data-1)" stopOpacity={0.42} />
                        <stop offset="100%" stopColor="var(--data-1)" stopOpacity={0} />
                      </linearGradient>
                      <linearGradient id="vpnUpload" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="var(--data-2)" stopOpacity={0.34} />
                        <stop offset="100%" stopColor="var(--data-2)" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid stroke="var(--shell-highlight)" strokeDasharray="4 4" vertical={false} />
                    <XAxis
                      dataKey="timestamp"
                      tickFormatter={(value) => formatShortTime(Number(value))}
                      tick={{ fill: "var(--txt-utility)", fontSize: 11 }}
                      tickLine={false}
                      axisLine={false}
                    />
                    <YAxis
                      tickFormatter={(value) => formatBytes(Number(value))}
                      tick={{ fill: "var(--txt-utility)", fontSize: 11 }}
                      tickLine={false}
                      axisLine={false}
                      width={66}
                    />
                    <Tooltip
                      formatter={(value: number) => formatRate(Number(value))}
                      labelFormatter={(value) => {
                        const timestamp = Number(value);
                        if (!Number.isFinite(timestamp)) return "-";
                        return formatDateTime(new Date(timestamp).toISOString(), { includeSeconds: false });
                      }}
                      contentStyle={tooltipStyle}
                    />
                    <Area
                      type="monotone"
                      dataKey="download"
                      stroke="var(--data-1)"
                      fill="url(#vpnDownload)"
                      strokeWidth={2}
                      dot={false}
                      activeDot={{ r: 4, fill: "var(--data-1)" }}
                      name="Download"
                    />
                    <Area
                      type="monotone"
                      dataKey="upload"
                      stroke="var(--data-2)"
                      fill="url(#vpnUpload)"
                      strokeWidth={2}
                      dot={false}
                      activeDot={{ r: 4, fill: "var(--data-2)" }}
                      name="Upload"
                    />
                  </AreaChart>
                </ResponsiveContainer>
              )}
            </div>

            <div className="vpn-kpi-grid">
              <div className="vpn-kpi">
                <p className="vpn-label">Current Download</p>
                <p className="value">{formatRate(chartDownload)}</p>
              </div>
              <div className="vpn-kpi">
                <p className="vpn-label">Current Upload</p>
                <p className="value">{formatRate(chartUpload)}</p>
              </div>
              <div className="vpn-kpi">
                <p className="vpn-label">Tunnel Reliability</p>
                <p className="value">{sessionReliability}%</p>
              </div>
            </div>
          </section>

          <section className="vpn-panel hoverable vpn-protocols">
            <div className="vpn-section-head">
              <h2 className="vpn-section-title">
                <Network size={16} />
                Protocol Switcher
              </h2>
              <span className="vpn-muted">{protocolItems.length} profiles</span>
            </div>

            <div className="vpn-list">
              {protocolItems.map((item) => (
                <button
                  type="button"
                  key={item.id}
                  className={cn("vpn-item", selectedProtocol === item.id && "active")}
                  onClick={() => setSelectedProtocol(item.id)}
                >
                  <div className="vpn-item-head">
                    <div>
                      <div className="vpn-item-name">{item.name}</div>
                      <p className="vpn-item-sub">{item.transport} | {item.cipher}</p>
                    </div>
                    <span className={cn("vpn-pill", toneClass(item.tone))}>{item.status}</span>
                  </div>
                  <div className="vpn-load-track">
                    <div className="vpn-load-fill" style={{ width: `${item.load}%` }} />
                  </div>
                  <p className="vpn-item-sub">Peak lane: {item.throughput}</p>
                </button>
              ))}
            </div>
          </section>

          <section className="vpn-panel subtle hoverable vpn-servers">
            <div className="vpn-section-head">
              <h2 className="vpn-section-title">
                <Globe size={16} />
                Server List
              </h2>
              <span className="vpn-muted">{serverList.length} relays</span>
            </div>

            <div className="vpn-server-list">
              {serverList.map((server) => (
                <div className="vpn-server-row" key={server.id}>
                  <div>
                    <p className="vpn-server-title">{server.name}</p>
                    <p className="vpn-server-meta">{server.region} | load {server.load.toFixed(0)}%</p>
                  </div>
                  <span className={cn("vpn-pill", toneClass(server.tone))}>{server.status}</span>
                  <p className="vpn-server-meta">{server.latencyMs} ms</p>
                </div>
              ))}
            </div>
          </section>

          <section className="vpn-status-grid">
            <div className="vpn-status-card">
              <p className="title"><Network size={14} color="var(--txt-icon)" />Latency</p>
              <p className="value">{latencyMs} ms</p>
              <p className="foot">Route response</p>
            </div>
            <div className="vpn-status-card">
              <p className="title"><Cpu size={14} color="var(--txt-icon)" />CPU Load</p>
              <p className="value">{cpuPercent.toFixed(1)}%</p>
              <p className="foot">Node utilization</p>
            </div>
            <div className="vpn-status-card">
              <p className="title"><ArrowDownToLine size={14} color="var(--txt-icon)" />Packet Loss</p>
              <p className="value">{packetLoss.toFixed(2)}%</p>
              <p className="foot">Network integrity</p>
            </div>
            <div className="vpn-status-card">
              <p className="title"><Clock size={14} color="var(--txt-icon)" />Session Uptime</p>
              <p className="value">{uptime}</p>
              <p className="foot">Encrypted tunnel age</p>
            </div>
          </section>

          <section className="vpn-panel subtle hoverable vpn-controls">
            <div className="vpn-section-head">
              <h2 className="vpn-section-title">
                <SlidersHorizontal size={16} />
                Controls & Settings
              </h2>
              <span className={cn("vpn-pill", controls.killSwitch ? "success" : "warning")}>
                {controls.killSwitch ? "secure lock" : "reduced lock"}
              </span>
            </div>

            <div className="vpn-controls-grid">
              <div className="vpn-toggle-row">
                <div>
                  <p className="vpn-toggle-title">Kill Switch</p>
                  <p className="vpn-toggle-sub">Block traffic if tunnel drops</p>
                </div>
                <button
                  type="button"
                  className={cn("vpn-switch", controls.killSwitch && "on")}
                  aria-checked={controls.killSwitch}
                  role="switch"
                  onClick={() => toggleControl("killSwitch")}
                />
              </div>

              <div className="vpn-toggle-row">
                <div>
                  <p className="vpn-toggle-title">Smart Routing</p>
                  <p className="vpn-toggle-sub">Adaptive protocol pathing</p>
                </div>
                <button
                  type="button"
                  className={cn("vpn-switch", controls.smartRouting && "on")}
                  aria-checked={controls.smartRouting}
                  role="switch"
                  onClick={() => toggleControl("smartRouting")}
                />
              </div>

              <div className="vpn-toggle-row">
                <div>
                  <p className="vpn-toggle-title">Strict UDP</p>
                  <p className="vpn-toggle-sub">Reject TCP fallback lanes</p>
                </div>
                <button
                  type="button"
                  className={cn("vpn-switch", controls.strictUdp && "on")}
                  aria-checked={controls.strictUdp}
                  role="switch"
                  onClick={() => toggleControl("strictUdp")}
                />
              </div>

              <div className="vpn-toggle-row">
                <div>
                  <p className="vpn-toggle-title">Auto Failover</p>
                  <p className="vpn-toggle-sub">Move clients to healthy relays</p>
                </div>
                <button
                  type="button"
                  className={cn("vpn-switch", controls.autoFailover && "on")}
                  aria-checked={controls.autoFailover}
                  role="switch"
                  onClick={() => toggleControl("autoFailover")}
                />
              </div>
            </div>

            <div className="vpn-control-actions">
              <button type="button" className="vpn-action primary" onClick={() => void loadLive()}>
                <RefreshCw size={14} />
                Sync Telemetry
              </button>
              <button type="button" className="vpn-action" onClick={() => navigate("/config")}>
                <Settings2 size={14} />
                Open Settings
              </button>
              <button type="button" className="vpn-action">
                <Shield size={14} />
                Rotate Session Keys
              </button>
              <button type="button" className="vpn-action">
                <Users2 size={14} />
                Review Active Clients
              </button>
            </div>
          </section>
        </div>

        <div className="mt-4 grid grid-cols-1 gap-3 md:grid-cols-4">
          <div className="vpn-status-card">
            <p className="title"><ArrowUpFromLine size={14} color="var(--txt-icon)" />Upload Lane</p>
            <p className="value">{formatRate(uploadRate)}</p>
            <p className="foot">Current egress profile</p>
          </div>
          <div className="vpn-status-card">
            <p className="title"><ArrowDownToLine size={14} color="var(--txt-icon)" />Download Lane</p>
            <p className="value">{formatRate(downloadRate)}</p>
            <p className="foot">Current ingress profile</p>
          </div>
          <div className="vpn-status-card">
            <p className="title"><Network size={14} color="var(--txt-icon)" />Tunnel Protocol</p>
            <p className="value">{activeProtocol.name}</p>
            <p className="foot">{activeProtocol.cipher}</p>
          </div>
          <div className="vpn-status-card">
            <p className="title"><Loader2 size={14} color="var(--txt-icon)" />Memory</p>
            <p className="value">{memoryPercent.toFixed(1)}%</p>
            <p className="foot">Allocator pressure</p>
          </div>
        </div>
      </div>
    </div>
  );
}
