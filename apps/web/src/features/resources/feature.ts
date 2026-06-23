import { KeyRound } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const resourcesFeature: FeatureModule = {
  id: "resources",
  nav: [
    {
      href: "/admin/resources",
      label: "Resources",
      icon: KeyRound,
      section: "infrastructure",
      isActive: (p) => p.startsWith("/admin/resources"),
    },
  ],
  invalidations: {
    shared_resources: [["resources"]],
  },
};
