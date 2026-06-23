import { vi } from "vitest";

/**
 * Reusable mock for the api singleton.
 *
 * Usage in a test file:
 * ```ts
 * vi.mock("@/lib/api", async () => {
 *   const { apiMock, UnauthorizedError } = await import("@/test/mocks/api");
 *   return { api: apiMock, UnauthorizedError };
 * });
 * ```
 */
export { UnauthorizedError } from "@/lib/api";

export const apiMock = {
  setToken: vi.fn(),
  setUnauthorizedHandler: vi.fn(),
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  patch: vi.fn(),
  delete: vi.fn(),
};

export function resetApiMock(): void {
  for (const fn of Object.values(apiMock)) {
    fn.mockReset();
  }
}
