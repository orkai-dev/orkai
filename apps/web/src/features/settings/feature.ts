import { Settings } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const settingsFeature: FeatureModule = {
  id: "settings",
  nav: [
    {
      href: "/admin/settings",
      label: "Settings",
      icon: Settings,
      section: "system",
      isActive: (p) => p.startsWith("/admin/settings"),
    },
  ],
};
