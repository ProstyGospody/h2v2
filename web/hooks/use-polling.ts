import { useCallback, useEffect, useRef } from "react";

type UsePollingOptions = {
  maxIntervalMs?: number;
  jitterRatio?: number;
};

function withJitter(baseMs: number, jitterRatio: number): number {
  const spread = baseMs * jitterRatio;
  const delta = (Math.random() * 2 - 1) * spread;
  return Math.max(750, Math.round(baseMs + delta));
}

export function usePolling(
  callback: (signal: AbortSignal) => Promise<void>,
  intervalMs: number,
  deps: unknown[] = [],
  options: UsePollingOptions = {},
) {
  const maxIntervalMs = options.maxIntervalMs ?? 60_000;
  const jitterRatio = options.jitterRatio ?? 0.2;
  const backoffRef = useRef(intervalMs);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const resetBackoff = useCallback(() => {
    backoffRef.current = intervalMs;
  }, [intervalMs]);

  const schedule = useCallback(() => {
    const nextDelay = withJitter(backoffRef.current, jitterRatio);
    timerRef.current = setTimeout(() => {
      void tick();
    }, nextDelay);
  }, [jitterRatio]); // eslint-disable-line react-hooks/exhaustive-deps

  const tick = useCallback(async () => {
    if (document.hidden) return;
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    try {
      await callback(ac.signal);
      backoffRef.current = intervalMs;
    } catch (err) {
      if (
        ac.signal.aborted ||
        (err instanceof DOMException && err.name === "AbortError")
      ) {
        return;
      }
      backoffRef.current = Math.min(backoffRef.current * 2, maxIntervalMs);
    }
    if (!ac.signal.aborted) {
      schedule();
    }
  }, [callback, intervalMs, maxIntervalMs, schedule]);

  const retryNow = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    resetBackoff();
    void tick();
  }, [resetBackoff, tick]);

  useEffect(() => {
    resetBackoff();
    void tick();

    function onVisibility() {
      if (document.hidden) {
        if (timerRef.current) {
          clearTimeout(timerRef.current);
          timerRef.current = null;
        }
        abortRef.current?.abort();
      } else {
        void tick();
      }
    }

    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      document.removeEventListener("visibilitychange", onVisibility);
      if (timerRef.current) clearTimeout(timerRef.current);
      abortRef.current?.abort();
    };
  }, [tick, resetBackoff, ...deps]); // eslint-disable-line react-hooks/exhaustive-deps

  return retryNow;
}
