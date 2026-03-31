import { QueryClient } from "@tanstack/react-query";

import { APIError } from "@/services/api";

export function shouldRetryQuery(failureCount: number, error: unknown): boolean {
  if (error instanceof DOMException && error.name === "AbortError") {
    return false;
  }
  if (error instanceof APIError) {
    return error.retriable && failureCount < 3;
  }
  return failureCount < 2;
}

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: shouldRetryQuery,
        staleTime: 10_000,
        gcTime: 5 * 60_000,
        refetchOnWindowFocus: false,
      },
    },
  });
}
