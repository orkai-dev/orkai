import { endpoints } from "./endpoints";
import { tokenStore } from "./token-store";

/** Base origin for API requests. Empty string = same origin (the Vite proxy / prod). */
const API_ORIGIN = "";

export type RequestConfig = {
  headers?: Record<string, string>;
  /** Abort the request when this signal fires. */
  signal?: AbortSignal;
  /** Abort the request after this many milliseconds. */
  timeoutMs?: number;
};

type RequestOptions = RequestConfig & {
  method?: string;
  body?: unknown;
};

/** A single field-level validation error (RFC 7807 `errors[]`). */
export type FieldError = { field: string; message: string };

/**
 * Typed error thrown by {@link ApiClient} for any non-2xx response (F-06).
 *
 * Mirrors the backend's RFC 7807 Problem Details shape so callers get a stable,
 * inspectable error instead of the raw parsed JSON. `message` is set to the
 * best human-readable string so `formatApiError`/Sentry render sensibly.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly detail: string;
  readonly title?: string;
  readonly type?: string;
  readonly fieldErrors?: FieldError[];
  /** The raw parsed response body, for callers that need provider-specific fields. */
  readonly body: unknown;

  constructor(status: number, body: unknown) {
    const obj = body && typeof body === "object" ? (body as Record<string, unknown>) : {};
    const detail =
      typeof obj.detail === "string" && obj.detail
        ? obj.detail
        : typeof obj.title === "string" && obj.title
          ? obj.title
          : "Request failed";
    super(detail);
    this.name = "ApiError";
    this.status = status;
    this.detail = detail;
    this.title = typeof obj.title === "string" ? obj.title : undefined;
    this.type = typeof obj.type === "string" ? obj.type : undefined;
    this.fieldErrors = Array.isArray(obj.errors) ? (obj.errors as FieldError[]) : undefined;
    this.body = body;
  }
}

// Custom error for 401 — components should catch and redirect via router
export class UnauthorizedError extends Error {
  constructor() {
    super("Unauthorized");
    this.name = "UnauthorizedError";
  }
}

/**
 * Auth endpoints that must NOT trigger the silent-refresh-on-401 flow: they are
 * either pre-authentication (no access token exists yet) or the refresh call
 * itself. Every other `/auth/*` route (e.g. `/auth/me`, `/auth/profile`) is
 * protected and should attempt a refresh like any other request — matching the
 * broad `/auth/` substring here would log users out whenever their short-lived
 * access token expired even when a valid refresh token was present.
 */
const NO_REFRESH_PATHS = new Set<string>([
  endpoints.auth.login(),
  endpoints.auth.register(),
  endpoints.auth.refresh(),
  endpoints.auth.oauth2fa(),
  endpoints.auth.setupStatus(),
  endpoints.auth.providers(),
]);

function isNoRefreshPath(path: string): boolean {
  return NO_REFRESH_PATHS.has(path.split("?")[0]);
}

export class ApiClient {
  private baseUrl: string;
  private token: string | null = null;
  private onUnauthorized: (() => void) | null = null;
  // Single-flight guard so concurrent 401s trigger exactly one refresh (F-12).
  private refreshInFlight: Promise<boolean> | null = null;
  // Guard so the redirect callback fires once even when multiple in-flight
  // requests fail the same refresh; reset whenever a valid token is set again.
  private unauthorizedHandled = false;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  setToken(token: string | null) {
    this.token = token;
    if (token) this.unauthorizedHandled = false;
  }

  // Called from the auth provider to register a redirect callback
  setUnauthorizedHandler(handler: () => void) {
    this.onUnauthorized = handler;
  }

  private async fetchWithTimeout(
    url: string,
    init: RequestInit,
    cfg: Pick<RequestConfig, "signal" | "timeoutMs">,
  ): Promise<Response> {
    const { signal, timeoutMs } = cfg;
    if (!signal && !timeoutMs) {
      return fetch(url, init);
    }

    const controller = new AbortController();
    const onAbort = () => controller.abort();
    if (signal) {
      if (signal.aborted) controller.abort();
      else signal.addEventListener("abort", onAbort);
    }
    const timer = timeoutMs ? setTimeout(() => controller.abort(), timeoutMs) : undefined;

    try {
      return await fetch(url, { ...init, signal: controller.signal });
    } finally {
      if (timer) clearTimeout(timer);
      if (signal) signal.removeEventListener("abort", onAbort);
    }
  }

  // Single-flight token refresh. Returns true if a fresh access token was obtained.
  private refresh(): Promise<boolean> {
    if (!this.refreshInFlight) {
      this.refreshInFlight = this.doRefresh().finally(() => {
        this.refreshInFlight = null;
      });
    }
    return this.refreshInFlight;
  }

  private async doRefresh(): Promise<boolean> {
    const refreshToken = tokenStore.getRefresh();
    if (!refreshToken) return false;
    try {
      const res = await fetch(`${this.baseUrl}${endpoints.auth.refresh()}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
      });
      if (!res.ok) return false;
      const data = (await res.json()) as { access_token?: string; refresh_token?: string };
      if (!data.access_token || !data.refresh_token) return false;
      tokenStore.set(data.access_token, data.refresh_token);
      this.setToken(data.access_token);
      return true;
    } catch {
      return false;
    }
  }

  private handleUnauthorized() {
    tokenStore.clear();
    this.token = null;
    // Fire the redirect callback at most once per logged-out session so two
    // concurrent requests both failing a refresh don't double-navigate.
    if (this.unauthorizedHandled) return;
    this.unauthorizedHandled = true;
    if (this.onUnauthorized) {
      this.onUnauthorized();
    } else if (typeof window !== "undefined") {
      // Fallback: direct redirect if callback not registered yet
      window.location.href = "/auth/login";
    }
  }

  private async request<T>(
    path: string,
    options: RequestOptions = {},
    isRetry = false,
  ): Promise<T> {
    const { method = "GET", body, headers = {}, signal, timeoutMs } = options;

    const requestHeaders: Record<string, string> = {
      "Content-Type": "application/json",
      ...headers,
    };

    if (this.token) {
      requestHeaders.Authorization = `Bearer ${this.token}`;
    }

    const res = await this.fetchWithTimeout(
      `${this.baseUrl}${path}`,
      {
        method,
        headers: requestHeaders,
        body: body ? JSON.stringify(body) : undefined,
      },
      { signal, timeoutMs },
    );

    if (!res.ok) {
      if (res.status === 401 && typeof window !== "undefined" && !isNoRefreshPath(path)) {
        // Attempt one silent refresh + replay before giving up (F-12).
        if (!isRetry && (await this.refresh())) {
          return this.request<T>(path, options, true);
        }
        this.handleUnauthorized();
        throw new UnauthorizedError();
      }
      const errorBody = await res.json().catch(() => ({ detail: "Request failed" }));
      throw new ApiError(res.status, errorBody);
    }

    if (res.status === 204) {
      return undefined as T;
    }

    return res.json();
  }

  get<T>(path: string, config?: RequestConfig) {
    return this.request<T>(path, config);
  }

  post<T>(path: string, body?: unknown, config?: RequestConfig) {
    return this.request<T>(path, { ...config, method: "POST", body });
  }

  put<T>(path: string, body?: unknown, config?: RequestConfig) {
    return this.request<T>(path, { ...config, method: "PUT", body });
  }

  patch<T>(path: string, body?: unknown, config?: RequestConfig) {
    return this.request<T>(path, { ...config, method: "PATCH", body });
  }

  delete<T>(path: string, config?: RequestConfig) {
    return this.request<T>(path, { ...config, method: "DELETE" });
  }
}

export const api = new ApiClient(API_ORIGIN);
