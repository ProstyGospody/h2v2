export class APIError extends Error {
  status: number;
  details: unknown;

  constructor(message: string, status: number, details: unknown = null) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.details = details;
  }
}

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

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {});
  const csrfHeaderName = import.meta.env.VITE_CSRF_HEADER_NAME || "X-CSRF-Token";
  const csrfToken = getCSRFToken();

  if (csrfToken && !headers.has(csrfHeaderName)) {
    headers.set(csrfHeaderName, csrfToken);
  }

  if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(path, {
    ...init,
    headers,
    credentials: "include",
    cache: "no-store",
  });

  const raw = await response.text();
  let payload: unknown = null;

  if (raw) {
    try {
      payload = JSON.parse(raw);
    } catch {
      payload = raw;
    }
  }

  if (!response.ok) {
    const message =
      typeof payload === "object" && payload !== null && "error" in payload
        ? String((payload as Record<string, unknown>).error || "Request failed")
        : `${response.status} ${response.statusText}`;
    const details = typeof payload === "object" && payload !== null && "details" in payload ? (payload as Record<string, unknown>).details : null;
    throw new APIError(message, response.status, details);
  }

  return payload as T;
}
