import { MutationCache, QueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ApiError, UnauthorizedError } from "./api";
import { formatApiError } from "./auth";
import { Sentry } from "./sentry";

/**
 * Per-mutation metadata read by the centralized error handler (F-06).
 *
 * Hooks set `meta.errorMessage` as the fallback toast text and otherwise only
 * declare `onSuccess` — error toasts are owned by the global handler below, so
 * the `onError: (err: any) => toast.error(...)` boilerplate disappears.
 */
declare module "@tanstack/react-query" {
  interface Register {
    mutationMeta: {
      /** Fallback message shown when the API error has no `detail`. */
      errorMessage?: string;
      /** Opt out of the global error toast (the mutation handles errors itself). */
      skipErrorToast?: boolean;
    };
  }
}

/**
 * Centralized mutation error handling. Every failed mutation flows through here:
 *   - 401s are already redirected by the {@link ApiClient}; stay silent.
 *   - everything else shows one toast using the API `detail` or the hook's
 *     `meta.errorMessage` fallback.
 *   - unexpected errors (non-API, or 5xx) are reported to Sentry.
 */
function handleMutationError(
  error: unknown,
  meta?: { errorMessage?: string; skipErrorToast?: boolean },
) {
  if (error instanceof UnauthorizedError) return;
  if (meta?.skipErrorToast) return;

  const fallback = meta?.errorMessage ?? "Something went wrong";
  toast.error(formatApiError(error, fallback));

  const isExpectedClientError = error instanceof ApiError && error.status < 500;
  if (!isExpectedClientError) {
    Sentry.captureException(error);
  }
}

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      // Live data is pushed via the SSE stream (see use-event-source.ts), so
      // refetching on every window focus is redundant and causes a refetch
      // "storm" (the whole page reloading) each time the user switches browser
      // tabs and comes back. Disable it and rely on SSE invalidation instead.
      queries: { staleTime: 30_000, retry: 1, refetchOnWindowFocus: false },
    },
    mutationCache: new MutationCache({
      onError: (error, _vars, _ctx, mutation) => {
        handleMutationError(error, mutation.options.meta);
      },
    }),
  });
}
