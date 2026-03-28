export type HysteriaStatsSnapshot = {
  id: number;
  hysteria_user_id: string;
  tx_bytes: number;
  rx_bytes: number;
  online_count: number;
  snapshot_at: string;
};

export type HysteriaStatsHistoryResponse = {
  items: HysteriaStatsSnapshot[];
};

export type OverviewTrendPoint = {
  timestamp: string;
  total_tx_bytes: number;
  total_rx_bytes: number;
  upload_bps: number;
  download_bps: number;
  online_count: number;
};
