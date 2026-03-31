export class APIError extends Error {
  status: number;
  details: unknown;
  code: string;
  retriable: boolean;

  constructor(
    message: string,
    status: number,
    details: unknown = null,
    code = "api_error",
    retriable = false,
  ) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.details = details;
    this.code = code;
    this.retriable = retriable;
  }
}

export type APIFetchInit = RequestInit & {
  timeoutMs?: number;
};

const DEFAULT_TIMEOUT_MS = 15_000;

function readCookie(name: string): string {
  if (typeof document === "undefined") {
    return "";
  }
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = document.cookie.match(new RegExp(`(?:^|; )${escaped}=([^;]*)`));
  if (!match) {
    return "";
  }
  return decodeURIComponent(match[1]);
}

export function getCSRFToken(): string {
  const cookieName = import.meta.env.VITE_CSRF_COOKIE_NAME || "pp_csrf";
  return readCookie(cookieName);
}

function messageFromStatus(status: number): string {
  switch (status) {
    case 400:
      return "Bad request";
    case 401:
      return "Authentication required";
    case 403:
      return "Access denied";
    case 404:
      return "Not found";
    case 408:
      return "Request timeout";
    case 409:
      return "Conflict";
    case 422:
      return "Validation failed";
    case 429:
      return "Too many requests";
    case 500:
      return "Server error";
    case 502:
    case 503:
    case 504:
      return "Service unavailable";
    default:
      return "Request failed";
  }
}

async function parseResponsePayload(response: Response): Promise<unknown> {
  const raw = await response.text();
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}

function extractPayloadErrorMessage(payload: unknown): string | null {
  if (typeof payload === "object" && payload !== null && "error" in payload) {
    const value = (payload as Record<string, unknown>).error;
    if (typeof value === "string" && value.trim().length > 0) {
      return value.trim();
    }
  }
  if (typeof payload === "string" && payload.trim().length > 0) {
    return payload.trim();
  }
  return null;
}

function extractPayloadDetails(payload: unknown): unknown {
  if (typeof payload === "object" && payload !== null && "details" in payload) {
    return (payload as Record<string, unknown>).details ?? null;
  }
  return null;
}

export async function toAPIError(response: Response): Promise<APIError> {
  const payload = await parseResponsePayload(response);
  const payloadMessage = extractPayloadErrorMessage(payload);
  const message = payloadMessage || messageFromStatus(response.status);
  const details = extractPayloadDetails(payload);
  const retriable = response.status === 408 || response.status === 429 || response.status >= 500;
  return new APIError(message, response.status, details, "http_error", retriable);
}

function createAbortError(): DOMException {
  return new DOMException("Aborted", "AbortError");
}

function mapUnknownFetchError(err: unknown): APIError {
  if (err instanceof APIError) return err;
  if (err instanceof TypeError) {
    return new APIError("Network error", 0, null, "network_error", true);
  }
  if (err instanceof Error) {
    return new APIError(err.message || "Request failed", 0, null, "unknown_error", false);
  }
  return new APIError("Request failed", 0, null, "unknown_error", false);
}

function createTimeoutError(timeoutMs: number): APIError {
  return new APIError(`Request timed out after ${Math.ceil(timeoutMs / 1000)}s`, 408, null, "timeout", true);
}

export function getAPIErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof APIError) return err.message;
  if (err instanceof Error && err.message) return err.message;
  return fallback;
}

export async function apiFetch<T>(path: string, init: APIFetchInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {});
  const csrfHeaderName = import.meta.env.VITE_CSRF_HEADER_NAME || "X-CSRF-Token";
  const csrfToken = getCSRFToken();
  const timeoutMs = init.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  const externalSignal = init.signal;

  if (csrfToken && !headers.has(csrfHeaderName)) {
    headers.set(csrfHeaderName, csrfToken);
  }

  if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const controller = new AbortController();
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, timeoutMs);

  const onExternalAbort = () => {
    controller.abort();
  };
  if (externalSignal) {
    if (externalSignal.aborted) {
      clearTimeout(timer);
      throw createAbortError();
    }
    externalSignal.addEventListener("abort", onExternalAbort, { once: true });
  }

  try {
    const response = await fetch(path, {
      ...init,
      signal: controller.signal,
      headers,
      credentials: "include",
      cache: "no-store",
    });

    if (!response.ok) {
      throw await toAPIError(response);
    }

    const payload = await parseResponsePayload(response);
    return payload as T;
  } catch (err) {
    if (timedOut) {
      throw createTimeoutError(timeoutMs);
    }
    if (externalSignal?.aborted) {
      throw createAbortError();
    }
    if (err instanceof DOMException && err.name === "AbortError") {
      throw createAbortError();
    }
    throw mapUnknownFetchError(err);
  } finally {
    clearTimeout(timer);
    if (externalSignal) {
      externalSignal.removeEventListener("abort", onExternalAbort);
    }
  }
}
