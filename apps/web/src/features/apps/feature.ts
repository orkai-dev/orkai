import { Layers } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const appsFeature: FeatureModule = {
  id: "apps",
  nav: [
    {
      href: "/apps",
      label: "Apps",
      icon: Layers,
      section: "main",
      isActive: (p) => p === "/apps" || (p.startsWith("/projects") && p.includes("/apps/")),
    },
  ],
  invalidations: {
    applications: [["apps"], ["projects"]],
    domains: [["apps"]],
  },
};
