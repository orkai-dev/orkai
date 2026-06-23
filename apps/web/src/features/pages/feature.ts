import { Globe } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const pagesFeature: FeatureModule = {
  id: "pages",
  nav: [
    {
      href: "/pages",
      label: "Pages",
      icon: Globe,
      section: "main",
      isActive: (p) => p === "/pages" || (p.startsWith("/projects") && p.includes("/pages/")),
    },
  ],
  invalidations: {
    pages: [["pages"], ["projects"]],
    page_deployments: [["pages"]],
  },
};
