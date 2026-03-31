import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { apiFetch, getAPIErrorMessage } from "@/services/api";
import { queryRefetchInterval } from "@/src/queries/polling";
import { AuditLogItem } from "@/types/common";

const AUDIT_POLL_MS = 10000;
const AUDIT_QUERY_KEY = ["audit", "feed"] as const;

type AuditFeedContextValue = {
  items: AuditLogItem[];
  loading: boolean;
  error: string;
  newCount: number;
  refresh: () => Promise<void>;
  clearError: () => void;
  markSeen: () => void;
};

const AuditFeedContext = createContext<AuditFeedContextValue | null>(null);

function itemTimestampMs(item: AuditLogItem): number {
  const ms = new Date(item.created_at).getTime();
  return Number.isFinite(ms) ? ms : 0;
}

export function AuditFeedProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [seenAtMs, setSeenAtMs] = useState(() => Date.now());
  const [dismissedError, setDismissedError] = useState(false);

  const auditQuery = useQuery({
    queryKey: AUDIT_QUERY_KEY,
    queryFn: async ({ signal }) => {
      const payload = await apiFetch<{ items: AuditLogItem[] }>("/api/audit?limit=250", { method: "GET", signal });
      const next = Array.isArray(payload.items) ? [...payload.items] : [];
      next.sort((a, b) => itemTimestampMs(b) - itemTimestampMs(a));
      return next;
    },
    staleTime: 3_000,
    refetchOnWindowFocus: true,
    refetchInterval: (query) => queryRefetchInterval(AUDIT_POLL_MS, query),
  });

  const items = auditQuery.data || [];
  const loading = auditQuery.isPending;
  const error = dismissedError ? "" : (auditQuery.error ? getAPIErrorMessage(auditQuery.error, "Failed to load audit log") : "");

  useEffect(() => {
    if (auditQuery.isSuccess) {
      setDismissedError(false);
    }
  }, [auditQuery.dataUpdatedAt, auditQuery.isSuccess]);

  const refresh = useCallback(async () => {
    setDismissedError(false);
    await queryClient.invalidateQueries({ queryKey: AUDIT_QUERY_KEY });
  }, [queryClient]);

  const clearError = useCallback(() => {
    setDismissedError(true);
  }, []);

  const markSeen = useCallback(() => {
    setSeenAtMs((current) => {
      const latest = items.length ? itemTimestampMs(items[0]) : Date.now();
      return Math.max(current, latest);
    });
  }, [items]);

  const newCount = useMemo(() => {
    return items.reduce((count, item) => (itemTimestampMs(item) > seenAtMs ? count + 1 : count), 0);
  }, [items, seenAtMs]);

  const value = useMemo<AuditFeedContextValue>(() => ({
    items,
    loading,
    error,
    newCount,
    refresh,
    clearError,
    markSeen,
  }), [items, loading, error, newCount, refresh, clearError, markSeen]);

  return <AuditFeedContext.Provider value={value}>{children}</AuditFeedContext.Provider>;
}

export function useAuditFeed(): AuditFeedContextValue {
  const ctx = useContext(AuditFeedContext);
  if (!ctx) {
    throw new Error("useAuditFeed must be used within AuditFeedProvider");
  }
  return ctx;
}
