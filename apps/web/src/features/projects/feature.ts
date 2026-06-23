import { FolderKanban } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const projectsFeature: FeatureModule = {
  id: "projects",
  nav: [
    {
      href: "/projects",
      label: "Projects",
      icon: FolderKanban,
      section: "main",
      isActive: (p) =>
        p.startsWith("/projects") &&
        !p.includes("/apps/") &&
        !p.includes("/pages/") &&
        !p.includes("/workers/") &&
        !p.includes("/databases/"),
    },
  ],
  invalidations: {
    projects: [["projects"]],
  },
};
