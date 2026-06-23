import type { QueryClient } from "@tanstack/react-query";
import { createRootRouteWithContext, Outlet } from "@tanstack/react-router";
import { ThemedToaster } from "@/components/themed-toaster";
import { Sentry } from "@/lib/sentry";

interface RouterContext {
  queryClient: QueryClient;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <Sentry.ErrorBoundary
      fallback={({ error, resetError }) => (
        <div className="flex min-h-screen items-center justify-center bg-background p-8">
          <div className="max-w-md text-center">
            <h1 className="text-2xl font-bold">Something went wrong</h1>
            <p className="mt-2 text-sm text-muted-foreground">
              {error?.toString() || "An unexpected error occurred."}
            </p>
            <button
              type="button"
              onClick={resetError}
              className="mt-4 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground"
            >
              Try again
            </button>
          </div>
        </div>
      )}
    >
      <Outlet />
      <ThemedToaster />
    </Sentry.ErrorBoundary>
  ),
});
