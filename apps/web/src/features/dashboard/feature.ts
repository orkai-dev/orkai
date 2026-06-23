import { LayoutDashboard } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const dashboardFeature: FeatureModule = {
  id: "dashboard",
  nav: [
    {
      href: "/dashboard",
      label: "Dashboard",
      icon: LayoutDashboard,
      section: "main",
      isActive: (p) => p === "/dashboard",
    },
  ],
};
