export type HistoryWindow = "1h" | "24h";

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
