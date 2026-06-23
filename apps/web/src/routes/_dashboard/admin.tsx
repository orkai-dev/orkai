import { createFileRoute, Outlet, redirect } from "@tanstack/react-router";
import { userKeys } from "@/features/auth";
import type { UserInfo } from "@/features/auth/types";
import { api } from "@/lib/api";

export const Route = createFileRoute("/_dashboard/admin")({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData({
      queryKey: userKeys.me,
      queryFn: () => api.get<UserInfo>("/api/v1/auth/me"),
    });
    if (user.role?.toLowerCase() !== "admin") {
      throw redirect({ to: "/dashboard" });
    }
  },
  component: () => <Outlet />,
});
