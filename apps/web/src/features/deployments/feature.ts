import { Rocket } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const deploymentsFeature: FeatureModule = {
  id: "deployments",
  nav: [
    {
      href: "/deployments",
      label: "Deployments",
      icon: Rocket,
      section: "main",
      isActive: (p) => p === "/deployments",
    },
  ],
  invalidations: {
    deployments: [["apps"], ["deployments"]],
  },
};
