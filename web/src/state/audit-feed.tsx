import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { apiFetch, getAPIErrorMessage } from "@/services/api";
import { queryRefetchInterval } from "@/src/queries/polling";
import { AuditLogItem } from "@/types/common";

const AUDIT_POLL_MS = 10000;
const AUDIT_QUERY_KEY = ["audit", "feed"] as const;
const EMPTY_AUDIT_ITEMS: AuditLogItem[] = [];
type UnknownRecord = Record<string, unknown>;

function asRecord(value: unknown): UnknownRecord | null {
  if (typeof value !== "object" || value === null) {
    return null;
  }
  return value as UnknownRecord;
}

function asString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function asNumber(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function asJSONText(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  if (value === null || value === undefined) {
    return "{}";
  }
  try {
    return JSON.stringify(value);
  } catch {
    return "{}";
  }
}

function normalizeAuditItem(raw: unknown, index: number): AuditLogItem {
  const record = asRecord(raw);
  const fallbackDate = new Date().toISOString();
  const adminID = record?.admin_id;
  const adminEmail = record?.admin_email;
  const entityID = record?.entity_id;

  return {
    id: asNumber(record?.id, index + 1),
    admin_id: typeof adminID === "string" || adminID === null ? adminID : null,
    admin_email: typeof adminEmail === "string" || adminEmail === null ? adminEmail : null,
    action: asString(record?.action, "unknown"),
    entity_type: asString(record?.entity_type, "unknown"),
    entity_id: typeof entityID === "string" || entityID === null ? entityID : null,
    payload_json: asJSONText(record?.payload_json),
    created_at: asString(record?.created_at, fallbackDate),
  };
}

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
      const payload = await apiFetch<{ items?: unknown }>("/api/audit?limit=250", { method: "GET", signal });
      const rawItems = Array.isArray(payload?.items) ? payload.items : [];
      const next = rawItems.map((item, index) => normalizeAuditItem(item, index));
      next.sort((a, b) => itemTimestampMs(b) - itemTimestampMs(a));
      return next;
    },
    staleTime: 3_000,
    refetchOnWindowFocus: true,
    refetchInterval: (query) => queryRefetchInterval(AUDIT_POLL_MS, query),
  });

  const items = auditQuery.data ?? EMPTY_AUDIT_ITEMS;
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
      if (items.length === 0) {
        return current;
      }
      const latest = itemTimestampMs(items[0]);
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
