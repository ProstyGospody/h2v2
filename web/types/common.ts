export type SystemLiveResponse = {
  collected_at: string;
  system: {
    cpu_usage_percent: number;
    memory_used_bytes: number;
    memory_total_bytes: number;
    memory_used_percent: number;
    uptime_seconds: number;
    network_rx_bps: number;
    network_tx_bps: number;
    tcp_sockets: number;
    udp_sockets: number;
    tcp_packets: number;
    udp_packets: number;
    tcp_packets_per_sec: number;
    udp_packets_per_sec: number;
    packets_collected_at: string;
    packets_source: string;
    packets_is_stale: boolean;
    collected_at: string;
    source: string;
    is_stale: boolean;
  };
  runtime: {
    enabled_users: number;
    total_tx_bytes: number;
    total_rx_bytes: number;
    online_count: number;
    connections_tcp: number;
    connections_udp: number;
    connections_breakdown_available: boolean;
    collected_at: string;
    source: string;
    is_stale: boolean;
  };
  services: Array<{
    service_name: string;
    status: string;
    last_check_at: string;
    source: string;
    is_stale: boolean;
    error?: string;
  }>;
  errors: string[];
};

export type SystemHistorySample = {
  timestamp: string;
  cpu_usage_percent: number;
  memory_used_percent: number;
  network_rx_bps: number;
  network_tx_bps: number;
  tcp_sockets?: number;
  udp_sockets?: number;
};

export type SystemHistoryResponse = {
  items: SystemHistorySample[];
};
