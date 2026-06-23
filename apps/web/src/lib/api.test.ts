import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError, UnauthorizedError } from "./api";

type MockResponseInit = {
  ok: boolean;
  status: number;
  body?: unknown;
  jsonRejects?: boolean;
};

function createFetchMock(init: MockResponseInit) {
  const json = init.jsonRejects
    ? vi.fn().mockRejectedValue(new Error("invalid json"))
    : vi.fn().mockResolvedValue(init.body ?? {});

  return vi.fn().mockResolvedValue({
    ok: init.ok,
    status: init.status,
    json,
  });
}

describe("ApiClient", () => {
  let client: ApiClient;

  beforeEach(() => {
    client = new ApiClient("");
    localStorage.clear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("injects Bearer token on requests when set", async () => {
    const fetchMock = createFetchMock({ ok: true, status: 200, body: { id: 1 } });
    vi.stubGlobal("fetch", fetchMock);

    client.setToken("test-token");
    await client.get("/api/v1/projects");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/projects",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }),
      }),
    );
  });

  it("omits Authorization when no token is set", async () => {
    const fetchMock = createFetchMock({ ok: true, status: 200, body: {} });
    vi.stubGlobal("fetch", fetchMock);

    await client.get("/api/v1/projects");

    const [, options] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(options.headers).not.toHaveProperty("Authorization");
  });

  it("returns undefined for 204 responses", async () => {
    const fetchMock = createFetchMock({ ok: true, status: 204 });
    vi.stubGlobal("fetch", fetchMock);

    const result = await client.delete("/api/v1/apps/app-1");
    expect(result).toBeUndefined();
  });

  it("throws a typed ApiError carrying status and detail", async () => {
    const fetchMock = createFetchMock({
      ok: false,
      status: 400,
      body: { detail: "Bad request", title: "Validation", type: "about:blank" },
    });
    vi.stubGlobal("fetch", fetchMock);

    const error = await client.get("/api/v1/projects").catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(400);
    expect(error.detail).toBe("Bad request");
    expect(error.title).toBe("Validation");
    expect(error.message).toBe("Bad request");
  });

  it("falls back to generic detail when error body is not JSON", async () => {
    const fetchMock = createFetchMock({ ok: false, status: 500, jsonRejects: true });
    vi.stubGlobal("fetch", fetchMock);

    const error = await client.get("/api/v1/projects").catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(500);
    expect(error.detail).toBe("Request failed");
  });

  it("on 401 clears tokens, calls onUnauthorized, and throws UnauthorizedError", async () => {
    const fetchMock = createFetchMock({ ok: false, status: 401 });
    vi.stubGlobal("fetch", fetchMock);

    localStorage.setItem("orkai_token", "old-access");
    localStorage.setItem("orkai_refresh", "old-refresh");
    client.setToken("old-access");

    const onUnauthorized = vi.fn();
    client.setUnauthorizedHandler(onUnauthorized);

    await expect(client.get("/api/v1/projects")).rejects.toBeInstanceOf(UnauthorizedError);
    expect(localStorage.getItem("orkai_token")).toBeNull();
    expect(localStorage.getItem("orkai_refresh")).toBeNull();
    expect(onUnauthorized).toHaveBeenCalledOnce();
  });

  it("on 401 redirects to login when no handler is registered", async () => {
    const fetchMock = createFetchMock({ ok: false, status: 401 });
    vi.stubGlobal("fetch", fetchMock);

    let href = "";
    vi.stubGlobal("location", {
      get href() {
        return href;
      },
      set href(value: string) {
        href = value;
      },
    });

    await expect(client.get("/api/v1/projects")).rejects.toBeInstanceOf(UnauthorizedError);
    expect(href).toBe("/auth/login");
  });

  it("does not apply 401 session handling on auth paths", async () => {
    const fetchMock = createFetchMock({
      ok: false,
      status: 401,
      body: { detail: "Invalid credentials" },
    });
    vi.stubGlobal("fetch", fetchMock);

    localStorage.setItem("orkai_token", "keep-me");
    client.setToken("keep-me");

    const onUnauthorized = vi.fn();
    client.setUnauthorizedHandler(onUnauthorized);

    const error = await client
      .post("/api/v1/auth/login", { email: "a", password: "b" })
      .catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error.detail).toBe("Invalid credentials");
    expect(localStorage.getItem("orkai_token")).toBe("keep-me");
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it("silently refreshes on 401 and replays the original request", async () => {
    const responses = [
      { ok: false, status: 401, json: async () => ({}) }, // original request
      {
        ok: true,
        status: 200,
        json: async () => ({ access_token: "new-a", refresh_token: "new-r" }),
      }, // refresh
      { ok: true, status: 200, json: async () => ({ id: 1 }) }, // replayed request
    ];
    let i = 0;
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(responses[i++]));
    vi.stubGlobal("fetch", fetchMock);

    localStorage.setItem("orkai_token", "old-a");
    localStorage.setItem("orkai_refresh", "old-r");
    client.setToken("old-a");

    const result = await client.get<{ id: number }>("/api/v1/projects");
    expect(result).toEqual({ id: 1 });

    // refresh was POSTed with the stored refresh token
    expect(fetchMock.mock.calls[1][0]).toContain("/api/v1/auth/refresh");
    expect(fetchMock.mock.calls[1][1].body).toBe(JSON.stringify({ refresh_token: "old-r" }));

    // tokens rotated and replay used the new access token
    expect(localStorage.getItem("orkai_token")).toBe("new-a");
    expect(localStorage.getItem("orkai_refresh")).toBe("new-r");
    expect(fetchMock.mock.calls[2][1].headers.Authorization).toBe("Bearer new-a");
  });

  it("clears session and throws UnauthorizedError when refresh fails", async () => {
    const fetchMock = createFetchMock({ ok: false, status: 401 });
    vi.stubGlobal("fetch", fetchMock);

    localStorage.setItem("orkai_token", "old-a");
    localStorage.setItem("orkai_refresh", "old-r");
    client.setToken("old-a");
    const onUnauthorized = vi.fn();
    client.setUnauthorizedHandler(onUnauthorized);

    await expect(client.get("/api/v1/projects")).rejects.toBeInstanceOf(UnauthorizedError);
    expect(localStorage.getItem("orkai_token")).toBeNull();
    expect(localStorage.getItem("orkai_refresh")).toBeNull();
    expect(onUnauthorized).toHaveBeenCalledOnce();
  });

  it("refreshes only once for concurrent 401s (single-flight)", async () => {
    let refreshCalls = 0;
    const seen = new Map<string, number>();
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.includes("/auth/refresh")) {
        refreshCalls++;
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({ access_token: "na", refresh_token: "nr" }),
        });
      }
      const n = (seen.get(url) ?? 0) + 1;
      seen.set(url, n);
      if (n === 1) return Promise.resolve({ ok: false, status: 401, json: async () => ({}) });
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ url }) });
    });
    vi.stubGlobal("fetch", fetchMock);

    localStorage.setItem("orkai_token", "old-a");
    localStorage.setItem("orkai_refresh", "old-r");
    client.setToken("old-a");

    const [a, b] = await Promise.all([
      client.get<{ url: string }>("/api/v1/a"),
      client.get<{ url: string }>("/api/v1/b"),
    ]);

    expect(refreshCalls).toBe(1);
    expect(a.url).toContain("/api/v1/a");
    expect(b.url).toContain("/api/v1/b");
  });

  it("calls onUnauthorized once when concurrent requests both fail to refresh", async () => {
    // Every request 401s and refresh fails, so both in-flight requests end up
    // in handleUnauthorized — the redirect callback must still fire only once.
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.includes("/auth/refresh")) {
        return Promise.resolve({ ok: false, status: 401, json: async () => ({}) });
      }
      return Promise.resolve({ ok: false, status: 401, json: async () => ({}) });
    });
    vi.stubGlobal("fetch", fetchMock);

    localStorage.setItem("orkai_token", "old-a");
    localStorage.setItem("orkai_refresh", "old-r");
    client.setToken("old-a");
    const onUnauthorized = vi.fn();
    client.setUnauthorizedHandler(onUnauthorized);

    const results = await Promise.allSettled([client.get("/api/v1/a"), client.get("/api/v1/b")]);

    expect(results.every((r) => r.status === "rejected")).toBe(true);
    expect(onUnauthorized).toHaveBeenCalledOnce();
  });

  it("attempts a silent refresh on 401 for protected auth routes like /auth/me", async () => {
    const responses = [
      { ok: false, status: 401, json: async () => ({}) }, // GET /auth/me
      {
        ok: true,
        status: 200,
        json: async () => ({ access_token: "new-a", refresh_token: "new-r" }),
      }, // refresh
      { ok: true, status: 200, json: async () => ({ id: "user-1" }) }, // replayed /auth/me
    ];
    let i = 0;
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(responses[i++]));
    vi.stubGlobal("fetch", fetchMock);

    localStorage.setItem("orkai_token", "old-a");
    localStorage.setItem("orkai_refresh", "old-r");
    client.setToken("old-a");

    const result = await client.get<{ id: string }>("/api/v1/auth/me");
    expect(result).toEqual({ id: "user-1" });
    expect(fetchMock.mock.calls[1][0]).toContain("/api/v1/auth/refresh");
  });

  it("serializes POST body as JSON", async () => {
    const fetchMock = createFetchMock({ ok: true, status: 200, body: { ok: true } });
    vi.stubGlobal("fetch", fetchMock);

    await client.post("/api/v1/projects", { name: "demo" });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/projects",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ name: "demo" }),
      }),
    );
  });
});
