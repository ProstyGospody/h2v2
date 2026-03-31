import { useCallback, useEffect, useRef } from "react";

export function usePolling(
  callback: (signal: AbortSignal) => Promise<void>,
  intervalMs: number,
  deps: unknown[] = [],
) {
  const backoffRef = useRef(intervalMs);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const resetBackoff = useCallback(() => {
    backoffRef.current = intervalMs;
  }, [intervalMs]);

  const schedule = useCallback(() => {
    timerRef.current = setTimeout(() => {
      void tick();
    }, backoffRef.current);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

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
      backoffRef.current = Math.min(backoffRef.current * 2, 60_000);
    }
    if (!ac.signal.aborted) {
      schedule();
    }
  }, [callback, intervalMs, schedule]);

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
