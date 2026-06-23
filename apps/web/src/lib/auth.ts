import { redirect } from "@tanstack/react-router";
import { api } from "./api";
import { endpoints } from "./endpoints";
import { tokenStore } from "./token-store";

// Cache /me verification once per session so dashboard navigation doesn't re-hit the API.
let verified: Promise<void> | null = null;

export function resetVerification() {
  verified = null;
}

export function verifyToken(): Promise<void> {
  if (!verified) {
    verified = api.get(endpoints.auth.me()).catch(() => {
      verified = null;
      clearTokens();
      throw redirect({ to: "/auth/login" });
    });
  }
  return verified;
}

export function formatApiError(err: unknown, fallback: string): string {
  if (!err || typeof err !== "object") return fallback;
  const e = err as { detail?: unknown; message?: unknown };
  if (typeof e.detail === "string" && e.detail) return e.detail;
  if (typeof e.message === "string" && e.message) return e.message;
  return fallback;
}

export function getToken(): string | null {
  return tokenStore.getAccess();
}

export function setTokens(accessToken: string, refreshToken: string) {
  tokenStore.set(accessToken, refreshToken);
  api.setToken(accessToken);
}

export function clearTokens() {
  tokenStore.clear();
  api.setToken(null);
}

// Initialize auth — call this once per page. Synchronous, safe to call before API.
export function initAuth() {
  const token = getToken();
  if (token) {
    api.setToken(token);
    return true;
  }
  return false;
}

// Register the 401 redirect handler (called from layout)
export function setupAuthRedirect(redirectFn: () => void) {
  api.setUnauthorizedHandler(redirectFn);
}

export type LoginResult = {
  user?: any;
  access_token?: string;
  refresh_token?: string;
  requires_2fa?: boolean;
};

export async function login(email: string, password: string, twoFACode?: string) {
  const body: Record<string, string> = { email, password };
  if (twoFACode) {
    body.two_fa_code = twoFACode;
  }
  const res = await api.post<LoginResult>(endpoints.auth.login(), body);
  if (res.access_token && res.refresh_token) {
    setTokens(res.access_token, res.refresh_token);
  }
  return res;
}

export async function completeOAuth2FA(challenge: string, code: string) {
  const res = await api.post<LoginResult>(endpoints.auth.oauth2fa(), {
    challenge,
    code,
  });
  if (res.access_token && res.refresh_token) {
    setTokens(res.access_token, res.refresh_token);
  }
  return res;
}

export async function register(
  email: string,
  password: string,
  displayName: string,
  orgName: string,
) {
  const res = await api.post<{
    user: any;
    access_token: string;
    refresh_token: string;
  }>(endpoints.auth.register(), {
    email,
    password,
    display_name: displayName,
    org_name: orgName,
  });
  setTokens(res.access_token, res.refresh_token);
  return res;
}
