import { QueryClient } from "@tanstack/react-query";

/** QueryClient tuned for tests: no retries, no console noise. */
export function createTestQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: Number.POSITIVE_INFINITY,
      },
      mutations: {
        retry: false,
      },
    },
  });
}
