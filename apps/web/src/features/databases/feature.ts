import { Database } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const databasesFeature: FeatureModule = {
  id: "databases",
  nav: [
    {
      href: "/databases",
      label: "Databases",
      icon: Database,
      section: "main",
      isActive: (p) =>
        p === "/databases" || (p.startsWith("/projects") && p.includes("/databases/")),
    },
  ],
  invalidations: {
    managed_databases: [["databases"], ["projects"]],
  },
};
