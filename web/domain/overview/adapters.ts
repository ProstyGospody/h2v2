import { HysteriaStatsSnapshot, OverviewTrendPoint } from "@/domain/overview/types";

type TrendBucket = {
  timestampMs: number;
  totalTxBytes: number;
  totalRxBytes: number;
  onlineCount: number;
};

export function buildOverviewTrends(items: HysteriaStatsSnapshot[], maxPoints = 72): OverviewTrendPoint[] {
  const grouped = new Map<number, TrendBucket>();

  for (const item of items) {
    const timestampMs = Date.parse(item.snapshot_at);
    if (Number.isNaN(timestampMs)) {
      continue;
    }
    const current = grouped.get(timestampMs) || {
      timestampMs,
      totalTxBytes: 0,
      totalRxBytes: 0,
      onlineCount: 0,
    };

    current.totalTxBytes += Math.max(0, item.tx_bytes || 0);
    current.totalRxBytes += Math.max(0, item.rx_bytes || 0);
    current.onlineCount += Math.max(0, item.online_count || 0);

    grouped.set(timestampMs, current);
  }

  const buckets = [...grouped.values()].sort((a, b) => a.timestampMs - b.timestampMs);
  if (buckets.length <= maxPoints) {
    return toTrendPoints(buckets);
  }
  return toTrendPoints(buckets.slice(-maxPoints));
}

function toTrendPoints(buckets: TrendBucket[]): OverviewTrendPoint[] {
  let previous: TrendBucket | null = null;

  return buckets.map((bucket) => {
    let uploadBps = 0;
    let downloadBps = 0;

    if (previous) {
      const deltaSeconds = (bucket.timestampMs - previous.timestampMs) / 1000;
      if (deltaSeconds > 0) {
        uploadBps = Math.max(0, bucket.totalTxBytes - previous.totalTxBytes) / deltaSeconds;
        downloadBps = Math.max(0, bucket.totalRxBytes - previous.totalRxBytes) / deltaSeconds;
      }
    }

    previous = bucket;

    return {
      timestamp: new Date(bucket.timestampMs).toISOString(),
      total_tx_bytes: bucket.totalTxBytes,
      total_rx_bytes: bucket.totalRxBytes,
      upload_bps: uploadBps,
      download_bps: downloadBps,
      online_count: bucket.onlineCount,
    };
  });
}
