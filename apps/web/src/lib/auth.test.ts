import { redirect } from "@tanstack/react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";
import {
  clearTokens,
  formatApiError,
  getToken,
  initAuth,
  resetVerification,
  setTokens,
  verifyToken,
} from "./auth";

describe("formatApiError", () => {
  const fallback = "Something went wrong";

  it("returns detail when present", () => {
    expect(formatApiError({ detail: "Invalid credentials" }, fallback)).toBe("Invalid credentials");
  });

  it("prefers detail over message", () => {
    expect(formatApiError({ detail: "Bad request", message: "Error" }, fallback)).toBe(
      "Bad request",
    );
  });

  it("falls back to message when detail is missing", () => {
    expect(formatApiError({ message: "Network error" }, fallback)).toBe("Network error");
  });

  it("returns fallback for empty detail and message", () => {
    expect(formatApiError({ detail: "", message: "" }, fallback)).toBe(fallback);
  });

  it("returns fallback for non-object values", () => {
    expect(formatApiError(null, fallback)).toBe(fallback);
    expect(formatApiError(undefined, fallback)).toBe(fallback);
    expect(formatApiError("error", fallback)).toBe(fallback);
    expect(formatApiError(42, fallback)).toBe(fallback);
  });
});

describe("token helpers", () => {
  beforeEach(() => {
    localStorage.clear();
    resetVerification();
    vi.spyOn(api, "setToken");
  });

  afterEach(() => {
    vi.restoreAllMocks();
    resetVerification();
  });

  it("getToken returns null when unset", () => {
    expect(getToken()).toBeNull();
  });

  it("setTokens stores tokens in localStorage and wires api", () => {
    setTokens("access-123", "refresh-456");

    expect(getToken()).toBe("access-123");
    expect(localStorage.getItem("orkai_refresh")).toBe("refresh-456");
    expect(api.setToken).toHaveBeenCalledWith("access-123");
  });

  it("clearTokens removes tokens from localStorage and clears api token", () => {
    setTokens("access-123", "refresh-456");

    clearTokens();

    expect(getToken()).toBeNull();
    expect(localStorage.getItem("orkai_refresh")).toBeNull();
    expect(api.setToken).toHaveBeenLastCalledWith(null);
  });

  it("initAuth returns false when no token is stored", () => {
    expect(initAuth()).toBe(false);
    expect(api.setToken).not.toHaveBeenCalled();
  });

  it("initAuth hydrates api from localStorage when token exists", () => {
    localStorage.setItem("orkai_token", "stored-token");

    expect(initAuth()).toBe(true);
    expect(api.setToken).toHaveBeenCalledWith("stored-token");
  });
});

describe("verifyToken", () => {
  beforeEach(() => {
    localStorage.clear();
    resetVerification();
    vi.spyOn(api, "get");
    vi.spyOn(api, "setToken");
  });

  afterEach(() => {
    vi.restoreAllMocks();
    resetVerification();
  });

  it("calls /api/v1/auth/me and caches the verification promise", async () => {
    vi.mocked(api.get).mockResolvedValue({ id: "user-1" });

    await verifyToken();
    await verifyToken();

    expect(api.get).toHaveBeenCalledTimes(1);
    expect(api.get).toHaveBeenCalledWith("/api/v1/auth/me");
  });

  it("clears tokens and rejects with redirect on verification failure", async () => {
    localStorage.setItem("orkai_token", "bad-access");
    localStorage.setItem("orkai_refresh", "bad-refresh");
    vi.mocked(api.get).mockRejectedValue(new Error("unauthorized"));

    await expect(verifyToken()).rejects.toEqual(redirect({ to: "/auth/login" }));
    expect(localStorage.getItem("orkai_token")).toBeNull();
    expect(localStorage.getItem("orkai_refresh")).toBeNull();
  });

  it("allows retry after a failed verification", async () => {
    vi.mocked(api.get).mockRejectedValue(new Error("network"));

    await expect(verifyToken()).rejects.toEqual(redirect({ to: "/auth/login" }));
    resetVerification();
    vi.mocked(api.get).mockResolvedValue({ id: "user-1" });

    await expect(verifyToken()).resolves.toEqual({ id: "user-1" });
    expect(api.get).toHaveBeenCalledTimes(2);
  });
});
