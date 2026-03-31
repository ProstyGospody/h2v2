type QueryLike = {
  state?: {
    fetchFailureCount?: number;
  };
};

function withJitter(baseMs: number, jitterRatio: number): number {
  const spread = baseMs * jitterRatio;
  const delta = (Math.random() * 2 - 1) * spread;
  return Math.max(750, Math.round(baseMs + delta));
}

export function pollingIntervalMs(
  baseMs: number,
  failureCount: number,
  {
    maxMs = 60_000,
    jitterRatio = 0.2,
  }: { maxMs?: number; jitterRatio?: number } = {},
): number {
  const exponential = Math.min(baseMs * 2 ** Math.max(0, failureCount), maxMs);
  return withJitter(exponential, jitterRatio);
}

export function queryRefetchInterval(
  baseMs: number,
  query: QueryLike | null | undefined,
  {
    enabled = true,
    maxMs = 60_000,
    jitterRatio = 0.2,
  }: { enabled?: boolean; maxMs?: number; jitterRatio?: number } = {},
): number | false {
  if (!enabled) return false;
  if (typeof document !== "undefined" && document.hidden) return false;
  const failureCount = query?.state?.fetchFailureCount ?? 0;
  return pollingIntervalMs(baseMs, failureCount, { maxMs, jitterRatio });
}
