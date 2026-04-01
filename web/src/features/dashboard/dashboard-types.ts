export type HistoryWindow = "24h" | "7d";

export type HistoryTrendPoint = {
  timestamp: Date;
  download: number;
  upload: number;
  connections: number;
  cpu: number;
  ram: number;
};

export type TrafficUsageBarPoint = {
  timestamp: Date;
  download_bytes: number;
  upload_bytes: number;
};
