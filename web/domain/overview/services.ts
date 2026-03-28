import { apiFetch } from "@/services/api";

import { HysteriaStatsHistoryResponse } from "@/domain/overview/types";

export async function getHysteriaStatsHistory(limit = 500): Promise<HysteriaStatsHistoryResponse> {
  return apiFetch<HysteriaStatsHistoryResponse>(`/api/hysteria/stats/history?limit=${limit}`, {
    method: "GET",
  });
}
