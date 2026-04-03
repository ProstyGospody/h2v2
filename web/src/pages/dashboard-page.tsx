import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useState } from "react";

import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import { apiFetch, getAPIErrorMessage } from "@/services/api";
import { Button, Dialog } from "@/src/components/ui";
import { queryRefetchInterval } from "@/src/queries/polling";
import { type ServiceDetails, type ServiceSummary, type SystemHistoryResponse, type SystemLiveResponse } from "@/types/common";
import { formatDateTime, formatUptime } from "@/utils/format";

import { DashboardMetrics } from "@/src/features/dashboard/dashboard-metrics";
import { DashboardTraffic } from "@/src/features/dashboard/dashboard-traffic";
import { type HistoryTrendPoint, type HistoryWindow, type TrafficUsageBarPoint } from "@/src/features/dashboard/dashboard-types";
import { clampPercent } from "@/src/features/dashboard/dashboard-utils";
import { ServiceGrid } from "@/src/features/dashboard/service-grid";

const LIVE_POLL_MS = 5000;
const HISTORY_WINDOW_CONFIG: Record<
  HistoryWindow,
  {
    queryWindow: string;
    stepSeconds: number;
    limit: number;
    bucketMs: number;
    bucketCount: number;
    refetchMs: number;
  }
> = {
  "24h": {
    queryWindow: "24h",
    stepSeconds: 60,
    limit: 40_000,
    bucketMs: 60 * 60 * 1_000,
    bucketCount: 24,
    refetchMs: 30_000,
  },
  "7d": {
    queryWindow: "168h",
    stepSeconds: 900,
    limit: 220_000,
    bucketMs: 24 * 60 * 60 * 1_000,
    bucketCount: 7,
    refetchMs: 120_000,
  },
};

export default function DashboardPage() {
  const [historyWindow, setHistoryWindow] = useState<HistoryWindow>("24h");
  const [servicesBusy, setServicesBusy] = useState(false);
  const [servicesActionError, setServicesActionError] = useState("");
  const [dismissedLiveError, setDismissedLiveError] = useState(false);
  const [dismissedHistoryError, setDismissedHistoryError] = useState(false);
  const [dismissedServicesError, setDismissedServicesError] = useState(false);
  const [serviceDetails, setServiceDetails] = useState<ServiceDetails | null>(null);
  const [serviceDetailsOpen, setServiceDetailsOpen] = useState(false);
  const historyConfig = HISTORY_WINDOW_CONFIG[historyWindow];

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
      const { queryWindow, stepSeconds, limit } = historyConfig;
      return apiFetch<SystemHistoryResponse>(`/api/system/history?window=${queryWindow}&step=${stepSeconds}&limit=${limit}`, { method: "GET", signal });
    },
    staleTime: 10_000,
    refetchOnWindowFocus: true,
    refetchInterval: (query) => queryRefetchInterval(historyConfig.refetchMs, query, { maxMs: historyConfig.refetchMs * 4 }),
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
  const historyItems = Array.isArray(historyQuery.data?.items) ? historyQuery.data.items : [];
  const serviceItems = useMemo<ServiceSummary[]>(() => {
    const source = Array.isArray(servicesQuery.data?.items) ? servicesQuery.data.items : [];
    if (source.length === 0) {
      return [];
    }

    const uniqueByName = new Map<string, ServiceSummary>();
    for (const item of source) {
      const name = typeof item?.service_name === "string" ? item.service_name.trim() : "";
      if (!name) {
        continue;
      }
      uniqueByName.set(name, item);
    }

    return Array.from(uniqueByName.values()).sort((a, b) => a.service_name.localeCompare(b.service_name));
  }, [servicesQuery.data?.items]);
  const loading = liveQuery.isPending;
  const historyLoading = historyQuery.isPending;
  const servicesLoading = servicesQuery.isPending;

  const liveError = dismissedLiveError ? "" : (liveQuery.error ? getAPIErrorMessage(liveQuery.error, "Failed to load dashboard data") : "");
  const historyError = dismissedHistoryError ? "" : (historyQuery.error ? getAPIErrorMessage(historyQuery.error, "Failed to load history") : "");
  const servicesQueryError = dismissedServicesError ? "" : (servicesQuery.error ? getAPIErrorMessage(servicesQuery.error, "Failed to load services") : "");
  const servicesError = servicesActionError || servicesQueryError;

  const retryLive = useCallback(() => {
    setDismissedLiveError(false);
    void liveQuery.refetch();
  }, [liveQuery]);

  const retryHistory = useCallback(() => {
    setDismissedHistoryError(false);
    void historyQuery.refetch();
  }, [historyQuery]);

  const dismissHistoryError = useCallback(() => {
    setDismissedHistoryError(true);
  }, []);

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

  const warningMessages = useMemo(
    () => (Array.isArray(live?.errors) ? live.errors.filter((item): item is string => typeof item === "string") : []),
    [live],
  );

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
  const onlineUsers = Math.max(0, live?.runtime.online_count ?? 0);
  const networkRx = Math.max(0, live?.system.network_rx_bps ?? 0);
  const networkTx = Math.max(0, live?.system.network_tx_bps ?? 0);
  const uptime = formatUptime(live?.system.uptime_seconds ?? 0);
  const totalTraffic = Math.max(0, (live?.runtime.total_rx_bytes ?? 0) + (live?.runtime.total_tx_bytes ?? 0));
  const tcpConnections = Math.max(0, Math.round(live?.system.tcp_sockets ?? 0));
  const udpConnections = Math.max(0, Math.round(live?.system.udp_sockets ?? 0));

  const historyPoints = useMemo<HistoryTrendPoint[]>(() => {
    const byTimestamp = new Map<number, HistoryTrendPoint>();

    for (const sample of historyItems) {
      const timestamp = new Date(sample.timestamp);
      const timestampMs = timestamp.getTime();
      if (!Number.isFinite(timestampMs)) {
        continue;
      }
      const tcp = Number(sample.tcp_sockets || 0);
      const udp = Number(sample.udp_sockets || 0);
      byTimestamp.set(timestampMs, {
        timestamp,
        download: Math.max(0, sample.network_rx_bps || 0),
        upload: Math.max(0, sample.network_tx_bps || 0),
        connections: Math.max(0, (Number.isFinite(tcp) ? tcp : 0) + (Number.isFinite(udp) ? udp : 0)),
        cpu: clampPercent(Number(sample.cpu_usage_percent || 0)),
        ram: clampPercent(Number(sample.memory_used_percent || 0)),
      });
    }

    return Array.from(byTimestamp.values()).sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
  }, [historyItems]);

  const trafficUsageBars = useMemo<TrafficUsageBarPoint[]>(() => {
    const { bucketMs, bucketCount, stepSeconds } = historyConfig;
    const nowMs = Date.now();
    const endBucketMs = Math.floor(nowMs / bucketMs) * bucketMs;
    const startBucketMs = endBucketMs - (bucketCount - 1) * bucketMs;
    const maxIntervalSeconds = Math.max(stepSeconds * 2, 60);

    const buckets: TrafficUsageBarPoint[] = Array.from({ length: bucketCount }, (_, index) => ({
      timestamp: new Date(startBucketMs + index * bucketMs),
      download_bytes: 0,
      upload_bytes: 0,
    }));

    const bucketIndex = new Map<number, number>();
    for (let index = 0; index < buckets.length; index++) {
      bucketIndex.set(buckets[index].timestamp.getTime(), index);
    }

    if (!historyPoints.length) {
      return buckets;
    }

    for (let index = 0; index < historyPoints.length; index++) {
      const point = historyPoints[index];
      const ms = point.timestamp.getTime();
      const prevMs = index > 0 ? historyPoints[index - 1].timestamp.getTime() : ms - stepSeconds * 1_000;
      const intervalSeconds = Math.max(1, (ms - prevMs) / 1000);
      const dt = Math.min(intervalSeconds, maxIntervalSeconds);
      const key = Math.floor(ms / bucketMs) * bucketMs;
      if (key < startBucketMs || key > endBucketMs) {
        continue;
      }
      const bucket = bucketIndex.get(key);
      if (bucket === undefined) {
        continue;
      }
      buckets[bucket].download_bytes += Math.max(0, point.download) * dt;
      buckets[bucket].upload_bytes += Math.max(0, point.upload) * dt;
    }

    return buckets;
  }, [historyConfig, historyPoints]);

  const trafficTotal = useMemo(() => {
    return trafficUsageBars.reduce((sum, item) => sum + item.download_bytes + item.upload_bytes, 0);
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
        <div className="rounded-xl border border-status-warning/20 bg-status-warning/8 px-5 py-3.5 text-[14px] text-status-warning">
          {warningMessages.join(" | ")}
        </div>
      )}

      <DashboardMetrics
        showInitialLoading={showInitialLoading}
        cpuPercent={cpuPercent}
        ramPercent={ramPercent}
        onlineUsers={onlineUsers}
        uptime={uptime}
        networkRx={networkRx}
        networkTx={networkTx}
        totalTraffic={totalTraffic}
        tcpConnections={tcpConnections}
        udpConnections={udpConnections}
      />

      <DashboardTraffic
        historyWindow={historyWindow}
        onHistoryWindowChange={setHistoryWindow}
        historyError={historyError}
        onDismissHistoryError={dismissHistoryError}
        onRetryHistory={retryHistory}
        showHistorySkeleton={showHistorySkeleton}
        trafficTotal={trafficTotal}
        trafficUsageBars={trafficUsageBars}
      />

      <ServiceGrid
        loading={servicesLoading}
        items={serviceItems}
        busy={servicesBusy}
        error={servicesError}
        canRetry={Boolean(servicesQueryError || servicesActionError)}
        onDismissError={() => {
          if (servicesActionError) {
            setServicesActionError("");
            return;
          }
          setDismissedServicesError(true);
        }}
        onRetryError={retryServices}
        onOpenDetails={(name) => void openServiceDetails(name)}
        onRunAction={(name, action) => void runServiceAction(name, action)}
      />

      <Dialog
        open={serviceDetailsOpen}
        onOpenChange={(next) => {
          if (!next) {
            setServiceDetailsOpen(false);
          }
        }}
        title={`${serviceDetails?.name || "Service"} details`}
        contentClassName="max-w-[760px]"
        footer={<Button onClick={() => setServiceDetailsOpen(false)}>Close</Button>}
      >
        {serviceDetails && (
          <div className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-xl bg-surface-0/50 p-4">
                <p className="text-[12px] font-medium text-txt-muted">Status</p>
                <p className="mt-1.5 text-[14px] font-medium text-txt">{serviceDetails.status_text}</p>
              </div>
              <div className="rounded-xl bg-surface-0/50 p-4">
                <p className="text-[12px] font-medium text-txt-muted">Active</p>
                <p className="mt-1.5 text-[14px] font-medium text-txt">{serviceDetails.active} / {serviceDetails.sub_state}</p>
              </div>
              <div className="rounded-xl bg-surface-0/50 p-4">
                <p className="text-[12px] font-medium text-txt-muted">PID</p>
                <p className="mt-1.5 text-[14px] font-medium text-txt">{serviceDetails.main_pid || 0}</p>
              </div>
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
