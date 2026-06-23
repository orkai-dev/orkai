import { QueryClientProvider } from "@tanstack/react-query";
import { createRouter, RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { initAuth } from "./lib/auth";
import { createQueryClient } from "./lib/query-client";
import { initSentry } from "./lib/sentry";
import { initTheme } from "./lib/theme";
import { routeTree } from "./routeTree.gen";
import "./fonts";
import "./index.css";

initSentry();
initAuth();
initTheme();

const queryClient = createQueryClient();

const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
