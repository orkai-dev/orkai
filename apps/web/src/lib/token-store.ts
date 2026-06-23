/**
 * Single source of truth for auth token persistence.
 *
 * Nothing else in the app should read or write the `orkai_token` /
 * `orkai_refresh` localStorage keys directly — go through this module so the
 * storage keys live in exactly one place (F-11). The {@link ApiClient}, SSE
 * hook, auth layer and routes all consume `tokenStore`.
 */

const ACCESS_KEY = "orkai_token";
const REFRESH_KEY = "orkai_refresh";

function read(key: string): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(key);
}

export const tokenStore = {
  getAccess(): string | null {
    return read(ACCESS_KEY);
  },

  getRefresh(): string | null {
    return read(REFRESH_KEY);
  },

  set(accessToken: string, refreshToken: string): void {
    if (typeof window === "undefined") return;
    localStorage.setItem(ACCESS_KEY, accessToken);
    localStorage.setItem(REFRESH_KEY, refreshToken);
  },

  clear(): void {
    if (typeof window === "undefined") return;
    localStorage.removeItem(ACCESS_KEY);
    localStorage.removeItem(REFRESH_KEY);
  },
};
