import { Users } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const teamsFeature: FeatureModule = {
  id: "teams",
  nav: [
    {
      href: "/admin/teams",
      label: "Teams",
      icon: Users,
      section: "system",
      isActive: (p) => p.startsWith("/admin/teams"),
    },
  ],
};
