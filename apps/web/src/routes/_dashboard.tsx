import { useQueryClient } from "@tanstack/react-query";
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";
import { AnimatedOutlet } from "@/components/animated-outlet";
import { AppSearch } from "@/components/app-search";
import { Sidebar, SidebarContext, useSidebarState } from "@/components/layout/sidebar";
import { useEventSource } from "@/hooks/use-event-source";
import { getToken, resetVerification, setupAuthRedirect, verifyToken } from "@/lib/auth";

export const Route = createFileRoute("/_dashboard")({
  beforeLoad: async () => {
    if (!getToken()) {
      throw redirect({ to: "/auth/login" });
    }
    await verifyToken();
  },
  component: DashboardLayout,
});

function DashboardLayout() {
  const navigate = useNavigate();
  const sidebar = useSidebarState();
  const qc = useQueryClient();

  useEffect(() => {
    setupAuthRedirect(() => {
      resetVerification();
      qc.removeQueries({ queryKey: ["auth", "setup-status"] });
      navigate({ to: "/auth/login" });
    });
  }, [navigate, qc]);

  useEventSource();

  return (
    <SidebarContext.Provider value={sidebar}>
      <div className="flex h-screen bg-background">
        <Sidebar />
        <main className="flex-1 overflow-auto bg-background">
          <div className="mx-auto max-w-6xl px-4 py-4">
            <AnimatedOutlet />
          </div>
        </main>
      </div>
      <AppSearch />
    </SidebarContext.Provider>
  );
}
