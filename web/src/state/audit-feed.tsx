import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { APIError, apiFetch } from "@/services/api";
import { AuditLogItem } from "@/types/common";

const AUDIT_POLL_MS = 10000;

type AuditFeedContextValue = {
  items: AuditLogItem[];
  loading: boolean;
  error: string;
  newCount: number;
  refresh: () => Promise<void>;
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

  const refresh = useCallback(async () => {
    setError("");
    try {
      const payload = await apiFetch<{ items: AuditLogItem[] }>("/api/audit?limit=250", { method: "GET" });
      const next = Array.isArray(payload.items) ? [...payload.items] : [];
      next.sort((a, b) => itemTimestampMs(b) - itemTimestampMs(a));
      setItems(next);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to load audit log");
    } finally {
      setLoading(false);
    }
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

  useEffect(() => {
    void refresh();
    const timer = setInterval(() => {
      void refresh();
    }, AUDIT_POLL_MS);
    return () => clearInterval(timer);
  }, [refresh]);

  const value = useMemo<AuditFeedContextValue>(() => ({
    items,
    loading,
    error,
    newCount,
    refresh,
    markSeen,
  }), [items, loading, error, newCount, refresh, markSeen]);

  return <AuditFeedContext.Provider value={value}>{children}</AuditFeedContext.Provider>;
}

export function useAuditFeed(): AuditFeedContextValue {
  const ctx = useContext(AuditFeedContext);
  if (!ctx) {
    throw new Error("useAuditFeed must be used within AuditFeedProvider");
  }
  return ctx;
}
