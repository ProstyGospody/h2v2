import { createContext, type ReactNode, useCallback, useContext, useMemo, useState } from "react";

import { usePolling } from "@/hooks/use-polling";
import { APIError, apiFetch } from "@/services/api";
import { AuditLogItem } from "@/types/common";

const AUDIT_POLL_MS = 10000;

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
  const [items, setItems] = useState<AuditLogItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [seenAtMs, setSeenAtMs] = useState(() => Date.now());

  const refresh = useCallback(async (signal?: AbortSignal) => {
    if (!signal?.aborted) {
      setError("");
    }
    try {
      const payload = await apiFetch<{ items: AuditLogItem[] }>("/api/audit?limit=250", { method: "GET", signal });
      const next = Array.isArray(payload.items) ? [...payload.items] : [];
      next.sort((a, b) => itemTimestampMs(b) - itemTimestampMs(a));
      setItems(next);
      setLoading(false);
    } catch (err) {
      if (signal?.aborted || (err instanceof DOMException && err.name === "AbortError")) {
        return;
      }
      setError(err instanceof APIError ? err.message : "Failed to load audit log");
      setLoading(false);
      throw err;
    }
  }, []);

  const pollingRefresh = useCallback(async (signal: AbortSignal) => {
    await refresh(signal);
  }, [refresh]);

  usePolling(pollingRefresh, AUDIT_POLL_MS);

  const markSeen = useCallback(() => {
    setSeenAtMs((current) => {
      const latest = items.length ? itemTimestampMs(items[0]) : Date.now();
      return Math.max(current, latest);
    });
  }, [items]);

  const newCount = useMemo(() => {
    return items.reduce((count, item) => (itemTimestampMs(item) > seenAtMs ? count + 1 : count), 0);
  }, [items, seenAtMs]);
  const clearError = useCallback(() => setError(""), []);

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
